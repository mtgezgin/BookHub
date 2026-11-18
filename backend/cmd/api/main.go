// Package main, BookHub API sunucusunun giriş noktasıdır.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/cache"
	"github.com/mtgezgin/bookhub/backend/internal/comments"
	"github.com/mtgezgin/bookhub/backend/internal/config"
	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/editions"
	"github.com/mtgezgin/bookhub/backend/internal/genres"
	"github.com/mtgezgin/bookhub/backend/internal/library"
	"github.com/mtgezgin/bookhub/backend/internal/moderation"
	"github.com/mtgezgin/bookhub/backend/internal/ratelimit"
	"github.com/mtgezgin/bookhub/backend/internal/ratings"
	"github.com/mtgezgin/bookhub/backend/internal/telemetry"
)

// main yalnızca çıkış kodunu işletim sistemine iletir; asıl iş run()'da.
//
// NEDEN BU AYRIM: os.Exit ve log.Fatal ERTELENMİŞ (defer) fonksiyonları ÇALIŞTIRMAZ.
// Eskiden sunucu goroutine'indeki bir log.Fatalf, pool.Close()'u atlayarak süreci
// öldürürdü. Şimdi her hata run()'dan normal bir dönüşle çıkar → bütün defer'lar
// (havuz, Redis, sinyal) sırayla çalışır → sonra main os.Exit'i çağırır.
func main() {
	// Konteyner sağlık kontrolü: distroless imajda curl/wget yok; binary kendini yoklar
	// (Dockerfile HEALTHCHECK ve compose healthcheck bunu çağırır).
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck("http://127.0.0.1:" + envOr("PORT", "8080") + "/health"))
	}
	os.Exit(run())
}

