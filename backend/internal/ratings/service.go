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

// ColdStartMean, hiç oy yokken genel ortalama C yerine kullanılan değer (1-10 ölçeğinin ortası).
const ColdStartMean = 5.5

// DefaultMinVotes, ağırlıklı puandaki m'nin varsayılanı. KÜÇÜK başlıyor (brief'teki 100 değil):
// az veride m=100, her kitabın skorunu C'ye çökertir ve sıralama anlamsızlaşır.
const DefaultMinVotes = 10

type Service struct {
	conn     db.DBTX
	q        *db.Queries
	minVotes int64
}

// NewService, m (minVotes) ayarıyla bir Service kurar; negatif m 0 sayılır.
func NewService(conn db.DBTX, minVotes int64) *Service {
	return &Service{conn: conn, q: db.New(conn), minVotes: max(minVotes, 0)}
}

// TopRatedBook, sıralamadaki bir kitap ve ağırlıklı skoru.
type TopRatedBook struct {
	Book          db.BookView
	WeightedScore float64
}

// TopRated, sıralama + onu üreten parametreler (şeffaflık: istemci m ve C'yi görebilir).
type TopRated struct {
	Books []TopRatedBook
	M     int64
	C     float64
}

// TopRated, onaylı ve en az bir oyu olan kitapları AĞIRLIKLI skora göre sıralar.
// Skor query-time hesaplanır (bkz. sql/queries/ratings.sql ListTopRated).
//
// C ve liste iki ayrı sorgu: arada gelen bir oy C'yi çok az kaydırabilir. Sıralama için
// önemsiz; kesinlik gerekseydi ikisi tek REPEATABLE READ transaction'da okunurdu.
func (s *Service) TopRated(ctx context.Context, limit int32) (TopRated, error) {
	c, err := s.q.GlobalAverageRating(ctx, ColdStartMean)
	if err != nil {
		return TopRated{}, fmt.Errorf("genel ortalama okunamadı: %w", err)
	}
	rows, err := s.q.ListTopRated(ctx, db.ListTopRatedParams{M: float64(s.minVotes), C: c, Limit: limit})
	if err != nil {
		return TopRated{}, fmt.Errorf("sıralama okunamadı: %w", err)
	}
	out := TopRated{M: s.minVotes, C: c, Books: make([]TopRatedBook, 0, len(rows))}
	for _, r := range rows {
		// sqlc.embed(b): satır kitabın tamamını db.BookView olarak taşır; alan alan elle kopya
		// (ve onu koruyan reflection testi, review bulgusu R5) Faz 16'da gereksiz kaldı.
		out.Books = append(out.Books, TopRatedBook{Book: r.BookView, WeightedScore: r.WeightedScore})
	}
	return out, nil
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
