// Package comments, kitap yorumlarını yönetir: yazma (giriş yapmış kullanıcı), okuma
// (herkes, yalnız görünürler) ve yumuşak silme (yalnız admin, denetim kaydıyla).
package comments

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

// MaxContentLen, yorumun karakter sınırı — DB'deki CHECK (char_length(content) <= 2000).
const MaxContentLen = 2000

// ErrNotFound, silinecek (görünür) yorum yok.
var ErrNotFound = errors.New("yorum bulunamadı")

type Service struct {
	conn db.DBTX
	q    *db.Queries
}

func NewService(conn db.DBTX) *Service {
	return &Service{conn: conn, q: db.New(conn)}
}

// Create, onaylı kitaba yorum ekler. Kitap yoksa ya da onaylı değilse books.ErrNotFound
// (onaysız kitabın varlığını sızdırmamak için "yok").
//
// XSS NOTU: içerik HAM saklanır. Kaçışlama (escape) gösterim anında yapılır — React bunu
// varsayılan olarak yapar. Sunucuda HTML'e çevirip saklamak, veriyi tek bir çıktı
// biçimine kilitlerdi ve çift kaçışlama hatalarına kapı açardı.
func (s *Service) Create(ctx context.Context, userID, bookID int64, content string) (db.CreateCommentRow, error) {
	c, err := s.q.CreateComment(ctx, db.CreateCommentParams{BookID: bookID, UserID: &userID, Content: content})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.CreateCommentRow{}, books.ErrNotFound
	}
	if err != nil {
		return db.CreateCommentRow{}, fmt.Errorf("yorum yazılamadı: %w", err)
	}
	return c, nil
}

// List, kitabın görünür yorumlarını (yazar adıyla) sayfalı döner.
func (s *Service) List(ctx context.Context, bookID int64, limit, offset int32) ([]db.ListCommentsRow, int64, error) {
	rows, err := s.q.ListComments(ctx, db.ListCommentsParams{BookID: bookID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("yorumlar okunamadı: %w", err)
	}
	total, err := s.q.CountComments(ctx, bookID)
	if err != nil {
		return nil, 0, fmt.Errorf("yorumlar sayılamadı: %w", err)
	}
	return rows, total, nil
}

// Delete, yorumu yumuşak siler (admin) ve denetim kaydını aynı transaction'da yazar.
func (s *Service) Delete(ctx context.Context, adminID, commentID int64) error {
	return database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		row, err := q.SoftDeleteComment(ctx, db.SoftDeleteCommentParams{ID: commentID, DeletedBy: &adminID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("yorum silinemedi: %w", err)
		}
		return audit.Write(ctx, q, adminID, "comment.delete", audit.EntityComment, row.ID, map[string]any{"book_id": row.BookID})
	})
}
