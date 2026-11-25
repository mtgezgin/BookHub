package database

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestMigration0018Reviews: inceleme tablosu (Faz 17B). Veritabanı 0017'ye kadar kurulur, 0018 uygulanır.
// Sınananlar:
//   - kullanıcı başına kitapta tek inceleme (Doğrulama 12'nin veritabanı yarısı);
//   - metin 1–5000 karakter, gizleme gerekçesi en çok 500;
//   - gizleme tutarlılığı iki yönlü: gizliyse gerekçe dolu, görünürse boş;
//   - silmeler: yazar silinince inceleme gider, kitap silinince gider, gizleyen yönetici silinince gizleme
//     kalır ve yalnız "kim gizledi" boşalır; puanı silmek incelemeyi silmez.
//
// Down tabloyu kaldırır.
func TestMigration0018Reviews(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, provider := migratedTo(ctx, t, "bookhub_m0018", 17)
	if _, err := provider.UpTo(ctx, 18); err != nil {
		t.Fatalf("0018 uygulanamadı: %v", err)
	}
	db := schemaDB{ctx: ctx, t: t, conn: conn}

	db.must(`INSERT INTO users (email, display_name, password_hash, role) VALUES
		('okur@example.test', 'Okur', 'x', 'USER'), ('admin@example.test', 'Yönetici', 'x', 'ADMIN'),
		('ikinci@example.test', 'İkinci', 'x', 'USER')`)
	db.must(`INSERT INTO books (title, slug, author, language, status)
		SELECT 'Kitap ' || g, 'kitap-' || g, 'Yazar', 'tr', 'APPROVED' FROM generate_series(1, 3) g`)

	for _, c := range []struct {
		name, sql string
		ok        bool
	}{
		{"geçerli inceleme", review("okur", "kitap-1", "'Güzel bir kitap.'"), true},
		{"aynı kitaba ikinci inceleme", review("okur", "kitap-1", "'Yine güzel.'"), false},
		{"başka okurun aynı kitaba incelemesi", review("ikinci", "kitap-1", "'Bence de.'"), true},
		{"boş metin", review("okur", "kitap-2", "''"), false},
		{"5000 karakter", review("okur", "kitap-2", "repeat('a', 5000)"), true},
		{"5001 karakter", review("okur", "kitap-3", "repeat('a', 5001)"), false},
		{"gerekçesiz gizleme", hide("kitap-1", "''"), false},
		{"501 karakter gerekçe", hide("kitap-1", "repeat('g', 501)"), false},
		{"gerekçeli gizleme", hide("kitap-1", "'Kişisel bilgi içeriyor'"), true},
		{"gerekçesi kalan geri açma", `UPDATE reviews SET hidden_at = NULL, hidden_by = NULL WHERE hidden_at IS NOT NULL`, false},
	} {
		if err := db.try(c.sql); (err == nil) != c.ok {
			t.Errorf("%s: kabul edilmesi bekleniyordu = %v, veritabanının cevabı: %v", c.name, c.ok, err)
		}
	}

	// Silmeler. Önce puan: inceleme kalmalı.
	db.must(`INSERT INTO ratings (user_id, book_id, score)
		SELECT u.id, b.id, 8 FROM users u, books b WHERE u.display_name = 'Okur' AND b.slug = 'kitap-1'`)
	db.must(`DELETE FROM ratings`)
	db.must(`DELETE FROM users WHERE display_name = 'Yönetici'`)
	db.must(`DELETE FROM books WHERE slug = 'kitap-2'`)
	for _, c := range []struct {
		name, sql string
		want      int
	}{
		{"puan silinince inceleme kalır", `SELECT count(*) FROM reviews WHERE user_id = (SELECT id FROM users WHERE display_name = 'Okur')`, 1},
		{"yönetici silinince gizleme kalır, gizleyen boşalır", `SELECT count(*) FROM reviews WHERE hidden_at IS NOT NULL AND hidden_by IS NULL AND hidden_reason <> ''`, 1},
		{"kitap silinince incelemesi gider", `SELECT count(*) FROM reviews WHERE book_id NOT IN (SELECT id FROM books)`, 0},
	} {
		if got := db.count(c.sql); got != c.want {
			t.Errorf("%s: %d satır, beklenen %d", c.name, got, c.want)
		}
	}
	db.must(`DELETE FROM users WHERE display_name = 'İkinci'`)
	if got := db.count(`SELECT count(*) FROM reviews`); got != 1 {
		t.Errorf("yazar silinince incelemesi gitmeli: %d inceleme kaldı, beklenen 1", got)
	}

	if _, err := provider.DownTo(ctx, 17); err != nil {
		t.Fatalf("0018 geri alınamadı: %v", err)
	}
	if got := db.count(`SELECT count(*) FROM information_schema.tables WHERE table_name = 'reviews'`); got != 0 {
		t.Error("Down'dan sonra reviews tablosu kalmamalı")
	}
}

// review, görünen adı verilen okurun slug'ı verilen kitaba incelemesi (metin bir SQL ifadesi).
// Değerler testin kendi sabitleri; dışarıdan girdi yok.
func review(user, slug, body string) string {
	return strings.NewReplacer("$user", user, "$slug", slug, "$body", body).Replace(`
		INSERT INTO reviews (user_id, book_id, body)
		SELECT u.id, b.id, $body FROM users u, books b
		WHERE u.email = '$user@example.test' AND b.slug = '$slug'`)
}

// hide, okurun o kitaptaki incelemesini yönetici adına verilen gerekçeyle gizler.
func hide(slug, reason string) string {
	return strings.NewReplacer("$slug", slug, "$reason", reason).Replace(`
		UPDATE reviews SET hidden_at = now(), hidden_reason = $reason,
		       hidden_by = (SELECT id FROM users WHERE email = 'admin@example.test')
		WHERE user_id = (SELECT id FROM users WHERE email = 'okur@example.test')
		  AND book_id = (SELECT id FROM books WHERE slug = '$slug')`)
}
