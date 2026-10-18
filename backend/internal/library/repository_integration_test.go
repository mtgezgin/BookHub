package library_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/library"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

func createUser(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, display_name, password_hash) VALUES ($1, 'Okur', 'x') RETURNING id`,
		testdb.Unique("reader")+"@example.com").Scan(&id)
	if err != nil {
		t.Fatalf("kullanıcı açılamadı: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM users WHERE id = $1", id) })
	return id
}

func createBook(t *testing.T, pool *pgxpool.Pool, status string) int64 {
	t.Helper()
	slug := testdb.Unique("shelved")
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO books (title, slug, author, status) VALUES ($1, $1, 'Yazar', $2) RETURNING id`, slug, status).Scan(&id)
	if err != nil {
		t.Fatalf("kitap açılamadı: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM books WHERE id = $1", id) })
	return id
}

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
		alice, book := createUser(t, pool), createBook(t, pool, books.StatusApproved)
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
		alice, pending := createUser(t, pool), createBook(t, pool, books.StatusPending)
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
		alice, bob, book := createUser(t, pool), createUser(t, pool), createBook(t, pool, books.StatusApproved)
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

	t.Run("boş kimlik listesi boş sonuç", func(t *testing.T) {
		m, err := repo.Statuses(ctx, createUser(t, pool), nil)
		if err != nil || m == nil || len(m) != 0 {
			t.Errorf("boş girdi: %v %v (nil olmayan boş map istenir)", m, err)
		}
	})

	t.Run("çıkarmak idempotent ve puana dokunmaz", func(t *testing.T) {
		alice, book := createUser(t, pool), createBook(t, pool, books.StatusApproved)
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
		alice, book := createUser(t, pool), createBook(t, pool, books.StatusApproved)
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
