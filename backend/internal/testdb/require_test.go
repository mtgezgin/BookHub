package testdb

import (
	"os"
	"testing"
)

// TestIntegrationEnvironment: REQUIRE_INTEGRATION=1 iken (CI) entegrasyon testlerinin ortamı EKSİKSE
// kırmızı biter (REVIEW_FEEDBACK F39). Entegrasyon testleri TEST_DATABASE_URL ya da TEST_REDIS_URL
// yokken kendilerini ATLAR; bu, geliştiricinin makinesinde doğru davranıştır. CI'da ise bir değişken
// adı değişirse ya da servis tanımı silinirse yüzlerce test koşmadan "yeşil" kalırdı.
func TestIntegrationEnvironment(t *testing.T) {
	if os.Getenv("REQUIRE_INTEGRATION") != "1" {
		t.Skip("REQUIRE_INTEGRATION=1 değil → ortam zorunlu tutulmuyor")
	}
	for _, name := range []string{"TEST_DATABASE_URL", "TEST_REDIS_URL"} {
		if os.Getenv(name) == "" {
			t.Errorf("%s tanımlı değil: entegrasyon testleri sessizce atlanırdı", name)
		}
	}
	// Değişken var ama veritabanı yoksa Pool zaten t.Fatal ile durur; burada gerçekten bağlan.
	if os.Getenv("TEST_DATABASE_URL") != "" {
		if err := Pool(t).Ping(t.Context()); err != nil {
			t.Errorf("test veritabanına ulaşılamadı: %v", err)
		}
	}
}
