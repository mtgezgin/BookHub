// Package database, Postgres bağlantı HAVUZUNU (pool) kurar.
// Domain bilgisi içermez; tek işi "çalışan bir bağlantı havuzu ver"dir.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/multitracer"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/reqstats"
)

// DefaultMaxConns, havuzun açabileceği en çok bağlantı (DB_MAX_CONNS verilmezse).
//
// Hesap: Supabase'in oturum havuzu varsayılan 15 bağlantı verir ve fazlasını sıraya sokar. Yayın
// sırasında eski ve yeni örnek birlikte çalışır (2 × 6 = 12), migration bir bağlantı daha açar;
// geriye elle bağlanmak için iki bağlantı kalır. pgx'in varsayılanı max(4, çekirdek sayısı) idi:
// konteynerin işlemci kotasını değil DÜĞÜMÜN çekirdek sayısını izler, yani öngörülemez
// (REVIEW_FEEDBACK F08).
const DefaultMaxConns = 6

// Bağlantı başına süre sınırları (her yeni bağlantıda SET edilir).
//
//   - statement_timeout: tek bir sorgunun tavanı. HTTP katmanının bütçesinden (okuma 5 sn, yazma 9 sn)
//     büyük seçildi: olağan durumda sorguyu context sonlandırır; bu sınır, context'siz kalan ya da
//     iptali sunucuya ulaşmayan bir sorgu için son hattır.
//
//   - lock_timeout: bir satır kilidini bekleme tavanı. Kilitler burada milisaniyeler sürer; saniyelerce
//     bekleyen bir yazma büyük olasılıkla takılmış bir işlemin arkasındadır ve beklemek yerine
//     "şu an olmuyor" demek daha doğrudur.
//
//   - idle_in_transaction_session_timeout: BEGIN'den sonra unutulmuş bir işlem kilitlerini sonsuza
//     kadar tutmasın.
//
//   - plan_cache_mode = force_custom_plan: pgx sorguları hazırlanmış deyim (prepared statement) olarak
//     çalıştırır. Postgres birkaç çalıştırmadan sonra deyimi parametrelerin DEĞERİNİ bilmeden bir kez
//     planlayıp ("genel plan") o planı kullanmaya geçebilir. LIMIT/OFFSET ve tür kimliği parametre olunca
//     genel plan kötüdür: 500 bin kitapta katalog listesi 0,1–0,8 ms yerine 42–165 ms (Faz 16c ölçümü).
//     Her çalıştırmayı gerçek değerlerle planlamak sorgu başına ~0,1–0,6 ms planlama süresi ekler.
//     (Paralel taramayı kapatmak da denendi: bir durumu düzeltip büyük taramaları üçe katladığı için
//     yapılmadı; docs/phases/phase-16c-catalog-queries.md.)
//
// NEDEN bağlantı adresinde değil: aynı DATABASE_URL'i migration aracı database/sql üzerinden de açıyor;
// migration'lar bu sınırlara tabi olmamalı (index kurmak dakikalar sürebilir).
const sessionLimits = `SET statement_timeout = '10s';
SET lock_timeout = '3s';
SET idle_in_transaction_session_timeout = '15s';
SET plan_cache_mode = 'force_custom_plan';`

// New, verilen DSN ile bir bağlantı havuzu açar ve bağlantıyı doğrular. maxConns <= 0 ise
// DefaultMaxConns kullanılır. tracers: istek sayacına EK sorgu izleyicileri (OpenTelemetry;
// internal/telemetry). Bu paket onları tanımaz; yalnız pgx'in arayüzünü bilir.
//
// NEDEN HAVUZ: her HTTP isteğinde sıfırdan bağlantı açmak (TCP + auth el sıkışması)
// çok pahalı. Havuz, açık bağlantıları hazır tutar ve eşzamanlı isteklere dağıtır;
// iş bitince bağlantı kapanmaz, havuza geri döner. (JS'teki pg.Pool'un karşılığı.)
func New(ctx context.Context, databaseURL string, maxConns int32, tracers ...pgx.QueryTracer) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// Buraya DSN parse hatasıyla düşülür (bozuk URL vb.).
		return nil, fmt.Errorf("havuz yapılandırılamadı: %w", err)
	}
	if maxConns <= 0 {
		maxConns = DefaultMaxConns
	}
	cfg.MaxConns = maxConns
	// Bir bağlantı hep hazır dursun: seyrek trafikte ilk istek TLS + kimlik doğrulama beklemesin.
	cfg.MinConns = 1
	// Boşta kalan fazladan bağlantılar 30 dakika (varsayılan) değil 5 dakika sonra kapanır: paylaşılan
	// havuzlayıcıda kullanılmayan yuvaları tutmayalım.
	cfg.MaxConnIdleTime = 5 * time.Minute
	// pgx tek izleyici kabul eder; multitracer birden çoğunu sırayla çağırır.
	cfg.ConnConfig.Tracer = multitracer.New(append([]pgx.QueryTracer{queryTracer{}}, tracers...)...)
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, sessionLimits); err != nil {
			return fmt.Errorf("oturum süre sınırları ayarlanamadı: %w", err)
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("havuz açılamadı: %w", err)
	}

	// Havuz TEMBELDİR: gerçek bağlantıyı ilk sorguya kadar erteleyebilir.
	// Fail-fast felsefemiz (config'teki gibi) gereği boot'ta Ping ile
	// "gerçekten bağlanabiliyor muyuz?" sorusunu ŞİMDİ cevaplıyoruz —
	// yanlış şifreyi ilk müşteri isteğinde değil, açılışta öğren.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close() // yarım kalan havuzu bırakma
		return nil, fmt.Errorf("veritabanına ulaşılamadı (DATABASE_URL doğru mu? docker compose up -d çalıştı mı?): %w", err)
	}
	return pool, nil
}

// queryTracer, her sorgunun süresini isteğin sayacına (reqstats) yazar. BEGIN/COMMIT de birer sorgu
// sayılır: canlıda her biri veritabanına ayrı bir gidiş-dönüştür ve sayılması istenen de budur.
// Sorgu metni ve argümanları KAYDEDİLMEZ (log kuralı: istek verisi loglanmaz).
type queryTracer struct{}

type queryStartKey struct{}

func (queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	if reqstats.From(ctx) == nil {
		return ctx
	}
	return context.WithValue(ctx, queryStartKey{}, time.Now())
}

func (queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if start, ok := ctx.Value(queryStartKey{}).(time.Time); ok {
		reqstats.From(ctx).AddDB(time.Since(start))
	}
}
