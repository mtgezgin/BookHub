package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// fakeBooks, bookGetter interface'inin SAHTE (fake) uygulaması: ne dönmesi
// söylendiyse onu döner. Gerçek DB yok → bu test saf birim testi ve ışık hızında.
// (Interface'i tüketen tarafta tanımlamanın meyvesi tam olarak bu satırlar.)
type fakeBooks struct {
	book db.Book
	err  error
}

func (f fakeBooks) Get(_ context.Context, _ int64) (db.Book, error) {
	return f.book, f.err
}

// TestHandleGetBook, GET /api/books/{id}'nin hata HARİTALAMASINI table-driven doğrular:
// geçersiz girdi → 400, yok → 404, repo hatası → 500 (detay sızdırmadan), var → 200.
func TestHandleGetBook(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		fake       fakeBooks
		wantStatus int
		wantCode   string // hata zarfındaki error.code ("" = başarı bekleniyor)
	}{
		{
			name:       "geçerli id → 200 + kitap",
			url:        "/api/books/1",
			fake:       fakeBooks{book: db.Book{ID: 1, Title: "Clean Code", Slug: "clean-code"}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "olmayan id → 404",
			url:        "/api/books/999",
			fake:       fakeBooks{err: books.ErrNotFound},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "sayı olmayan id → 400 (sorguya hiç gitmeden)",
			url:        "/api/books/abc",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_id",
		},
		{
			name:       "sıfır/negatif id → 400",
			url:        "/api/books/0",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_id",
		},
		{
			name:       "beklenmeyen repo hatası → 500, detay SIZMAZ",
			url:        "/api/books/1",
			fake:       fakeBooks{err: errors.New("bağlantı koptu: çok gizli detay")},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			// Handler'ı ÇIPLAK çağırmıyoruz: chi.URLParam(r, "id") ancak istek
			// router'dan geçerse dolar. Router'a sahte repo'yu enjekte ediyoruz.
			newRouter(tt.fake).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", tt.wantStatus, rec.Code, rec.Body)
			}

			if tt.wantCode != "" {
				// Hata bekleniyor → tutarlı zarfı doğrula.
				var body errorBody
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("hata cevabı geçerli JSON değil: %v", err)
				}
				if body.Error.Code != tt.wantCode {
					t.Errorf("error.code: istenen %q, gelen %q", tt.wantCode, body.Error.Code)
				}
				return
			}

			// Başarı bekleniyor → kitabı doğrula.
			var got db.Book
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("kitap cevabı geçerli JSON değil: %v", err)
			}
			if got.ID != tt.fake.book.ID || got.Title != tt.fake.book.Title {
				t.Errorf("kitap yanlış: istenen %+v, gelen %+v", tt.fake.book, got)
			}
		})
	}
}

// TestHandleGetBook_500DetaySizdirmaz, 500 yolunun "detay SIZMAZ" iddiasını
// GÖVDE ÜZERİNDEN kanıtlar: error.code'un "internal" olması yetmez — sürücü/şema
// bilgisi taşıyan hata metninin cevabın HİÇBİR yerinde geçmemesi gerekir.
// (Detay operatör için log'a gider; istemciye gitmesi bilgi sızıntısıdır.)
func TestHandleGetBook_500DetaySizdirmaz(t *testing.T) {
	// Gerçekçi bir iç hata: repository'nin %w ile sardığı sürücü mesajı.
	const surucuDetayi = `ERROR: relation "books" does not exist (SQLSTATE 42P01) @ 10.0.0.7:5432`
	fake := fakeBooks{err: fmt.Errorf("kitap sorgusu (id=1): %w", errors.New(surucuDetayi))}

	req := httptest.NewRequest(http.MethodGet, "/api/books/1", nil)
	rec := httptest.NewRecorder()

	newRouter(fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("durum kodu: istenen %d, gelen %d", http.StatusInternalServerError, rec.Code)
	}

	govde := rec.Body.String()
	// Tam metin + tek tek sızıntı işaretleri: biri bile geçerse cevap detay taşıyor.
	for _, sizinti := range []string{surucuDetayi, "relation", "SQLSTATE", "42P01", "5432", "kitap sorgusu"} {
		if strings.Contains(govde, sizinti) {
			t.Errorf("iç detay gövdeye sızdı (%q): %s", sizinti, govde)
		}
	}
}
