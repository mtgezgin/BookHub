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
	list []db.Book
	err  error
}

func (f fakeBooks) Get(_ context.Context, _ int64) (db.Book, error) {
	return f.book, f.err
}

func (f fakeBooks) List(_ context.Context) ([]db.Book, error) {
	return f.list, f.err
}

// Create, gelen params'ı saklamaz; bu sahte yalnızca DÖNÜŞ değerini kontrol eder.
// Handler'ın params'a NE YAZDIĞINI doğrulamak için aşağıdaki capturingBooks var.
func (f fakeBooks) Create(_ context.Context, _ db.CreateBookParams) (db.Book, error) {
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
			newRouter(deps{books: tt.fake}).ServeHTTP(rec, req)

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

	newRouter(deps{books: fake}).ServeHTTP(rec, req)

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

// capturingBooks, Create'e GEÇİLEN params'ı kaydeden sahte. fakeBooks'tan farkı
// pointer receiver: handler'ın DB'ye ne yazdığını doğrulayabilmek için duruma
// yazması gerekiyor. "Statüyü sunucu belirler" iddiası ancak böyle kanıtlanır.
type capturingBooks struct {
	got  db.CreateBookParams
	book db.Book
	err  error
}

func (f *capturingBooks) Get(_ context.Context, _ int64) (db.Book, error) { return f.book, f.err }
func (f *capturingBooks) List(_ context.Context) ([]db.Book, error)       { return nil, f.err }
func (f *capturingBooks) Create(_ context.Context, p db.CreateBookParams) (db.Book, error) {
	f.got = p
	return f.book, f.err
}

// TestHandleListBooks, liste ucunun üç yolunu doğrular. En kritik senaryo boş liste:
// repository nil slice döndüğünde gövde "null" DEĞİL "[]" olmalı — aradaki fark
// istemcide books.map(...) çağrısını patlatır.
func TestHandleListBooks(t *testing.T) {
	tests := []struct {
		name       string
		fake       fakeBooks
		wantStatus int
		wantBody   string // "" = gövde kontrolü yapma
		wantCount  int
	}{
		{
			name:       "boş liste → JSON null DEĞİL, [] olmalı",
			fake:       fakeBooks{list: nil},
			wantStatus: http.StatusOK,
			wantBody:   "[]",
		},
		{
			name: "dolu liste → 200 + kitaplar",
			fake: fakeBooks{list: []db.Book{
				{ID: 1, Title: "Clean Code"},
				{ID: 2, Title: "The Go Programming Language"},
			}},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "repo hatası → 500",
			fake:       fakeBooks{err: errors.New("bağlantı koptu")},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/books", nil)
			rec := httptest.NewRecorder()

			newRouter(deps{books: tt.fake}).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantBody != "" && strings.TrimSpace(rec.Body.String()) != tt.wantBody {
				t.Errorf("gövde: istenen %q, gelen %q", tt.wantBody, strings.TrimSpace(rec.Body.String()))
			}
			if tt.wantCount > 0 {
				var got []db.Book
				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatalf("cevap geçerli JSON değil: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("kitap sayısı: istenen %d, gelen %d", tt.wantCount, len(got))
				}
			}
		})
	}
}

// TestHandleCreateBook, create ucunun girdi doğrulaması ve hata haritalamasını
// table-driven doğrular. "status gönderilirse 400" senaryosu bir REGRESYON testidir:
// bu uç bir ara istemcinin status yollamasına izin veriyordu (mass-assignment) ve
// gönderen kitabını kendi yayına alabiliyordu.
func TestHandleCreateBook(t *testing.T) {
	const gecerliGovde = `{"title":"The Go Programming Language","slug":"gopl","author":"Donovan"}`

	tests := []struct {
		name       string
		govde      string
		fake       fakeBooks
		wantStatus int
		wantCode   string // hata zarfındaki error.code ("" = başarı beklenir)
	}{
		{
			name:       "geçerli gövde → 201",
			govde:      gecerliGovde,
			fake:       fakeBooks{book: db.Book{ID: 7, Title: "The Go Programming Language"}},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "REGRESYON: istemci status yollarsa → 400 (kendi kitabını yayına alamaz)",
			govde:      `{"title":"Sızma","slug":"sizma","author":"X","status":"APPROVED"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "bilinmeyen alan (typo) → 400",
			govde:      `{"titel":"Yanlış","slug":"y","author":"X"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "bozuk JSON → 400",
			govde:      `{"title": "yarım`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "boş gövde → 400",
			govde:      ``,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "zorunlu alanlar eksik → 400",
			govde:      `{"description":"sadece açıklama"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "sadece boşluktan ibaret başlık → 400 (trim sonrası boş)",
			govde:      `{"title":"   ","slug":"s","author":"a"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "page_count sıfır → 400",
			govde:      `{"title":"T","slug":"s","author":"a","page_count":0}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "publication_year aralık dışı → 400",
			govde:      `{"title":"T","slug":"s","author":"a","publication_year":1200}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_error",
		},
		{
			name:       "aynı slug → 409 (500 DEĞİL: istemcinin düzeltebileceği bir çakışma)",
			govde:      gecerliGovde,
			fake:       fakeBooks{err: books.ErrDuplicateSlug},
			wantStatus: http.StatusConflict,
			wantCode:   "slug_conflict",
		},
		{
			name:       "beklenmeyen repo hatası → 500",
			govde:      gecerliGovde,
			fake:       fakeBooks{err: errors.New("bağlantı koptu: gizli detay")},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/admin/books", strings.NewReader(tt.govde))
			rec := httptest.NewRecorder()

			newRouter(deps{books: tt.fake}).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode == "" {
				return
			}
			var body errorBody
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("hata cevabı geçerli JSON değil: %v", err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("error.code: istenen %q, gelen %q", tt.wantCode, body.Error.Code)
			}
		})
	}
}

// TestHandleCreateBook_SunucuKararlari, handler'ın DB'ye NE YAZDIĞINI doğrular —
// cevaba değil, parametrelere bakar. İki iddia:
//  1. Statüyü sunucu belirler (istemci gönderemez, gönderirse zaten 400 alır).
//  2. language boş gelirse "tr"ye düşer — migration'daki DEFAULT devreye girmediği için
//     (sqlc parametreyi hep açıkça yazar) bu varsayılanı uygulama koymak zorunda.
func TestHandleCreateBook_SunucuKararlari(t *testing.T) {
	fake := &capturingBooks{book: db.Book{ID: 1}}

	govde := `{"title":"  Boşluklu Başlık  ","slug":"bosluklu","author":"Yazar"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/books", strings.NewReader(govde))
	rec := httptest.NewRecorder()

	newRouter(deps{books: fake}).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", http.StatusCreated, rec.Code, rec.Body)
	}
	if fake.got.Status != books.StatusApproved {
		t.Errorf("statüyü sunucu belirlemeliydi: istenen %q, gelen %q", books.StatusApproved, fake.got.Status)
	}
	if fake.got.Language != "tr" {
		t.Errorf("language varsayılanı: istenen \"tr\", gelen %q", fake.got.Language)
	}
	if fake.got.Title != "Boşluklu Başlık" {
		t.Errorf("başlık kırpılmalıydı: gelen %q", fake.got.Title)
	}
}
