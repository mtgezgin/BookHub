// Package editions, kitap baskılarının (Faz 16, #36) iş kuralları.
//
// Kitap (books) eserdir; baskı (editions) elde tutulan somut kitaptır: dil, çevirmen, yayınevi,
// yıl, ISBN, sayfa sayısı, kapak. Puan ve yorum eserde kalır.
//
// Neden service (düz CRUD değil): gerçek kurallar var.
//   - Kitabın her zaman TAM OLARAK bir varsayılan baskısı olur: "en çok bir"i kısmi benzersiz
//     index, "en az bir"i bu paket sağlar (ilk baskı varsayılan olur, varsayılan silinemez).
//   - Varsayılanı değiştirmek tek transaction'da iki adımdır (önce temizle, sonra ata).
//   - Her yazım denetim kaydıyla aynı transaction'da; kayıt kitabın geçmişinde görünür.
package editions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/audit"
	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// ErrNotFound, istenen baskı yokken döner.
var ErrNotFound = errors.New("baskı bulunamadı")

// ErrPrimaryEditionDelete: varsayılan baskı silinemez; önce başka bir baskı varsayılan yapılır.
var ErrPrimaryEditionDelete = errors.New("varsayılan baskı silinemez; önce başka bir baskıyı varsayılan yapın")

// Denetim kaydı eylemleri. entity_type = book, entity_id = kitap: baskı değişiklikleri
// kitabın geçmişinde (/api/admin/books/{id}/audit) görünür.
const (
	ActionCreate     = "edition.create"
	ActionUpdate     = "edition.update"
	ActionDelete     = "edition.delete"
	ActionSetPrimary = "edition.set_primary"
)

// Service, baskı okuma ve yazma işlemleri.
type Service struct {
	conn db.DBTX
	q    *db.Queries
	// onPublicChange: public görünümü etkileyen bir yazım commit edilince çağrılır (en iyi
	// puanlılar önbelleği kapak taşıyor). Moderasyon servisindeki kancanın aynısı.
	onPublicChange func(ctx context.Context)
}

// Option, Service'i kurarken opsiyonel davranış ekler.
type Option func(*Service)

// WithPublicChangeHook, her başarılı yazımdan sonra fn'i çağırır.
func WithPublicChangeHook(fn func(ctx context.Context)) Option {
	return func(s *Service) { s.onPublicChange = fn }
}

// NewService, verilen bağlantıyla bir Service kurar.
func NewService(conn db.DBTX, opts ...Option) *Service {
	s := &Service{conn: conn, q: db.New(conn)}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Service) publicChanged(ctx context.Context, err error) {
	if err == nil && s.onPublicChange != nil {
		s.onPublicChange(context.WithoutCancel(ctx))
	}
}

// ListByBook, kitabın baskıları: varsayılan önce, sonra dil, yıl (bilinmeyen sonda), id.
// Kitabın varlığını ve görünürlüğünü çağıran denetler (public uç önce onaylı kitabı okur).
func (s *Service) ListByBook(ctx context.Context, bookID int64) ([]db.Edition, error) {
	list, err := s.q.ListEditionsByBook(ctx, bookID)
	if err != nil {
		return nil, fmt.Errorf("baskılar okunamadı (kitap=%d): %w", bookID, err)
	}
	return list, nil
}

// CreateParams, yeni baskı. IsPrimary yok: kitabın varsayılanı yoksa yeni baskı varsayılan olur,
// varsa olmaz; varsayılanı değiştirmek ayrı işlem (SetPrimary).
type CreateParams struct {
	Title           string
	Language        string
	Translator      string
	Publisher       string
	PublicationYear *int32
	Isbn            string
	PageCount       *int32
	CoverURL        string
}

// Create, kitaba baskı ekler. Kitap yoksa books.ErrNotFound.
func (s *Service) Create(ctx context.Context, actorID, bookID int64, p CreateParams) (db.Edition, error) {
	var ed db.Edition
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := lockBook(ctx, q, bookID); err != nil {
			return err
		}
		// Kilit altında okunur: aynı kitaba aynı anda iki baskı eklense de ikisi birden
		// "varsayılan yok" görüp varsayılan olmaya çalışmaz.
		hasPrimary, err := q.HasPrimaryEdition(ctx, bookID)
		if err != nil {
			return fmt.Errorf("varsayılan baskı denetlenemedi (kitap=%d): %w", bookID, err)
		}
		ed, err = q.CreateEdition(ctx, db.CreateEditionParams{
			BookID:          bookID,
			Title:           p.Title,
			Language:        p.Language,
			Translator:      p.Translator,
			Publisher:       p.Publisher,
			PublicationYear: p.PublicationYear,
			Isbn:            p.Isbn,
			PageCount:       p.PageCount,
			CoverUrl:        p.CoverURL,
			IsPrimary:       !hasPrimary,
		})
		if err != nil {
			return fmt.Errorf("baskı eklenemedi (kitap=%d): %w", bookID, err)
		}
		return audit.Write(ctx, q, actorID, ActionCreate, audit.EntityBook, bookID,
			map[string]any{"edition_id": ed.ID, "is_primary": ed.IsPrimary})
	})
	s.publicChanged(ctx, err)
	return ed, err
}

