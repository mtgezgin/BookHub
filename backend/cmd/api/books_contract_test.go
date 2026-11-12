package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// TestGetBookResponseContract, GET /api/books/{id}'nin GÖVDE SÖZLEŞMESİNİ
// (public JSON şeklini) izin listesiyle (allow-list) sabitler.
//
// Handler artık bookResponse DTO'su dönüyor (GO_CONVENTIONS §11), yani yeni DB
// kolonları kendiliğinden sızmıyor. Bu test yine de sözleşmenin İKİ yönünü sabitler:
// fazladan alan çıkarsa (DTO'ya dikkatsizce alan eklendi) ve bir alan düşerse
// (istemciyi kıran değişiklik) kırılır. `status` bilerek yok: public uçlar yalnızca
// APPROVED döndürdüğü için bilgi taşımaz.
func TestGetBookResponseContract(t *testing.T) {
	allowedFields := publicBookFields

	year := int32(2008)
	pages := int32(464)
	fake := fakeBooks{book: db.BookView{
		ID:              1,
		Title:           "Clean Code",
		Slug:            "clean-code",
		Author:          "Robert C. Martin",
		Description:     "Bir yazılım zanaatkârlığı el kitabı",
		CoverUrl:        "https://example.test/clean-code.jpg",
		PublicationYear: &year,
		PageCount:       &pages,
		Language:        "en",
		Status:          "APPROVED",
		CreatedAt:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/books/1", nil)
	rec := httptest.NewRecorder()

	newRouter(deps{books: fake}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", http.StatusOK, rec.Code, rec.Body)
	}

	// Struct'a değil map'e çözüyoruz: struct'a çözmek fazladan alanları SESSİZCE
	// yutardı — oysa yakalamak istediğimiz tam olarak "fazladan alan".
	var body map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("cevap geçerli JSON nesnesi değil: %v", err)
	}

	checkContract(t, body, allowedFields)
}

// publicBookFields, herkese açık kitap JSON'unun izin verilen ÜST DÜZEY alanları = bugünkü
// public sözleşme. Yeni alan buraya ancak "istemciye gitmesi kasıtlı" kararıyla eklenmeli.
// Kitaplık öğesinin içindeki `book` da aynı sözleşmeye bağlı (library_contract_test.go).
var publicBookFields = []string{
	"id",
	"title",
	"slug",
	"author",
	"description",
	"cover_url",
	"publication_year",
	"page_count",
	"language",
	"genres",
	"average_rating", // Faz 5: türetilmiş puan istatistiği (bilinçli ekleme)
	"rating_count",
	"created_at",
	"updated_at",
}

// checkContract, bir JSON nesnesinin alanlarını izin listesiyle iki yönden karşılaştırır.
func checkContract(t *testing.T, body map[string]json.RawMessage, allowedFields []string) {
	t.Helper()
	allowed := make(map[string]bool, len(allowedFields))
	for _, field := range allowedFields {
		allowed[field] = true
	}

	// 1) Fazladan alan YOK: şemaya eklenen kolonlar kendiliğinden public olmasın.
	var extra []string
	for field := range body {
		if !allowed[field] {
			extra = append(extra, field)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Errorf("izin listesinde OLMAYAN alan(lar) cevaba sızdı: %v — bilinçli bir karar mı? (GO_CONVENTIONS §11: DTO kullan)", extra)
	}

	// 2) Eksik alan YOK: listedeki her alan cevapta olmalı (sözleşme kırılmasın).
	var missing []string
	for _, field := range allowedFields {
		if _, present := body[field]; !present {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		t.Errorf("public sözleşmedeki alan(lar) cevaptan düştü: %v", missing)
	}
}
