// Package cache, uygulamanın önbellek soyutlaması: Redis (paylaşılan) ya da Noop (kapalı).
//
// TASARIM İLKESİ: cache bir HIZLANDIRICIDIR, doğruluk kaynağı DEĞİL. Redis düşerse ya da
// hiç yapılandırılmamışsa uygulama yavaşlar ama doğru cevap vermeye devam eder — bu yüzden
// çağıranlar cache hatasını loglayıp veritabanına düşer, isteği başarısız saymaz.
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache, bayt dizisi saklayan minimal anahtar-değer önbelleği.
type Cache interface {
	// Get: ok=false → anahtar yok (hata DEĞİL; "cache miss").
	Get(ctx context.Context, key string) (val []byte, ok bool, err error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// Noop, cache kapalıyken kullanılır: her Get bir "miss", yazmalar yok sayılır. Çağıranın
// "cache var mı?" diye dallanmasına gerek kalmaz (null object deseni).
type Noop struct{}

func (Noop) Get(context.Context, string) ([]byte, bool, error)        { return nil, false, nil }
func (Noop) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (Noop) Delete(context.Context, ...string) error                  { return nil }

// Redis, go-redis istemcisini Cache arayüzüne uyarlar.
type Redis struct {
	client *redis.Client
}

// NewRedis, REDIS_URL'den (redis://host:port/db) istemci kurar ve 3 sn içinde PING atar
// (Postgres'teki gibi: bağlanamıyorsak açılışta öğrenelim, ilk istekte değil).
func NewRedis(ctx context.Context, url string) (*Redis, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("REDIS_URL ayrıştırılamadı: %w", err)
	}
	// Kısa zaman aşımları: yavaşlamış bir Redis, isteklerimizi de yavaşlatmasın — beklemek
	// yerine "miss" sayıp DB'ye düşmek daha iyi.
	opts.DialTimeout = 2 * time.Second
	opts.ReadTimeout = 500 * time.Millisecond
	opts.WriteTimeout = 500 * time.Millisecond

	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis'e bağlanılamadı: %w", err)
	}
	return &Redis{client: client}, nil
}

// Client, ham istemci (rate limiter gibi Cache arayüzünden fazlasına ihtiyaç duyanlar için).
func (r *Redis) Client() *redis.Client { return r.client }

func (r *Redis) Close() error { return r.client.Close() }

func (r *Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) { // go-redis "anahtar yok"u hata olarak döner — biz miss'e çeviriyoruz
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (r *Redis) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, val, ttl).Err()
}

func (r *Redis) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}
