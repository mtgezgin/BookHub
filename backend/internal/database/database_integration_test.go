package database

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestNew_MutluYol, New'in MUTLU yolunu gerçek Postgres'e karşı doğrular:
// hata yolları (bozuk DSN, ulaşılamayan hedef) database_test.go'da DB'siz
// kanıtlanıyor, ama "çalışan bir havuz gerçekten döndü mü" sorusu ancak
// canlı bir veritabanıyla cevaplanabilir.
//
// Asıl iddia: New yalnızca nesne kurmakla kalmaz, boot'ta Ping ile bağlantıyı
// SINAR (pgxpool tembeldir; Ping olmasa hata ilk isteğe kadar saklanırdı).
// Bu yüzden dönen havuzun hemen kullanılabilir olması beklenir.
//
// Çalıştırmak için:
//
//	docker compose up -d
//	TEST_DATABASE_URL='postgres://bookhub:bookhub_dev@localhost:5432/bookhub?sslmode=disable' \
//	  go test ./internal/database/...
func TestNew_MutluYol(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}

	ctx := context.Background()

	pool, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("geçerli DSN ile havuz kurulamadı (docker compose up -d çalıştı mı?): %v", err)
	}
	if pool == nil {
		t.Fatal("hata yokken havuz nil dönmemeliydi")
	}
	// Havuzu testin sonunda kapat — main.go'daki defer pool.Close()'un karşılığı.
	t.Cleanup(pool.Close)

	// Havuz gerçekten KULLANILABİLİR mi? Ping yetmez, bir sorgu koştur.
	sorguCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var bir int
	if err := pool.QueryRow(sorguCtx, "SELECT 1").Scan(&bir); err != nil {
		t.Fatalf("dönen havuzla sorgu çalıştırılamadı: %v", err)
	}
	if bir != 1 {
		t.Errorf("sorgu sonucu: istenen 1, gelen %d", bir)
	}
}
