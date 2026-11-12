package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// TestGetBookEndpointRealDB, GET /api/books/{id}'yi UÇTAN UCA doğrular:
// router → handler → GERÇEK repository → Compose Postgres.
//
// Neden ayrı bir test: handler birim testleri sahte repository kullanır, repository
// entegrasyon testi ise HTTP'yi hiç görmez. Görünürlük sınırının HTTP cevabına DOĞRU
// çevrildiğini — PENDING kitabın istemciye 404 olarak göründüğünü — yalnızca bu dikey
// kesit kanıtlar. Dikişteki (seam) bir hata, ör. ErrNotFound'un 500'e düşmesi, burada yakalanır.
func TestGetBookEndpointRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := books.NewRepository(pool)
	router := newRouter(deps{books: repo})

	ids := make(map[string]int64, 2)
	for _, status := range []string{books.StatusApproved, books.StatusPending} {
		book, err := repo.Create(context.Background(), books.CreateParams{CreateBookParams: db.CreateBookParams{
			Title:    "Uçtan Uca Testi " + status,
			Slug:     testdb.Unique("e2e-" + strings.ToLower(status)),
			Author:   "Test Yazar",
			Language: "tr",
			Status:   status,
		}})
		if err != nil {
			t.Fatalf("%s kitabı eklenemedi: %v", status, err)
		}
		ids[status] = book.ID
		t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM books WHERE id = $1", book.ID) })
	}

	t.Run("PENDING kitap → 404", func(t *testing.T) {
		rec := getBook(router, ids[books.StatusPending])
		if rec.Code != http.StatusNotFound {
			t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", http.StatusNotFound, rec.Code, rec.Body)
		}
		// Gövde de "yok" demeli: 404 ama sızdıran bir gövde yine IDOR'dur.
		body := rec.Body.String()
		if strings.Contains(body, "Uçtan Uca Testi PENDING") {
			t.Errorf("onaylanmamış kitabın verisi gövdeye sızdı: %s", body)
		}
		var env errorBody
		if err := json.NewDecoder(strings.NewReader(body)).Decode(&env); err != nil {
			t.Fatalf("hata cevabı geçerli JSON değil: %v", err)
		}
		if env.Error.Code != "not_found" {
			t.Errorf("error.code: istenen %q, gelen %q", "not_found", env.Error.Code)
		}
	})

	// Kontrol grubu: aynı yol APPROVED kitapta 200 dönmeli. Bu olmadan 404 testi
	// "her şey 404 dönüyor" gibi yanlış bir sebeple de yeşil kalabilirdi.
	t.Run("APPROVED kitap → 200", func(t *testing.T) {
		rec := getBook(router, ids[books.StatusApproved])
		if rec.Code != http.StatusOK {
			t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", http.StatusOK, rec.Code, rec.Body)
		}
		var got bookResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("kitap cevabı geçerli JSON değil: %v", err)
		}
		if got.ID != ids[books.StatusApproved] {
			t.Errorf("kitap id: istenen %d, gelen %d", ids[books.StatusApproved], got.ID)
		}
	})
}

// getBook, router'a GET /api/books/{id} isteği geçirip kaydediciyi döner.
func getBook(router http.Handler, id int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/books/"+strconv.FormatInt(id, 10), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
