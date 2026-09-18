package moderation

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

// Service, kitabın yaşam döngüsündeki YAZMA işlemlerini ve admin okumalarını yönetir.
//
// Neden servis (Faz 2'de kitap CRUD'unda yoktu): artık her yazımın iki kuralı var —
// (1) statü geçişi durum makinesine uymalı, (2) her değişiklik denetim kaydıyla AYNI
// transaction'da yazılmalı. Bu kurallar ne HTTP'nin ne tek bir SQL sorgusunun işi.
type Service struct {
	conn db.DBTX
	q    *db.Queries
}

func NewService(conn db.DBTX) *Service {
	return &Service{conn: conn, q: db.New(conn)}
}

// Suggest, kullanıcının önerisini PENDING olarak kaydeder. Statü ve öneren BURADA
// belirlenir; istemci ikisini de seçemez.
func (s *Service) Suggest(ctx context.Context, userID int64, p db.CreateBookParams) (db.Book, error) {
	p.Status = books.StatusPending
	p.SubmittedBy = &userID
	return s.create(ctx, userID, "book.suggest", p)
}

// CreateApproved, admin'in doğrudan eklediği kitabı yayında (APPROVED) başlatır.
func (s *Service) CreateApproved(ctx context.Context, actorID int64, p db.CreateBookParams) (db.Book, error) {
	p.Status = books.StatusApproved
	p.SubmittedBy = &actorID
	return s.create(ctx, actorID, "book.create", p)
}

func (s *Service) create(ctx context.Context, actorID int64, action string, p db.CreateBookParams) (db.Book, error) {
	var book db.Book
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		// books.Repository'yi tx ÜZERİNDE kuruyoruz: slug çakışması → ErrDuplicateSlug
		// çevirisi tek yerde (repository'de) kalıyor, burada tekrar yazılmıyor.
		b, err := books.NewRepository(tx).Create(ctx, p)
		if err != nil {
			return err
		}
		book = b
		return audit.Write(ctx, db.New(tx), actorID, action, audit.EntityBook, b.ID, map[string]any{"status": b.Status})
	})
	return book, err
}

// Approve / Reject / Archive / Restore: durum makinesindeki geçişler.
func (s *Service) Approve(ctx context.Context, actorID, bookID int64) (db.Book, error) {
	return s.transition(ctx, actorID, bookID, ActionApprove, nil)
}

// Reject, opsiyonel bir gerekçeyle reddeder (boş gerekçe = NULL).
func (s *Service) Reject(ctx context.Context, actorID, bookID int64, reason string) (db.Book, error) {
	var r *string
	if reason != "" {
		r = &reason
	}
	return s.transition(ctx, actorID, bookID, ActionReject, r)
}

func (s *Service) Archive(ctx context.Context, actorID, bookID int64) (db.Book, error) {
	return s.transition(ctx, actorID, bookID, ActionArchive, nil)
}

func (s *Service) Restore(ctx context.Context, actorID, bookID int64) (db.Book, error) {
	return s.transition(ctx, actorID, bookID, ActionRestore, nil)
}

// transition, bir geçişi uygular: KİLİTLE → kuralı denetle → güncelle → denetim kaydı.
//
// FOR UPDATE neden şart: iki admin aynı PENDING kitabı aynı anda açtı; biri onaylıyor,
// diğeri reddediyor. Kilitsiz, ikisi de "PENDING" okur, ikisi de geçişi geçerli bulur,
// son yazan kazanır ve audit'te BİRBİRİYLE ÇELİŞEN iki karar kalır. Kilitle, ikinci
// admin birincinin commit'ini bekler, sonra "APPROVED" görür ve reddi 409 alır.
func (s *Service) transition(ctx context.Context, actorID, bookID int64, a Action, reason *string) (db.Book, error) {
	var book db.Book
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockBookForModeration(ctx, bookID)
		if errors.Is(err, pgx.ErrNoRows) {
			return books.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("kitap kilitlenemedi (id=%d): %w", bookID, err)
		}

		next, err := Next(cur.Status, a)
		if err != nil {
			return err
		}

		updated, err := q.SetBookStatus(ctx, db.SetBookStatusParams{
			ID:              bookID,
			Status:          next,
			RejectionReason: reason,
			ModeratedBy:     &actorID,
		})
		if err != nil {
			return fmt.Errorf("statü güncellenemedi (id=%d): %w", bookID, err)
		}
		book = updated

		details := map[string]any{"from": cur.Status, "to": next}
		if reason != nil {
			details["reason"] = *reason
		}
		return audit.Write(ctx, q, actorID, "book."+string(a), audit.EntityBook, bookID, details)
	})
	return book, err
}

