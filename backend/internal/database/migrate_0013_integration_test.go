package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestMigration0013DropsLegacyBookColumns: daraltma (Faz 16, Doğrulama 7'nin şema tarafı). Taze bir
// veritabanı 0012'ye kadar kurulur; iki kitap yazılır:
//   - "baskılı": 0011'den sonra yeni kodla eklenmiş gibi, varsayılan baskısı var ve baskının kapağı
//     eski kolondakinden farklı (yeni kod yalnız baskıyı günceller);
//   - "arada": 0011 ile yeni kodun yayını arasında ESKİ kodla eklenmiş gibi, baskısı yok.
//
// 0013'ten sonra: "arada" eski kolonlardaki kapak ve sayfayla varsayılan baskı kazanır, "baskılı"nın
// baskısına dokunulmaz, iki kolon yoktur ve görünüm değerleri baskıdan verir. Down kolonları
// varsayılan baskının değerleriyle geri getirir.
func TestMigration0013DropsLegacyBookColumns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, provider := migratedTo(ctx, t, "bookhub_m0013", 12)

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO books (title, slug, author, cover_url, page_count, language, status) VALUES
		('Baskılı', 'baskili', 'Yazar', 'https://example.test/eski.jpg', 100, 'tr', 'APPROVED'),
		('Arada', 'arada', 'Yazar', 'https://example.test/arada.jpg', 250, 'en', 'APPROVED')`)
	exec(`INSERT INTO editions (book_id, title, language, page_count, cover_url, is_primary)
		SELECT id, title, language, 320, 'https://example.test/yeni.jpg', true FROM books WHERE slug = 'baskili'`)

	if _, err := provider.UpTo(ctx, 13); err != nil {
		t.Fatalf("0013 uygulanamadı: %v", err)
	}

	type view struct {
		cover    string
		pages    sql.NullInt32
		editions int
	}
	read := func(slug string) view {
		t.Helper()
		var v view
		if err := conn.QueryRowContext(ctx, `
			SELECT bv.cover_url, bv.page_count,
			       (SELECT count(*) FROM editions e WHERE e.book_id = bv.id AND e.is_primary)
			FROM book_view bv WHERE bv.slug = $1`, slug).Scan(&v.cover, &v.pages, &v.editions); err != nil {
			t.Fatalf("%s okunamadı: %v", slug, err)
		}
		return v
	}
	want := map[string]view{
		"baskili": {"https://example.test/yeni.jpg", sql.NullInt32{Int32: 320, Valid: true}, 1},
		"arada":   {"https://example.test/arada.jpg", sql.NullInt32{Int32: 250, Valid: true}, 1},
	}
	for slug, w := range want {
		if got := read(slug); got != w {
			t.Errorf("%s: görünüm varsayılan baskıyı vermeli\n got: %+v\nwant: %+v", slug, got, w)
		}
	}

	var legacy int
	if err := conn.QueryRowContext(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'books' AND column_name IN ('cover_url', 'page_count')`).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	if legacy != 0 {
		t.Errorf("0013 sonrası books'ta %d eski kolon kaldı", legacy)
	}

	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("0013 Down: %v", err)
	}
	for slug, w := range want {
		var cover string
		var pages sql.NullInt32
		if err := conn.QueryRowContext(ctx, `SELECT cover_url, page_count FROM books WHERE slug = $1`, slug).Scan(&cover, &pages); err != nil {
			t.Fatalf("Down sonrası %s: %v", slug, err)
		}
		if cover != w.cover || pages != w.pages {
			t.Errorf("%s: Down eski kolonlara varsayılan baskının değerlerini yazmalı: %q %v", slug, cover, pages)
		}
	}
}
