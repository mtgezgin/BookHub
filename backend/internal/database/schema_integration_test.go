package database

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestEveryForeignKeyHasIndex: yabancı anahtar kolonu, o tablodaki bir index'in İLK kolonu olmalı
// (REVIEW_FEEDBACK F28). Olmazsa işaret edilen satır silinirken (ya da anahtarı değişirken) Postgres
// bağlı tablonun tamamını okur. Kural 0001'de yazılıydı ama üç kolonda atlanmıştı; artık yeni bir
// migration kuralı bozarsa bu test kırılır.
//
// Kısmi index sayılmaz: denetim tablonun bütün satırlarını arar. Tek istisna, yüklemi "kolon IS NOT
// NULL" olan kısmi index'tir; denetim NULL aramadığı için o index kullanılabilir.
func TestEveryForeignKeyHasIndex(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, provider := migratedTo(ctx, t, "bookhub_schema_fk", 1)
	if _, err := provider.Up(ctx); err != nil { // en son sürüme kadar
		t.Fatalf("migration'lar uygulanamadı: %v", err)
	}

	rows, err := conn.QueryContext(ctx, `
		SELECT c.conrelid::regclass::text, a.attname, c.conname
		FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
		WHERE c.contype = 'f'
		  AND c.connamespace = 'public'::regnamespace
		  AND NOT EXISTS (
		        SELECT 1 FROM pg_index i
		        WHERE i.indrelid = c.conrelid
		          AND i.indisvalid
		          AND i.indkey[0] = c.conkey[1]
		          AND (i.indpred IS NULL
		               OR pg_get_expr(i.indpred, i.indrelid) = format('(%I IS NOT NULL)', a.attname)))`)
	if err != nil {
		t.Fatalf("katalog sorgusu: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var missing []string
	for rows.Next() {
		var table, column, constraint string
		if err := rows.Scan(&table, &column, &constraint); err != nil {
			t.Fatal(err)
		}
		missing = append(missing, table+"."+column+" ("+constraint+")")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("index'siz yabancı anahtar(lar): %s", strings.Join(missing, ", "))
	}
}

// TestMigration0015Indexes: 0015 index'leri değiştirir, Down eski hâline döndürür; iki yönde de
// tablo verisi yerinde kalır.
func TestMigration0015Indexes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, provider := migratedTo(ctx, t, "bookhub_m0015", 14)

	indexes := func() map[string]bool {
		t.Helper()
		rows, err := conn.QueryContext(ctx, `SELECT indexname FROM pg_indexes
			WHERE schemaname = 'public' AND tablename IN ('books', 'comments', 'ratings', 'library_entries')`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		out := map[string]bool{}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatal(err)
			}
			out[name] = true
		}
		return out
	}
	added := []string{"comments_deleted_by_idx", "comments_book_recent_idx", "library_entries_book_id_idx", "ratings_book_score_idx", "books_status_recent_idx"}
	removed := []string{"comments_book_visible_idx", "ratings_book_id_idx", "books_approved_recent_idx", "books_pending_idx"}

	if _, err := conn.ExecContext(ctx, `INSERT INTO books (title, slug, author, language, status)
		VALUES ('Dune', 'dune', 'Frank Herbert', 'en', 'APPROVED')`); err != nil {
		t.Fatal(err)
	}

	if _, err := provider.UpTo(ctx, 15); err != nil {
		t.Fatalf("0015 uygulanamadı: %v", err)
	}
	got := indexes()
	for _, name := range added {
		if !got[name] {
			t.Errorf("0015'ten sonra %s olmalı", name)
		}
	}
	for _, name := range removed {
		if got[name] {
			t.Errorf("0015'ten sonra %s kalmamalı", name)
		}
	}

	if _, err := provider.DownTo(ctx, 14); err != nil {
		t.Fatalf("0015 geri alınamadı: %v", err)
	}
	got = indexes()
	for _, name := range removed {
		if !got[name] {
			t.Errorf("Down'dan sonra %s geri gelmeli", name)
		}
	}
	for _, name := range added {
		if got[name] {
			t.Errorf("Down'dan sonra %s kalmamalı", name)
		}
	}
	var n int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM books").Scan(&n); err != nil || n != 1 {
		t.Errorf("kitap verisi yerinde kalmalı: %d, %v", n, err)
	}
}
