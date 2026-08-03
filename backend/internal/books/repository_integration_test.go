package books

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// TestRepositoryGet_GorunurlukSiniri, GERÇEK Postgres'e karşı Get'in görünürlük
// sınırını doğrular: yalnızca status='APPROVED' kitap döner; PENDING/REJECTED
// "yok" (ErrNotFound) muamelesi görür — onaylanmamış kitabın VARLIĞI bile
// sızmaz (OWASP BOLA/IDOR; sınır sql/queries/books.sql'de).
//
// Handler testleri sahte repository kullandığı için bu sınırı yalnızca burası
// kanıtlar — filtre SQL'den düşerse kırılacak tek test budur.
//
// Çalıştırmak için (GO_CONVENTIONS §8: entegrasyon testleri Compose Postgres'e karşı):
//
//	docker compose up -d
//	TEST_DATABASE_URL='postgres://bookhub:bookhub_dev@localhost:5432/bookhub?sslmode=disable' \
//	  go test ./internal/books/...
func TestRepositoryGet_GorunurlukSiniri(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("havuz kurulamadı: %v", err)
	}
	t.Cleanup(pool.Close)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		t.Fatalf("TEST_DATABASE_URL'e bağlanılamadı (docker compose up -d çalıştı mı?): %v", err)
	}

	repo := NewRepository(pool)

	// Her statüden birer kitap ekle. slug UNIQUE olduğu için benzersiz ek şart;
	// böylece test tekrar tekrar (ve paralel çalışan paketlerle) çakışmadan koşar.
	benzersiz := time.Now().UnixNano()
	ids := make(map[string]int64, 3)
	for _, status := range []string{"APPROVED", "PENDING", "REJECTED"} {
		book, err := repo.Create(ctx, db.CreateBookParams{
			Title:    "Görünürlük Testi " + status,
			Slug:     fmt.Sprintf("gorunurluk-testi-%s-%d", strings.ToLower(status), benzersiz),
			Author:   "Test Yazar",
			Language: "tr",
			Status:   status,
		})
		if err != nil {
			t.Fatalf("%s kitabı eklenemedi: %v", status, err)
		}
		ids[status] = book.ID

		t.Cleanup(func() {
			// Temizlik kendi context'iyle: test context'i iptal olsa bile satır kalmasın.
			if _, err := pool.Exec(context.Background(), "DELETE FROM books WHERE id = $1", book.ID); err != nil {
				t.Errorf("temizlik başarısız (id=%d): %v", book.ID, err)
			}
		})
	}

	tests := []struct {
		status    string
		wantFound bool
	}{
		{status: "APPROVED", wantFound: true},
		{status: "PENDING"},
		{status: "REJECTED"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got, err := repo.Get(ctx, ids[tt.status])

			if !tt.wantFound {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("%s kitap SIZDI: istenen ErrNotFound, gelen (%+v, %v)", tt.status, got, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("onaylı kitap okunamadı: %v", err)
			}
			if got.ID != ids[tt.status] {
				t.Errorf("kitap id: istenen %d, gelen %d", ids[tt.status], got.ID)
			}
			if got.Status != "APPROVED" {
				t.Errorf("statü: istenen APPROVED, gelen %q", got.Status)
			}
		})
	}
}

// TestRepositoryList_SiraVeGorunurluk, List'in İKİ sözünü gerçek Postgres'e karşı
// kanıtlar — ikisi de yalnızca SQL'de yaşadığı için sahte (fake) ile kanıtlanamaz:
//
//  1. GÖRÜNÜRLÜK: yalnızca status='APPROVED' listelenir (PENDING/REJECTED sızmaz).
//  2. SIRA: en yeni en başta (ORDER BY created_at DESC) — migration'daki partial
//     index'in ((created_at DESC) WHERE status='APPROVED') hedeflediği sorgu budur.
//
// Liste GLOBAL olduğu için (parametresiz sorgu) mutlak içerik iddia edilmez;
// yalnızca bu testin eklediği satırların GÖRECELİ sırası ve varlığı/yokluğu
// doğrulanır. Böylece test, DB'de başka veri olsa da güvenilir kalır.
func TestRepositoryList_SiraVeGorunurluk(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("havuz kurulamadı: %v", err)
	}
	t.Cleanup(pool.Close)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		t.Fatalf("TEST_DATABASE_URL'e bağlanılamadı (docker compose up -d çalıştı mı?): %v", err)
	}

	repo := NewRepository(pool)
	benzersiz := time.Now().UnixNano()

	// created_at'i ELLE veriyoruz: art arda INSERT'lerde now() farkı mikrosaniye
	// mertebesinde kalır ve sıra iddiası kırılgan olur. Açık zaman damgası testi
	// deterministik yapar.
	fixtures := []struct {
		ad        string
		status    string
		createdAt time.Time
	}{
		{ad: "eski-onayli", status: "APPROVED", createdAt: time.Now().Add(-2 * time.Hour)},
		{ad: "yeni-onayli", status: "APPROVED", createdAt: time.Now().Add(-1 * time.Hour)},
		{ad: "bekleyen", status: "PENDING", createdAt: time.Now().Add(-90 * time.Minute)},
	}

	ids := make(map[string]int64, len(fixtures))
	for _, f := range fixtures {
		book, err := repo.Create(ctx, db.CreateBookParams{
			Title:    "Liste Testi " + f.ad,
			Slug:     fmt.Sprintf("liste-testi-%s-%d", f.ad, benzersiz),
			Author:   "Test Yazar",
			Language: "tr",
			Status:   f.status,
		})
		if err != nil {
			t.Fatalf("%s kitabı eklenemedi: %v", f.ad, err)
		}
		ids[f.ad] = book.ID

		t.Cleanup(func() {
			if _, err := pool.Exec(context.Background(), "DELETE FROM books WHERE id = $1", book.ID); err != nil {
				t.Errorf("temizlik başarısız (id=%d): %v", book.ID, err)
			}
		})

		if _, err := pool.Exec(ctx, "UPDATE books SET created_at = $1 WHERE id = $2", f.createdAt, book.ID); err != nil {
			t.Fatalf("%s kitabının created_at'i ayarlanamadı: %v", f.ad, err)
		}
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("liste okunamadı: %v", err)
	}

	// id → listedeki sıra numarası
	sira := make(map[int64]int, len(got))
	for i, book := range got {
		sira[book.ID] = i
	}

	// 1) Görünürlük: PENDING listede OLMAMALI.
	if _, varMi := sira[ids["bekleyen"]]; varMi {
		t.Errorf("PENDING kitap listeye SIZDI (id=%d)", ids["bekleyen"])
	}

	// 2) Her onaylı kitap listede olmalı.
	yeniSira, yeniVar := sira[ids["yeni-onayli"]]
	eskiSira, eskiVar := sira[ids["eski-onayli"]]
	if !yeniVar || !eskiVar {
		t.Fatalf("onaylı kitaplar listede olmalıydı (yeni=%v, eski=%v; liste uzunluğu %d)", yeniVar, eskiVar, len(got))
	}

	// 3) Sıra: yeni olan eskiden ÖNCE gelmeli (created_at DESC).
	if yeniSira >= eskiSira {
		t.Errorf("sıra yanlış: yeni kitap eskiden önce gelmeliydi (yeni=%d, eski=%d)", yeniSira, eskiSira)
	}

	// 4) Listedeki her satır APPROVED olmalı — sorgu filtresinin genel güvencesi.
	for _, book := range got {
		if book.Status != "APPROVED" {
			t.Errorf("listede APPROVED olmayan kitap var: id=%d statü=%q", book.ID, book.Status)
		}
	}
}
