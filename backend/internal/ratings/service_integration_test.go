package ratings

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

func createUser(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, display_name, password_hash) VALUES ($1, 'Okur', 'x') RETURNING id`,
		testdb.Unique("rater")+"@example.com").Scan(&id)
	if err != nil {
		t.Fatalf("kullanıcı açılamadı: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM users WHERE id = $1", id) })
	return id
}

func createBook(t *testing.T, pool *pgxpool.Pool, status string) int64 {
	t.Helper()
	slug := testdb.Unique("rated")
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO books (title, slug, author, status) VALUES ($1, $1, 'Yazar', $2) RETURNING id`, slug, status).Scan(&id)
	if err != nil {
		t.Fatalf("kitap açılamadı: %v", err)
	}
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM books WHERE id = $1", id) })
	return id
}

func TestRateLifecycleRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool, DefaultMinVotes)
	ctx := context.Background()
	book := createBook(t, pool, books.StatusApproved)
	u1, u2 := createUser(t, pool), createUser(t, pool)

	r, err := s.Rate(ctx, u1, book, 8)
	if err != nil || !r.Created || r.Stats.Count != 1 || *r.Stats.Average != 8 {
		t.Fatalf("ilk puan: %+v %v", r, err)
	}
	r, err = s.Rate(ctx, u2, book, 5)
	if err != nil || r.Stats.Count != 2 || *r.Stats.Average != 6.5 {
		t.Fatalf("ikinci kullanıcı: %+v %v", r, err)
	}

	// Aynı kullanıcı tekrar → GÜNCELLEME (ikinci satır değil): sayı 2 kalır.
	r, err = s.Rate(ctx, u1, book, 10)
	if err != nil || r.Created || r.Stats.Count != 2 || *r.Stats.Average != 7.5 {
		t.Fatalf("güncelleme: %+v %v", r, err)
	}

	mine, err := s.Mine(ctx, u1, book)
	if err != nil || mine.Score != 10 {
		t.Errorf("Mine: %+v %v", mine, err)
	}

	sum, err := s.Summary(ctx, book)
	if err != nil || sum.Count != 2 || sum.Distribution[9] != 1 || sum.Distribution[4] != 1 {
		t.Errorf("özet/dağılım: %+v %v", sum, err)
	}

	stats, err := s.Remove(ctx, u1, book)
	if err != nil || stats.Count != 1 || *stats.Average != 5 {
		t.Errorf("silme sonrası: %+v %v", stats, err)
	}
	if _, err := s.Remove(ctx, u1, book); !errors.Is(err, ErrNotRated) {
		t.Errorf("ikinci silme ErrNotRated vermeli: %v", err)
	}
	stats, err = s.Remove(ctx, u2, book)
	if err != nil || stats.Count != 0 || stats.Average != nil {
		t.Errorf("son puan silinince ortalama NULL olmalı (0 değil): %+v %v", stats, err)
	}
}

func TestRateRulesRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool, DefaultMinVotes)
	ctx := context.Background()
	user := createUser(t, pool)

	for _, bad := range []int{0, 11, -3} {
		if _, err := s.Rate(ctx, user, createBook(t, pool, books.StatusApproved), bad); !errors.Is(err, ErrInvalidScore) {
			t.Errorf("puan %d reddedilmeli: %v", bad, err)
		}
	}
	// Onaylı olmayan kitaba puan verilemez (varlığı da sızmaz: "yok").
	if _, err := s.Rate(ctx, user, createBook(t, pool, books.StatusPending), 7); !errors.Is(err, books.ErrNotFound) {
		t.Errorf("PENDING kitap ErrNotFound vermeli: %v", err)
	}
	if _, err := s.Mine(ctx, user, createBook(t, pool, books.StatusApproved)); !errors.Is(err, ErrNotRated) {
		t.Errorf("puansız Mine ErrNotRated vermeli: %v", err)
	}
}

