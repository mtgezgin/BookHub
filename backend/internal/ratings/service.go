// Package ratings, kitap puanlarını ve türetilmiş istatistiklerini (ortalama, sayı) yönetir.
package ratings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// Puan aralığı — DB'deki CHECK (score BETWEEN 1 AND 10) ile aynı.
const (
	MinScore = 1
	MaxScore = 10
)

var (
	// ErrInvalidScore, 1-10 dışındaki puan.
	ErrInvalidScore = fmt.Errorf("puan %d-%d arasında olmalı", MinScore, MaxScore)
	// ErrNotRated, kullanıcının bu kitaba puanı yokken okuma/silme.
	ErrNotRated = errors.New("bu kitaba henüz puan vermedin")
)

// Stats, kitabın türetilmiş puan istatistikleri. Average nil = henüz puan yok.
type Stats struct {
	Average *float64
	Count   int32
}

// Result, bir puanlama işleminin çıktısı. Created: ilk kez mi verildi (201) yoksa
// güncellendi mi (200).
type Result struct {
	Score   int16
	Created bool
	Stats   Stats
}

// Summary, public özet: istatistik + 1..10 dağılımı (Distribution[i] = i+1 puan sayısı).
type Summary struct {
	Stats
	Distribution [MaxScore]int32
}

type Service struct {
	conn db.DBTX
	q    *db.Queries
}

func NewService(conn db.DBTX) *Service {
	return &Service{conn: conn, q: db.New(conn)}
}

// Rate, kullanıcının kitaba puanını verir ya da günceller ve istatistikleri yeniler.
//
// SAHİPLİK (IDOR): userID her zaman access token'dan gelir, URL'den/gövdeden ASLA. Bu
// yüzden "başkasının puanını değiştirmek" bir yetki kontrolüyle değil, ifade edilemez
// olmasıyla engelleniyor — isteğin taşıyabileceği tek kimlik, çağıranın kendisi.
//
// Adımlar TEK transaction'da:
//  1. kitabı FOR UPDATE kilitle (ve onaylı olduğunu doğrula)
//  2. INSERT ... ON CONFLICT DO UPDATE (bir kez puan + güncelleme, tek deyim)
//  3. AVG/COUNT'u ratings tablosundan yeniden türet ve books'a yaz
//
// Kilit neden: bkz. sql/queries/ratings.sql LockApprovedBook ve eşzamanlılık testi.
func (s *Service) Rate(ctx context.Context, userID, bookID int64, score int) (Result, error) {
	if score < MinScore || score > MaxScore {
		return Result{}, ErrInvalidScore
	}
	var res Result
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := lockApproved(ctx, q, bookID); err != nil {
			return err
		}
		row, err := q.UpsertRating(ctx, db.UpsertRatingParams{UserID: userID, BookID: bookID, Score: int16(score)})
		if err != nil {
			return fmt.Errorf("puan yazılamadı: %w", err)
		}
		stats, err := refresh(ctx, q, bookID)
		if err != nil {
			return err
		}
		res = Result{Score: row.Score, Created: row.Inserted, Stats: stats}
		return nil
	})
	return res, err
}

// Remove, kullanıcının puanını siler ve istatistikleri yeniler (aynı kilit + tx deseni).
func (s *Service) Remove(ctx context.Context, userID, bookID int64) (Stats, error) {
	var stats Stats
	err := database.WithTx(ctx, s.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := lockApproved(ctx, q, bookID); err != nil {
			return err
		}
		n, err := q.DeleteRating(ctx, db.DeleteRatingParams{UserID: userID, BookID: bookID})
		if err != nil {
			return fmt.Errorf("puan silinemedi: %w", err)
		}
		if n == 0 {
			return ErrNotRated
		}
		stats, err = refresh(ctx, q, bookID)
		return err
	})
	return stats, err
}

// Mine, kullanıcının bu kitaba verdiği puan.
func (s *Service) Mine(ctx context.Context, userID, bookID int64) (db.GetUserRatingRow, error) {
	r, err := s.q.GetUserRating(ctx, db.GetUserRatingParams{UserID: userID, BookID: bookID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.GetUserRatingRow{}, ErrNotRated
	}
	if err != nil {
		return db.GetUserRatingRow{}, fmt.Errorf("puan okunamadı: %w", err)
	}
	return r, nil
}

// Summary, onaylı kitabın puan özetini döner (onaysız kitap = ErrNotFound, public kural).
func (s *Service) Summary(ctx context.Context, bookID int64) (Summary, error) {
	b, err := books.NewRepository(s.conn).Get(ctx, bookID)
	if err != nil {
		return Summary{}, err
	}
	rows, err := s.q.RatingDistribution(ctx, bookID)
	if err != nil {
		return Summary{}, fmt.Errorf("puan dağılımı okunamadı: %w", err)
	}
	sum := Summary{Stats: Stats{Average: b.AverageRating, Count: b.RatingCount}}
	for _, r := range rows {
		sum.Distribution[r.Score-MinScore] = r.Votes
	}
	return sum, nil
}

func lockApproved(ctx context.Context, q *db.Queries, bookID int64) error {
	_, err := q.LockApprovedBook(ctx, bookID)
	if errors.Is(err, pgx.ErrNoRows) {
		return books.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("kitap kilitlenemedi (id=%d): %w", bookID, err)
	}
	return nil
}

func refresh(ctx context.Context, q *db.Queries, bookID int64) (Stats, error) {
	row, err := q.RefreshBookRatingStats(ctx, bookID)
	if err != nil {
		return Stats{}, fmt.Errorf("puan istatistiği yenilenemedi: %w", err)
	}
	return Stats{Average: row.AverageRating, Count: row.RatingCount}, nil
}
