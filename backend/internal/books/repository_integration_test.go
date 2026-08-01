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