// TestConcurrentRatingsRealDB — projenin #1 doğruluk dersinin kanıtı. N kullanıcı AYNI
// kitaba AYNI ANDA puan veriyor. Sonunda rating_count tam N, ortalama tam doğru olmalı.
// Kitap satırı kilitlenmeseydi her işlem AVG/COUNT'u yalnız kendi (commit'lenmemiş) oyuyla
// hesaplar, sonra yazan öncekinin sayımını ezerdi (kayıp güncelleme).
func TestConcurrentRatingsRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool, DefaultMinVotes)
	ctx := context.Background()
	book := createBook(t, pool, books.StatusApproved)

	const n = 25
	users := make([]int64, n)
	for i := range users {
		users[i] = createUser(t, pool)
	}

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
		errs  = make(chan error, n)
		sum   int
	)
	for i, u := range users {
		score := i%10 + 1
		sum += score
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := s.Rate(ctx, u, book, score); err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("eşzamanlı puan hatası: %v", err)
	}

	got, err := s.Summary(ctx, book)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	want := math.Round(float64(sum)/n*100) / 100
	gotAvg := -1.0
	if got.Average != nil {
		gotAvg = *got.Average
	}
	if got.Count != n || gotAvg != want {
		t.Errorf("kayıp güncelleme: istenen count=%d avg=%.2f, gelen count=%d avg=%.2f", n, want, got.Count, gotAvg)
	}
}

// TestTopRatedRealDB: (1) SQL'in hesapladığı skor, spesifikasyon olan saf WeightedScore ile
// AYNI olmalı; (2) brief'in iddiası gerçekçi bir katalogda tutmalı: 1 oyla 10 alan kitap,
// 30 oyla 9 alan kitabın ÖNÜNE geçemez (basit ortalamada geçerdi).
//
// "GERÇEKÇİ KATALOG" NEDEN ŞART — testin ilk hâlinden öğrendiğimiz: yalnız bu iki kitap
// varken genel ortalama C ≈ 9.03 çıkıyordu (oyların 30'u 9'luk kitaptan). O zaman 9'luk kitap
// C'nin ALTINDA kalır ve formül onu ~9.01'de tutarken, 1 oylu 10'luk kitap ~9.12 alır —
// ve bu istatistiksel olarak DOĞRU: katalog ortalaması 9 olan bir dünyada "10 veren tek kişi"
// zayıf ama olumlu bir kanıttır. Brief'in iddiası C her iki kitabın da altındayken geçerli
// (brief'in örneğinde C=7). Bu yüzden 40 oyla 5 alan bir "arka plan" kitabı ekliyoruz: C ≈ 6.9.
func TestTopRatedRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool, DefaultMinVotes)
	ctx := context.Background()

	rateN := func(book int64, n, score int) {
		t.Helper()
		for range n {
			if _, err := s.Rate(ctx, createUser(t, pool), book, score); err != nil {
				t.Fatalf("Rate: %v", err)
			}
		}
	}
	lucky := createBook(t, pool, books.StatusApproved)  // 1 oy, 10
	steady := createBook(t, pool, books.StatusApproved) // 30 oy, 9
	rateN(lucky, 1, 10)
	rateN(steady, 30, 9)
	rateN(createBook(t, pool, books.StatusApproved), 40, 5) // arka plan: C'yi gerçekçi seviyeye çeker

	top, err := s.TopRated(ctx, 50)
	if err != nil {
		t.Fatalf("TopRated: %v", err)
	}
	pos := map[int64]int{}
	for i, b := range top.Books {
		pos[b.Book.ID] = i
		want := WeightedScore(*b.Book.AverageRating, int64(b.Book.RatingCount), top.M, top.C)
		if math.Abs(want-b.WeightedScore) > 1e-9 {
			t.Errorf("kitap %d: SQL skoru %.6f ≠ Go spesifikasyonu %.6f", b.Book.ID, b.WeightedScore, want)
		}
		if i > 0 && b.WeightedScore > top.Books[i-1].WeightedScore {
			t.Errorf("sıralama azalan değil: %d. sırada %.4f > %.4f", i, b.WeightedScore, top.Books[i-1].WeightedScore)
		}
	}
	pl, okL := pos[lucky]
	ps, okS := pos[steady]
	if !okL || !okS {
		t.Fatalf("iki kitap da listede olmalı (lucky=%v steady=%v)", okL, okS)
	}
	if top.C >= 9 {
		t.Fatalf("test kurgusu bozuk: C=%.2f, 9'un altında olmalı", top.C)
	}
	if ps > pl {
		t.Errorf("C=%.2f iken 30 oyla 9 (sıra %d), 1 oyla 10'un (sıra %d) önünde olmalı", top.C, ps, pl)
	}
}
