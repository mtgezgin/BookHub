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
	db := readingDB{ctx: ctx, t: t, conn: conn}

	db.must(`INSERT INTO users (email, display_name, password_hash) VALUES ('okur@example.test', 'Okur', 'x')`)
	db.must(`INSERT INTO books (title, slug, author, language, status)
		SELECT 'Kitap ' || g, 'kitap-' || g, 'Yazar', 'tr', 'APPROVED' FROM generate_series(1, 5) g`)
	db.must(insertEntry, "kitap-1", "READ", "page", 120, 480, "2026-11-01") // bayat: eski kod durumu değiştirdi
	db.must(insertEntry, "kitap-2", "READING", "page", 120, 480, "2026-11-01")
	db.must(insertEntry, "kitap-3", "READING", "percent", 35, nil, nil)
	db.must(insertEntry, "kitap-4", "READING", nil, nil, nil, nil) // Faz 15'ten kalan okuma: "İlerlemeni gir"

	if _, err := provider.UpTo(ctx, 17); err != nil {
		t.Fatalf("0017 uygulanamadı: %v", err)
	}

	page, percent := sql.NullString{String: "page", Valid: true}, sql.NullString{String: "percent", Valid: true}
	n := func(v int32) sql.NullInt32 { return sql.NullInt32{Int32: v, Valid: true} }
	for slug, want := range map[string]readingRow{
		"kitap-1": {status: "READ"}, // bayat alanlar temizlendi, durum kaldı
		"kitap-2": {status: "READING", unit: page, progress: n(120), total: n(480), started: true},
		"kitap-3": {status: "READING", unit: percent, progress: n(35)},
		"kitap-4": {status: "READING"},
	} {
		if got := db.read(slug); got != want {
			t.Errorf("%s: %+v, beklenen %+v", slug, got, want)
		}
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
		if err := db.tryEntry(c.status, c.unit, c.progress, c.total, c.started); (err == nil) != c.ok {
			t.Errorf("%s: kabul edilmesi bekleniyordu = %v, veritabanının cevabı: %v", c.name, c.ok, err)
		}
	}

	if _, err := provider.DownTo(ctx, 16); err != nil {
		t.Fatalf("0017 geri alınamadı: %v", err)
	}
	if err := db.tryEntry("READ", "page", 120, 480, nil); err != nil {
		t.Errorf("Down'dan sonra kısıt kalmamalı: %v", err)
	}
}

// insertEntry, $1 slug'lı kitaba tek okurun kitaplık satırı: durum, birim, ilerleme, toplam, başlangıç.
const insertEntry = `INSERT INTO library_entries (user_id, book_id, status, unit, progress, total_pages, started_on)
	SELECT u.id, b.id, $2, $3, $4, $5, $6::date FROM users u, books b WHERE b.slug = $1`

// readingRow, bir kitaplık satırının okuma alanları (karşılaştırılabilir: beklenenle == ile kıyaslanır).
type readingRow struct {
	status          string
	unit            sql.NullString
	progress, total sql.NullInt32
	started         bool
}

// readingDB, 0017 testinin veritabanı yardımcıları; test fonksiyonu yalnız senaryoyu anlatsın.
type readingDB struct {
	ctx  context.Context
	t    *testing.T
	conn *sql.DB
}

func (d readingDB) must(q string, args ...any) {
	d.t.Helper()
	if _, err := d.conn.ExecContext(d.ctx, q, args...); err != nil {
		d.t.Fatalf("%s: %v", q, err)
	}
}

func (d readingDB) read(slug string) readingRow {
	d.t.Helper()
	var r readingRow
	var started sql.NullTime
	if err := d.conn.QueryRowContext(d.ctx, `
		SELECT le.status, le.unit, le.progress, le.total_pages, le.started_on
		FROM library_entries le JOIN books b ON b.id = le.book_id WHERE b.slug = $1`, slug).
		Scan(&r.status, &r.unit, &r.progress, &r.total, &started); err != nil {
		d.t.Fatalf("%s okunamadı: %v", slug, err)
	}
	r.started = started.Valid
	return r
}

// tryEntry, kitap-5'e bir satır yazmayı dener ve satırı siler: sıradaki durum aynı kitabı yeniden dener.
func (d readingDB) tryEntry(status string, unit, progress, total, started any) error {
	d.t.Helper()
	_, err := d.conn.ExecContext(d.ctx, insertEntry, "kitap-5", status, unit, progress, total, started)
	d.must(`DELETE FROM library_entries WHERE book_id = (SELECT id FROM books WHERE slug = 'kitap-5')`)
	return err
}
