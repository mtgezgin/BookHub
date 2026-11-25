package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestMigration0016AddsReadingColumns: okuma takibinin genişletme adımı (Faz 17A-1). Taze bir veritabanı
// 0015'e kadar kurulur ve Faz 15'ten kalma bir "Şu an okuyorum" kaydı yazılır. 0016'dan sonra:
//   - kayıt yerinde, okuma alanları boş ("ilerleme henüz girilmedi"), version 1;
//   - ESKİ kodun yazımı (UpsertLibraryEntry'nin sqlc'nin ürettiği hâli: kolon listeli INSERT, açık
//     kolonlu RETURNING) hâlâ çalışır — yayın penceresinde canlıdaki eski kod budur;
//   - reading_log kısıtlarını baştan taşır (bitiş başlangıçtan önce olamaz, sonuç iki değerden biri).
//
// Down tabloyu ve kolonları kaldırır; kitaplık kaydı kalır.
func TestMigration0016AddsReadingColumns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, provider := migratedTo(ctx, t, "bookhub_m0016", 15)

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
	must(`INSERT INTO books (title, slug, author, language, status) VALUES
		('Dune', 'dune', 'Frank Herbert', 'en', 'APPROVED'),
		('Vakıf', 'vakif', 'Isaac Asimov', 'tr', 'APPROVED')`)
	must(`INSERT INTO library_entries (user_id, book_id, status)
		SELECT u.id, b.id, 'READING' FROM users u, books b WHERE b.slug = 'dune'`)

	if _, err := provider.UpTo(ctx, 16); err != nil {
		t.Fatalf("0016 uygulanamadı: %v", err)
	}

	var (
		status   string
		unit     sql.NullString
		progress sql.NullInt32
		started  sql.NullTime
		version  int
	)
	if err := conn.QueryRowContext(ctx, `
		SELECT le.status, le.unit, le.progress, le.started_on, le.version
		FROM library_entries le JOIN books b ON b.id = le.book_id WHERE b.slug = 'dune'`).
		Scan(&status, &unit, &progress, &started, &version); err != nil {
		t.Fatalf("Faz 15 kaydı okunamadı: %v", err)
	}
	if status != "READING" || unit.Valid || progress.Valid || started.Valid || version != 1 {
		t.Errorf("Faz 15 kaydı olduğu gibi kalmalı, okuma alanları boş ve version 1 olmalı: %s %v %v %v %d",
			status, unit, progress, started, version)
	}

	// Eski kodun yazımı (internal/db/library.sql.go, 0016 öncesi): yeni kolonları anmaz.
	var returned string
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO library_entries (user_id, book_id, status)
		SELECT u.id, b.id, 'WANT_TO_READ' FROM users u, books b WHERE b.slug = 'vakif' AND b.status = 'APPROVED'
		ON CONFLICT (user_id, book_id) DO UPDATE SET status = EXCLUDED.status, updated_at = now()
		RETURNING user_id, book_id, status, created_at, updated_at`).
		Scan(new(int64), new(int64), &returned, new(time.Time), new(time.Time)); err != nil || returned != "WANT_TO_READ" {
		t.Errorf("eski kodun yazımı 0016'dan sonra da çalışmalı: %q, %v", returned, err)
	}

	logRow := `INSERT INTO reading_log (user_id, book_id, outcome, started_on, ended_on)
		SELECT u.id, b.id, $1, $2::date, $3::date FROM users u, books b WHERE b.slug = 'dune'`
	if err := exec(logRow, "FINISHED", "2026-11-01", "2026-11-12"); err != nil {
		t.Errorf("geçerli günlük satırı yazılmalı: %v", err)
	}
	if err := exec(logRow, "FINISHED", nil, nil); err != nil {
		t.Errorf("tarihsiz günlük satırı (\"Zaten okudum\") yazılmalı: %v", err)
	}
	if err := exec(logRow, "FINISHED", "2026-11-12", "2026-11-01"); err == nil {
		t.Error("bitiş başlangıçtan önce olan günlük satırı reddedilmeli")
	}
	if err := exec(logRow, "READING", nil, nil); err == nil {
		t.Error("FINISHED/ABANDONED dışındaki sonuç reddedilmeli")
	}

	if _, err := provider.DownTo(ctx, 15); err != nil {
		t.Fatalf("0016 geri alınamadı: %v", err)
	}
	var leftovers int
	if err := conn.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM information_schema.columns
		        WHERE table_name = 'library_entries'
		          AND column_name IN ('edition_id', 'unit', 'progress', 'total_pages', 'started_on', 'version'))
		     + (SELECT count(*) FROM information_schema.tables WHERE table_name = 'reading_log')`).
		Scan(&leftovers); err != nil {
		t.Fatal(err)
	}
	if leftovers != 0 {
		t.Errorf("Down'dan sonra 0016'nın %d kolonu/tablosu kaldı", leftovers)
	}
	var entries int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM library_entries").Scan(&entries); err != nil || entries != 2 {
		t.Errorf("kitaplık kayıtları Down'dan sonra yerinde kalmalı: %d, %v", entries, err)
	}
}
