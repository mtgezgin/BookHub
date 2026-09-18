package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestMemoryFixedWindow(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	m := NewMemory(3, time.Minute)
	m.now = func() time.Time { return now }

	for i := 1; i <= 3; i++ {
		if d, _ := m.Allow(ctx, "login:1.2.3.4"); !d.Allowed {
			t.Fatalf("%d. istek izinli olmalı", i)
		}
	}
	now = now.Add(20 * time.Second)
	d, _ := m.Allow(ctx, "login:1.2.3.4")
	if d.Allowed || d.RetryAfter != 40*time.Second {
		t.Errorf("4. istek reddedilmeli, RetryAfter pencerenin kalanı (40s) olmalı: %+v", d)
	}
	if d, _ := m.Allow(ctx, "login:5.6.7.8"); !d.Allowed {
		t.Error("başka IP'nin sayacı ayrı olmalı")
	}

	now = now.Add(40 * time.Second) // pencere doldu
	if d, _ := m.Allow(ctx, "login:1.2.3.4"); !d.Allowed {
		t.Error("yeni pencerede yeniden izin verilmeli")
	}
}

func TestMemorySweep(t *testing.T) {
	now := time.Now()
	m := NewMemory(1, time.Minute)
	m.now = func() time.Time { return now }
	for i := range 100 {
		_, _ = m.Allow(context.Background(), fmt.Sprintf("ip-%d", i))
	}
	now = now.Add(2 * time.Minute)
	_, _ = m.Allow(context.Background(), "yeni")
	if len(m.buckets) != 1 {
		t.Errorf("süresi geçmiş 100 pencere temizlenmeli, kalan %d", len(m.buckets))
	}
}

func TestUnlimited(t *testing.T) {
	for range 1000 {
		if d, _ := (Unlimited{}).Allow(context.Background(), "x"); !d.Allowed {
			t.Fatal("Unlimited hep izin vermeli")
		}
	}
}

// TestRedisSharedAcrossInstances: iki AYRI Limiter (iki API instance'ı gibi) aynı Redis'i
// paylaşınca sınır TOPLAMDA uygulanır. Memory ile her biri kendi 3'ünü verirdi (toplam 6).
func TestRedisSharedAcrossInstances(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL tanımlı değil → Redis testi atlanıyor")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	key := fmt.Sprintf("test:%d", time.Now().UnixNano())
	t.Cleanup(func() { client.Del(ctx, "rate-limit:"+key) })

	instanceA := NewRedis(client, 3, time.Minute)
	instanceB := NewRedis(client, 3, time.Minute)
	allowed := 0
	for i := range 6 {
		lim := instanceA
		if i%2 == 1 {
			lim = instanceB
		}
		d, err := lim.Allow(ctx, key)
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if d.Allowed {
			allowed++
		} else if d.RetryAfter <= 0 || d.RetryAfter > time.Minute {
			t.Errorf("RetryAfter pencere içinde olmalı: %v", d.RetryAfter)
		}
	}
	if allowed != 3 {
		t.Errorf("iki instance TOPLAMDA 3 izin vermeli, verilen %d", allowed)
	}

	// Memory ile aynı senaryo: sınır instance başına → toplam 6 (dersin karşı örneği).
	memA, memB := NewMemory(3, time.Minute), NewMemory(3, time.Minute)
	memAllowed := 0
	for i := range 6 {
		lim := memA
		if i%2 == 1 {
			lim = memB
		}
		if d, _ := lim.Allow(ctx, key); d.Allowed {
			memAllowed++
		}
	}
	if memAllowed != 6 {
		t.Errorf("bellek içi sayaçlar instance başına: 6 bekleniyordu, %d", memAllowed)
	}
}

// TestRedisAllowExecError: pipeline Exec başarısız olursa (burada kapatılmış istemci —
// gerçek Redis gerekmez) Allow boş bir Decision ve "rate limit sayacı" ile sarılmış
// hatayı döndürmeli; hata asla izin olarak yorumlanmamalı.
func TestRedisAllowExecError(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	d, err := NewRedis(client, 3, time.Minute).Allow(context.Background(), "closed")
	if err == nil {
		t.Fatal("kapalı istemcide hata bekleniyordu")
	}
	if !errors.Is(err, redis.ErrClosed) {
		t.Errorf("asıl hata sarılmalı (errors.Is ErrClosed), gelen %v", err)
	}
	if !strings.HasPrefix(err.Error(), "rate limit sayacı: ") {
		t.Errorf("hata 'rate limit sayacı' ile sarılmalı, gelen %q", err)
	}
	if d != (Decision{}) {
		t.Errorf("hata durumunda boş Decision dönmeli, gelen %+v", d)
	}
}
