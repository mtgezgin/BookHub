package database

import (
	"context"
	"fmt"

	// database/sql sürücüsü olarak pgx (stdlib): goose standart *sql.DB ister, uygulamanın
	// geri kalanı pgxpool kullanır. Aynı sürücü, iki arayüz.
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/mtgezgin/bookhub/backend/migrations"
)

// Migrate, gömülü migration'ların henüz uygulanmamış olanlarını sırayla uygular.
// Idempotent: şema güncelse hiçbir şey yapmaz. goose kendi goose_db_version
// tablosuyla nerede kaldığını bilir ("şema için git").
//
// UYARI (prod): Supabase'in transaction-mode pooler'ının (6543) arkasında migration
// ÇALIŞTIRMA: oturum seviyesindeki advisory lock orada tutulmaz. Session pooler ya da
// doğrudan bağlantı kullan; Supabase'te doğrudan bağlantı yalnız IPv6 olduğu için canlıda
// (ve GitHub runner'larında) session pooler. API'de otomatik migration bu yüzden bir ayar
// (AUTO_MIGRATE) ve varsayılanı kapalı.
func Migrate(ctx context.Context, dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("migration bağlantısı açılamadı: %w", err)
	}
	// KİLİT BEKLEME SINIRI (REVIEW_FEEDBACK F38): ALTER TABLE gibi bir deyim, tabloyu kullanan uzun bir
	// işlemin arkasında beklerken o tabloya gelen BÜTÜN sorguları kendi arkasına dizer. Sınırla
	// migration beklemek yerine hata verir; yayın durur, site çalışmaya devam eder, yeniden denenir.
	// Her migration dosyası bunu ayrıca SET LOCAL ile yazabilir; burası unutulanlar için varsayılandır.
	// statement_timeout BİLEREK yok: index kurmak ya da veri taşımak dakikalar sürebilir.
	db := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET lock_timeout = '5s'")
		return err
	}))
	// Kapanış hatası burada eyleme dönüştürülemez (bağlantı yalnızca migration için
	// açıldı ve iş bitti); yine de bilinçli olarak yok saydığımızı açıkça yazıyoruz.
	defer func() { _ = db.Close() }()

	// KİLİT: aynı anda birden fazla süreç migrate edebilir — paralel koşan test paketleri
	// (go test ./... her paketi ayrı süreçte çalıştırır) ya da AUTO_MIGRATE=true ile aynı
	// anda açılan birkaç API örneği. goose VARSAYILAN OLARAK KİLİTLEMEZ; iki süreç aynı
	// migration'ı "uygulanmamış" görüp ikisi birden çalıştırmaya kalkabilir.
	//
	// Dürüst not: kendi denememizde (4 eşzamanlı Up, taze DB, küçük migration'lar) bu yarış
	// yeniden ÜRETİLEMEDİ — pencere çok dar. Yine de "denemede çıkmadı" ≠ "olamaz"; goose'un
	// önerdiği Postgres advisory lock ile garantiye alıyoruz. Bekleyen süreç saniyede bir
	// yeniden dener (varsayılan 5 sn test takımını gereksiz yavaşlatıyordu), en çok 60 sn.
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 60))
	if err != nil {
		return fmt.Errorf("migration kilidi kurulamadı: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("migration sağlayıcısı kurulamadı: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migration uygulanamadı: %w", err)
	}
	return nil
}
