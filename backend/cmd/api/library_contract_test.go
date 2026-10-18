package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// TestLibraryItemContract, GET /api/me/library öğesinin JSON şeklini sabitler. Öğe kişisel
// veri taşıdığı için sızıntı iki yönlü önemli: kullanıcı kimliği gibi bir alan cevaba
// girmemeli, içteki `book` da herkese açık kitap sözleşmesinden sapmamalı.
func TestLibraryItemContract(t *testing.T) {
	added := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	fake := &fakeLibrary{total: 1, rows: []db.ListLibraryRow{{
		Book:      db.Book{ID: 1, Title: "Dune", Slug: "dune", Author: "Frank Herbert", Status: "APPROVED", CreatedAt: added, UpdatedAt: added},
		Status:    "READ",
		AddedAt:   added,
		MyRating:  ptr(int16(9)),
		InCatalog: true,
	}}}

	rec := do(t, libraryRouter(fake), http.MethodGet, "/api/me/library", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("durum kodu %d (gövde: %s)", rec.Code, rec.Body)
	}

	var page struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
		t.Fatalf("cevap çözülemedi: %v (gövde: %s)", err, rec.Body)
	}
	item := page.Items[0]
	checkContract(t, item, []string{"book", "status", "added_at", "my_rating", "in_catalog"})

	var book map[string]json.RawMessage
	if err := json.Unmarshal(item["book"], &book); err != nil {
		t.Fatalf("book nesnesi çözülemedi: %v", err)
	}
	checkContract(t, book, publicBookFields)
}
