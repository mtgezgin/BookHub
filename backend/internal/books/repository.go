// Package books, kitap domain'inin veri erişim katmanı (repository).
package books

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// ErrNotFound, istenen kitap yokken dönen DOMAIN hatası.
// Amaç: üst katman (handler) pgx'in iç hatası yerine bunu tanısın —
// "hangi sürücüyü kullanıyoruz" bilgisi bu paketin içinde hapsolsun.
var ErrNotFound = errors.New("kitap bulunamadı")

// ErrDuplicateSlug, aynı slug ile ikinci kez kitap eklenmeye çalışıldığında döner.
// Bu bir SUNUCU hatası değil, istemcinin düzeltebileceği bir çakışmadır — handler
// bunu 409 Conflict'e çevirir (500 değil).
var ErrDuplicateSlug = errors.New("bu slug zaten kullanılıyor")

// ErrUnknownGenre, kitaba var olmayan bir tür bağlanmaya çalışıldığında döner.
var ErrUnknownGenre = errors.New("bilinmeyen tür")

// MaxGenresPerBook, bir kitaba bağlanabilecek en fazla tür (makul sınır; istemcinin
// binlerce id'lik dizi göndermesini engeller).
const MaxGenresPerBook = 10

// Kitap statüleri — migration'daki CHECK kısıtının Go tarafındaki karşılığı
// (bkz. migrations/0001_books_genres.sql). Sabit olarak durmalarının sebebi:
// "APPROVED" gibi bir dizgiyi handler'lara serpiştirmek, yazım hatasını derleme
// zamanında değil ancak DB kısıtı reddedince fark ettirir.
const (
	StatusPending  = "PENDING"
	StatusApproved = "APPROVED"
	StatusRejected = "REJECTED"
	StatusArchived = "ARCHIVED" // Faz 4: yayından kaldırıldı (silinmedi)
)

// ErrUnknownStatus, dışarıdan gelen (ör. ?status=) geçersiz statü değeri.
var ErrUnknownStatus = errors.New("bilinmeyen statü")

// ParseStatus, dış dünyadan gelen bir statü metnini SINIRDA doğrular (WHY_GO_VS_TS §1):
// içeri giren her değer bu dört sabitten biridir; "approved" / "APROVED" gibi yazımlar
// sorguya ulaşıp sessizce boş liste döndürmez, 400 olur.
func ParseStatus(s string) (string, error) {
	switch s {
	case StatusPending, StatusApproved, StatusRejected, StatusArchived:
		return s, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownStatus, s)
}

// Repository, kitap verisine erişimin tek kapısı. sqlc'nin ürettiği Queries'i
// sarar; katma değeri ŞU AN iki şey: (1) sürücü hatalarını domain hatasına
// çevirmek, (2) ileride birden çok sorguyu tek işlemde toplamak (m2m, tx).
// İş mantığı YOK — o (gerektiğinde, Faz 3+) Service katmanına ait.
type Repository struct {
	conn db.DBTX // transaction açmak için (pool) — bkz. SetGenres
	q    *db.Queries
	// average: genel ortalamanın (C) süreç içi önbelleği; nil ise her seferinde veritabanından okunur.
	average func(ctx context.Context) (float64, error)
}

// UseGlobalAverage, puana göre sıralamada kullanılan genel ortalamanın kaynağını değiştirir
// (ratings.GlobalAverage; main.go bağlar). Kaynak boş katalogdaki değeri (cold start) de kendisi bilir.
func (r *Repository) UseGlobalAverage(fn func(ctx context.Context) (float64, error)) { r.average = fn }

func (r *Repository) globalAverage(ctx context.Context, coldStart float64) (float64, error) {
	if r.average != nil {
		return r.average(ctx)
	}
	return r.q.GlobalAverageRating(ctx, coldStart)
}

// NewRepository, verilen bağlantıyla bir Repository kurar.
// Parametrenin tipi db.DBTX (üretilen interface): *pgxpool.Pool de pgx.Tx de
// bunu sağlar → aynı repository ileride transaction İÇİNDE de kullanılabilir.
func NewRepository(dbtx db.DBTX) *Repository {
	return &Repository{conn: dbtx, q: db.New(dbtx)}
}

