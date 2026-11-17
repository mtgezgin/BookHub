package books

import (
	"context"
	"slices"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// searchFixture, arama testlerinin ortak kurulumu: kitaplar teste özgü bir türe bağlanır ve her
// arama o türle süzülür. Böylece sorgu metni kullanıcının yazacağı hâliyle kalır ("savaş") ve
// veritabanındaki başka kitaplar sonucu kirletmez.
type searchFixture struct {
	fixture
	genreID   int64
	genreSlug string
}

func newSearchFixture(t *testing.T) searchFixture {
	t.Helper()
	f := newFixture(t)
	s := searchFixture{fixture: f, genreID: f.createGenre(t)}
	if err := f.pool.QueryRow(context.Background(), "SELECT slug FROM genres WHERE id = $1", s.genreID).Scan(&s.genreSlug); err != nil {
		t.Fatal(err)
	}
	return s
}

// book, kitabı varsayılan baskısıyla ekler (uygulamanın yolu: Create) ve teste özgü türe bağlar.
func (s searchFixture) book(t *testing.T, title, author, status string) int64 {
	t.Helper()
	ctx := context.Background()
	b, err := s.repo.Create(ctx, CreateParams{CreateBookParams: db.CreateBookParams{
		Title: title, Slug: testdb.Unique("ara"), Author: author, Language: "tr", Status: status,
	}})
	if err != nil {
		t.Fatalf("kitap eklenemedi (%s): %v", title, err)
	}
	t.Cleanup(func() { testdb.Exec(t, s.pool, "DELETE FROM books WHERE id = $1", b.ID) })
	if err := s.repo.SetGenres(ctx, b.ID, []int64{s.genreID}); err != nil {
		t.Fatal(err)
	}
	return b.ID
}

// search, türle süzülmüş aramanın kitap kimliklerini SIRASIYLA ve toplamı döner.
func (s searchFixture) search(t *testing.T, f ListFilter) ([]int64, int64) {
	t.Helper()
	f.Genre = s.genreSlug
	if f.Limit == 0 {
		f.Limit = 50
	}
	hits, total, err := s.repo.Search(context.Background(), f)
	if err != nil {
		t.Fatalf("arama (%q): %v", f.Query, err)
	}
	ids := make([]int64, 0, len(hits))
	for _, h := range hits {
		if h.Book.Status != StatusApproved {
			t.Errorf("yayında olmayan kitap arama sonucunda: %+v", h.Book)
		}
		ids = append(ids, h.Book.ID)
	}
	return ids, total
}

// TestSearchMatchingRealDB: eşleşme kuralları, gerçek Postgres'le. Canlıda bulunan hatalar burada
// örnek olarak duruyor: "ince memed" (büyük İ), "kurk" (aksan), "atay oğuz" (kelime sırası),
// "dostoyevski suç" (iki alana yayılan sorgu).
func TestSearchMatchingRealDB(t *testing.T) {
	s := newSearchFixture(t)
	ince := s.book(t, "İnce Memed", "Yaşar Kemal", StatusApproved)
	kurk := s.book(t, "Kürk Mantolu Madonna", "Sabahattin Ali", StatusApproved)
	tutunamayanlar := s.book(t, "Tutunamayanlar", "Oğuz Atay", StatusApproved)
	suc := s.book(t, "Suç ve Ceza", "Fyodor Dostoyevski", StatusApproved)
	isik := s.book(t, "IŞIK Bahçeleri", "Işıl Irmak", StatusApproved)
	s.book(t, "İnce Memed Taslağı", "Yaşar Kemal", StatusPending) // yayında değil: hiçbir aramada dönmemeli
	testdb.Exec(t, s.pool, `INSERT INTO editions (book_id, title, language, translator, publisher, is_primary)
		VALUES ($1, 'Crime and Punishment', 'en', 'Constance Garnett', 'Penguin Classics', false)`, suc)

	for _, tc := range []struct {
		name, query string
		want        []int64
	}{
		{"büyük İ küçük i ile bulunur", "ince memed", []int64{ince}},
		{"hepsi büyük harf", "İNCE MEMED", []int64{ince}},
		{"noktasız I ile yazılmış sorgu", "INCE MEMED", []int64{ince}},
		{"aksansız yazım", "kurk mantolu", []int64{kurk}},
		{"aksanlı ve büyük harf", "KÜRK", []int64{kurk}},
		{"kelime sırası önemsiz (yazar)", "atay oğuz", []int64{tutunamayanlar}},
		{"aksansız yazar", "oguz atay", []int64{tutunamayanlar}},
		{"sorgu başlık ve yazara yayılır", "dostoyevski suç", []int64{suc}},
		{"başlık + yazar, başka kitap", "sabahattin kürk", []int64{kurk}},
		{"son kelime önek", "tutunam", []int64{tutunamayanlar}},
		{"önek yalnız son kelimede", "tutunam atay", nil},
		{"noktalı ve noktasız i ayrımı yok", "isik", []int64{isik}},
		{"ışık → IŞIK", "ışık bahçeleri", []int64{isik}},
		{"başka baskının adı", "crime punishment", []int64{suc}},
		{"çevirmen", "constance garnett", []int64{suc}},
		{"yayınevi aranmaz", "penguin", nil},
		{"eşleşmeyen kelime sonucu boşaltır (VE)", "ince ahmet", nil},
		{"noktalama yok sayılır", "ince-memed!", []int64{ince}},
		{"yalnız noktalama: sonuç yok, hata yok", "%", nil},
		{"alt çizgi joker değil", "_", nil},
		{"işleç karakterleri VEYA'ya dönüşmez", "ince | tutunamayanlar", nil},
		{"emoji: sonuç yok, hata yok", "📚", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, total := s.search(t, ListFilter{Query: tc.query})
			if !slices.Equal(got, tc.want) && (len(got) != 0 || len(tc.want) != 0) {
				t.Errorf("%q: gelen %v, istenen %v", tc.query, got, tc.want)
			}
			if total != int64(len(tc.want)) {
				t.Errorf("%q: toplam %d, istenen %d", tc.query, total, len(tc.want))
			}
		})
	}

	t.Run("türler aynı sorgudan gelir", func(t *testing.T) {
		hits, _, err := s.repo.Search(context.Background(), ListFilter{Query: "ince memed", Genre: s.genreSlug, Limit: 5})
		if err != nil || len(hits) != 1 {
			t.Fatalf("tek sonuç bekleniyordu: %d sonuç, hata %v", len(hits), err)
		}
		if len(hits[0].Genres) != 1 || hits[0].Genres[0].ID != s.genreID || hits[0].Genres[0].Slug != s.genreSlug || hits[0].Genres[0].Name == "" {
			t.Errorf("kitabın türü sonuçla birlikte dönmeli: %+v", hits[0].Genres)
		}
	})

	t.Run("tür süzgeci olmadan türsüz kitap da bulunur, türleri boş dizidir", func(t *testing.T) {
		title := "Türsüz " + testdb.Unique("kitap")
		id := testdb.CreateBook(t, s.pool, title, StatusApproved)
		hits, total, err := s.repo.Search(context.Background(), ListFilter{Query: title, Limit: 5})
		if err != nil || total != 1 || len(hits) != 1 || hits[0].Book.ID != id {
			t.Fatalf("türsüz kitap bulunmalıydı: toplam %d, %d sonuç, hata %v", total, len(hits), err)
		}
		if hits[0].Genres == nil || len(hits[0].Genres) != 0 {
			t.Errorf("türsüz kitapta türler boş dizi olmalı (nil değil): %#v", hits[0].Genres)
		}
	})
}

