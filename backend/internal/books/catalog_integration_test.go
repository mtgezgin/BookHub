package books

import (
	"context"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// genreWithBooks, yeni bir tür açar ve ona n onaylı kitap bağlar; slug'ı ve kitapların EN YENİDEN
// eskiye kimliklerini döner. Test veritabanı başka paketlerle paylaşıldığı için sayım ve sıra
// iddiaları yalnız bu türün içinde kurulur.
func genreWithBooks(t *testing.T, f fixture, n int) (string, []int64) {
	t.Helper()
	ctx := context.Background()
	slug := testdb.Unique("tur")
	var gid int64
	if err := f.pool.QueryRow(ctx, "INSERT INTO genres (name, slug) VALUES ($1, $2) RETURNING id",
		testdb.Unique("Tür"), slug).Scan(&gid); err != nil {
		t.Fatalf("tür eklenemedi: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, f.pool, "DELETE FROM genres WHERE id = $1", gid) })
	ids := make([]int64, n)
	for i := range n {
		b := f.createBook(t, StatusApproved)
		if err := f.repo.SetGenres(ctx, b.ID, []int64{gid}); err != nil {
			t.Fatalf("SetGenres: %v", err)
		}
		ids[n-1-i] = b.ID // sonra eklenen daha yeni: başa
	}
	return slug, ids
}

// TestListInGenreRealDB: tür süzgeci kimlikle çalışır; sayfalar kararlı ve eksiksizdir (önce kimlik,
// sonra görünüm), bilinmeyen tür hata değil boş sayfadır (Faz 16c).
func TestListInGenreRealDB(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	slug, want := genreWithBooks(t, f, 7)
	f.createBook(t, StatusPending) // onaysız: hiçbir sayfada görünmemeli

	var got []int64
	for offset := int32(0); offset < 9; offset += 3 {
		page, total, err := f.repo.List(ctx, ListFilter{Genre: slug, Limit: 3, Offset: offset})
		if err != nil {
			t.Fatalf("List(offset %d): %v", offset, err)
		}
		if total != 7 {
			t.Errorf("toplam 7 bekleniyordu, gelen %d", total)
		}
		for _, b := range page {
			got = append(got, b.ID)
			if b.Title == "" {
				t.Error("görünüm alanları (book_view) dolu gelmeli")
			}
		}
	}
	if !equalIDs(got, want) {
		t.Errorf("sayfalar birleşince en yeniden eskiye sıra bekleniyordu:\n gelen %v\n istenen %v", got, want)
	}

	page, total, err := f.repo.List(ctx, ListFilter{Genre: "boyle-bir-tur-yok", Limit: 20})
	if err != nil || total != 0 || page == nil || len(page) != 0 {
		t.Errorf("bilinmeyen tür boş sayfa (nil değil) dönmeli: %v, %d, %v", page, total, err)
	}

	byRating, _, err := f.repo.List(ctx, ListFilter{Genre: slug, Sort: SortRating, MinVotes: 10, ColdStart: 5.5, Limit: 20})
	if err != nil || len(byRating) != 7 {
		t.Errorf("puan sırası da türle süzülmeli: %d kitap, %v", len(byRating), err)
	}
}

// TestCatalogCacheRealDB: önbellek açıkken toplam sayı süre dolana kadar yeniden hesaplanmaz (yeni
// kitap listede hemen görünür, yalnız toplam gecikir); süre dolunca tazelenir. Önbellekte olmayan bir
// tür, tür tablosu yeniden okunarak bulunur (en sık saniyede bir).
func TestCatalogCacheRealDB(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.repo.UseCatalogCache(time.Minute)
	now := time.Now()
	f.repo.catalog.now = func() time.Time { return now }

	slug, _ := genreWithBooks(t, f, 2)
	if _, total, err := f.repo.List(ctx, ListFilter{Genre: slug, Limit: 20}); err != nil || total != 2 {
		t.Fatalf("ilk sayım 2 olmalı: %d, %v", total, err)
	}

	// Türe üçüncü kitap eklenir: sayfa onu HEMEN gösterir, toplam 60 sn boyunca eski kalır.
	var gid int64
	if err := f.pool.QueryRow(ctx, "SELECT id FROM genres WHERE slug = $1", slug).Scan(&gid); err != nil {
		t.Fatal(err)
	}
	b := f.createBook(t, StatusApproved)
	if err := f.repo.SetGenres(ctx, b.ID, []int64{gid}); err != nil {
		t.Fatal(err)
	}
	page, total, err := f.repo.List(ctx, ListFilter{Genre: slug, Limit: 20})
	if err != nil || len(page) != 3 || total != 2 {
		t.Errorf("yeni kitap listede olmalı, toplam önbellekten (2) gelmeli: %d kitap, toplam %d, %v", len(page), total, err)
	}
	now = now.Add(time.Minute)
	if _, total, _ := f.repo.List(ctx, ListFilter{Genre: slug, Limit: 20}); total != 3 {
		t.Errorf("süre dolunca toplam tazelenmeli (3), gelen %d", total)
	}

	// Önbellek yüklendikten SONRA eklenen tür: bir saniye içinde yeniden okunmaz, sonra bulunur.
	newSlug, _ := genreWithBooks(t, f, 1)
	if page, _, _ := f.repo.List(ctx, ListFilter{Genre: newSlug, Limit: 20}); len(page) != 0 {
		t.Errorf("tür tablosu saniyede birden sık okunmamalı; gelen %d kitap", len(page))
	}
	now = now.Add(2 * time.Second)
	if page, total, _ := f.repo.List(ctx, ListFilter{Genre: newSlug, Limit: 20}); len(page) != 1 || total != 1 {
		t.Errorf("yeni tür yeniden okunarak bulunmalı: %d kitap, toplam %d", len(page), total)
	}
}