// Get, HERKESE AÇIK tekil okumadır: yalnızca APPROVED kitabı döner, aksi halde
// ErrNotFound (onaylanmamış kitabın varlığını sızdırmamak için "yok" muamelesi).
// Görünürlük sınırı bilerek SQL'de (GetApprovedBook) — bkz. sql/queries/books.sql.
// Admin'in tüm statüleri okuması gerektiğinde AYRI bir sorgu + metot eklenecek
// (Faz 4), bu metodun davranışı gevşetilmeyecek.
// ctx'i sorguya İLETİR (iptal/deadline zinciri).
func (r *Repository) Get(ctx context.Context, id int64) (db.BookView, error) {
	book, err := r.q.GetApprovedBook(ctx, id)
	if err != nil {
		// pgx, ":one" sorgusu boş dönerse ErrNoRows verir — bu bizim için
		// "hata" değil "yok" demek; domain diline çevir.
		if errors.Is(err, pgx.ErrNoRows) {
			return db.BookView{}, ErrNotFound
		}
		// Gerçek hata (bağlantı koptu vb.): bağlam ekleyip yukarı sar (%w zinciri).
		return db.BookView{}, fmt.Errorf("kitap sorgusu (id=%d): %w", id, err)
	}
	return book, nil
}

// CreateParams, yeni kitap: eserin alanları (books) + varsayılan baskısının kapağı ve sayfa
// sayısı (editions). Faz 16'dan beri bu iki alan kitapta değil baskıda durur.
type CreateParams struct {
	db.CreateBookParams
	CoverURL  string
	PageCount *int32 // nil = bilinmiyor
}

// UpdateParams, kitap PATCH'i: nil alan = dokunma. Kapak ve sayfa sayısı varsayılan baskıya
// yazılır; Language yalnız eserin (kitabın) dilidir, baskının dili ayrı uçla değişir.
type UpdateParams struct {
	db.UpdateBookDetailsParams
	CoverURL  *string
	PageCount *int32
}

// Create, yeni kitabı varsayılan baskısıyla birlikte ekler ve kitabın tam hâlini (id, zaman
// damgaları, baskıdan gelen kapak) book_view'dan okuyup döner.
// İki INSERT tek transaction'da: r bir havuzsa yeni transaction, zaten bir transaction'sa
// (moderasyon servisi) savepoint açılır — baskısız kitap ya da kitapsız baskı kalmaz.
// Slug çakışmasını (Postgres 23505) ErrDuplicateSlug'a çevirir; diğer hatalar sarılır.
func (r *Repository) Create(ctx context.Context, p CreateParams) (db.BookView, error) {
	var book db.BookView
	err := database.WithTx(ctx, r.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		id, err := q.CreateBook(ctx, p.CreateBookParams)
		if err != nil {
			// Postgres 23505 = unique_violation. books tablosunda istemcinin ihlal
			// EDEBİLECEĞİ tek benzersizlik kısıtı slug'dır (id'yi DB üretiyor). İkinci bir
			// UNIQUE kolon eklenirse genres.translateWriteErr gibi kısıt adına bakmalı.
			if _, ok := database.UniqueViolation(err); ok {
				return ErrDuplicateSlug
			}
			return fmt.Errorf("kitap ekleme sorgusu: %w", err)
		}
		if err := q.CreatePrimaryEdition(ctx, db.CreatePrimaryEditionParams{
			BookID:    id,
			Title:     p.Title,
			Language:  p.Language,
			CoverUrl:  p.CoverURL,
			PageCount: p.PageCount,
		}); err != nil {
			return fmt.Errorf("varsayılan baskı eklenemedi (kitap=%d): %w", id, err)
		}
		book, err = q.GetBook(ctx, id)
		if err != nil {
			return fmt.Errorf("eklenen kitap okunamadı (id=%d): %w", id, err)
		}
		return nil
	})
	return book, err
}

// Editions, kitabın baskıları (Faz 16): varsayılan önce, sonra dil, yıl (bilinmeyen sonda), id.
// Düz okuma; kitabın görünürlüğünü çağıran denetler (public uç önce Get ile onaylı kitabı okur).
// Baskı YAZIMLARI kurallı olduğu için internal/editions servisinde.
func (r *Repository) Editions(ctx context.Context, bookID int64) ([]db.Edition, error) {
	list, err := r.q.ListEditionsByBook(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("baskılar okunamadı (kitap=%d): %w", bookID, err)
	}
	return list, nil
}

// Sort, katalog sırası.
type Sort string

const (
	// SortDefault: aramasız listede en yeni önce; aramada ilgi sırası (en iyi eşleşme önce).
	SortDefault Sort = ""
	// SortRelevance, aramanın ilgi sırası. Aramasız listede karşılığı yoktur, varsayılana düşer.
	SortRelevance Sort = "relevance"
	// SortNewest, en yeni önce: aramasız listenin varsayılanı, aramada açıkça istenirse.
	SortNewest Sort = "newest"
	// SortRating, ağırlıklı (Bayes) puan; oysuzlar sonda.
	SortRating Sort = "rating"
)

