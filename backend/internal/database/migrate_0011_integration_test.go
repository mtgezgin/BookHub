package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/mtgezgin/bookhub/backend/migrations"
)

// TestMigration0011CopiesBooksIntoEditions: 0011 taşıması (Faz 16, Doğrulama 3). Taze bir
// veritabanı 0010'a kadar kurulur, eski şemaya kitaplar yazılır, sonra 0011 uygulanır:
//   - N kitap → N baskı, hepsi varsayılan;
//   - kapak, sayfa sayısı, başlık ve dil birebir kopyalanır;
//   - görünüm (book_view) eski kolonlardaki değerleri aynen verir; puan istatistikleri değişmez;
//   - Down görünümü ve tabloyu kaldırır, books dokunulmamış kalır.
func TestMigration0011CopiesBooksIntoEditions(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	admin, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("bağlantı: %v", err)
	}
	t.Cleanup(admin.Close)
	name := fmt.Sprintf("bookhub_m0011_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Skipf("geçici veritabanı açılamadı (yetki?): %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("geçici veritabanı silinemedi: %v", err)
		}
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("DSN: %v", err)
	}
	u.Path = "/" + name

	conn, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 10); err != nil {
		t.Fatalf("0010'a kadar: %v", err)
	}

	// Eski şemada üç kitap: kapaklı + sayfalı + puanlı, boş kapaklı + sayfasız, Rusça.
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO books (title, slug, author, cover_url, page_count, language, status, average_rating, rating_count) VALUES
		  ('Suç ve Ceza', 'suc-ve-ceza', 'Dostoyevski', 'https://example.test/suc.jpg', 687, 'ru', 'APPROVED', 8.50, 4),
		  ('Dune', 'dune', 'Frank Herbert', '', NULL, 'en', 'APPROVED', NULL, 0),
		  ('Bekleyen', 'bekleyen', 'Yazar', 'https://example.test/b.jpg', 120, 'tr', 'PENDING', NULL, 0)`); err != nil {
		t.Fatalf("eski şemaya kitap yazılamadı: %v", err)
	}

	type snap struct {
		id                  int64
		cover               string
		pages               sql.NullInt32
		avg                 sql.NullFloat64
		count               int32
		title, lang, status string
	}
	read := func(q string) map[int64]snap {
		t.Helper()
		rows, err := conn.QueryContext(ctx, q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		defer func() { _ = rows.Close() }()
		out := map[int64]snap{}
		for rows.Next() {
			var s snap
			if err := rows.Scan(&s.id, &s.cover, &s.pages, &s.avg, &s.count, &s.title, &s.lang, &s.status); err != nil {
				t.Fatal(err)
			}
			out[s.id] = s
		}
		return out
	}
	const cols = `id, cover_url, page_count, average_rating, rating_count, title, language, status`
	before := read(`SELECT ` + cols + ` FROM books`)

	if _, err := provider.UpTo(ctx, 11); err != nil {
		t.Fatalf("0011 uygulanamadı: %v", err)
	}

	after := read(`SELECT ` + cols + ` FROM book_view`)
	if len(after) != len(before) {
		t.Fatalf("görünümde %d kitap, öncesinde %d", len(after), len(before))
	}
	for id, b := range before {
		if after[id] != b {
			t.Errorf("kitap %d: görünüm eski değerleri birebir vermeli\nönce:  %+v\nsonra: %+v", id, b, after[id])
		}
	}

	var total, primaries, mismatched int
	if err := conn.QueryRowContext(ctx, `
		SELECT count(*), count(*) FILTER (WHERE e.is_primary),
		       count(*) FILTER (WHERE e.title <> b.title OR e.language <> b.language
		                          OR e.cover_url <> b.cover_url
		                          OR e.page_count IS DISTINCT FROM b.page_count)
		FROM editions e JOIN books b ON b.id = e.book_id`).Scan(&total, &primaries, &mismatched); err != nil {
		t.Fatal(err)
	}
	if total != len(before) || primaries != len(before) || mismatched != 0 {
		t.Errorf("her kitaba birebir kopya tek varsayılan baskı: toplam %d, varsayılan %d, farklı %d (kitap %d)",
			total, primaries, mismatched, len(before))
	}

	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("0011 Down: %v", err)
	}
	var views, tables int
	if err := conn.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM pg_views WHERE viewname = 'book_view'),
		       (SELECT count(*) FROM pg_tables WHERE tablename = 'editions')`).Scan(&views, &tables); err != nil {
		t.Fatal(err)
	}
	if views != 0 || tables != 0 {
		t.Errorf("Down görünümü ve tabloyu kaldırmalı: görünüm %d, tablo %d", views, tables)
	}
	if got := read(`SELECT ` + cols + ` FROM books`); len(got) != len(before) {
		t.Errorf("Down books'a dokunmamalı: %d kitap", len(got))
	}
}
