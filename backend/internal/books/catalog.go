package books

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// catalogCache, katalog listesinin iki pahalı ama yavaş değişen girdisini süreç içinde tutar
// (Faz 16c; REVIEW_FEEDBACK F24, F25):
//
//   - tür slug'ı → kimlik: liste sorgusu türü kimlikle süzer (ölçüm: slug'la 4–391 ms, kimlikle
//     0,7–2,9 ms). Tür tablosu küçüktür ve seyrek değişir.
//   - toplam sayı (tür başına): sayım liste sorgusunun kendisinden pahalıdır (500 bin kitapta
//     50–278 ms) ve her sayfa isteğinde aynıdır. 60 sn bayat olabilir: yeni onaylanan kitap listede
//     HEMEN görünür, yalnız "N sonuç" sayısı gecikebilir (kullanıcı kararı, 2026-11-18).
//
// Önbellek isteğe bağlıdır (UseCatalogCache; main.go açar). Testlerde kapalıdır: her List çağrısı
// taze sayar ve türleri yeniden okur.
//
// NEDEN Redis değil: iki küçük değer; Redis'e sormak, kaçınmaya çalıştığımız gidiş-dönüşün aynısıdır.
// Bedeli: iki API örneği kısa süre farklı toplam gösterebilir (yayın sırasında).
type catalogCache struct {
	ttl time.Duration
	now func() time.Time

	mu       sync.Mutex
	genres   map[string]int64
	genresAt time.Time
	counts   map[int64]countEntry // anahtar tür kimliği; 0 = bütün katalog
}

type countEntry struct {
	n  int64
	at time.Time
}

// genreReloadMinInterval: önbellekte olmayan bir slug için tür tablosunu en sık bu aralıkla yeniden
// okur. Yeni eklenen bir tür hemen bulunur, ama rastgele slug'larla gelen istekler her seferinde
// veritabanına gidemez.
const genreReloadMinInterval = time.Second

// UseCatalogCache, tür eşlemesini ve toplam sayıları ttl boyunca süreç içinde tutar.
func (r *Repository) UseCatalogCache(ttl time.Duration) {
	r.catalog = &catalogCache{ttl: ttl, now: time.Now, counts: map[int64]countEntry{}}
}

// genreID, tür slug'ını kimliğe çevirir. ok=false: böyle bir tür yok (liste boştur, sorgu atılmaz).
func (r *Repository) genreID(ctx context.Context, slug string) (int64, bool, error) {
	c := r.catalog
	if c == nil {
		m, err := r.loadGenres(ctx)
		if err != nil {
			return 0, false, err
		}
		id, ok := m[slug]
		return id, ok, nil
	}

	c.mu.Lock()
	id, found := c.genres[slug]
	age := c.now().Sub(c.genresAt)
	c.mu.Unlock()
	if found && age < c.ttl {
		return id, true, nil
	}
	if !found && c.genres != nil && age < genreReloadMinInterval {
		return 0, false, nil
	}

	m, err := r.loadGenres(ctx)
	if err != nil {
		return 0, false, err
	}
	c.mu.Lock()
	c.genres, c.genresAt = m, c.now()
	c.mu.Unlock()
	id, found = m[slug]
	return id, found, nil
}

func (r *Repository) loadGenres(ctx context.Context) (map[string]int64, error) {
	rows, err := r.q.ListGenreSlugs(ctx)
	if err != nil {
		return nil, fmt.Errorf("tür eşlemesi okunamadı: %w", err)
	}
	m := make(map[string]int64, len(rows))
	for _, g := range rows {
		m[g.Slug] = g.ID
	}
	return m, nil
}

// count, onaylı kitapların sayısını döner (genreID 0: bütün katalog).
//
// Kilit sorgu sırasında TUTULMAZ: pahalı bir sayım, başka türlerin (ya da önbellekteki) okumalarını
// bekletmesin. Süre dolduğu anda aynı türü isteyen birkaç istek aynı sayımı birlikte çalıştırabilir;
// bu en çok dakikada bir olur ve kabul edildi.
func (r *Repository) count(ctx context.Context, genreID int64) (int64, error) {
	c := r.catalog
	if c != nil {
		c.mu.Lock()
		e, ok := c.counts[genreID]
		c.mu.Unlock()
		if ok && c.now().Sub(e.at) < c.ttl {
			return e.n, nil
		}
	}

	var (
		n   int64
		err error
	)
	if genreID == 0 {
		n, err = r.q.CountBooks(ctx)
	} else {
		n, err = r.q.CountBooksInGenre(ctx, genreID)
	}
	if err != nil {
		return 0, fmt.Errorf("kitap sayma sorgusu: %w", err)
	}
	if c != nil {
		c.mu.Lock()
		c.counts[genreID] = countEntry{n: n, at: c.now()}
		c.mu.Unlock()
	}
	return n, nil
}

// page, sıralamaya ve türe göre doğru liste sorgusunu çalıştırır.
func (r *Repository) page(ctx context.Context, f ListFilter, genreID int64) ([]db.BookView, error) {
	var out []db.BookView
	collect := func(n int, at func(int) db.BookView) {
		out = make([]db.BookView, 0, n)
		for i := range n {
			out = append(out, at(i))
		}
	}

	if f.Sort == SortRating {
		c, err := r.globalAverage(ctx, f.ColdStart)
		if err != nil {
			return nil, fmt.Errorf("genel ortalama sorgusu: %w", err)
		}
		if genreID == 0 {
			rows, err := r.q.ListBooksByRating(ctx, db.ListBooksByRatingParams{M: f.MinVotes, C: c, Limit: f.Limit, Offset: f.Offset})
			if err != nil {
				return nil, fmt.Errorf("kitap listeleme sorgusu: %w", err)
			}
			collect(len(rows), func(i int) db.BookView { return rows[i].BookView })
			return out, nil
		}
		rows, err := r.q.ListBooksByRatingInGenre(ctx, db.ListBooksByRatingInGenreParams{
			M: f.MinVotes, C: c, GenreID: genreID, Limit: f.Limit, Offset: f.Offset,
		})
		if err != nil {
			return nil, fmt.Errorf("kitap listeleme sorgusu: %w", err)
		}
		collect(len(rows), func(i int) db.BookView { return rows[i].BookView })
		return out, nil
	}

	if genreID == 0 {
		rows, err := r.q.ListBooks(ctx, db.ListBooksParams{Limit: f.Limit, Offset: f.Offset})
		if err != nil {
			return nil, fmt.Errorf("kitap listeleme sorgusu: %w", err)
		}
		collect(len(rows), func(i int) db.BookView { return rows[i].BookView })
		return out, nil
	}
	rows, err := r.q.ListBooksInGenre(ctx, db.ListBooksInGenreParams{GenreID: genreID, Limit: f.Limit, Offset: f.Offset})
	if err != nil {
		return nil, fmt.Errorf("kitap listeleme sorgusu: %w", err)
	}
	collect(len(rows), func(i int) db.BookView { return rows[i].BookView })
	return out, nil
}
