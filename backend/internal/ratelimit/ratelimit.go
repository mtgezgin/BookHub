// Package ratelimit, "bir anahtar (ör. IP) bir zaman penceresinde en çok N istek" kuralını
// uygular. İki uygulama, AYNI semantik (sabit pencere / fixed window):
//
//   - Memory: sayaçlar süreç belleğinde. Tek instance'ta doğru; birden çok instance'ta her
//     biri kendi sayacını tuttuğu için gerçek sınır N × instance olur.
//   - Redis: sayaçlar paylaşılan Redis'te. Kaç instance olursa olsun sınır N.
//
// Bu ikisi arasındaki fark, Faz 7'nin "neden paylaşılan state?" dersinin kendisi — canlı
// olarak iki API örneğiyle ölçüldü (bkz. docs/learning-notes).
package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Decision, bir isteğin sonucu. RetryAfter yalnız reddedilince anlamlı: pencerenin kalanı.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter, anahtar başına sınır uygular.
type Limiter interface {
	Allow(ctx context.Context, key string) (Decision, error)
}

// Unlimited, her isteğe izin verir (sınır kapalı / testler).
type Unlimited struct{}

func (Unlimited) Allow(context.Context, string) (Decision, error) {
	return Decision{Allowed: true}, nil
}

// Memory, bellek içi sabit pencere sayacı.
type Memory struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	now       func() time.Time
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	start time.Time
	count int
}

func NewMemory(limit int, window time.Duration) *Memory {
	return &Memory{limit: limit, window: window, now: time.Now, buckets: map[string]*bucket{}}
}

func (m *Memory) Allow(_ context.Context, key string) (Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)

	b, ok := m.buckets[key]
	if !ok || now.Sub(b.start) >= m.window {
		b = &bucket{start: now}
		m.buckets[key] = b
	}
	b.count++
	if b.count > m.limit {
		return Decision{Allowed: false, RetryAfter: b.start.Add(m.window).Sub(now)}, nil
	}
	return Decision{Allowed: true}, nil
}

// sweep, süresi geçmiş pencereleri (en çok pencere başına bir kez) temizler: her farklı IP
// bir anahtar açtığı için temizlenmeyen bir map, bellek sızıntısına dönüşürdü.
func (m *Memory) sweep(now time.Time) {
	if now.Sub(m.lastSweep) < m.window {
		return
	}
	for k, b := range m.buckets {
		if now.Sub(b.start) >= m.window {
			delete(m.buckets, k)
		}
	}
	m.lastSweep = now
}

// Redis, paylaşılan sabit pencere sayacı: INCR + (ilk artışta) EXPIRE.
type Redis struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func NewRedis(client *redis.Client, limit int, window time.Duration) *Redis {
	return &Redis{client: client, limit: limit, window: window}
}

// Allow: INCR atomiktir — iki instance aynı anda artırsa bile sayaç doğru kalır. EXPIRE NX
// yalnız TTL'i olmayan anahtara süre koyar: pencere ilk istekte başlar, sonrakiler uzatmaz.
// Üç komut TEK pipeline'da (MULTI/EXEC): bir ağ gidiş-dönüşü.
func (r *Redis) Allow(ctx context.Context, key string) (Decision, error) {
	k := "rate-limit:" + key
	pipe := r.client.TxPipeline()
	incr := pipe.Incr(ctx, k)
	pipe.ExpireNX(ctx, k, r.window)
	ttl := pipe.PTTL(ctx, k)
	if _, err := pipe.Exec(ctx); err != nil {
		return Decision{}, fmt.Errorf("rate limit sayacı: %w", err)
	}
	if incr.Val() > int64(r.limit) {
		return Decision{Allowed: false, RetryAfter: ttl.Val()}, nil
	}
	return Decision{Allowed: true}, nil
}
