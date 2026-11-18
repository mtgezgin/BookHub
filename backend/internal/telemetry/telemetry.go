// Package telemetry, OpenTelemetry'yi kurar: uygulamanın ürettiği İZLERİ (trace) ve METRİKLERİ bir
// toplayıcıya gönderir.
//
// ÜÇ ARACIN İŞ BÖLÜMÜ (öğrenme notu: docs/learning-notes/infra-and-devops.md):
//   - OpenTelemetry (bu paket ve cmd/api'deki ara katman): veriyi uygulamanın İÇİNDE üretir ve OTLP
//     protokolüyle dışarı iter. Saklamaz, göstermez.
//   - Prometheus: metrikleri saklar ve sorgular (PromQL). İzler Tempo'da durur.
//   - Grafana: ikisini de gösterir (pano, keşif, alarm).
//
// KAPALIYKEN: OTEL_EXPORTER_OTLP_ENDPOINT boşsa Setup hiçbir şey kurmaz. OpenTelemetry'nin varsayılan
// sağlayıcıları "yok" sağlayıcılardır: span ve ölçüm çağrıları hiçbir şey yapmadan döner, ağ bağlantısı
// açılmaz. Kod her iki durumda da aynıdır; açıp kapatan yalnız ortam değişkenidir.
//
// GİZLİLİK (CLAUDE.md log kuralı izlere de uygulanır): izler üçüncü bir tarafa gider. Sorgu dizesi,
// istek gövdesi, başlıklar, SQL argümanları, Redis anahtarları ve veritabanı bağlantı bilgisi
// GÖNDERİLMEZ. SQL METNİ gönderilir: sorgular sqlc'den gelen sabit metinlerdir, değer içermez.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config, Setup'ın girdisi (internal/config'ten gelir).
type Config struct {
	// Endpoint: OTLP/HTTP toplayıcısının adresi (OTEL_EXPORTER_OTLP_ENDPOINT). Boş → kapalı.
	Endpoint string
	// ServiceName: Grafana'da bu sürecin adı (service.name).
	ServiceName string
	// Version: çalışan sürüm (commit); "hangi yayından sonra yavaşladı?" sorusu için.
	Version string
}

// Enabled, telemetri açık mı.
func (c Config) Enabled() bool { return c.Endpoint != "" }

// Setup, iz ve metrik sağlayıcılarını kurar ve küresel olarak kaydeder. Dönen fonksiyon kapanışta
// çağrılır: bellekte bekleyen izleri ve son metrikleri gönderir (çağrılmazsa son saniyeler kaybolur).
//
// Yapılandırmanın geri kalanı OpenTelemetry'nin STANDART ortam değişkenlerinden okunur (SDK kendisi
// okur; her dilde ve her araçta aynı adlar):
//
//	OTEL_EXPORTER_OTLP_HEADERS    kimlik başlığı (Grafana Cloud: Authorization=Basic …). SIRDIR.
//	OTEL_TRACES_SAMPLER / _ARG    örnekleme (ör. parentbased_traceidratio ve 0.1 → isteklerin %10'u).
//	OTEL_METRIC_EXPORT_INTERVAL   metrik gönderme aralığı (ms; varsayılan 60000).
func Setup(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	if !cfg.Enabled() {
		return func(context.Context) error { return nil }, nil
	}

	// Kaynak (resource): bu verinin KİMDEN geldiği. Her ize ve metriğe eklenir.
	// WithFromEnv sonda: OTEL_RESOURCE_ATTRIBUTES (ör. service.version=abc1234,deployment.environment=prod)
	// buradaki değerlerin üzerine yazar. Konteyner imajında git bilgisi olmadığı için sürüm canlıda
	// bu değişkenle verilir.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.Version),
		),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetri kaynağı kurulamadı: %w", err)
	}

	// İZLER. Batcher: span'ler bellekte biriktirilir ve toplu gönderilir; isteğin kendisi gönderimi
	// beklemez. Toplayıcıya ulaşılamazsa span'ler düşer, istek etkilenmez.
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("iz göndericisi kurulamadı: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	// METRİKLER. PeriodicReader: sayaçlar bellekte toplanır, aralıkla (varsayılan 60 sn) gönderilir.
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		_ = tracerProvider.Shutdown(ctx)
		return nil, fmt.Errorf("metrik göndericisi kurulamadı: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	// W3C traceparent başlığı: gelen istek bir izin parçasıysa (ileride arayüz ya da NGINX) devam ettirilir.
	otel.SetTextMapPropagator(propagation.TraceContext{})
	// Gönderim hataları (toplayıcı kapalı, kota doldu) uygulamayı durdurmaz; uyarı olarak loglanır.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Warn("telemetri gönderilemedi", "err", err)
	}))

	return func(ctx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}, nil
}

// sqlcName, sqlc'nin her sorgunun başına koyduğu "-- name: ListBooks :many" satırındaki adı yakalar.
var sqlcName = regexp.MustCompile(`^-- name: (\w+)`)

// QueryName, bir SQL metninden span adını üretir: sqlc sorgusuysa adı (ListBooks), değilse ilk
// kelimesi (BEGIN, COMMIT, SET). Ad DÜŞÜK KARDİNALİTELİ olmalı: Grafana'da "en yavaş sorgular" bu ada
// göre gruplanır; metnin tamamı ad olsaydı her sorgu ayrı bir satır olurdu.
func QueryName(sql string) string {
	sql = strings.TrimSpace(sql)
	if m := sqlcName.FindStringSubmatch(sql); m != nil {
		return m[1]
	}
	if i := strings.IndexAny(sql, " \n\t;"); i > 0 {
		return strings.ToUpper(sql[:i])
	}
	return strings.ToUpper(sql)
}

// PgxTracer, veritabanı sorgularını span'e çeviren pgx izleyicisi (database.New'e verilir).
//
// Gönderilen: sorgu adı, SQL metni, süre, hata. GÖNDERİLMEYEN: argümanlar (kütüphanenin varsayılanı;
// e-posta ve arama terimi argümandır) ve bağlantı bilgisi (sunucu adı, veritabanı kullanıcısı).
func PgxTracer() pgx.QueryTracer {
	return otelpgx.NewTracer(
		otelpgx.WithSpanNameFunc(QueryName),
		otelpgx.WithDisableConnectionDetailsInAttributes(),
	)
}

// RecordPoolStats, bağlantı havuzunun durumunu (açık, boşta, bekleyen) metrik olarak yayınlar:
// "havuz doldu mu?" sorusunun cevabı.
func RecordPoolStats(pool *pgxpool.Pool) error {
	return otelpgx.RecordStats(pool, otelpgx.WithMinimumReadDBStatsInterval(5*time.Second))
}

// InstrumentRedis, Redis komutlarını span'e çevirir.
//
// Komut ADI gönderilir (get, set, incr); komutun TAMAMI gönderilmez: anahtarlar veri taşır
// (rate limit anahtarı ziyaretçinin IP'sini içerir). Çağıran fonksiyon bilgisi de kapalı
// (her komutta yığın izi çıkarmak pahalıdır).
func InstrumentRedis(client *redis.Client) error {
	return redisotel.InstrumentTracing(client,
		redisotel.WithDBStatement(false),
		redisotel.WithCallerEnabled(false),
	)
}
