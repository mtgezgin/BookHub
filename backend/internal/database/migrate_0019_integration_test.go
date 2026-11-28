package database

import (
	"context"
	"testing"
	"time"
)

// Canlıdaki kodun (0018 dönemi) sorgu metinleri, internal/db/reviews.sql.go'nun o günkü hâlinden
// aynen. Genişletme migration'ının sözü "eski kod çalışmaya devam eder"; söz bu metinlerle sınanır.
const (
	oldInsertReview = `INSERT INTO reviews (user_id, book_id, body, has_spoiler)
SELECT $1, b.id, $2, $3
FROM books b
WHERE b.id = $4 AND b.status = 'APPROVED'
ON CONFLICT (user_id, book_id) DO NOTHING
RETURNING id, user_id, book_id, body, has_spoiler, version, created_at, edited_at, hidden_at, hidden_by, hidden_reason`
	oldUpdateReview = `UPDATE reviews
SET body = $1, has_spoiler = $2,
    edited_at = now(), version = version + 1
WHERE id = $3
RETURNING id, user_id, book_id, body, has_spoiler, version, created_at, edited_at, hidden_at, hidden_by, hidden_reason`
)

// TestMigration0019ReviewDoc: incelemenin belge kolonu (Faz 17C, genişletme). Veritabanı 0018'e kadar
// kurulur, eski kodla bir inceleme yazılır, 0019 uygulanır. Sınananlar:
//   - var olan satır yerinde, belgesi boş (NULL);
//   - eski kodun yazımı ve düzenlemesi 0019'dan sonra da çalışır, belgeyi boş bırakır;
//   - kolon yalnız JSON nesnesi kabul eder: JSON null, dizi, metin ve sayı reddedilir;
//   - belgeyi boşaltmak (SQL NULL) serbesttir.
//
// Down kolonu kaldırır, inceleme kalır.
func TestMigration0019ReviewDoc(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, provider := migratedTo(ctx, t, "bookhub_m0019", 18)
	db := schemaDB{ctx: ctx, t: t, conn: conn}

	db.must(`INSERT INTO users (email, display_name, password_hash, role) VALUES
		('okur@example.test', 'Okur', 'x', 'USER'), ('ikinci@example.test', 'İkinci', 'x', 'USER')`)
	db.must(`INSERT INTO books (title, slug, author, language, status) VALUES
		('Kitap', 'kitap', 'Yazar', 'tr', 'APPROVED')`)
	db.must(oldInsertReview, userID(db, "okur"), "Eski inceleme.", true, bookID(db))

	if _, err := provider.UpTo(ctx, 19); err != nil {
		t.Fatalf("0019 uygulanamadı: %v", err)
	}

	if got := db.count(`SELECT count(*) FROM reviews WHERE body = 'Eski inceleme.' AND has_spoiler AND body_doc IS NULL`); got != 1 {
		t.Errorf("var olan inceleme yerinde ve belgesiz olmalı: %d satır", got)
	}

	// Eski kod, yeni şema.
	db.must(oldInsertReview, userID(db, "ikinci"), "0019'dan sonra eski kodla.", false, bookID(db))
	db.must(oldUpdateReview, "Eski kodla düzenlendi.", false, reviewID(db, "okur"))
	if got := db.count(`SELECT count(*) FROM reviews WHERE body_doc IS NULL`); got != 2 {
		t.Errorf("eski kodun yazdığı iki inceleme de belgesiz olmalı: %d satır", got)
	}
	if got := db.count(`SELECT count(*) FROM reviews WHERE body = 'Eski kodla düzenlendi.' AND version = 2`); got != 1 {
		t.Errorf("eski kodun düzenlemesi uygulanmalı: %d satır", got)
	}

	for _, c := range []struct {
		name, doc string
		ok        bool
	}{
		{"belge nesnesi", `'{"type":"doc","content":[{"type":"paragraph"}]}'::jsonb`, true},
		{"JSON null", `'null'::jsonb`, false},
		{"dizi", `'[]'::jsonb`, false},
		{"metin", `'"<p>html</p>"'::jsonb`, false},
		{"sayı", `'5'::jsonb`, false},
		{"SQL NULL (belgeyi boşaltmak)", `NULL`, true},
	} {
		err := db.try(`UPDATE reviews SET body_doc = `+c.doc+` WHERE id = $1`, reviewID(db, "okur"))
		if (err == nil) != c.ok {
			t.Errorf("%s: kabul edilmesi bekleniyordu = %v, veritabanının cevabı: %v", c.name, c.ok, err)
		}
	}

	if _, err := provider.DownTo(ctx, 18); err != nil {
		t.Fatalf("0019 geri alınamadı: %v", err)
	}
	if db.count(`SELECT count(*) FROM information_schema.columns WHERE table_name = 'reviews' AND column_name = 'body_doc'`) != 0 {
		t.Error("Down'dan sonra body_doc kolonu kalmamalı")
	}
	if got := db.count(`SELECT count(*) FROM reviews`); got != 2 {
		t.Errorf("Down incelemeleri silmemeli: %d inceleme, beklenen 2", got)
	}
}

func userID(db schemaDB, name string) int {
	return db.count(`SELECT id FROM users WHERE email = $1`, name+"@example.test")
}

func bookID(db schemaDB) int {
	return db.count(`SELECT id FROM books WHERE slug = 'kitap'`)
}

func reviewID(db schemaDB, user string) int {
	return db.count(`SELECT r.id FROM reviews r JOIN users u ON u.id = r.user_id WHERE u.email = $1`, user+"@example.test")
}
