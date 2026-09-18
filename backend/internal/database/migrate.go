package database

import (
	"context"
	"database/sql"
	"fmt"

	// database/sql sürücüsü olarak pgx: goose standart *sql.DB ister, uygulamanın
	// geri kalanı pgxpool kullanır. Aynı sürücü, iki arayüz.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/mtgezgin/bookhub/backend/migrations"
)

// Migrate, gömülü migration'ların henüz uygulanmamış olanlarını sırayla uygular.
// Idempotent: şema güncelse hiçbir şey yapmaz. goose kendi goose_db_version
// tablosuyla nerede kaldığını bilir ("şema için git").
//
// UYARI (prod): Supabase gibi transaction-mode pooler'ın arkasında migration
// ÇALIŞTIRMA — doğrudan bağlantıyı (5432) kullan. Bu yüzden API'de otomatik
// migration bir ayar (AUTO_MIGRATE) ve varsayılanı kapalı.
func Migrate(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("migration bağlantısı açılamadı: %w", err)
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("migration sağlayıcısı kurulamadı: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migration uygulanamadı: %w", err)
	}
	return nil
}
