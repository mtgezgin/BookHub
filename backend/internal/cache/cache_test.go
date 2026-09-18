package cache

import (
	"context"
	"os"
	"testing"
	"time"
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
}

func TestNewRedisBadURL(t *testing.T) {
	if _, err := NewRedis(context.Background(), "http://yanlis-sema"); err == nil {
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
	r, err := NewRedis(ctx, url)
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
