package ratings

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// memCache, cache.Cache'in bellek içi sahtesi; getErr ile Redis arızası taklit edilir.
type memCache struct {
	mu     sync.Mutex
	data   map[string][]byte
	getErr error
	sets   int
}

func newMemCache() *memCache { return &memCache{data: map[string][]byte{}} }

func (m *memCache) Get(_ context.Context, k string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return nil, false, m.getErr
	}
	v, ok := m.data[k]
	return v, ok, nil
}
func (m *memCache) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[k] = v
	m.sets++
	return nil
}
func (m *memCache) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

// cachedWithLoader: DB'siz CachedService — yükleyici her çağrıyı sayar.
func cachedWithLoader(c *memCache, loads *int) *CachedService {
	s := NewCached(&Service{}, c)
	s.load = func(_ context.Context, limit int32) (TopRated, error) {
		*loads++
		if limit != MaxTopRated {
			return TopRated{}, errors.New("cache her zaman en iyi 50'yi yüklemeli")
		}
		books := make([]TopRatedBook, 0, 30)
		for i := range 30 {
			books = append(books, TopRatedBook{Book: db.Book{ID: int64(i + 1)}, WeightedScore: float64(100 - i)})
		}
		return TopRated{Books: books, M: 10, C: 6.5}, nil
	}
	return s
}

func TestCachedTopRated(t *testing.T) {
	ctx := context.Background()
	c := newMemCache()
	loads := 0
	s := cachedWithLoader(c, &loads)

	first, err := s.TopRated(ctx, 10)
	if err != nil || loads != 1 || len(first.Books) != 10 {
		t.Fatalf("ilk çağrı DB'den yüklemeli ve 10'a dilimlemeli: loads=%d len=%d err=%v", loads, len(first.Books), err)
	}
	second, err := s.TopRated(ctx, 20)
	if err != nil || loads != 1 || len(second.Books) != 20 || second.C != 6.5 {
		t.Errorf("ikinci çağrı CACHE'ten gelmeli (farklı limitle bile): loads=%d len=%d", loads, len(second.Books))
	}

	s.InvalidateTopRated(ctx)
	if _, err := s.TopRated(ctx, 10); err != nil || loads != 2 {
		t.Errorf("invalidation sonrası yeniden yüklemeli: loads=%d", loads)
	}
}

// TestCachedTopRatedDegradesGracefully: Redis arızası ve bozuk kayıt isteği DÜŞÜRMEZ.
func TestCachedTopRatedDegradesGracefully(t *testing.T) {
	ctx := context.Background()
	c := newMemCache()
	loads := 0
	s := cachedWithLoader(c, &loads)

	c.getErr = errors.New("redis: connection refused")
	if got, err := s.TopRated(ctx, 5); err != nil || len(got.Books) != 5 {
		t.Fatalf("cache arızasında DB'ye düşmeli: %v", err)
	}

	c.getErr = nil
	c.data[TopRatedCacheKey] = []byte("{bozuk json")
	if _, err := s.TopRated(ctx, 5); err != nil || loads != 2 {
		t.Errorf("bozuk kayıt yok sayılıp yeniden yüklenmeli: loads=%d err=%v", loads, err)
	}
}

// ctxRecordingCache, Delete'e gelen context'in iptal durumunu kaydeder.
type ctxRecordingCache struct {
	memCache
	deleteCtxErr error
}

func (c *ctxRecordingCache) Delete(ctx context.Context, keys ...string) error {
	c.deleteCtxErr = ctx.Err()
	return c.memCache.Delete(ctx, keys...)
}

// TestInvalidateSurvivesCanceledRequest (review bulgusu R5): yazım commit edildikten sonra
// istemci koparsa (istek ctx'i iptal) cache silme yine de İPTAL EDİLMEMİŞ bir ctx'le yapılmalı.
func TestInvalidateSurvivesCanceledRequest(t *testing.T) {
	c := &ctxRecordingCache{memCache: memCache{data: map[string][]byte{TopRatedCacheKey: []byte("{}")}}}
	s := NewCached(&Service{}, c)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.InvalidateTopRated(ctx)
	if c.deleteCtxErr != nil {
		t.Errorf("silme iptal edilmiş ctx ile yapıldı: %v", c.deleteCtxErr)
	}
	if _, ok := c.data[TopRatedCacheKey]; ok {
		t.Error("anahtar silinmeliydi")
	}
}

// failingSetCache, Set'i hep başarısız olan cache (Redis yazımı düştü).
type failingSetCache struct{ *memCache }

func (failingSetCache) Set(context.Context, string, []byte, time.Duration) error {
	return errors.New("redis: write timeout")
}

// TestCachedTopRatedSetFailure: cache'e yazılamaması isteği düşürmez — yüklenen liste döner.
func TestCachedTopRatedSetFailure(t *testing.T) {
	c := newMemCache()
	loads := 0
	s := cachedWithLoader(c, &loads)
	s.cache = failingSetCache{c}

	got, err := s.TopRated(context.Background(), 5)
	if err != nil || loads != 1 || len(got.Books) != 5 || got.C != 6.5 {
		t.Errorf("Set hatasında yüklenen liste hatasız dönmeli: loads=%d len=%d err=%v", loads, len(got.Books), err)
	}
}
