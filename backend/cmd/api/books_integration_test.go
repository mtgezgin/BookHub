package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// TestGetBookEndpoint_GercekDB, GET /api/books/{id}'yi UÇTAN UCA doğrular:
// router → handler → GERÇEK repository → Compose Postgres.
//
// Neden ayrı bir test: handler birim testleri sahte (fake) repository kullanır,
// repository entegrasyon testi ise HTTP'yi hiç görmez. Görünürlük sınırının
// (yalnız APPROVED döner) HTTP cevabına DOĞRU çevrildiğini — yani PENDING kitabın
// istemciye 404 olarak göründüğünü — yalnızca bu dikey kesit kanıtlar. Dikişteki
// (seam) bir hata, örneğin ErrNotFound'un 500'e düşmesi, sadece burada yakalanır.
//
// Çalıştırmak için (GO_CONVENTIONS §8: entegrasyon testleri Compose Postgres'e karşı):
//
//	docker compose up -d
//	TEST_DATABASE_URL='postgres://bookhub:bookhub_dev@localhost:5432/bookhub?sslmode=disable' \
//	  go test ./cmd/api/...
func TestGetBookEndpoint_GercekDB(t *testing.T) {
	dsn := testDatabaseURL(t)

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

	// Sahte değil GERÇEK repository — testin bütün amacı bu dikiş.
	repo := books.NewRepository(pool)
	router := newRouter(deps{books: repo})

	// slug UNIQUE olduğu için benzersiz ek şart: test tekrar tekrar koşabilsin.
	benzersiz := time.Now().UnixNano()
	ids := make(map[string]int64, 2)
	for _, status := range []string{"APPROVED", "PENDING"} {
		book, err := repo.Create(ctx, db.CreateBookParams{
			Title:    "Uçtan Uca Testi " + status,
			Slug:     fmt.Sprintf("uctan-uca-testi-%s-%d", strings.ToLower(status), benzersiz),
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

	t.Run("PENDING kitap → 404", func(t *testing.T) {
		rec := istek(router, ids["PENDING"])

		if rec.Code != http.StatusNotFound {
			t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", http.StatusNotFound, rec.Code, rec.Body)
		}

		// Gövde de "yok" demeli: 404 ama sızdıran bir gövde yine IDOR'dur.
		govde := rec.Body.String()
		if strings.Contains(govde, "Uçtan Uca Testi PENDING") {
			t.Errorf("onaylanmamış kitabın verisi gövdeye sızdı: %s", govde)
		}

		var body errorBody
		if err := json.NewDecoder(strings.NewReader(govde)).Decode(&body); err != nil {
			t.Fatalf("hata cevabı geçerli JSON değil: %v", err)
		}
		if body.Error.Code != "not_found" {
			t.Errorf("error.code: istenen %q, gelen %q", "not_found", body.Error.Code)
		}
	})

	// Kontrol grubu: aynı yol APPROVED kitapta 200 dönmeli. Bu olmadan 404 testi
	// "her şey 404 dönüyor" gibi yanlış bir sebeple de yeşil kalabilirdi.
	t.Run("APPROVED kitap → 200", func(t *testing.T) {
		rec := istek(router, ids["APPROVED"])

		if rec.Code != http.StatusOK {
			t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", http.StatusOK, rec.Code, rec.Body)
		}

		var got db.Book
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("kitap cevabı geçerli JSON değil: %v", err)
		}
		if got.ID != ids["APPROVED"] {
			t.Errorf("kitap id: istenen %d, gelen %d", ids["APPROVED"], got.ID)
		}
	})
}

// testDatabaseURL, entegrasyon testleri için DSN'i döner; tanımlı değilse testi atlar.
func testDatabaseURL(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}
	return dsn
}

// istek, router'a GET /api/books/{id} isteği geçirip kaydediciyi döner.
func istek(router http.Handler, id int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/books/"+strconv.FormatInt(id, 10), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