// Update, kitabın bilgilerini kısmen günceller (nil alan = dokunma) ve hangi alanların
// gönderildiğini denetim kaydına yazar.
func (s *Service) Update(ctx context.Context, actorID, bookID int64, p db.UpdateBookDetailsParams) (db.Book, error) {
	p.ID = bookID
	var book db.Book
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		b, err := q.UpdateBookDetails(ctx, p)
		if errors.Is(err, pgx.ErrNoRows) {
			return books.ErrNotFound
		}
		if _, ok := database.UniqueViolation(err); ok {
			return books.ErrDuplicateSlug // books tablosundaki tek UNIQUE (id dışında) slug
		}
		if err != nil {
			return fmt.Errorf("kitap güncellenemedi (id=%d): %w", bookID, err)
		}
		book = b
		return audit.Write(ctx, q, actorID, "book.update", audit.EntityBook, bookID, map[string]any{"fields": changedFields(p)})
	})
	return book, err
}

// changedFields, PATCH'te GÖNDERİLEN alanların adları (değerleri değil: audit kaydı
// "ne değişti"yi söylesin, uzun açıklama metinlerini kopyalamasın).
func changedFields(p db.UpdateBookDetailsParams) []string {
	var f []string
	add := func(name string, set bool) {
		if set {
			f = append(f, name)
		}
	}
	add("title", p.Title != nil)
	add("slug", p.Slug != nil)
	add("author", p.Author != nil)
	add("description", p.Description != nil)
	add("cover_url", p.CoverUrl != nil)
	add("publication_year", p.PublicationYear != nil)
	add("page_count", p.PageCount != nil)
	add("language", p.Language != nil)
	return f
}

// Get, admin okuması: statüden bağımsız.
func (s *Service) Get(ctx context.Context, bookID int64) (db.Book, error) {
	b, err := s.q.GetBook(ctx, bookID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Book{}, books.ErrNotFound
	}
	if err != nil {
		return db.Book{}, fmt.Errorf("kitap okunamadı (id=%d): %w", bookID, err)
	}
	return b, nil
}

// List, tek statüdeki kitapları sayfalı döner (admin kuyruğu).
func (s *Service) List(ctx context.Context, status string, limit, offset int32) ([]db.Book, int64, error) {
	list, err := s.q.ListBooksByStatus(ctx, db.ListBooksByStatusParams{Status: status, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("kitaplar listelenemedi: %w", err)
	}
	total, err := s.q.CountBooksByStatus(ctx, status)
	if err != nil {
		return nil, 0, fmt.Errorf("kitaplar sayılamadı: %w", err)
	}
	return list, total, nil
}

// History, kitabın denetim kaydı (en yeniden eskiye). Kitap yoksa ErrNotFound.
func (s *Service) History(ctx context.Context, bookID int64) ([]db.AuditLog, error) {
	if _, err := s.Get(ctx, bookID); err != nil {
		return nil, err
	}
	logs, err := s.q.ListAuditLogs(ctx, db.ListAuditLogsParams{EntityType: audit.EntityBook, EntityID: bookID})
	if err != nil {
		return nil, fmt.Errorf("denetim kaydı okunamadı (id=%d): %w", bookID, err)
	}
	return logs, nil
}
