// Package books, kitap domain'inin veri erişim katmanı (repository).
package books

import (
	"context"
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

// Kitap statüleri — migration'daki CHECK kısıtının Go tarafındaki karşılığı
// (bkz. migrations/0001_books_genres.sql). Sabit olarak durmalarının sebebi:
// "APPROVED" gibi bir dizgiyi handler'lara serpiştirmek, yazım hatasını derleme
// zamanında değil ancak DB kısıtı reddedince fark ettirir.
const (
	StatusPending  = "PENDING"
	StatusApproved = "APPROVED"
	StatusRejected = "REJECTED"
)

// Repository, kitap verisine erişimin tek kapısı. sqlc'nin ürettiği Queries'i
// sarar; katma değeri ŞU AN iki şey: (1) sürücü hatalarını domain hatasına
// çevirmek, (2) ileride birden çok sorguyu tek işlemde toplamak (m2m, tx).
// İş mantığı YOK — o (gerektiğinde, Faz 3+) Service katmanına ait.
type Repository struct {
	q *db.Queries
}

// NewRepository, verilen bağlantıyla bir Repository kurar.
// Parametrenin tipi db.DBTX (üretilen interface): *pgxpool.Pool de pgx.Tx de
// bunu sağlar → aynı repository ileride transaction İÇİNDE de kullanılabilir.
func NewRepository(dbtx db.DBTX) *Repository {
	return &Repository{q: db.New(dbtx)}
}

// Get, HERKESE AÇIK tekil okumadır: yalnızca APPROVED kitabı döner, aksi halde
// ErrNotFound (onaylanmamış kitabın varlığını sızdırmamak için "yok" muamelesi).
// Görünürlük sınırı bilerek SQL'de (GetApprovedBook) — bkz. sql/queries/books.sql.
// Admin'in tüm statüleri okuması gerektiğinde AYRI bir sorgu + metot eklenecek
// (Faz 4), bu metodun davranışı gevşetilmeyecek.
// ctx'i sorguya İLETİR (iptal/deadline zinciri).
func (r *Repository) Get(ctx context.Context, id int64) (db.Book, error) {
	book, err := r.q.GetApprovedBook(ctx, id)
	if err != nil {
		// pgx, ":one" sorgusu boş dönerse ErrNoRows verir — bu bizim için
		// "hata" değil "yok" demek; domain diline çevir.
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Book{}, ErrNotFound
		}
		// Gerçek hata (bağlantı koptu vb.): bağlam ekleyip yukarı sar (%w zinciri).
		return db.Book{}, fmt.Errorf("kitap sorgusu (id=%d): %w", id, err)
	}
	return book, nil
}

// Create, yeni kitabı ekler ve DB'nin ürettiği hâlini (id + zaman damgaları dolu)
// geri döner — sorgudaki RETURNING * bunun için var.
// Slug çakışmasını (Postgres 23505) ErrDuplicateSlug'a çevirir; diğer hatalar sarılır.
func (r *Repository) Create(ctx context.Context, params db.CreateBookParams) (db.Book, error) {
	book, err := r.q.CreateBook(ctx, params)
	if err != nil {
		// Postgres 23505 = unique_violation. books tablosunda istemcinin ihlal
		// EDEBİLECEĞİ tek benzersizlik kısıtı slug'dır (id'yi DB üretiyor). İkinci bir
		// UNIQUE kolon eklenirse genres.translateWriteErr gibi kısıt adına bakmalı.
		if _, ok := database.UniqueViolation(err); ok {
			return db.Book{}, ErrDuplicateSlug
		}
		return db.Book{}, fmt.Errorf("kitap ekleme sorgusu: %w", err)
	}
	return book, nil
}

// List, onaylı kitapları en yeniden eskiye döner.
// Get'ten farklı olarak ErrNotFound çevirisi YOK: boş sonuç bir hata değildir,
// ":many" sorgusu satır bulamazsa boş liste döner. (Hata yolunda slice'ın zero
// value'su nil'dir — çağıran zaten önce err'e bakar.)
func (r *Repository) List(ctx context.Context) ([]db.Book, error) {
	books, err := r.q.ListBooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("kitap listeleme sorgusu: %w", err)
	}
	return books, nil
}
