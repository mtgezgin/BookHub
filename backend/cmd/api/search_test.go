package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// recordingBooks, liste ucunun hangi yolu (List ya da Search) çağırdığını ve sorguya verdiği
// context'in o andaki durumunu kaydeder.
type recordingBooks struct {
	fakeBooks
	listCalls, searchCalls int
	ctxErr                 error // sorgu çağrıldığı anda context iptal miydi
	hasDeadline            bool
	untilDeadline          time.Duration
}

func (r *recordingBooks) record(ctx context.Context) {
	r.ctxErr = ctx.Err()
	if d, ok := ctx.Deadline(); ok {
		r.hasDeadline, r.untilDeadline = true, time.Until(d)
	}
}

func (r *recordingBooks) List(ctx context.Context, f books.ListFilter) ([]db.BookView, int64, error) {
	r.listCalls++
	r.record(ctx)
	return r.fakeBooks.List(ctx, f)
}

func (r *recordingBooks) Search(ctx context.Context, f books.ListFilter) ([]books.SearchHit, int64, error) {
	r.searchCalls++
	r.record(ctx)
	return r.fakeBooks.Search(ctx, f)
}

// TestListBooksSearchBranch: ?q= varsa arama yolu (tek sorgu: sayfa + toplam + türler), yoksa liste
// yolu çalışır; iki yol aynı cevap biçimini üretir.
func TestListBooksSearchBranch(t *testing.T) {
	sample := fakeBooks{
		total: 2,
		list:  []db.BookView{{ID: 7, Title: "İnce Memed"}, {ID: 3, Title: "Yer Demir Gök Bakır"}},
		genres: map[int64][]db.Genre{
			7: {{ID: 10, Name: "Roman", Slug: "roman"}},
		},
	}
	tests := []struct {
		name, url          string
		wantList, wantSrch int
	}{
		{"aramasız → liste yolu", "/api/books", 1, 0},
		{"arama → arama yolu", "/api/books?q=ince%20memed", 0, 1},
		{"yalnız boşluktan oluşan q arama sayılmaz", "/api/books?q=%20%20", 1, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &recordingBooks{fakeBooks: sample}
			rec := do(t, newRouter(deps{books: store}), http.MethodGet, tc.url, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("durum kodu: istenen 200, gelen %d (%s)", rec.Code, rec.Body)
			}
			if store.listCalls != tc.wantList || store.searchCalls != tc.wantSrch {
				t.Errorf("çağrılar: %d liste, %d arama; istenen %d ve %d", store.listCalls, store.searchCalls, tc.wantList, tc.wantSrch)
			}
			var page bookPage
			if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
				t.Fatalf("geçersiz JSON: %v", err)
			}
			if page.Total != 2 || len(page.Items) != 2 || page.Items[0].ID != 7 || page.Items[1].ID != 3 {
				t.Fatalf("sayfa içeriği ve sırası korunmalı: %+v", page)
			}
			if len(page.Items[0].Genres) != 1 || page.Items[0].Genres[0].Slug != "roman" {
				t.Errorf("1. kitabın türü cevapta olmalı: %+v", page.Items[0].Genres)
			}
			if page.Items[1].Genres == nil || len(page.Items[1].Genres) != 0 {
				t.Errorf("türsüz kitapta genres [] olmalı (null değil), gelen %#v", page.Items[1].Genres)
			}
		})
	}

	t.Run("boş arama sonucu → items [] (null değil)", func(t *testing.T) {
		rec := do(t, newRouter(deps{books: &recordingBooks{}}), http.MethodGet, "/api/books?q=yok", "")
		var raw struct {
			Items json.RawMessage `json:"items"`
			Total int64           `json:"total"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("istenen 200 + JSON, gelen %d %v", rec.Code, err)
		}
		if string(raw.Items) != "[]" || raw.Total != 0 {
			t.Errorf("boş sonuç items [] ve total 0 olmalı: %s, %d", raw.Items, raw.Total)
		}
	})

	t.Run("arama hatası → 500 internal", func(t *testing.T) {
		store := &recordingBooks{fakeBooks: fakeBooks{err: errors.New("arama sorgusu koptu")}}
		rec := do(t, newRouter(deps{books: store}), http.MethodGet, "/api/books?q=dune", "")
		if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "internal" {
			t.Errorf("500 internal bekleniyordu: %d %s", rec.Code, rec.Body)
		}
	})
}

// TestListBooksRejectsInvalidTextBeforeQuery: geçersiz UTF-8 ya da NUL içeren arama metni sorguya
// ulaşmadan 400 alır. Ulaşsaydı Postgres "invalid byte sequence" hatası verir, istemcinin
// düzeltebileceği bir girdi 500 ve ERROR satırı üretirdi.
func TestListBooksRejectsInvalidTextBeforeQuery(t *testing.T) {
	for _, q := range []string{"%FF", "a%00b", "a%C3", "%ED%A0%80"} {
		t.Run(q, func(t *testing.T) {
			store := &recordingBooks{}
			rec := do(t, newRouter(deps{books: store}), http.MethodGet, "/api/books?q="+q, "")
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_error" {
				t.Errorf("istenen 400 validation_error, gelen %d %s", rec.Code, rec.Body)
			}
			if store.listCalls+store.searchCalls != 0 {
				t.Errorf("geçersiz metin sorguya ulaşmamalıydı: %d liste, %d arama", store.listCalls, store.searchCalls)
			}
		})
	}
}

// TestListBooksQueryOutlivesClient: istemci cevabı beklemeden gitse de (yazarken aramada yeni harf,
// sayfadan ayrılma) sorgu İPTAL EDİLMEZ, kendi süre sınırıyla çalışır. İptal sorguya ulaşsaydı pgx
// o bağlantıyı kapatırdı (yarıda kesilen 20 istek = 19 yeni veritabanı bağlantısı; REVIEW_FEEDBACK F21).
// Gitmiş istemciye gövde yazılmaz; erişim loguna 499 düşer (200 ya da 500 değil).
func TestListBooksQueryOutlivesClient(t *testing.T) {
	for _, url := range []string{"/api/books", "/api/books?q=dune"} {
		t.Run(url, func(t *testing.T) {
			store := &recordingBooks{fakeBooks: fakeBooks{total: 1, list: []db.BookView{{ID: 1, Title: "Dune"}}}}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			ctx, cancel := context.WithCancel(req.Context())
			cancel() // istemci, sorgu başlamadan gitti
			rec := httptest.NewRecorder()
			newRouter(deps{books: store}).ServeHTTP(rec, req.WithContext(ctx))

			if store.listCalls+store.searchCalls != 1 {
				t.Fatalf("sorgu bir kez çalışmalıydı: %d liste, %d arama", store.listCalls, store.searchCalls)
			}
			if store.ctxErr != nil {
				t.Errorf("sorgunun context'i istemci iptalinden ayrılmış olmalı, gelen %v", store.ctxErr)
			}
			if !store.hasDeadline || store.untilDeadline <= 0 || store.untilDeadline > listQueryTimeout {
				t.Errorf("sorgunun kendi süre sınırı olmalı (≤ %v), gelen %v (var mı: %v)", listQueryTimeout, store.untilDeadline, store.hasDeadline)
			}
			if rec.Code != statusClientClosedRequest || rec.Body.Len() != 0 {
				t.Errorf("gitmiş istemciye gövdesiz %d bekleniyordu, gelen %d %q", statusClientClosedRequest, rec.Code, rec.Body)
			}
		})
	}

	t.Run("istemci bekliyorsa süre sınırı yine vardır ve cevap 200'dür", func(t *testing.T) {
		store := &recordingBooks{}
		rec := do(t, newRouter(deps{books: store}), http.MethodGet, "/api/books?q=dune", "")
		if rec.Code != http.StatusOK || !store.hasDeadline {
			t.Errorf("200 ve süre sınırı bekleniyordu: %d, sınır var mı: %v", rec.Code, store.hasDeadline)
		}
	})
}
