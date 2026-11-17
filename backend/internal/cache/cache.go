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
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mtgezgin/bookhub/backend/internal/reqstats"
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

// NewRedis, REDIS_URL'den (redis://host:port/db) istemci kurar. Bağlantı AÇMAZ (istemci tembeldir);
// ulaşılabilirliği Ping söyler. Dönen hata yalnız bozuk adres içindir.
func NewRedis(url string) (*Redis, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("REDIS_URL ayrıştırılamadı: %w", err)
	}
	// Kısa zaman aşımları: yavaşlamış bir Redis, isteklerimizi de yavaşlatmasın — beklemek
	// yerine "miss" sayıp DB'ye düşmek daha iyi.
	opts.DialTimeout = time.Second
	opts.DialerRetries = 1 // varsayılan 5: ulaşılamayan Redis'te tek komut 5 × DialTimeout beklerdi
	opts.ContextTimeoutEnabled = true
	opts.ReadTimeout = 500 * time.Millisecond
	opts.WriteTimeout = 500 * time.Millisecond
	// Varsayılan 3 yeniden deneme: düşmüş bir Redis'te her komut dört kez zaman aşımına uğrar ve
	// "cache yoksa DB'ye düş" yolu saniyelerce gecikir. Tek yeniden deneme, havuzdaki bayat bir
	// bağlantıyı tolere etmeye yeter.
	opts.MaxRetries = 1

	client := redis.NewClient(opts)
	client.AddHook(statsHook{}) // dışta: devre kesicinin reddettiği komut ~0 ms sayılır
	client.AddHook(&breakerHook{cooldown: breakerCooldown, now: time.Now})
	return &Redis{client: client}, nil
}

// ErrUnavailable, Redis kısa süre önce ulaşılamaz bulunduğu için komutun HİÇ DENENMEDİĞİNİ söyler.
var ErrUnavailable = errors.New("redis şu an kullanılamıyor")

// breakerCooldown, başarısız bir komuttan sonra Redis'in yeniden denenmeyeceği süre.
const breakerCooldown = 5 * time.Second

// commandBudget, tek bir Redis komutunun (bağlantı kurma dahil) en uzun süresi. Kesinti sırasında
// Redis'i yoklayan o tek istek de bundan uzun beklemez.
const commandBudget = 600 * time.Millisecond

// breakerHook, basit bir devre kesicidir: Redis'e ulaşılamadığında sonraki komutlar cooldown boyunca
// denenmeden ErrUnavailable alır.
//
// NEDEN (ölçüm, 17 Kasım 2026): Redis durdurulmuşken önbellek kullanan tek bir istek 10 sn'yi aştı,
// çünkü her komut bağlantı kurmayı baştan deniyor ve zaman aşımını bekliyordu. "Cache bir
// hızlandırıcıdır" ilkesi, Redis'in yokluğunun isteği YAVAŞLATMAMASINI da gerektirir. Kesiciyle
// kesinti boyunca cooldown başına en çok bir komut bekler; gerisi hemen veritabanına düşer.
// İstemcinin hook'u olduğu için rate limit'in pipeline'ını da kapsar.
type breakerHook struct {
	openUntil atomic.Int64 // UnixNano; 0 → kapalı (komutlar geçer)
	cooldown  time.Duration
	now       func() time.Time
}

func (b *breakerHook) allow() bool { return b.now().UnixNano() >= b.openUntil.Load() }

// record, komutun sonucuna bakar. Sunucunun CEVAP verdiği hatalar (redis.Nil, WRONGTYPE…) Redis'in
// ayakta olduğunu gösterir; istemcinin isteği iptal etmesi de Redis'in kusuru değildir.
func (b *breakerHook) record(err error) {
	var reply redis.Error
	if err == nil || errors.As(err, &reply) || errors.Is(err, context.Canceled) {
		return
	}
	b.openUntil.Store(b.now().Add(b.cooldown).UnixNano())
}

func (b *breakerHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (b *breakerHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if !b.allow() {
			cmd.SetErr(ErrUnavailable)
			return ErrUnavailable
		}
		ctx, cancel := context.WithTimeout(ctx, commandBudget)
		defer cancel()
		err := next(ctx, cmd)
		b.record(err)
		return err
	}
}

func (b *breakerHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if !b.allow() {
			for _, c := range cmds {
				c.SetErr(ErrUnavailable)
			}
			return ErrUnavailable
		}
		ctx, cancel := context.WithTimeout(ctx, commandBudget)
		defer cancel()
		err := next(ctx, cmds)
		b.record(err)
		return err
	}
}

// Ping, Redis'e 3 sn içinde ulaşılabiliyor mu diye bakar.
func (r *Redis) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := r.client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("redis'e bağlanılamadı: %w", err)
	}
	return nil
}

// statsHook, her Redis komutunun süresini isteğin sayacına yazar (reqstats; REVIEW_FEEDBACK F32).
// Komutun kendisi ve anahtarları kaydedilmez.
type statsHook struct{}

func (statsHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (statsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		reqstats.From(ctx).AddRedis(time.Since(start))
		return err
	}
}

func (statsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		reqstats.From(ctx).AddRedis(time.Since(start))
		return err
	}
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
