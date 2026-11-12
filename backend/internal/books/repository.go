// Package books, kitap domain'inin veri erişim katmanı (repository).
package books

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// Sort, katalog sırası.
type Sort string

const (
	SortNewest Sort = ""       // varsayılan: en yeni önce
	SortRating Sort = "rating" // ağırlıklı (Bayes) puan, oysuzlar sonda
)

// ListFilter, public kitap listesinin filtre, sıra ve sayfalama parametreleri.
// Boş string = o filtre yok. Sınırların doğrulanması (limit ≤ 100 vb.) HTTP katmanının işi.
type ListFilter struct {
	Genre string // tür slug'ı
	Query string // başlık/yazar içinde aranan metin
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
func (r *Repository) List(ctx context.Context, f ListFilter) ([]db.BookView, int64, error) {
	genre, query := optional(f.Genre), optional(EscapeLike(f.Query))

	var (
		list []db.BookView
		err  error
	)
	if f.Sort == SortRating {
		var c float64
		c, err = r.q.GlobalAverageRating(ctx, f.ColdStart)
		if err != nil {
			return nil, 0, fmt.Errorf("genel ortalama sorgusu: %w", err)
		}
		list, err = r.q.ListBooksByRating(ctx, db.ListBooksByRatingParams{
			Genre: genre, Query: query, M: f.MinVotes, C: c, Limit: f.Limit, Offset: f.Offset,
		})
	} else {
		list, err = r.q.ListBooks(ctx, db.ListBooksParams{Genre: genre, Query: query, Limit: f.Limit, Offset: f.Offset})
	}
	if err != nil {
		return nil, 0, fmt.Errorf("kitap listeleme sorgusu: %w", err)
	}
	total, err := r.q.CountBooks(ctx, db.CountBooksParams{Genre: genre, Query: query})
	if err != nil {
		return nil, 0, fmt.Errorf("kitap sayma sorgusu: %w", err)
	}
	return list, total, nil
}

// optional, boş string'i NULL'a (nil) çevirir: SQL'deki `param IS NULL OR ...` deseni
// filtreyi böylece devre dışı bırakır.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// EscapeLike, kullanıcının aradığı metindeki LIKE jokerlerini (%, _) ve kaçış karakterini
// (\) kaçışlar. Güvenlik açığı değil (sorgu zaten parametreli) ama anlam hatası: "%50"
// araması "50 ile biten her şey" demek olmamalı.
func EscapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
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
