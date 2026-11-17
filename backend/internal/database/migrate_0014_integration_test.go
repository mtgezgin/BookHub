package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestMigration0014BuildsSearchDocuments: arama (DISC-01) genişletmesi. Veritabanı 0013'e kadar
// kurulur ve ESKİ kodun bıraktığı gibi doldurulur (kitaplar, baskılar; arama belgesi yok). 0014'ten
// sonra:
//   - var olan her kitabın belgesi oluşmuştur (durumundan bağımsız; görünürlük sorguda süzülür),
//   - belge başlığı, yazarı, baskı adlarını ve çevirmenleri taşır; yayınevi ve ISBN taşımaz,
//   - tetikleyiciler sonraki yazımları (eski kod da yazsa) belgeye yansıtır,
//   - Down bütün nesneleri kaldırır ve kitaplara dokunmaz.
func TestMigration0014BuildsSearchDocuments(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, provider := migratedTo(ctx, t, "bookhub_m0014", 13)

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO books (title, slug, author, language, status, rating_count, average_rating) VALUES
		('İnce Memed', 'ince-memed', 'Yaşar Kemal', 'tr', 'APPROVED', 12, 8.50),
		('Suç ve Ceza', 'suc-ve-ceza', 'Fyodor Dostoyevski', 'ru', 'APPROVED', 0, NULL),
		('Bekleyen Kitap', 'bekleyen', 'Yeni Yazar', 'tr', 'PENDING', 0, NULL)`)
	exec(`INSERT INTO editions (book_id, title, language, translator, publisher, isbn, is_primary)
		SELECT id, 'Suç ve Ceza', 'tr', 'Mazlum Beyhan', 'İletişim Yayınları', '9789750500000', true FROM books WHERE slug = 'suc-ve-ceza'`)
	exec(`INSERT INTO editions (book_id, title, language, is_primary)
		SELECT id, 'Преступление и наказание', 'ru', false FROM books WHERE slug = 'suc-ve-ceza'`)

	if _, err := provider.UpTo(ctx, 14); err != nil {
		t.Fatalf("0014 uygulanamadı: %v", err)
	}

	// matches: bu sorgu bu kitabın belgesiyle eşleşiyor mu (uygulamanın kullandığı iki fonksiyonla).
	matches := func(slug, query string) bool {
		t.Helper()
		var ok bool
		if err := conn.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM book_search s JOIN books b ON b.id = s.book_id
				WHERE b.slug = $1 AND s.tsv @@ search_tsquery(search_fold($2), ''))`, slug, query).Scan(&ok); err != nil {
			t.Fatalf("eşleşme sorgusu (%s, %q): %v", slug, query, err)
		}
		return ok
	}

	t.Run("ilk doldurma: her kitabın belgesi var", func(t *testing.T) {
		var booksN, docsN int
		if err := conn.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM books), (SELECT count(*) FROM book_search)`).Scan(&booksN, &docsN); err != nil {
			t.Fatal(err)
		}
		if booksN != 3 || docsN != 3 {
			t.Fatalf("3 kitap ve 3 belge bekleniyordu: %d kitap, %d belge", booksN, docsN)
		}
		var status string
		var votes int
		var avg sql.NullFloat64
		var fold string
		if err := conn.QueryRowContext(ctx, `
			SELECT s.status, s.rating_count, s.average_rating, s.title_fold
			FROM book_search s JOIN books b ON b.id = s.book_id WHERE b.slug = 'ince-memed'`).Scan(&status, &votes, &avg, &fold); err != nil {
			t.Fatal(err)
		}
		if status != "APPROVED" || votes != 12 || !avg.Valid || avg.Float64 != 8.5 || fold != "ince memed" {
			t.Errorf("belge kitabın durumunu, puanını ve katlanmış başlığını taşımalı: %q %d %v %q", status, votes, avg, fold)
		}
	})

	t.Run("eşleşme: Türkçe harf, aksan, büyük/küçük harf, kelime sırası, önek", func(t *testing.T) {
		for _, tc := range []struct {
			slug, query string
			want        bool
		}{
			{"ince-memed", "ince memed", true},
			{"ince-memed", "İNCE MEMED", true},
			{"ince-memed", "Ince Memed", true}, // noktasız büyük I
			{"ince-memed", "ınce memed", true}, // noktasız küçük ı
			{"ince-memed", "memed ince", true}, // sıra önemsiz
			{"ince-memed", "yasar kemal", true},
			{"ince-memed", "ince mem", true},  // son kelime önek
			{"ince-memed", "in memed", false}, // önek yalnız SON kelimede
			{"ince-memed", "ince ahmet", false},
			{"suc-ve-ceza", "suc ve ceza", true},
			{"suc-ve-ceza", "dostoyevski suç", true}, // başlık + yazar
			{"suc-ve-ceza", "mazlum beyhan", true},   // çevirmen
			{"suc-ve-ceza", "преступление", true},    // başka baskının adı (başka alfabe)
			{"suc-ve-ceza", "ПРЕСТУПЛЕНИЕ", true},    // büyük harf, veritabanı yerel ayarından bağımsız
			{"suc-ve-ceza", "iletişim", false},       // yayınevi belgede yok
			{"suc-ve-ceza", "9789750500000", false},  // ISBN belgede yok
			{"bekleyen", "bekleyen kitap", true},     // belge her durumda var; görünürlük sorguda
		} {
			if got := matches(tc.slug, tc.query); got != tc.want {
				t.Errorf("%s ← %q: eşleşme %v, istenen %v", tc.slug, tc.query, got, tc.want)
			}
		}
	})

	t.Run("search_fold: aynı sayılan yazımlar tek biçime iner", func(t *testing.T) {
		for _, tc := range []struct{ in, want string }{
			{"İnce Memed", "ince memed"},
			{"IŞIK ILGAZ", "isik ilgaz"},
			{"ışık ılgaz", "isik ilgaz"},
			{"İstanbul", "istanbul"}, // I + birleşen nokta (ayrışmış İ)
			{"Kürk Mantolu Madonna", "kurk mantolu madonna"},
			{"Çağdaş Öykü Şöleni", "cagdas oyku soleni"},
			{"Søren Kierkegaard", "soren kierkegaard"},
			{"Stanisław Lem", "stanislaw lem"},
			{"“Aşk” — Elif Şafak…", "ask elif safak"},
			{"  Çok   boşluklu\tmetin  ", "cok bosluklu metin"},
			{"Catch-22", "catch 22"},
			{"%50 _indirim_ \\ters", "50 indirim ters"},
			{"Sineklerin Tanrısı’nın", "sineklerin tanrisi nin"},
			{"a|b&c!d(e)f:*g", "a b c d e f g"},
			{"ﬁnans", "finans"}, // bitişik harf (NFKD)
			{"Лев Толстой", "лев толстои"},
			{"!!!", ""},
			{"", ""},
		} {
			var got string
			if err := conn.QueryRowContext(ctx, `SELECT search_fold($1)`, tc.in).Scan(&got); err != nil {
				t.Fatalf("search_fold(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("search_fold(%q) = %q, istenen %q", tc.in, got, tc.want)
			}
		}
	})

	t.Run("search_tsquery: son kelime önek, tek harfte tam kelime, boş sorguda NULL", func(t *testing.T) {
		for _, tc := range []struct {
			folded, weights string
			want            sql.NullString
		}{
			{"sav", "", sql.NullString{String: "'sav':*", Valid: true}},
			{"s", "", sql.NullString{String: "'s'", Valid: true}},                                // tek harf: önek değil
			{"savas v", "", sql.NullString{String: "'savas' & 'v':*", Valid: true}},              // çok kelimede son kelime hep önek
			{"ince mem", "A", sql.NullString{String: "'ince':A & 'mem':*A", Valid: true}},        // yalnız başlıkta
			{"ince memed", "AB", sql.NullString{String: "'ince':AB & 'memed':*AB", Valid: true}}, // başlık + yazar
			{"", "", sql.NullString{}},
		} {
			var got sql.NullString
			if err := conn.QueryRowContext(ctx, `SELECT search_tsquery($1, $2)::text`, tc.folded, tc.weights).Scan(&got); err != nil {
				t.Fatalf("search_tsquery(%q, %q): %v", tc.folded, tc.weights, err)
			}
			if got != tc.want {
				t.Errorf("search_tsquery(%q, %q) = %+v, istenen %+v", tc.folded, tc.weights, got, tc.want)
			}
		}
	})

	t.Run("işleç ve joker karakterleri sorgunun yapısını değiştiremez", func(t *testing.T) {
		// Katlama bunların hepsini boşluğa çevirir; kalan kelimeler düz kelime olarak aranır.
		for _, q := range []string{"ince | ahmet", "!ince", "ince & (memed | x)", "ince:*", "'ince'", "ince <-> memed", "📚", "ince 📚", "%", "_", "\\"} {
			var n int
			if err := conn.QueryRowContext(ctx,
				`SELECT count(*) FROM book_search s WHERE s.tsv @@ search_tsquery(search_fold($1), '')`, q).Scan(&n); err != nil {
				t.Errorf("sorgu %q hata verdi: %v", q, err)
			}
		}
		if matches("ince-memed", "ince | ahmet") {
			t.Error(`"ince | ahmet" VEYA gibi çalıştı; iki kelime de aranmalıydı (eşleşme olmamalı)`)
		}
	})

	t.Run("tetikleyiciler: kitap ve baskı yazımları belgeye yansır", func(t *testing.T) {
		// Eski kodun yazacağı biçimde: yalnız books ve editions'a yazılır, book_search'e dokunulmaz.
		exec(`UPDATE books SET title = 'Yer Demir Gök Bakır' WHERE slug = 'ince-memed'`)
		if matches("ince-memed", "ince memed") || !matches("ince-memed", "yer demir gok bakir") {
			t.Error("başlık değişince belge yeni başlığı taşımalı, eskisini taşımamalı")
		}
		exec(`UPDATE books SET status = 'ARCHIVED', rating_count = 40, average_rating = 9.25 WHERE slug = 'ince-memed'`)
		var status string
		var votes int
		var avg float64
		if err := conn.QueryRowContext(ctx, `
			SELECT s.status, s.rating_count, s.average_rating FROM book_search s JOIN books b ON b.id = s.book_id
			WHERE b.slug = 'ince-memed'`).Scan(&status, &votes, &avg); err != nil {
			t.Fatal(err)
		}
		if status != "ARCHIVED" || votes != 40 || avg != 9.25 {
			t.Errorf("durum ve puan belgeye yansımalı: %q %d %v", status, votes, avg)
		}

		exec(`INSERT INTO books (title, slug, author, language, status) VALUES ('Tutunamayanlar', 'tutunamayanlar', 'Oğuz Atay', 'tr', 'APPROVED')`)
		if !matches("tutunamayanlar", "oguz atay tutunamayanlar") {
			t.Error("yeni kitabın belgesi eklenirken oluşmalı")
		}
		exec(`INSERT INTO editions (book_id, title, language, translator, is_primary)
			SELECT id, 'The Disconnected', 'en', 'Sevin Seydi', false FROM books WHERE slug = 'tutunamayanlar'`)
		if !matches("tutunamayanlar", "disconnected") || !matches("tutunamayanlar", "sevin seydi") {
			t.Error("yeni baskının adı ve çevirmeni belgeye girmeli")
		}
		exec(`UPDATE editions SET translator = 'Başka Çevirmen' WHERE title = 'The Disconnected'`)
		if matches("tutunamayanlar", "sevin seydi") || !matches("tutunamayanlar", "baska cevirmen") {
			t.Error("çevirmen değişince belge güncellenmeli")
		}
		exec(`DELETE FROM editions WHERE title = 'The Disconnected'`)
		if matches("tutunamayanlar", "disconnected") {
			t.Error("silinen baskının adı belgeden çıkmalı")
		}
		exec(`DELETE FROM books WHERE slug = 'tutunamayanlar'`)
		var left int
		if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM book_search s WHERE NOT EXISTS (SELECT 1 FROM books b WHERE b.id = s.book_id)`).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if left != 0 {
			t.Errorf("silinen kitabın belgesi de silinmeli; %d sahipsiz belge kaldı", left)
		}
	})

	t.Run("Down: arama nesneleri kalkar, kitaplar kalır", func(t *testing.T) {
		if _, err := provider.Down(ctx); err != nil {
			t.Fatalf("0014 Down: %v", err)
		}
		var objects int
		if err := conn.QueryRowContext(ctx, `
			SELECT (SELECT count(*) FROM pg_class WHERE relname = 'book_search')
			     + (SELECT count(*) FROM pg_proc WHERE proname IN ('search_fold', 'search_tsquery', 'refresh_book_search', 'books_search_sync', 'editions_search_sync'))
			     + (SELECT count(*) FROM pg_trigger WHERE tgname IN ('books_search_sync', 'editions_search_sync'))`).Scan(&objects); err != nil {
			t.Fatal(err)
		}
		if objects != 0 {
			t.Errorf("Down sonrası %d arama nesnesi kaldı", objects)
		}
		// Tetikleyiciler kalktığı için eski kodun yazımı yine çalışmalı.
		exec(`UPDATE books SET title = 'Down Sonrası' WHERE slug = 'bekleyen'`)
		if _, err := provider.UpTo(ctx, 14); err != nil {
			t.Fatalf("0014 yeniden uygulanamadı: %v", err)
		}
		if !matches("bekleyen", "down sonrasi") {
			t.Error("yeniden uygulanınca ilk doldurma güncel başlığı almalı")
		}
	})
}
