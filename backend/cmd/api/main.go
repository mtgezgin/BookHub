// Package main, BookHub API sunucusunun giriş noktasıdır.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/cache"
	"github.com/mtgezgin/bookhub/backend/internal/comments"
	"github.com/mtgezgin/bookhub/backend/internal/config"
	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/genres"
	"github.com/mtgezgin/bookhub/backend/internal/moderation"
	"github.com/mtgezgin/bookhub/backend/internal/ratings"
)

func main() {
	// Ayarları env'den yükle ve doğrula (fail-fast).
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config yüklenemedi: %v", err)
	}

	// [context + defer] SIGINT/SIGTERM gelince ctx iptal olur.
	// (Havuzdan ÖNCE kuruldu ki açılış işlemleri de sinyalle iptal edilebilsin.)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Şemayı güncelle (yalnızca AUTO_MIGRATE=true ise — prod'da bilinçli olarak kapalı).
	if cfg.AutoMigrate {
		if err := database.Migrate(ctx, cfg.DatabaseURL); err != nil {
			log.Fatalf("migration başarısız: %v", err)
		}
		log.Println("migration'lar güncel")
	}

	// Veritabanı havuzunu aç (içinde Ping var → bağlantı yoksa BURADA ölürüz).
	pool, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("veritabanı hazır değil: %v", err)
	}
	defer pool.Close() // kapanışta havuzdaki bağlantıları düzgün bırak

	// Cache (Faz 7): REDIS_URL yoksa Noop — uygulama Redis'siz de doğru çalışır.
	var appCache cache.Cache = cache.Noop{}
	if cfg.RedisURL != "" {
		rc, err := cache.NewRedis(ctx, cfg.RedisURL)
		if err != nil {
			log.Fatalf("redis hazır değil: %v", err)
		}
		defer func() { _ = rc.Close() }()
		appCache = rc
		log.Println("redis cache açık")
	}

	tokens, err := auth.NewTokenManager([]byte(cfg.JWTSecret), cfg.AccessTokenTTL)
	if err != nil {
		log.Fatalf("token yöneticisi kurulamadı: %v", err)
	}

	// ELLE DI ZİNCİRİ (GO_CONVENTIONS §3): pool → repository'ler → (servis) → router.
	// Framework yok; bağımlılık grafiğinin tamamı burada, tek yerde okunuyor.
	// Auth ve moderasyonun araya bir servisi var (gerçek iş mantığı); okuma/tür CRUD'unun yok.
	// Puan servisi cache'le sarılı; moderasyon, public görünümü değiştiren her commit'te
	// top-rated cache'ini geçersiz kılar. İki servis birbirini tanımıyor — bağ burada kuruluyor.
	ratingSvc := ratings.NewCached(ratings.NewService(pool, int64(cfg.RatingMinVotes)), appCache)
	moderationSvc := moderation.NewService(pool, moderation.WithPublicChangeHook(ratingSvc.InvalidateTopRated))

	d := deps{
		books:      books.NewRepository(pool),
		genres:     genres.NewRepository(pool),
		moderation: moderationSvc,
		comments:   comments.NewService(pool),
		ratings:    ratingSvc,
		auth:       auth.NewService(auth.NewRepository(pool), tokens, cfg.RefreshTokenTTL),
		tokens:     tokens,
	}

	// http.Server'ı elle kuruyoruz: timeout'lar + Shutdown() erişimi.
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      newRouter(d),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// [goroutine] Sunucuyu ayrı akışta çalıştır; graceful kapanışta gelen
	// http.ErrServerClosed beklenen durumdur.
	go func() {
		log.Printf("BookHub API %s adresinde dinliyor", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("sunucu başlatılamadı: %v", err)
		}
	}()

	// [channel] Sinyal gelene kadar burada bekle.
	<-ctx.Done()
	log.Println("kapanış sinyali alındı, nazikçe kapanıyor...")

	// [context + defer] Kapanışa 10sn deadline; devam eden istekleri bitir.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown başarısız: %v", err)
	}
	log.Println("sunucu temiz kapandı")
}