// ListFilter, public kitap listesinin ve aramanın filtre, sıra ve sayfalama parametreleri.
// Boş string = o filtre yok. Sınırların doğrulanması (limit ≤ 100 vb.) HTTP katmanının işi.
type ListFilter struct {
	Genre string // tür slug'ı
	// Query, aranan metin (ham hâliyle; katlama ve kelimelere ayırma veritabanında yapılır).
	// Yalnız Search okur; List bu alana bakmaz.
	Query string
	Sort  Sort
	// Yalnız SortRating için: ağırlıklı skorun m'i ve hiç oy yokken genel ortalamanın yerine
	// geçen değer. Değerler ratings paketinden gelir; books onu içe aktaramaz (ratings books'u
	// aktarıyor), bu yüzden çağıran (HTTP katmanı) verir.
	MinVotes  float64
	ColdStart float64
	Limit     int32
	Offset    int32
}

// List, filtreye uyan onaylı kitapların bir SAYFASINI ve filtreye uyan TOPLAM sayıyı
// döner (varsayılan en yeniden eskiye). Boş sonuç hata değildir; ErrNotFound çevirisi yoktur.
// Metin araması burada değil Search'tedir.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]db.BookView, int64, error) {
	genre := optional(f.Genre)

	var (
		list []db.BookView
		err  error
	)
	if f.Sort == SortRating {
		var c float64
		c, err = r.globalAverage(ctx, f.ColdStart)
		if err != nil {
			return nil, 0, fmt.Errorf("genel ortalama sorgusu: %w", err)
		}
		list, err = r.q.ListBooksByRating(ctx, db.ListBooksByRatingParams{
			Genre: genre, M: f.MinVotes, C: c, Limit: f.Limit, Offset: f.Offset,
		})
	} else {
		list, err = r.q.ListBooks(ctx, db.ListBooksParams{Genre: genre, Limit: f.Limit, Offset: f.Offset})
	}
	if err != nil {
		return nil, 0, fmt.Errorf("kitap listeleme sorgusu: %w", err)
	}
	total, err := r.q.CountBooks(ctx, genre)
	if err != nil {
		return nil, 0, fmt.Errorf("kitap sayma sorgusu: %w", err)
	}
	return list, total, nil
}

// SearchHit, arama sonucundaki bir kitap: kitabın kendisi ve türleri (aynı sorgudan).
type SearchHit struct {
	Book   db.BookView
	Genres []db.Genre
}

// Search, f.Query ile eşleşen onaylı kitapların bir SAYFASINI ve eşleşen TOPLAM sayıyı döner.
// Eşleşme Türkçe harfe, aksana, büyük/küçük harfe ve kelime sırasına duyarsızdır; son kelime önek
// sayılır (tanım: migrations/0014_book_search.sql, sorgu: sql/queries/books.sql SearchBooks).
//
// List'ten farkı: sayfa, toplam ve türler TEK sorguda gelir. Yazarken arama bu ucu sık çağırır ve
// canlıda her sorgu ayrı bir ağ gidiş-dönüşüdür. Puan sırası tek istisna: genel ortalama (C) ayrı
// okunur, çünkü tek tanımı ratings.sql'dedir.
func (r *Repository) Search(ctx context.Context, f ListFilter) ([]SearchHit, int64, error) {
	p := db.SearchBooksParams{
		Query: f.Query, Sort: string(searchSort(f.Sort)), Genre: optional(f.Genre),
		Limit: f.Limit, Offset: f.Offset,
	}
	if f.Sort == SortRating {
		c, err := r.globalAverage(ctx, f.ColdStart)
		if err != nil {
			return nil, 0, fmt.Errorf("genel ortalama sorgusu: %w", err)
		}
		p.M, p.C = f.MinVotes, c
	}
	rows, err := r.q.SearchBooks(ctx, p)
	if err != nil {
		return nil, 0, fmt.Errorf("kitap arama sorgusu: %w", err)
	}
	hits := make([]SearchHit, 0, len(rows))
	for _, row := range rows {
		genres, err := decodeGenres(row.Genres)
		if err != nil {
			return nil, 0, fmt.Errorf("arama sonucunun türleri çözülemedi (kitap=%d): %w", row.BookView.ID, err)
		}
		hits = append(hits, SearchHit{Book: row.BookView, Genres: genres})
	}
	if len(rows) > 0 {
		return hits, rows[0].Total, nil
	}
	if f.Offset == 0 {
		return hits, 0, nil
	}
	// Sayfa sonun ötesinde: satır yok, toplamı taşıyan satır da yok. İlk satırı isteyip toplamı ondan
	// al (arayüz boşalmış bir sayfadan son gerçek sayfaya bu toplamla döner; kitaplıktaki desen).
	p.Limit, p.Offset = 1, 0
	first, err := r.q.SearchBooks(ctx, p)
	if err != nil {
		return nil, 0, fmt.Errorf("kitap arama toplamı sorgusu: %w", err)
	}
	if len(first) == 0 {
		return hits, 0, nil
	}
	return hits, first[0].Total, nil
}