// TestSearchOrderRealDB: üç sıra. İlgi sırasında kademeler (başlık aynı > başlıkla başlıyor > başlıkta >
// yazarda > baskıda) oy sayısından ÖNCE gelir; oy sayıları bunu kanıtlamak için ters dizildi.
func TestSearchOrderRealDB(t *testing.T) {
	s := newSearchFixture(t)
	// Eklenme sırası a → e: "en yeni" sırası bunun tersi.
	a := s.book(t, "Savaş", "Birinci Yazar", StatusApproved)            // başlık sorguyla aynı
	b := s.book(t, "Savaş ve Barış", "İkinci Yazar", StatusApproved)    // başlık sorguyla başlıyor
	c := s.book(t, "Büyük Savaş", "Üçüncü Yazar", StatusApproved)       // kelime başlıkta
	d := s.book(t, "Başka Bir Kitap", "Savaş Dördüncü", StatusApproved) // kelime yazarda
	e := s.book(t, "Bambaşka", "Beşinci Yazar", StatusApproved)         // kelime yalnız bir baskının adında
	testdb.Exec(t, s.pool, `INSERT INTO editions (book_id, title, language, is_primary) VALUES ($1, 'Savaş Günlüğü', 'tr', false)`, e)

	// Oylar: ilgi sırasının tersine artar (e en çok oylu); ortalamalar puan sırasını ikisinden de
	// farklı kılar. Oy sayıları büyük: genel ortalama (C) veritabanındaki başka kitaplara bağlıdır,
	// 1.000 oyda ağırlığı sırayı değiştiremeyecek kadar küçük kalır.
	for _, r := range []struct {
		id    int64
		count int
		avg   any
	}{{a, 1000, 9.0}, {b, 2000, 5.0}, {c, 3000, 6.0}, {d, 0, nil}, {e, 5000, 7.0}} {
		testdb.Exec(t, s.pool, "UPDATE books SET rating_count = $1, average_rating = $2 WHERE id = $3", r.count, r.avg, r.id)
	}

	for _, tc := range []struct {
		name string
		sort Sort
		want []int64
	}{
		{"varsayılan = ilgi sırası", SortDefault, []int64{a, b, c, d, e}},
		{"ilgi sırası açıkça", SortRelevance, []int64{a, b, c, d, e}},
		{"en yeni", SortNewest, []int64{e, d, c, b, a}},
		{"puana göre (oysuz en sonda)", SortRating, []int64{a, e, c, b, d}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, total := s.search(t, ListFilter{Query: "savaş", Sort: tc.sort, MinVotes: 10, ColdStart: 5.5})
			if !slices.Equal(got, tc.want) || total != 5 {
				t.Errorf("sıra: gelen %v (toplam %d), istenen %v", got, total, tc.want)
			}
		})
	}

	t.Run("aynı kademede çok oylanan önce", func(t *testing.T) {
		// "yazar": a, b, c ve e'nin yazarında geçer (hepsi aynı kademe: başlık + yazar); d'de geçmez.
		got, _ := s.search(t, ListFilter{Query: "yazar"})
		if want := []int64{e, c, b, a}; !slices.Equal(got, want) {
			t.Errorf("oy sırası: gelen %v, istenen %v", got, want)
		}
	})

	t.Run("sayfalama: toplam sayfadan bağımsız, sonun ötesinde de doğru", func(t *testing.T) {
		got, total := s.search(t, ListFilter{Query: "savaş", Limit: 2, Offset: 2})
		if want := []int64{c, d}; !slices.Equal(got, want) || total != 5 {
			t.Errorf("ikinci sayfa: gelen %v (toplam %d), istenen %v ve 5", got, total, want)
		}
		got, total = s.search(t, ListFilter{Query: "savaş", Limit: 2, Offset: 40})
		if len(got) != 0 || total != 5 {
			t.Errorf("sonun ötesi: %d sonuç, toplam %d; istenen 0 ve 5", len(got), total)
		}
		got, total = s.search(t, ListFilter{Query: "hiçbirkitaptayokbu", Limit: 2, Offset: 40})
		if len(got) != 0 || total != 0 {
			t.Errorf("eşleşmesiz aramada sonun ötesi: %d sonuç, toplam %d; istenen 0 ve 0", len(got), total)
		}
	})
}

