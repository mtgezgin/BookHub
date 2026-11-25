// Package library, kişisel kitaplık (Faz 15) ve okuma takibi (Faz 17A-1).
//
// Faz 15'te yalnız bu repository vardı: iş kuralları (yalnız yayındaki kitap eklenir, kayıt sahibine
// aittir) SQL'de yaşıyordu. Faz 17'de okuma takibiyle gerçek iş mantığı geldi (durum makinesi, günlük,
// sürüm); yazımlar artık Service'te (service.go, commands.go), listeler ve rozetler burada kalır
// (GO_CONVENTIONS §4: katman, mantık çıkınca eklenir).
package library

import (
	"context"
	"fmt"

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

// Repository, kitaplığın OKUMA tarafı: Kitaplığım listesi ve kart rozetleri. Yazımların tek kapısı
// Service'tir (komutlar); burada yazan bir yöntem yok.
type Repository struct {
	q *db.Queries
}

// NewRepository, verilen bağlantıyla bir Repository kurar.
func NewRepository(dbtx db.DBTX) *Repository {
	return &Repository{q: db.New(dbtx)}
}

// List, kullanıcının kitaplığını süzer, sıralar ve sayfalar; ikinci dönüş sayfadan bağımsız toplam.
func (r *Repository) List(ctx context.Context, userID int64, f ListFilter) ([]db.ListLibraryRow, int64, error) {
	status, genre, rated := optional(f.Status), optional(f.Genre), optional(f.Rated)
	query := optional(f.Query)
	sort := f.Sort
	if sort == "" {
		sort = SortAdded
	}
	params := db.ListLibraryParams{
		UserID: userID, Status: status, Genre: genre, Query: query, Rated: rated,
		Sort: sort, Limit: f.Limit, Offset: f.Offset,
	}
	rows, err := r.q.ListLibrary(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("kitaplık listesi sorgusu: %w", err)
	}
	if len(rows) > 0 {
		return rows, rows[0].Total, nil
	}
	if f.Offset == 0 {
		return rows, 0, nil
	}
	// Sayfa sonun ötesinde: satır yok, toplamı taşıyan satır da yok. İlk satırı isteyip toplamı ondan al
	// (arayüz boşalmış bir sayfadan son gerçek sayfaya bu toplamla döner).
	params.Limit, params.Offset = 1, 0
	first, err := r.q.ListLibrary(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("kitaplık toplamı sorgusu: %w", err)
	}
	if len(first) == 0 {
		return rows, 0, nil
	}
	return rows, first[0].Total, nil
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
