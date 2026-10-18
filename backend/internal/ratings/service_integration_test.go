package ratings

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

func TestRateLifecycleRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool, DefaultMinVotes)
	ctx := context.Background()
	book := testdb.CreateBook(t, pool, "", books.StatusApproved)
	u1, u2 := testdb.CreateUser(t, pool, "Okur"), testdb.CreateUser(t, pool, "Okur")

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
	user := testdb.CreateUser(t, pool, "Okur")

	for _, bad := range []int{0, 11, -3} {
		if _, err := s.Rate(ctx, user, testdb.CreateBook(t, pool, "", books.StatusApproved), bad); !errors.Is(err, ErrInvalidScore) {
			t.Errorf("puan %d reddedilmeli: %v", bad, err)
		}
	}
	// Onaylı olmayan kitaba puan verilemez (varlığı da sızmaz: "yok").
	if _, err := s.Rate(ctx, user, testdb.CreateBook(t, pool, "", books.StatusPending), 7); !errors.Is(err, books.ErrNotFound) {
		t.Errorf("PENDING kitap ErrNotFound vermeli: %v", err)
	}
	if _, err := s.Mine(ctx, user, testdb.CreateBook(t, pool, "", books.StatusApproved)); !errors.Is(err, ErrNotRated) {
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
	book := testdb.CreateBook(t, pool, "", books.StatusApproved)

	const n = 25
	users := make([]int64, n)
	for i := range users {
		users[i] = testdb.CreateUser(t, pool, "Okur")
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
			if _, err := s.Rate(ctx, testdb.CreateUser(t, pool, "Okur"), book, score); err != nil {
				t.Fatalf("Rate: %v", err)
			}
		}
	}
	lucky := testdb.CreateBook(t, pool, "", books.StatusApproved)  // 1 oy, 10
	steady := testdb.CreateBook(t, pool, "", books.StatusApproved) // 30 oy, 9
	rateN(lucky, 1, 10)
	rateN(steady, 30, 9)
	rateN(testdb.CreateBook(t, pool, "", books.StatusApproved), 40, 5) // arka plan: C'yi gerçekçi seviyeye çeker

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

// TestCachedInvalidationRealDB: gerçek DB + sahte cache — başarılı puan yazımı cache'i siler,
// başarısız (onaysız kitap) yazım silmez.
func TestCachedInvalidationRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	c := newMemCache()
	s := NewCached(NewService(pool, DefaultMinVotes), c)
	ctx := context.Background()
	user := testdb.CreateUser(t, pool, "Okur")

	if _, err := s.TopRated(ctx, 5); err != nil {
		t.Fatalf("TopRated: %v", err)
	}
	if _, ok := c.data[TopRatedCacheKey]; !ok {
		t.Fatal("ilk okuma cache'e yazmalı")
	}

	if _, err := s.Rate(ctx, user, testdb.CreateBook(t, pool, "", books.StatusPending), 7); err == nil {
		t.Fatal("onaysız kitaba puan reddedilmeli")
	}
	if _, ok := c.data[TopRatedCacheKey]; !ok {
		t.Error("BAŞARISIZ yazım cache'i silmemeli")
	}

	if _, err := s.Rate(ctx, user, testdb.CreateBook(t, pool, "", books.StatusApproved), 7); err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if _, ok := c.data[TopRatedCacheKey]; ok {
		t.Error("başarılı puan yazımı top-rated cache'ini silmeli")
	}
}

// TestUserDeletionRefreshesStatsRealDB (review bulgusu F4): kullanıcı silinince puanları
// CASCADE ile gider; kitabın istatistikleri de (trigger ile) yeniden hesaplanmalı —
// "sayım 2, satır 0" gibi hayalet oylar kalmamalı.
func TestUserDeletionRefreshesStatsRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool, DefaultMinVotes)
	ctx := context.Background()
	book := testdb.CreateBook(t, pool, "", books.StatusApproved)
	stays, leaves := testdb.CreateUser(t, pool, "Okur"), testdb.CreateUser(t, pool, "Okur")

	if _, err := s.Rate(ctx, stays, book, 8); err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if _, err := s.Rate(ctx, leaves, book, 2); err != nil {
		t.Fatalf("Rate: %v", err)
	}
	testdb.Exec(t, pool, "DELETE FROM users WHERE id = $1", leaves)

	sum, err := s.Summary(ctx, book)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.Count != 1 || sum.Average == nil || *sum.Average != 8 {
		t.Errorf("silinen kullanıcının oyu istatistikten düşmeli: count=%d avg=%v", sum.Count, sum.Average)
	}

	testdb.Exec(t, pool, "DELETE FROM users WHERE id = $1", stays)
	sum, err = s.Summary(ctx, book)
	if err != nil || sum.Count != 0 || sum.Average != nil {
		t.Errorf("son oy da gidince count=0, ortalama NULL olmalı: %+v %v", sum.Stats, err)
	}
}