// TestSearchFollowsWritesRealDB: arama belgesi uygulamanın yazımlarını izler (tetikleyiciler). Onay
// bekleyen kitap aramada görünmez; onaylanınca görünür, arşivlenince kaybolur; başlık düzeltmesi
// eski adı aramadan çıkarır.
func TestSearchFollowsWritesRealDB(t *testing.T) {
	s := newSearchFixture(t)
	// Baskısız kitap (doğrudan books'a): baskının adı da belgeye girdiği için, başlık düzeltmesinin
	// etkisini yalnız kitabın kendi başlığı üzerinden görmek istiyoruz. Baskı yolunun testi
	// internal/database/migrate_0014_integration_test.go'da.
	id := testdb.CreateBook(t, s.pool, "Saatleri Ayarlama Enstitüsü", StatusPending)
	if err := s.repo.SetGenres(context.Background(), id, []int64{s.genreID}); err != nil {
		t.Fatal(err)
	}

	expect := func(query string, found bool) {
		t.Helper()
		got, _ := s.search(t, ListFilter{Query: query})
		if (len(got) == 1 && got[0] == id) != found {
			t.Errorf("%q: bulundu=%v, istenen %v (sonuç %v)", query, !found, found, got)
		}
	}
	expect("saatleri ayarlama", false) // PENDING
	testdb.Exec(t, s.pool, "UPDATE books SET status = $1 WHERE id = $2", StatusApproved, id)
	expect("saatleri ayarlama", true)
	expect("ayarlama enstitusu", true)

	title := "Huzur"
	if _, err := s.repo.q.UpdateBookDetails(context.Background(), db.UpdateBookDetailsParams{ID: id, Title: &title}); err != nil {
		t.Fatal(err)
	}
	expect("saatleri ayarlama", false)
	expect("huzur", true)

	testdb.Exec(t, s.pool, "UPDATE books SET status = $1 WHERE id = $2", StatusArchived, id)
	expect("huzur", false)
}
