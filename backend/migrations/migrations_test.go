package migrations

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// noCover: Open Library'de kapağı olmadığı için kapak migration'ında BİLEREK olmayan demo kitaplar.
var noCover = map[string]string{
	"tehlikeli-oyunlar": "Open Library'de kapağı yok; üretilmiş kapak kalır",
}

// TestDemoBookCovers: kapak migration'ı demo kataloğun gerçek kitaplarına (slug + yazar) bağlı. Seed'de
// bir kitap değişirse o kapak sessizce hiç uygulanmaz; seed'e kitap eklenip kapağı unutulursa da
// fark edilmez. Dosya `make demo-seed`'de psql ile de çalıştığı için Down bölümü olamaz (psql ikisini
// de koşar, Down kapakları geri silerdi).
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

	// Seed'deki kitaplar: ('Başlık', 'slug', 'Yazar', … — slug → yazar.
	seedBooks := map[string]string{}
	for _, m := range regexp.MustCompile(`\(\s*'(?:[^']|'')*',\s*'([a-z0-9-]+)',\s*'((?:[^']|'')*)',`).FindAllStringSubmatch(string(seed), -1) {
		seedBooks[m[1]] = strings.ReplaceAll(m[2], "''", "'")
	}
	if len(seedBooks) == 0 {
		t.Fatal("demo seed'de kitap bulunamadı (seed biçimi değişti mi?)")
	}

	rows := regexp.MustCompile(`\('([a-z0-9-]+)',\s*'((?:[^']|'')*)',\s*'([^']+)'\)`).FindAllStringSubmatch(string(sql), -1)
	if want := strings.Count(string(sql), "covers.openlibrary.org"); len(rows) != want {
		t.Fatalf("%d kapak adresinden %d satır okunabildi; okunamayan satır hiçbir denetimden geçmez", want, len(rows))
	}
	url := regexp.MustCompile(`^https://covers\.openlibrary\.org/b/id/[1-9][0-9]*-L\.jpg\?default=false$`)
	covered := map[string]bool{}
	for _, r := range rows {
		slug, author, u := r[1], strings.ReplaceAll(r[2], "''", "'"), r[3]
		if covered[slug] {
			t.Errorf("%s iki kez geçiyor", slug)
		}
		covered[slug] = true
		if !url.MatchString(u) {
			t.Errorf("%s: kapak adresi Open Library kapak kimliği biçiminde değil (?default=false dahil): %s", slug, u)
		}
		seedAuthor, ok := seedBooks[slug]
		switch {
		case !ok:
			t.Errorf("%s demo seed'de yok; bu kapak hiçbir kitaba uygulanmaz", slug)
		case seedAuthor != author:
			t.Errorf("%s: yazar seed'dekiyle aynı değil (%q ↔ %q); kapak uygulanmaz", slug, author, seedAuthor)
		}
	}
	for slug := range seedBooks {
		if _, skip := noCover[slug]; !covered[slug] && !skip {
			t.Errorf("%s seed'de var ama kapağı yok; kapak ekle ya da noCover'a gerekçesiyle yaz", slug)
		}
	}
}