// TestDeleteTriggerWaitsForBookLockRealDB (review bulgusu R3): bir puan yazımı kitabı
// FOR UPDATE ile tutarken başka bir kullanıcı silinirse, trigger agregatı kilit ALINDIKTAN
// sonra hesaplamalı. Kilitsiz trigger, bekleyen UPDATE'i ESKİ agregatla uygular ve az önce
// yazılan oy istatistikten düşer (Faz 5'teki kayıp güncelleme).
func TestDeleteTriggerWaitsForBookLockRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool, DefaultMinVotes)
	ctx := context.Background()
	book := testdb.CreateBook(t, pool, "", books.StatusApproved)
	rater, leaver := testdb.CreateUser(t, pool, "Okur"), testdb.CreateUser(t, pool, "Okur")
	if _, err := s.Rate(ctx, leaver, book, 5); err != nil {
		t.Fatalf("Rate: %v", err)
	}

	// Rate'in yaptığını elle, adım adım: kitabı kilitle ...
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT id FROM books WHERE id = $1 FOR UPDATE`, book); err != nil {
		t.Fatalf("kilit: %v", err)
	}

	// ... bu sırada başka bir oturum puan vermiş kullanıcıyı siler (trigger kilidi bekler) ...
	done := make(chan error, 1)
	go func() {
		_, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, leaver)
		done <- err
	}()
	time.Sleep(300 * time.Millisecond) // silme trigger'ı kitap kilidinde beklesin

	// ... kilit sahibi kendi oyunu yazar, istatistiği yeniler ve commit eder.
	q := db.New(tx)
	if _, err := q.UpsertRating(ctx, db.UpsertRatingParams{UserID: rater, BookID: book, Score: 9}); err != nil {
		t.Fatalf("UpsertRating: %v", err)
	}
	if _, err := q.RefreshBookRatingStats(ctx, book); err != nil {
		t.Fatalf("RefreshBookRatingStats: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("DELETE: %v", err)
	}

	sum, err := s.Summary(ctx, book)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.Count != 1 || sum.Average == nil || *sum.Average != 9 {
		t.Errorf("kalan tek oy (9) sayılmalı: count=%d avg=%v", sum.Count, sum.Average)
	}
}

// TestCachedRemoveInvalidationRealDB: başarılı puan silme top-rated cache'ini siler;
// başarısız silme (ErrNotRated) cache'e dokunmaz.
func TestCachedRemoveInvalidationRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	c := newMemCache()
	s := NewCached(NewService(pool, DefaultMinVotes), c)
	ctx := context.Background()
	user := testdb.CreateUser(t, pool, "Okur")
	book := testdb.CreateBook(t, pool, "", books.StatusApproved)

	if _, err := s.Rate(ctx, user, book, 7); err != nil {
		t.Fatalf("Rate: %v", err)
	}
	c.data[TopRatedCacheKey] = []byte("{}")
	if _, err := s.Remove(ctx, user, book); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := c.data[TopRatedCacheKey]; ok {
		t.Error("başarılı puan silme top-rated cache'ini silmeli")
	}

	c.data[TopRatedCacheKey] = []byte("{}")
	if _, err := s.Remove(ctx, user, book); !errors.Is(err, ErrNotRated) {
		t.Fatalf("ikinci silme ErrNotRated vermeli: %v", err)
	}
	if _, ok := c.data[TopRatedCacheKey]; !ok {
		t.Error("BAŞARISIZ silme cache'i silmemeli")
	}
}

// TestNonApprovedBookRealDB: onaylı olmayan (ya da olmayan) kitapta puan silme ve özet
// books.ErrNotFound vermeli — public görünürlük kuralı.
func TestNonApprovedBookRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	s := NewService(pool, DefaultMinVotes)
	ctx := context.Background()
	user := testdb.CreateUser(t, pool, "Okur")

	cases := map[string]int64{
		"PENDING":  testdb.CreateBook(t, pool, "", books.StatusPending),
		"ARCHIVED": testdb.CreateBook(t, pool, "", books.StatusArchived),
		"yok":      math.MaxInt64,
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Remove(ctx, user, id); !errors.Is(err, books.ErrNotFound) {
				t.Errorf("Remove ErrNotFound vermeli: %v", err)
			}
			if name == "yok" {
				return
			}
			if _, err := s.Summary(ctx, id); !errors.Is(err, books.ErrNotFound) {
				t.Errorf("Summary ErrNotFound vermeli: %v", err)
			}
		})
	}
}

// TestListBooksByRatingMatchesWeightedScoreRealDB: katalogdaki "en yüksek puan" sırası (books.sql
// ListBooksByRating) ile spesifikasyon olan WeightedScore AYNI sırayı vermeli. C dışarıdan verilir;
// bu iki kitabın sırası C ≈ 6,33'te yer değiştirir. Formül SQL'de ya da Go'da tek başına değişirse
// iki C değerinden en az birinde sıra tutmaz.
func TestListBooksByRatingMatchesWeightedScoreRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	token := testdb.Unique("agirlik")
	lucky := testdb.CreateBook(t, pool, "", books.StatusApproved)  // 1 oy, ortalama 10
	steady := testdb.CreateBook(t, pool, "", books.StatusApproved) // 10 oy, ortalama 7
	for _, r := range []struct {
		id    int64
		count int
		avg   float64
	}{{lucky, 1, 10}, {steady, 10, 7}} {
		testdb.Exec(t, pool, "UPDATE books SET title = $1, rating_count = $2, average_rating = $3 WHERE id = $4",
			"Kitap "+token, r.count, r.avg, r.id)
	}

	const m = 10
	q := db.New(pool)
	for _, c := range []float64{5, 8} {
		got, err := q.ListBooksByRating(context.Background(), db.ListBooksByRatingParams{Query: &token, M: m, C: c, Limit: 10})
		if err != nil {
			t.Fatalf("C=%v: liste okunamadı: %v", c, err)
		}
		if len(got) != 2 {
			t.Fatalf("C=%v: 2 kitap bekleniyordu, %d geldi", c, len(got))
		}
		want := lucky
		if WeightedScore(10, 1, m, c) < WeightedScore(7, 10, m, c) {
			want = steady
		}
		if got[0].ID != want {
			t.Errorf("C=%v: ilk sırada %d bekleniyordu (WeightedScore'a göre), SQL %d verdi", c, want, got[0].ID)
		}
	}
}