func run() int {
	// Ayarları env'den yükle ve doğrula (fail-fast).
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config yüklenemedi", "err", err)
		return 1
	}
	setupLogging(cfg.LogFormat)

	// [context + defer] SIGINT/SIGTERM gelince ctx iptal olur.
	// (Havuzdan ÖNCE kuruldu ki açılış işlemleri de sinyalle iptal edilebilsin.)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Telemetri (OpenTelemetry): OTEL_EXPORTER_OTLP_ENDPOINT boşsa hiçbir şey kurulmaz. Havuzdan ve
	// Redis'ten ÖNCE: onların izleyicileri küresel sağlayıcıyı kullanır.
	tel := telemetry.Config{Endpoint: cfg.OTLPEndpoint, ServiceName: cfg.ServiceName, Version: buildVersion()}
	shutdownTelemetry, err := telemetry.Setup(ctx, tel)
	if err != nil {
		slog.Error("telemetri kurulamadı", "err", err)
		return 1
	}
	defer func() {
		// Kapanışta bekleyen izleri gönder; kendi süre sınırıyla (ctx o anda iptal edilmiş olur).
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(flushCtx); err != nil {
			slog.Warn("telemetri kapanışta gönderilemedi", "err", err)
		}
	}()
	var dbTracers []pgx.QueryTracer
	if tel.Enabled() {
		dbTracers = append(dbTracers, telemetry.PgxTracer())
		slog.Info("telemetri açık: izler ve metrikler OTLP ile gönderiliyor", "service", tel.ServiceName)
	}

	// Şemayı güncelle (yalnızca AUTO_MIGRATE=true ise — prod'da bilinçli olarak kapalı).
	if cfg.AutoMigrate {
		if err := database.Migrate(ctx, cfg.DatabaseURL); err != nil {
			slog.Error("migration başarısız", "err", err)
			return 1
		}
		slog.Info("migration'lar güncel")
	}

	// Veritabanı havuzunu aç (içinde Ping var → bağlantı yoksa BURADA dururuz).
	pool, err := database.New(ctx, cfg.DatabaseURL, int32(cfg.DBMaxConns), dbTracers...) //nolint:gosec // config 1-100 aralığını doğrular
	if err != nil {
		slog.Error("veritabanı hazır değil", "err", err)
		return 1
	}
	defer pool.Close() // kapanışta havuzdaki bağlantıları düzgün bırak
	if tel.Enabled() {
		if err := telemetry.RecordPoolStats(pool); err != nil {
			slog.Warn("havuz metrikleri kaydedilemedi", "err", err)
		}
	}

	// Cache + rate limit (Faz 7): REDIS_URL yoksa Noop cache ve bellek içi sınırlayıcı —
	// uygulama Redis'siz de doğru çalışır (tek instance için). Redis varsa ikisi de
	// paylaşılır: birden çok API örneği aynı sayaçları ve aynı cache'i görür.
	var appCache cache.Cache = cache.Noop{}
	var limiter ratelimit.Limiter = ratelimit.NewMemory(cfg.AuthRateLimit, time.Minute)
	if cfg.RedisURL != "" {
		// Bozuk adres bir yapılandırma hatasıdır → açılma. Ulaşılamayan Redis ise DEĞİL: cache bir
		// hızlandırıcıdır ve çalışırken düşen Redis'te uygulama zaten ayakta kalıyor; açılışta da aynı
		// kural geçerli olmalı (yoksa Redis'in kesintisi sırasında yeniden başlayan API hiç açılamaz).
		// İstemci bağlantıyı tembel kurar: Redis geri gelince cache ve rate limit kendiliğinden çalışır.
		rc, err := cache.NewRedis(cfg.RedisURL)
		if err != nil {
			slog.Error("redis yapılandırılamadı", "err", err)
			return 1
		}
		defer func() { _ = rc.Close() }()
		appCache = rc
		limiter = ratelimit.NewRedis(rc.Client(), cfg.AuthRateLimit, time.Minute)
		if tel.Enabled() {
			if err := telemetry.InstrumentRedis(rc.Client()); err != nil {
				slog.Warn("redis izleri kurulamadı", "err", err)
			}
		}
		if err := rc.Ping(ctx); err != nil {
			slog.Warn("redis'e ulaşılamadı; cache'siz ve rate limit'siz açılıyor, bağlantı arka planda yeniden denenecek", "err", err)
		} else {
			slog.Info("redis açık: cache + paylaşılan rate limit")
		}
	}

	tokens, err := auth.NewTokenManager([]byte(cfg.JWTSecret), cfg.AccessTokenTTL)
	if err != nil {
		slog.Error("token yöneticisi kurulamadı", "err", err)
		return 1
	}

	// ELLE DI ZİNCİRİ (GO_CONVENTIONS §3): pool → repository'ler → (servis) → router.
	// Framework yok; bağımlılık grafiğinin tamamı burada, tek yerde okunuyor.
	// Auth ve moderasyonun araya bir servisi var (gerçek iş mantığı); okuma/tür CRUD'unun yok.
	// Puan servisi cache'le sarılı; moderasyon, public görünümü değiştiren her commit'te
	// top-rated cache'ini geçersiz kılar. İki servis birbirini tanımıyor — bağ burada kuruluyor.
	// Genel ortalama (ağırlıklı puandaki C) süreç içinde 60 sn tutulur: puana göre her sıralama
	// isteğinde ayrı bir sorgu çalışmasın (Faz 16c). Kaynağı tek: puan servisi.
	baseRatings := ratings.NewService(pool, int64(cfg.RatingMinVotes))
	globalAverage := ratings.NewGlobalAverage(ratings.GlobalAverageTTL, baseRatings.LoadGlobalAverage)
	baseRatings.UseGlobalAverage(globalAverage.Get)
	bookRepo := books.NewRepository(pool)
	bookRepo.UseGlobalAverage(globalAverage.Get)
	// Tür eşlemesi ve toplam sayılar 60 sn süreç içinde (Faz 16c; books/catalog.go).
	bookRepo.UseCatalogCache(60 * time.Second)
	ratingSvc := ratings.NewCached(baseRatings, appCache)
	moderationSvc := moderation.NewService(pool, moderation.WithPublicChangeHook(ratingSvc.InvalidateTopRated))
	// Baskılar (Faz 16) kapak taşıyor; top-rated cache'i de onları gösterir.
	editionSvc := editions.NewService(pool, ratingSvc.InvalidateTopRated)

	d := deps{
		books:      bookRepo,
		genres:     genres.NewRepository(pool),
		moderation: moderationSvc,
		editions:   editionSvc,
		comments:   comments.NewService(pool),
		ratings:    ratingSvc,
		library:    library.NewRepository(pool),
		// Katalogun ?sort=rating sırası top-rated ile aynı m'yi kullanır.
		ratingMinVotes: int64(cfg.RatingMinVotes),
		auth:           auth.NewService(auth.NewRepository(pool), tokens, cfg.RefreshTokenTTL, cfg.SessionMaxAge),
		tokens:         tokens,
		limiter:        limiter,

		trustedProxies: cfg.TrustedProxies,
		// Refresh token artık cevabın gövdesinde değil, httpOnly bir çerezde (Faz 13).
		// Adı ayarlanabilir çünkü çerezler PORT'a göre değil HOST'a göre kapsanır: aynı
		// makinedeki compose (:3000) ve kind (:3001) yığınları tek bir "localhost" kavanozunu
		// paylaşır ve aynı adı kullansalardı birbirlerinin oturumunu ezerlerdi.
		cookies: cookieSettingsFrom(cfg),
	}

	// http.Server'ı elle kuruyoruz: timeout'lar + Shutdown() erişimi.
	//   ReadHeaderTimeout: başlıkları yavaş yavaş gönderip bağlantı tutan (slowloris) istemci
	//   IdleTimeout: keep-alive bağlantısı boşta en çok bu kadar açık kalır (yoksa sınırsız)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           newRouter(d),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// [goroutine + channel] Sunucu ayrı akışta çalışır; açılamazsa (port dolu gibi) hatayı
	// KANALA yazar — kendisi süreci öldürmez, karar run()'ın.
	serveErr := make(chan error, 1)
	go func() {
		slog.Info("BookHub API dinliyor", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	// [select] Hangisi önce gelirse: kapanış sinyali ya da sunucu hatası.
	exitCode := 0
	select {
	case <-ctx.Done():
		slog.Info("kapanış sinyali alındı, nazikçe kapanıyor")
	case err := <-serveErr:
		slog.Error("sunucu başlatılamadı", "err", err)
		exitCode = 1
	}

	// [context + defer] Kapanışa 10sn deadline; devam eden istekleri bitir.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown başarısız", "err", err)
		return 1
	}
	slog.Info("sunucu temiz kapandı")
	return exitCode
}

// setupLogging, varsayılan logger'ı kurar: dev'de okunur metin, konteynerde JSON
// (log toplayıcılar — Loki, CloudWatch, Datadog — alanları ayrıştırabilsin).
// slog.SetDefault, standart "log" paketini de aynı handler'a yönlendirir: log.Printf
// kullanan eski satırlar da aynı biçimde çıkar.
func setupLogging(format string) {
	var h slog.Handler = slog.NewTextHandler(os.Stdout, nil)
	if format == "json" {
		h = slog.NewJSONHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(h))
}

// healthcheck, URL'ye GET atar; 200 → 0, aksi → 1 (konteyner "unhealthy").
func healthcheck(url string) int {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// buildVersion, derlemenin commit'ini döner (Go, modül içinden derlenen binary'ye VCS bilgisini
// gömer). Telemetride service.version: "hangi yayından sonra yavaşladı?" sorusunun cevabı.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "bilinmiyor"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			return s.Value[:7]
		}
	}
	return "bilinmiyor"
}
