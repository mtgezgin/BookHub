package migrations

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestDemoBookCovers: kapak migration'ı demo kataloğun gerçek slug'larına bağlı. Seed'de bir slug
// değişirse o kapak sessizce hiç uygulanmaz; bu test onu yakalar. Dosya `make demo-seed`'de psql
// ile de çalıştığı için Down bölümü olamaz (psql ikisini de koşar, Down kapakları geri silerdi).
func TestDemoBookCovers(t *testing.T) {
	sql, err := os.ReadFile("0009_demo_book_covers.sql")
	if err != nil {
		t.Fatalf("kapak migration'ı okunamadı: %v", err)
	}
	seed, err := os.ReadFile("../scripts/demo_seed.sql")
	if err != nil {
		t.Fatalf("demo seed okunamadı: %v", err)
	}
	if strings.Contains(string(sql), "+goose Down") {
		t.Error("kapak migration'ında Down bölümü var; make demo-seed onu da çalıştırıp kapakları silerdi")
	}

	pairs := regexp.MustCompile(`\('([a-z0-9-]+)',\s*'([^']+)'\)`).FindAllStringSubmatch(string(sql), -1)
	if len(pairs) == 0 {
		t.Fatal("migration'da (slug, url) çifti yok")
	}
	url := regexp.MustCompile(`^https://covers\.openlibrary\.org/b/id/[1-9][0-9]*-L\.jpg$`)
	seen := map[string]bool{}
	for _, p := range pairs {
		slug, u := p[1], p[2]
		if seen[slug] {
			t.Errorf("%s iki kez geçiyor", slug)
		}
		seen[slug] = true
		if !url.MatchString(u) {
			t.Errorf("%s: kapak adresi Open Library kapak kimliği biçiminde değil: %s", slug, u)
		}
		if !strings.Contains(string(seed), "'"+slug+"'") {
			t.Errorf("%s demo seed'de yok; bu kapak hiçbir kitaba uygulanmaz", slug)
		}
	}
}
