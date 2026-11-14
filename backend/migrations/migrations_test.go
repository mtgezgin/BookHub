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

// TestDemoEditions: baskı migration'ı (0012, Faz 16) 0009 gibi demo kataloğa slug + yazarla bağlı ve
// make demo-seed'de psql ile yeniden çalışıyor. Kırılmaması gerekenler:
//   - varsayılan baskı satırları 0009'un kapak satırlarıyla BİREBİR aynı kitaplar ve aynı kapak adresi
//     (canlıda 0011'in kopyaladığı değer değişmesin; taze kurulumda boş kapak dolsun);
//   - her ek (özgün dil) baskı seed'deki bir kitaba bağlı, dili kitabın varsayılan baskısından farklı;
//   - dil kodları geçerli, kapak adresleri Open Library deseninde, Down bölümü yok.
func TestDemoEditions(t *testing.T) {
	sql := readFile(t, "0012_demo_editions.sql")
	covers := readFile(t, "0009_demo_book_covers.sql")
	seed := readFile(t, "../scripts/demo_seed.sql")
	if strings.Contains(sql, "+goose Down") {
		t.Error("baskı migration'ında Down bölümü var; make demo-seed onu da çalıştırırdı")
	}

	seedBooks := map[string]string{}
	for _, m := range regexp.MustCompile(`\(\s*'(?:[^']|'')*',\s*'([a-z0-9-]+)',\s*'((?:[^']|'')*)',`).FindAllStringSubmatch(seed, -1) {
		seedBooks[m[1]] = unquote(m[2])
	}
	coverOf := map[string]string{} // slug → 0009'daki adres
	for _, m := range regexp.MustCompile(`\('([a-z0-9-]+)',\s*'((?:[^']|'')*)',\s*'([^']+)'\)`).FindAllStringSubmatch(covers, -1) {
		coverOf[m[1]] = m[3]
	}

	str := `'((?:[^']|'')*)'`
	// (slug, yazar, dil, yayınevi, çevirmen, yıl, ad, kapak)
	primary := regexp.MustCompile(`\('([a-z0-9-]+)', `+str+`, '([a-z-A-Z]+)', `+str+`, `+str+`, (\d{4}|NULL::integer), ('(?:[^']|'')*'|NULL::text), '([^']+)'\)`).
		FindAllStringSubmatch(sql, -1)
	// (slug, yazar, ad, dil, yayınevi, yıl, isbn, sayfa, kapak)
	extra := regexp.MustCompile(`\('([a-z0-9-]+)', `+str+`, `+str+`, '([a-z-A-Z]+)', `+str+`, (\d{4}), '([0-9X]*)', (\d+|NULL::integer), '([^']+)'\)`).
		FindAllStringSubmatch(sql, -1)
	if want := strings.Count(sql, "covers.openlibrary.org/b/id/"); len(primary)+len(extra) != want {
		t.Fatalf("%d kapak adresinden %d satır okunabildi; okunamayan satır hiçbir denetimden geçmez", want, len(primary)+len(extra))
	}

	lang := regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)
	url := regexp.MustCompile(`^https://covers\.openlibrary\.org/b/id/[1-9][0-9]*-L\.jpg\?default=false$`)
	primaryLang := map[string]string{}
	for _, r := range primary {
		slug, author, l, u := r[1], unquote(r[2]), r[3], r[8]
		if _, dup := primaryLang[slug]; dup {
			t.Errorf("%s varsayılan baskıda iki kez geçiyor", slug)
		}
		primaryLang[slug] = l
		if seedBooks[slug] != author {
			t.Errorf("%s: yazar seed'dekiyle aynı değil (%q ↔ %q); satır hiçbir baskıya uygulanmaz", slug, author, seedBooks[slug])
		}
		if !lang.MatchString(l) {
			t.Errorf("%s: dil kodu geçersiz: %q", slug, l)
		}
		if coverOf[slug] != u {
			t.Errorf("%s: kapak 0009'dakiyle aynı olmalı (%q ↔ %q)", slug, u, coverOf[slug])
		}
	}
	for slug := range coverOf {
		if _, ok := primaryLang[slug]; !ok {
			t.Errorf("%s 0009'da kapaklı ama 0012'de varsayılan baskısı yok; taze kurulumda kapak boş kalır", slug)
		}
	}

	if len(extra) == 0 {
		t.Fatal("özgün dil baskısı okunamadı")
	}
	for _, r := range extra {
		slug, author, l, u := r[1], unquote(r[2]), r[4], r[9]
		if seedBooks[slug] != author {
			t.Errorf("ek baskı %s: seed'deki bir kitaba bağlı değil (yazar %q ↔ %q)", slug, author, seedBooks[slug])
		}
		if !lang.MatchString(l) {
			t.Errorf("ek baskı %s: dil kodu geçersiz: %q", slug, l)
		}
		if primaryLang[slug] == l {
			t.Errorf("ek baskı %s: dili varsayılan baskıyla aynı (%s); şerit iki dil göstermeli", slug, l)
		}
		if !url.MatchString(u) {
			t.Errorf("ek baskı %s: kapak adresi deseni yanlış: %s", slug, u)
		}
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s okunamadı: %v", name, err)
	}
	return string(b)
}

func unquote(s string) string { return strings.ReplaceAll(s, "''", "'") }
