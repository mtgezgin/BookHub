// Package testdb, entegrasyon testlerinin ORTAK Postgres kurulumu.
//
// Neden ayrı paket: her entegrasyon testi aynı ~15 satırı (env kontrolü, havuz, Ping,
// temizlik) kopyalıyordu — kopyalar zamanla ayrışır (birinde timeout değişir, diğerinde
// değişmez). Tek kaynak, tek davranış.
package testdb

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/database"
)

var (
	migrateOnce sync.Once
	migrateErr  error
	counter     atomic.Int64
)

// Pool, TEST_DATABASE_URL'e bağlı, şeması güncel bir havuz döner; değişken tanımlı
// değilse testi ATLAR (DB'siz `go test ./...` yeşil kalsın — CI bunu böyle koşar).
// Migration'lar süreç başına BİR kez uygulanır.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	migrateOnce.Do(func() { migrateErr = database.Migrate(ctx, dsn) })
	if migrateErr != nil {
		t.Fatalf("test veritabanına migration uygulanamadı (docker compose up -d çalıştı mı?): %v", migrateErr)
	}

	pool, err := database.New(ctx, dsn)
	if err != nil {
		t.Fatalf("test veritabanına bağlanılamadı: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Unique, test verisi için çakışmasız bir sonek üretir (slug/e-posta gibi UNIQUE
// alanlar). Paralel paketlerde de benzersiz kalsın diye zaman + sayaç.
func Unique(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), counter.Add(1))
}

// Exec, temizlik gibi yardımcı sorgular için; hata testi düşürür.
// Temizlikte kendi context'ini kullanır: test context'i iptal olsa bile satır kalmasın.
func Exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Errorf("yardımcı sorgu başarısız (%s): %v", sql, err)
	}
}
