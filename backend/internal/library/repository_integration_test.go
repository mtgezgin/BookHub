package library_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/library"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

func countEntries(t *testing.T, pool *pgxpool.Pool, userID, bookID int64) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM library_entries WHERE user_id = $1 AND book_id = $2`, userID, bookID).Scan(&n)
	if err != nil {
		t.Fatalf("kayıt sayılamadı: %v", err)
	}
	return n
}

func TestLibraryRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := library.NewRepository(pool)
	ctx := context.Background()

	t.Run("ekle, değiştir, tek kayıt kalır", func(t *testing.T) {
		alice, book := testdb.CreateUser(t, pool, "Okur"), testdb.CreateBook(t, pool, "", books.StatusApproved)
		e1, err := repo.Put(ctx, alice, book, library.StatusWantToRead)
		if err != nil || e1.Status != library.StatusWantToRead {
			t.Fatalf("ilk ekleme: %+v %v", e1, err)
		}
		e2, err := repo.Put(ctx, alice, book, library.StatusRead)
		if err != nil || e2.Status != library.StatusRead {
			t.Fatalf("durum değişimi: %+v %v", e2, err)
		}
		if !e2.CreatedAt.Equal(e1.CreatedAt) {
			t.Errorf("eklenme zamanı değişmemeli: %v → %v", e1.CreatedAt, e2.CreatedAt)
		}
		if n := countEntries(t, pool, alice, book); n != 1 {
			t.Errorf("kayıt sayısı = %d, istenen 1", n)
		}
	})

	t.Run("yayında olmayan ya da olmayan kitap → ErrNotFound", func(t *testing.T) {
		alice, pending := testdb.CreateUser(t, pool, "Okur"), testdb.CreateBook(t, pool, "", books.StatusPending)
		if _, err := repo.Put(ctx, alice, pending, library.StatusRead); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("bekleyen kitap: err = %v, istenen ErrNotFound", err)
		}
		if _, err := repo.Put(ctx, alice, 999_999_999, library.StatusRead); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("olmayan kitap: err = %v, istenen ErrNotFound", err)
		}
		if n := countEntries(t, pool, alice, pending); n != 0 {
			t.Errorf("bekleyen kitaba kayıt yazılmamalı, sayı = %d", n)
		}
	})

	t.Run("başkasının kaydı görünmez", func(t *testing.T) {
		alice, bob, book := testdb.CreateUser(t, pool, "Okur"), testdb.CreateUser(t, pool, "Okur"), testdb.CreateBook(t, pool, "", books.StatusApproved)
		if _, err := repo.Put(ctx, alice, book, library.StatusReading); err != nil {
			t.Fatal(err)
		}
		m, err := repo.Statuses(ctx, bob, []int64{book})
		if err != nil || len(m) != 0 {
			t.Errorf("bob alice'in kaydını görmemeli: %v %v", m, err)
		}
		m, err = repo.Statuses(ctx, alice, []int64{book, 999_999_999})
		if err != nil || len(m) != 1 || m[book] != library.StatusReading {
			t.Errorf("alice kendi kaydını görmeli: %v %v", m, err)
		}
	})

	t.Run("başkası yazamaz: bob'un ekleme ve silmesi alice'in kaydına dokunmaz", func(t *testing.T) {
		alice, bob, book := testdb.CreateUser(t, pool, "Okur"), testdb.CreateUser(t, pool, "Okur"), testdb.CreateBook(t, pool, "", books.StatusApproved)
		if _, err := repo.Put(ctx, alice, book, library.StatusRead); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Put(ctx, bob, book, library.StatusAbandoned); err != nil {
			t.Fatal(err)
		}
		if err := repo.Remove(ctx, bob, book); err != nil {
			t.Fatal(err)
		}
		if m, err := repo.Statuses(ctx, alice, []int64{book}); err != nil || m[book] != library.StatusRead {
			t.Errorf("alice'in kaydı READ kalmalı: %v %v", m, err)
		}
		if n := countEntries(t, pool, bob, book); n != 0 {
			t.Errorf("bob'un kendi kaydı silinmeli, sayı = %d", n)
		}
	})

	t.Run("boş kimlik listesi boş sonuç", func(t *testing.T) {
		m, err := repo.Statuses(ctx, testdb.CreateUser(t, pool, "Okur"), nil)
		if err != nil || m == nil || len(m) != 0 {
			t.Errorf("boş girdi: %v %v (nil olmayan boş map istenir)", m, err)
		}
	})

	t.Run("çıkarmak idempotent ve puana dokunmaz", func(t *testing.T) {
		alice, book := testdb.CreateUser(t, pool, "Okur"), testdb.CreateBook(t, pool, "", books.StatusApproved)
		if _, err := repo.Put(ctx, alice, book, library.StatusRead); err != nil {
			t.Fatal(err)
		}
		testdb.Exec(t, pool, `INSERT INTO ratings (user_id, book_id, score) VALUES ($1, $2, 8)`, alice, book)
		for i := range 2 {
			if err := repo.Remove(ctx, alice, book); err != nil {
				t.Fatalf("%d. çıkarma: %v", i+1, err)
			}
		}
		if n := countEntries(t, pool, alice, book); n != 0 {
			t.Errorf("kayıt silinmeli, sayı = %d", n)
		}
		var score int
		if err := pool.QueryRow(ctx, `SELECT score FROM ratings WHERE user_id = $1 AND book_id = $2`, alice, book).Scan(&score); err != nil || score != 8 {
			t.Errorf("puan yerinde kalmalı: %d %v", score, err)
		}
	})

	t.Run("arşivlenen kitap güncellenemez, çıkarılabilir", func(t *testing.T) {
		alice, book := testdb.CreateUser(t, pool, "Okur"), testdb.CreateBook(t, pool, "", books.StatusApproved)
		if _, err := repo.Put(ctx, alice, book, library.StatusRead); err != nil {
			t.Fatal(err)
		}
		testdb.Exec(t, pool, `UPDATE books SET status = 'ARCHIVED' WHERE id = $1`, book)
		if _, err := repo.Put(ctx, alice, book, library.StatusReading); !errors.Is(err, books.ErrNotFound) {
			t.Errorf("arşivlenmiş kitapta durum değişimi: err = %v, istenen ErrNotFound", err)
		}
		if m, err := repo.Statuses(ctx, alice, []int64{book}); err != nil || m[book] != library.StatusRead {
			t.Errorf("kayıt eski durumuyla kalmalı: %v %v", m, err)
		}
		if err := repo.Remove(ctx, alice, book); err != nil {
			t.Errorf("arşivlenmiş kitap çıkarılabilmeli: %v", err)
		}
	})
}

func titles(rows []db.ListLibraryRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.BookView.Title
	}
	return out
}

func ids(rows []db.ListLibraryRow) []int64 {
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.BookView.ID
	}
	return out
}

func mustPut(t *testing.T, repo *library.Repository, userID, bookID int64, status string) {
	t.Helper()
	if _, err := repo.Put(context.Background(), userID, bookID, status); err != nil {
		t.Fatalf("kitaplığa eklenemedi: %v", err)
	}
}

func mustList(t *testing.T, repo *library.Repository, userID int64, f library.ListFilter) ([]db.ListLibraryRow, int64) {
	t.Helper()
	if f.Limit == 0 {
		f.Limit = 20
	}
	rows, total, err := repo.List(context.Background(), userID, f)
	if err != nil {
		t.Fatalf("liste: %v", err)
	}
	return rows, total
}

func TestLibraryListRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := library.NewRepository(pool)

	t.Run("başlık Türkçe alfabeyle sıralanır", func(t *testing.T) {
		alice := testdb.CreateUser(t, pool, "Okur")
		for _, title := range []string{"Şato", "Dune", "İnce", "Cesur", "Sapiens", "Ilık", "Çalıkuşu"} {
			mustPut(t, repo, alice, testdb.CreateBook(t, pool, title, books.StatusApproved), library.StatusRead)
		}
		rows, total := mustList(t, repo, alice, library.ListFilter{Sort: library.SortTitle})
		want := []string{"Cesur", "Çalıkuşu", "Dune", "Ilık", "İnce", "Sapiens", "Şato"}
		if total != 7 || !slices.Equal(titles(rows), want) {
			t.Errorf("başlık sırası = %v (toplam %d), istenen %v", titles(rows), total, want)
		}
	})

	t.Run("varsayılan sıra en yeni eklenen önce", func(t *testing.T) {
		alice := testdb.CreateUser(t, pool, "Okur")
		first, second, third := testdb.CreateBook(t, pool, "", books.StatusApproved), testdb.CreateBook(t, pool, "", books.StatusApproved), testdb.CreateBook(t, pool, "", books.StatusApproved)
		for _, b := range []int64{first, second, third} {
			mustPut(t, repo, alice, b, library.StatusWantToRead)
		}
		rows, _ := mustList(t, repo, alice, library.ListFilter{})
		if want := []int64{third, second, first}; !slices.Equal(ids(rows), want) {
			t.Errorf("sıra = %v, istenen %v", ids(rows), want)
		}
	})

	t.Run("puanıma göre, puansızlar sonda", func(t *testing.T) {
		alice := testdb.CreateUser(t, pool, "Okur")
		seven, nine, unrated := testdb.CreateBook(t, pool, "", books.StatusApproved), testdb.CreateBook(t, pool, "", books.StatusApproved), testdb.CreateBook(t, pool, "", books.StatusApproved)
		for _, b := range []int64{unrated, seven, nine} {
			mustPut(t, repo, alice, b, library.StatusRead)
		}
		testdb.Exec(t, pool, `INSERT INTO ratings (user_id, book_id, score) VALUES ($1, $2, 7), ($1, $3, 9)`, alice, seven, nine)
		rows, _ := mustList(t, repo, alice, library.ListFilter{Sort: library.SortMyRating})
		if want := []int64{nine, seven, unrated}; !slices.Equal(ids(rows), want) {
			t.Errorf("sıra = %v, istenen %v", ids(rows), want)
		}
	})

	t.Run("durum, tür ve puan filtreleri; sayı eşleşir", func(t *testing.T) {
		alice, bob := testdb.CreateUser(t, pool, "Okur"), testdb.CreateUser(t, pool, "Okur")
		read, reading, other := testdb.CreateBook(t, pool, "Vakıf", books.StatusApproved), testdb.CreateBook(t, pool, "Dune Mesihi", books.StatusApproved), testdb.CreateBook(t, pool, "", books.StatusApproved)
		mustPut(t, repo, alice, read, library.StatusRead)
		mustPut(t, repo, alice, reading, library.StatusReading)
		mustPut(t, repo, bob, other, library.StatusRead)
		slug := testdb.Unique("tur")
		var genreID int64
		if err := pool.QueryRow(context.Background(), `INSERT INTO genres (name, slug) VALUES ($1, $1) RETURNING id`, slug).Scan(&genreID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM genres WHERE id = $1", genreID) })
		testdb.Exec(t, pool, `INSERT INTO book_genres (book_id, genre_id) VALUES ($1, $2)`, reading, genreID)
		testdb.Exec(t, pool, `INSERT INTO ratings (user_id, book_id, score) VALUES ($1, $2, 8)`, alice, read)

		cases := []struct {
			name string
			f    library.ListFilter
			want []int64
		}{
			{"hepsi (yalnız kendi kaydı)", library.ListFilter{}, []int64{reading, read}},
			{"durum", library.ListFilter{Status: library.StatusRead}, []int64{read}},
			{"tür", library.ListFilter{Genre: slug}, []int64{reading}},
			{"puanladıklarım", library.ListFilter{Rated: library.RatedYes}, []int64{read}},
			{"puanlamadıklarım", library.ListFilter{Rated: library.RatedNo}, []int64{reading}},
			{"arama", library.ListFilter{Query: "mesih"}, []int64{reading}},
		}
		for _, c := range cases {
			rows, total := mustList(t, repo, alice, c.f)
			if !slices.Equal(ids(rows), c.want) || total != int64(len(c.want)) {
				t.Errorf("%s: %v (toplam %d), istenen %v", c.name, ids(rows), total, c.want)
			}
		}
		rows, _ := mustList(t, repo, alice, library.ListFilter{Status: library.StatusRead})
		if len(rows) != 1 || rows[0].MyRating == nil || *rows[0].MyRating != 8 {
			t.Errorf("puanım alanı 8 olmalı: %+v", rows)
		}
	})

	t.Run("sayfalama toplamı değiştirmez", func(t *testing.T) {
		alice := testdb.CreateUser(t, pool, "Okur")
		for range 3 {
			mustPut(t, repo, alice, testdb.CreateBook(t, pool, "", books.StatusApproved), library.StatusRead)
		}
		rows, total := mustList(t, repo, alice, library.ListFilter{Limit: 2, Offset: 2})
		if len(rows) != 1 || total != 3 {
			t.Errorf("ikinci sayfa: %d öğe, toplam %d; istenen 1 ve 3", len(rows), total)
		}
	})

	t.Run("sonun ötesindeki sayfada satır yok ama toplam doğru", func(t *testing.T) {
		alice := testdb.CreateUser(t, pool, "Okur")
		for range 3 {
			mustPut(t, repo, alice, testdb.CreateBook(t, pool, "", books.StatusApproved), library.StatusRead)
		}
		rows, total := mustList(t, repo, alice, library.ListFilter{Limit: 2, Offset: 20})
		if len(rows) != 0 || total != 3 {
			t.Errorf("sonun ötesi: %d öğe, toplam %d; istenen 0 ve 3", len(rows), total)
		}
		if _, total := mustList(t, repo, testdb.CreateUser(t, pool, "Boş"), library.ListFilter{Offset: 20}); total != 0 {
			t.Errorf("boş kitaplıkta toplam %d, istenen 0", total)
		}
	})

	t.Run("yüzde işareti joker değil", func(t *testing.T) {
		alice := testdb.CreateUser(t, pool, "Okur")
		mustPut(t, repo, alice, testdb.CreateBook(t, pool, "Martin Eden", books.StatusApproved), library.StatusRead)
		for _, q := range []string{"%", "_"} {
			if _, total := mustList(t, repo, alice, library.ListFilter{Query: q}); total != 0 {
				t.Errorf("q=%q: toplam %d, istenen 0", q, total)
			}
		}
	})

	t.Run("arşivlenen kitap in_catalog=false ile listede", func(t *testing.T) {
		alice, book := testdb.CreateUser(t, pool, "Okur"), testdb.CreateBook(t, pool, "", books.StatusApproved)
		mustPut(t, repo, alice, book, library.StatusRead)
		rows, _ := mustList(t, repo, alice, library.ListFilter{})
		if len(rows) != 1 || !rows[0].InCatalog {
			t.Fatalf("yayındaki kitap in_catalog=true olmalı: %+v", rows)
		}
		testdb.Exec(t, pool, `UPDATE books SET status = 'ARCHIVED' WHERE id = $1`, book)
		rows, total := mustList(t, repo, alice, library.ListFilter{})
		if total != 1 || rows[0].InCatalog || rows[0].Status != library.StatusRead {
			t.Errorf("arşivlenen kitap listede kalmalı ve in_catalog=false olmalı: %+v", rows)
		}
	})
}

func TestValidStatus(t *testing.T) {
	for _, s := range []string{"WANT_TO_READ", "READING", "READ", "ABANDONED"} {
		if !library.ValidStatus(s) {
			t.Errorf("%q geçerli olmalı", s)
		}
	}
	for _, s := range []string{"", "read", "DONE", "READ "} {
		if library.ValidStatus(s) {
			t.Errorf("%q geçersiz olmalı", s)
		}
	}
}