// Update, baskıyı kısmen günceller (nil alan = dokunma). Gönderilen alanların ADLARI denetim
// kaydına yazılır (değerleri değil).
func (s *Service) Update(ctx context.Context, actorID, editionID int64, p db.UpdateEditionParams) (db.Edition, error) {
	p.ID = editionID
	var ed db.Edition
	err := s.withLockedEdition(ctx, editionID, func(q *db.Queries, cur db.Edition) error {
		var err error
		ed, err = q.UpdateEdition(ctx, p)
		if err != nil {
			return fmt.Errorf("baskı güncellenemedi (id=%d): %w", editionID, err)
		}
		return audit.Write(ctx, q, actorID, ActionUpdate, audit.EntityBook, cur.BookID,
			map[string]any{"edition_id": editionID, "fields": changedFields(p)})
	})
	s.publicChanged(ctx, err)
	return ed, err
}

// Delete, baskıyı siler. Varsayılan baskı silinemez (ErrPrimaryEditionDelete): kitabın her
// zaman bir varsayılan baskısı olmalı (kapak ve sayfa sayısı ondan gelir).
func (s *Service) Delete(ctx context.Context, actorID, editionID int64) error {
	err := s.withLockedEdition(ctx, editionID, func(q *db.Queries, cur db.Edition) error {
		if cur.IsPrimary {
			return ErrPrimaryEditionDelete
		}
		if err := q.DeleteEdition(ctx, editionID); err != nil {
			return fmt.Errorf("baskı silinemedi (id=%d): %w", editionID, err)
		}
		return audit.Write(ctx, q, actorID, ActionDelete, audit.EntityBook, cur.BookID,
			map[string]any{"edition_id": editionID})
	})
	s.publicChanged(ctx, err)
	return err
}

// SetPrimary, baskıyı kitabın varsayılanı yapar. Zaten varsayılansa hiçbir şey yazılmaz.
// Sıra: önce eski varsayılan temizlenir, sonra yenisi atanır. Tersi bir an için iki varsayılan
// demektir ve kısmi benzersiz index (editions_one_primary_idx) ikinci adımı reddeder.
func (s *Service) SetPrimary(ctx context.Context, actorID, editionID int64) (db.Edition, error) {
	var ed db.Edition
	err := s.withLockedEdition(ctx, editionID, func(q *db.Queries, cur db.Edition) error {
		if cur.IsPrimary {
			ed = cur
			return nil
		}
		if err := q.ClearPrimaryEdition(ctx, cur.BookID); err != nil {
			return fmt.Errorf("eski varsayılan baskı temizlenemedi (kitap=%d): %w", cur.BookID, err)
		}
		if err := q.MarkPrimaryEdition(ctx, editionID); err != nil {
			return fmt.Errorf("varsayılan baskı atanamadı (id=%d): %w", editionID, err)
		}
		var err error
		if ed, err = q.GetEdition(ctx, editionID); err != nil {
			return fmt.Errorf("baskı okunamadı (id=%d): %w", editionID, err)
		}
		return audit.Write(ctx, q, actorID, ActionSetPrimary, audit.EntityBook, cur.BookID,
			map[string]any{"edition_id": editionID})
	})
	s.publicChanged(ctx, err)
	return ed, err
}

// withLockedEdition, baskının kitabını kilitler ve baskıyı KİLİT ALTINDA yeniden okuyup fn'e
// verir, hepsi tek transaction'da. İlk okuma yalnız kitabı bulmak için: kilit alınana kadar
// başka bir istek varsayılanı değiştirmiş olabilir, kararlar ikinci okumaya göre verilir.
func (s *Service) withLockedEdition(ctx context.Context, editionID int64, fn func(q *db.Queries, cur db.Edition) error) error {
	return database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		first, err := getEdition(ctx, q, editionID)
		if err != nil {
			return err
		}
		if err := lockBook(ctx, q, first.BookID); err != nil {
			return err
		}
		cur, err := getEdition(ctx, q, editionID) // kilitten sonra: silinmiş ya da değişmiş olabilir
		if err != nil {
			return err
		}
		return fn(q, cur)
	})
}

func getEdition(ctx context.Context, q *db.Queries, id int64) (db.Edition, error) {
	ed, err := q.GetEdition(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Edition{}, ErrNotFound
	}
	if err != nil {
		return db.Edition{}, fmt.Errorf("baskı okunamadı (id=%d): %w", id, err)
	}
	return ed, nil
}

// lockBook, kitap satırını kilitler: aynı kitabın baskılarına yapılan yazımlar sıraya girer
// (moderasyon geçişleriyle aynı kilit).
func lockBook(ctx context.Context, q *db.Queries, bookID int64) error {
	_, err := q.LockBookForModeration(ctx, bookID)
	if errors.Is(err, pgx.ErrNoRows) {
		return books.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("kitap kilitlenemedi (id=%d): %w", bookID, err)
	}
	return nil
}

// changedFields, PATCH'te gönderilen alanların adları.
func changedFields(p db.UpdateEditionParams) []string {
	var f []string
	add := func(name string, set bool) {
		if set {
			f = append(f, name)
		}
	}
	add("title", p.Title != nil)
	add("language", p.Language != nil)
	add("translator", p.Translator != nil)
	add("publisher", p.Publisher != nil)
	add("publication_year", p.PublicationYear != nil)
	add("isbn", p.Isbn != nil)
	add("page_count", p.PageCount != nil)
	add("cover_url", p.CoverUrl != nil)
	return f
}