// searchSort, aramada geçerli sırayı seçer: belirtilmemişse (ya da tanınmıyorsa) ilgi sırası.
func searchSort(s Sort) Sort {
	switch s {
	case SortNewest, SortRating:
		return s
	}
	return SortRelevance
}

// decodeGenres, SearchBooks'un jsonb olarak döndürdüğü tür dizisini çözer. Alan adları sorgudaki
// jsonb_build_object ile eşleşir; db.Genre'nin üretilmiş JSON etiketlerine bağlı değildir.
func decodeGenres(raw []byte) ([]db.Genre, error) {
	var rows []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	out := make([]db.Genre, 0, len(rows))
	for _, g := range rows {
		out = append(out, db.Genre{ID: g.ID, Name: g.Name, Slug: g.Slug})
	}
	return out, nil
}

// optional, boş string'i NULL'a (nil) çevirir: SQL'deki `param IS NULL OR ...` deseni
// filtreyi böylece devre dışı bırakır.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GenresForBook, tek kitabın türlerini ada göre sıralı döner.
func (r *Repository) GenresForBook(ctx context.Context, bookID int64) ([]db.Genre, error) {
	list, err := r.q.ListGenresForBook(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("kitap türleri sorgusu (id=%d): %w", bookID, err)
	}
	return list, nil
}

// GenresForBooks, birden çok kitabın türlerini TEK sorguda çeker ve kitap id'sine göre
// gruplar (N+1'den kaçınma: kitap sayısı ne olursa olsun tek sorgu). Türü olmayan kitap
// haritada hiç yer almaz; çağıran bunu "boş liste" olarak ele almalı.
func (r *Repository) GenresForBooks(ctx context.Context, bookIDs []int64) (map[int64][]db.Genre, error) {
	out := make(map[int64][]db.Genre, len(bookIDs))
	if len(bookIDs) == 0 {
		return out, nil // boş dizi için sorgu atmaya gerek yok
	}
	rows, err := r.q.ListGenresForBooks(ctx, bookIDs)
	if err != nil {
		return nil, fmt.Errorf("toplu kitap türleri sorgusu: %w", err)
	}
	for _, row := range rows {
		out[row.BookID] = append(out[row.BookID], db.Genre{ID: row.ID, Name: row.Name, Slug: row.Slug})
	}
	return out, nil
}

// SetGenres, kitabın türlerini verilen listeyle DEĞİŞTİRİR (replace semantiği: boş liste
// = tüm türleri kaldır). Tek transaction'da: kitabı kilitle → eski bağları sil → yenileri
// ekle. Ara adımda hata olursa hiçbiri kalıcı olmaz. Kitabın statüsüne bakmaz (admin işi;
// onay bekleyen bir kitaba da tür atanabilir).
func (r *Repository) SetGenres(ctx context.Context, bookID int64, genreIDs []int64) error {
	ids := dedupe(genreIDs) // aynı id iki kez gelirse PK ihlali olurdu; istemcinin hatası değil
	return database.WithTx(ctx, r.conn, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)

		if _, err := q.LockBook(ctx, bookID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("kitap kilitlenemedi (id=%d): %w", bookID, err)
		}
		if err := q.ClearBookGenres(ctx, bookID); err != nil {
			return fmt.Errorf("eski türler silinemedi (id=%d): %w", bookID, err)
		}
		if len(ids) == 0 {
			return nil
		}
		err := q.AddBookGenres(ctx, db.AddBookGenresParams{BookID: bookID, GenreIds: ids})
		if _, ok := database.ForeignKeyViolation(err); ok {
			return ErrUnknownGenre // kitabı kilitledik; kalan tek FK genre_id'dir
		}
		if err != nil {
			return fmt.Errorf("türler eklenemedi (id=%d): %w", bookID, err)
		}
		return nil
	})
}

// dedupe, sırayı koruyarak tekrarlanan id'leri atar.
func dedupe(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
