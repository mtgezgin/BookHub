package cache

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNoop(t *testing.T) {
	var c Cache = Noop{}
	ctx := context.Background()
	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := c.Get(ctx, "k"); ok || err != nil {
		t.Errorf("Noop her zaman miss vermeli: ok=%v err=%v", ok, err)
	}
	if err := c.Delete(ctx, "k"); err != nil {
		t.Errorf("Noop.Delete nil dönmeli: %v", err)
	}
}

func TestNewRedisBadURL(t *testing.T) {
	if _, err := NewRedis("http://yanlis-sema"); err == nil {
		t.Error("geçersiz URL reddedilmeli")
	}
}

// TestRedisRoundTrip: gerçek Redis'e karşı (TEST_REDIS_URL yoksa atlanır).
func TestRedisRoundTrip(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL tanımlı değil → Redis testi atlanıyor")
	}
	ctx := context.Background()
	r, err := NewRedis(url)
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	key := "test:cache:" + time.Now().Format(time.RFC3339Nano)
	if _, ok, err := r.Get(ctx, key); ok || err != nil {
		t.Fatalf("olmayan anahtar miss olmalı (hata değil): ok=%v err=%v", ok, err)
	}
	if err := r.Set(ctx, key, []byte("değer"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := r.Get(ctx, key)
	if err != nil || !ok || string(got) != "değer" {
		t.Fatalf("Get: %q ok=%v err=%v", got, ok, err)
	}
	if err := r.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := r.Get(ctx, key); ok {
		t.Error("silinen anahtar miss olmalı")
	}
	if ttl := r.Client().TTL(ctx, key).Val(); ttl > 0 {
		t.Errorf("silinmiş anahtarın TTL'i olmamalı: %v", ttl)
	}
}

// TestNewRedisUnreachable: geçerli URL ama ulaşılamayan sunucu → istemci YİNE kurulur (API Redis'siz
// açılabilsin; bağlantı sonradan gelir). Ulaşılamadığını Ping söyler; komutlar hata döner ve
// çağıranlar veritabanına düşer.
func TestNewRedisUnreachable(t *testing.T) {
	r, err := NewRedis("redis://127.0.0.1:1/0")
	if err != nil || r == nil {
		t.Fatalf("istemci kurulmalıydı: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if err := r.Ping(context.Background()); err == nil || !strings.Contains(err.Error(), "redis'e bağlanılamadı") {
		t.Fatalf("\"redis'e bağlanılamadı\" hatası bekleniyordu, gelen %v", err)
	}
	if _, _, err := r.Get(context.Background(), "k"); err == nil {
		t.Error("ulaşılamayan Redis'te Get hata dönmeli (çağıran veritabanına düşer)")
	}
}

// TestRedisDeleteNoKeys: anahtarsız Delete nil dönmeli. Redis argümansız DEL'i REDDEDER
// ("wrong number of arguments"), dolayısıyla nil sonuç komutun gönderilmediğini gösterir.
func TestRedisDeleteNoKeys(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL tanımlı değil → Redis testi atlanıyor")
	}
	ctx := context.Background()
	r, err := NewRedis(url)
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	if err := r.Client().Do(ctx, "DEL").Err(); err == nil {
		t.Fatal("ön koşul: Redis argümansız DEL'i reddetmeli")
	}
	if err := r.Delete(ctx); err != nil {
		t.Errorf("anahtarsız Delete nil dönmeli (DEL gönderilmemeli): %v", err)
	}
}

// TestRedisGetErrorIsNotAMiss: redis.Nil DIŞINDAKİ bir hata (kapalı istemci, bağlantı kopması)
// "miss" diye yutulmamalı — çağıran loglayıp DB'ye düşebilsin diye hata olarak dönmeli.
func TestRedisGetErrorIsNotAMiss(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL tanımlı değil → Redis testi atlanıyor")
	}
	r, err := NewRedis(url)
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	_ = r.Close()
	val, ok, err := r.Get(context.Background(), "herhangi")
	if err == nil || ok || val != nil {
		t.Errorf("kapalı istemcide (nil, false, err) bekleniyordu: %q %v %v", val, ok, err)
	}
}

// TestBreakerSkipsUnreachableRedis: Redis'e ulaşılamadığında yalnız İLK komut bağlanmayı dener;
// sonrakiler bekleme süresi boyunca denenmeden ErrUnavailable alır (istek Redis yüzünden yavaşlamaz).
// Süre dolunca yeniden denenir. Sunucunun cevap verdiği hatalar ve istemci iptali kesiciyi açmaz.
func TestBreakerSkipsUnreachableRedis(t *testing.T) {
	now := time.Now()
	b := &breakerHook{cooldown: 5 * time.Second, now: func() time.Time { return now }}
	calls := 0
	netErr := errors.New("dial tcp: bağlantı reddedildi")
	result := netErr
	run := b.ProcessHook(func(context.Context, redis.Cmder) error { calls++; return result })
	cmd := func() redis.Cmder { return redis.NewStringCmd(context.Background(), "get", "k") }

	if err := run(context.Background(), cmd()); !errors.Is(err, netErr) || calls != 1 {
		t.Fatalf("ilk komut denenmeli: %v, %d çağrı", err, calls)
	}
	c := cmd()
	if err := run(context.Background(), c); !errors.Is(err, ErrUnavailable) || calls != 1 || !errors.Is(c.Err(), ErrUnavailable) {
		t.Fatalf("kesici açıkken komut denenmemeli: %v, %d çağrı, cmd hatası %v", err, calls, c.Err())
	}
	pipeCalls := 0
	pipe := b.ProcessPipelineHook(func(context.Context, []redis.Cmder) error { pipeCalls++; return nil })
	if err := pipe(context.Background(), []redis.Cmder{cmd()}); !errors.Is(err, ErrUnavailable) || pipeCalls != 0 {
		t.Errorf("pipeline da reddedilmeli (rate limit): %v, %d çağrı", err, pipeCalls)
	}

	now = now.Add(5 * time.Second) // bekleme doldu, Redis geri geldi
	result = nil
	if err := run(context.Background(), cmd()); err != nil || calls != 2 {
		t.Fatalf("bekleme dolunca yeniden denenmeli: %v, %d çağrı", err, calls)
	}

	for name, e := range map[string]error{"anahtar yok": redis.Nil, "istemci iptali": context.Canceled} {
		result = e
		_ = run(context.Background(), cmd())
		if !b.allow() {
			t.Errorf("%s kesiciyi açmamalı", name)
		}
	}
}
