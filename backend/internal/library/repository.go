// Package library, kişisel kitaplığın veri erişim katmanı (Faz 15).
//
// Service katmanı yok: iş kuralları (yalnız yayındaki kitap eklenir, kayıt sahibine aittir)
// SQL'de yaşıyor, Go'da yapılacak bir karar kalmıyor (GO_CONVENTIONS §4).
package library

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// Status, bir kitaplık kaydının durumu; migration'daki CHECK kısıtının Go karşılığı.
type Status = string

const (
	StatusWantToRead Status = "WANT_TO_READ"
	StatusReading    Status = "READING"
	StatusRead       Status = "READ"
	StatusAbandoned  Status = "ABANDONED"
)

// ValidStatus, dışarıdan gelen bir durumu sınırda doğrular: yazım hatası DB kısıtına
// çarpıp 500 olmadan önce 400 olsun.
func ValidStatus(s string) bool {
	switch s {
	case StatusWantToRead, StatusReading, StatusRead, StatusAbandoned:
		return true
	}
	return false
}

// "Puanım" süzgecinin değerleri; boş = hepsi.
const (
	RatedYes = "yes"
	RatedNo  = "no"
)

// Sıralama seçenekleri; boş = SortAdded.
const (
	SortAdded    = "added"
	SortTitle    = "title"
	SortMyRating = "my_rating"
)

// ListFilter, Kitaplığım süzgeci. Değerler handler'da doğrulanmış gelir; boş string = süzgeç yok.
type ListFilter struct {
	Status, Query, Genre, Rated, Sort string
	Limit, Offset                     int32
}

// Repository, kitaplık verisine erişimin tek kapısı.
type Repository struct {
	conn db.DBTX
	q    *db.Queries
}

// NewRepository, verilen bağlantıyla bir Repository kurar.
func NewRepository(dbtx db.DBTX) *Repository {
	return &Repository{conn: dbtx, q: db.New(dbtx)}
}

// Put, kitabı kullanıcının kitaplığına ekler ya da durumunu değiştirir. Kitap yoksa ya da
// yayında değilse books.ErrNotFound döner; ikisi bilerek ayırt edilmez.
func (r *Repository) Put(ctx context.Context, userID, bookID int64, status string) (db.LibraryEntry, error) {
	e, err := r.q.UpsertLibraryEntry(ctx, db.UpsertLibraryEntryParams{UserID: userID, BookID: bookID, Status: status})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.LibraryEntry{}, books.ErrNotFound
	}
	if err != nil {
		return db.LibraryEntry{}, fmt.Errorf("kitaplık kaydı yazılamadı: %w", err)
	}
	return e, nil
}

// Remove, kitabı kullanıcının kitaplığından çıkarır; kayıt yoksa da hata değildir.
func (r *Repository) Remove(ctx context.Context, userID, bookID int64) error {
	if err := r.q.DeleteLibraryEntry(ctx, db.DeleteLibraryEntryParams{UserID: userID, BookID: bookID}); err != nil {
		return fmt.Errorf("kitaplık kaydı silinemedi: %w", err)
	}
	return nil
}

// List, kullanıcının kitaplığını süzer, sıralar ve sayfalar; ikinci dönüş sayfadan bağımsız toplam.
func (r *Repository) List(ctx context.Context, userID int64, f ListFilter) ([]db.ListLibraryRow, int64, error) {
	status, genre, rated := optional(f.Status), optional(f.Genre), optional(f.Rated)
	query := optional(books.EscapeLike(f.Query))
	sort := f.Sort
	if sort == "" {
		sort = SortAdded
	}
	rows, err := r.q.ListLibrary(ctx, db.ListLibraryParams{
		UserID: userID, Status: status, Genre: genre, Query: query, Rated: rated,
		Sort: sort, Limit: f.Limit, Offset: f.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("kitaplık listesi sorgusu: %w", err)
	}
	total, err := r.q.CountLibrary(ctx, db.CountLibraryParams{
		UserID: userID, Status: status, Genre: genre, Query: query, Rated: rated,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("kitaplık sayma sorgusu: %w", err)
	}
	return rows, total, nil
}

// optional, boş string'i NULL'a çevirir: SQL'deki `param IS NULL OR …` süzgeci kapatır.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Statuses, verilen kitaplardan kullanıcının kitaplığında olanların durumunu döner.
// Boş girdi için sorgu atılmaz; sonuç her durumda nil olmayan bir map'tir.
func (r *Repository) Statuses(ctx context.Context, userID int64, bookIDs []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(bookIDs))
	if len(bookIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.ListLibraryStatuses(ctx, db.ListLibraryStatusesParams{UserID: userID, BookIds: bookIDs})
	if err != nil {
		return nil, fmt.Errorf("kitaplık durumları okunamadı: %w", err)
	}
	for _, row := range rows {
		out[row.BookID] = row.Status
	}
	return out, nil
}
