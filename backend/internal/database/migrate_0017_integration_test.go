package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestMigration0017ReadingChecks: okuma takibinin daraltma adımı (Faz 17A-2). Taze bir veritabanı 0016'ya
// kadar kurulur ve 0016'nın kısıtsız bıraktığı satırlar yazılır:
//   - yayın penceresinde eski kodun bayat bıraktığı satır (READ ama okuma alanları dolu) → temizlenir;
//   - geçerli okumalar (sayfa, yüzde, Faz 15'ten kalan boş okuma) → olduğu gibi kalır.
//
// 0017'den sonra her kural dışı yazım veritabanında reddedilir (spec Doğrulama 11). Tablo NULL'lı
// durumları da sınar: CHECK ifadesi NULL dönerse Postgres satırı KABUL eder; spec'teki taslak bu
// yüzden "birim yok, ilerleme 5" satırını geçiriyordu.
//
// Down kısıtları kaldırır (temizlik geri gelmez).
func TestMigration0017ReadingChecks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, provider := migratedTo(ctx, t, "bookhub_m0017", 16)

	exec := func(q string, args ...any) error {
		t.Helper()
		_, err := conn.ExecContext(ctx, q, args...)
		return err
	}
	must := func(q string, args ...any) {
		t.Helper()
		if err := exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	must(`INSERT INTO users (email, display_name, password_hash) VALUES ('okur@example.test', 'Okur', 'x')`)
	must(`INSERT INTO books (title, slug, author, language, status)
		SELECT 'Kitap ' || g, 'kitap-' || g, 'Yazar', 'tr', 'APPROVED' FROM generate_series(1, 5) g`)
	entry := `INSERT INTO library_entries (user_id, book_id, status, unit, progress, total_pages, started_on)
		SELECT u.id, b.id, $2, $3, $4, $5, $6::date FROM users u, books b WHERE b.slug = $1`
	must(entry, "kitap-1", "READ", "page", 120, 480, "2026-11-01") // bayat: eski kod durumu değiştirdi
	must(entry, "kitap-2", "READING", "page", 120, 480, "2026-11-01")
	must(entry, "kitap-3", "READING", "percent", 35, nil, nil)
	must(entry, "kitap-4", "READING", nil, nil, nil, nil) // Faz 15'ten kalan okuma: "İlerlemeni gir"

	if _, err := provider.UpTo(ctx, 17); err != nil {
		t.Fatalf("0017 uygulanamadı: %v", err)
	}

	type row struct {
		status          string
		unit            sql.NullString
		progress, total sql.NullInt32
		started         sql.NullTime
	}
	read := func(slug string) row {
		t.Helper()
		var r row
		if err := conn.QueryRowContext(ctx, `
			SELECT le.status, le.unit, le.progress, le.total_pages, le.started_on
			FROM library_entries le JOIN books b ON b.id = le.book_id WHERE b.slug = $1`, slug).
			Scan(&r.status, &r.unit, &r.progress, &r.total, &r.started); err != nil {
			t.Fatalf("%s okunamadı: %v", slug, err)
		}
		return r
	}
	if r := read("kitap-1"); r.status != "READ" || r.unit.Valid || r.progress.Valid || r.total.Valid || r.started.Valid {
		t.Errorf("bayat okuma alanları temizlenmeli, durum kalmalı: %+v", r)
	}
	if r := read("kitap-2"); r.unit.String != "page" || r.progress.Int32 != 120 || r.total.Int32 != 480 || !r.started.Valid {
		t.Errorf("geçerli sayfa okuması değişmemeli: %+v", r)
	}
	if r := read("kitap-3"); r.unit.String != "percent" || r.progress.Int32 != 35 {
		t.Errorf("geçerli yüzde okuması değişmemeli: %+v", r)
	}
	if r := read("kitap-4"); r.status != "READING" || r.unit.Valid || r.progress.Valid {
		t.Errorf("Faz 15'ten kalan okuma değişmemeli: %+v", r)
	}

	// Her durum kitap-5'e yazılır, sonra silinir: kabul edilenin ardından reddedilen aynı kitabı dener.
	write := func(status string, unit, progress, total, started any) error {
		t.Helper()
		err := exec(entry, "kitap-5", status, unit, progress, total, started)
		must(`DELETE FROM library_entries WHERE book_id = (SELECT id FROM books WHERE slug = 'kitap-5')`)
		return err
	}
	for _, c := range []struct {
		name                           string
		status                         string
		unit, progress, total, started any
		ok                             bool
	}{
		{"sayfa, toplam biliniyor", "READING", "page", 480, 480, "2026-11-01", true},
		{"sayfa, toplam bilinmiyor", "READING", "page", 120, nil, nil, true},
		{"yüzde 0 ve 100", "READING", "percent", 100, nil, nil, true},
		{"girilmemiş ilerleme", "READING", nil, nil, nil, "2026-11-01", true},
		{"okunmayan kitapta yalnız durum", "WANT_TO_READ", nil, nil, nil, nil, true},

		{"okunmayan kitapta ilerleme", "READ", "page", 120, 480, nil, false},
		{"okunmayan kitapta başlangıç günü", "ABANDONED", nil, nil, nil, "2026-11-01", false},
		{"toplamı aşan sayfa", "READING", "page", 481, 480, nil, false},
		{"negatif sayfa", "READING", "page", -1, nil, nil, false},
		{"sıfır sayfa toplamı", "READING", "page", 0, 0, nil, false},
		{"100'ü aşan yüzde", "READING", "percent", 101, nil, nil, false},
		{"toplamlı yüzde", "READING", "percent", 35, 480, nil, false},
		{"bilinmeyen birim", "READING", "chapter", 3, nil, nil, false},
		// NULL tuzağı: spec'teki taslak bu üçünü geçiriyordu.
		{"birimsiz ilerleme", "READING", nil, 5, nil, nil, false},
		{"ilerlemesiz sayfa", "READING", "page", nil, 480, nil, false},
		{"ilerlemesiz yüzde", "READING", "percent", nil, nil, nil, false},
	} {
		err := write(c.status, c.unit, c.progress, c.total, c.started)
		if c.ok && err != nil {
			t.Errorf("%s: kabul edilmeli: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: veritabanı reddetmeli", c.name)
		}
	}

	if _, err := provider.DownTo(ctx, 16); err != nil {
		t.Fatalf("0017 geri alınamadı: %v", err)
	}
	if err := write("READ", "page", 120, 480, nil); err != nil {
		t.Errorf("Down'dan sonra kısıt kalmamalı: %v", err)
	}
}
