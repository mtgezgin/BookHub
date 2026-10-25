package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/library"
)

type fakeLibrary struct {
	entry    db.LibraryEntry
	statuses map[int64]string
	rows     []db.ListLibraryRow
	total    int64
	err      error

	gotUser, gotBook int64
	gotStatus        string
	gotIDs           []int64
	gotFilter        *library.ListFilter
	calls            int
}

func (f *fakeLibrary) Put(_ context.Context, user, book int64, status string) (db.LibraryEntry, error) {
	f.calls++
	f.gotUser, f.gotBook, f.gotStatus = user, book, status
	return f.entry, f.err
}
func (f *fakeLibrary) Remove(_ context.Context, user, book int64) error {
	f.calls++
	f.gotUser, f.gotBook = user, book
	return f.err
}
func (f *fakeLibrary) Statuses(_ context.Context, user int64, ids []int64) (map[int64]string, error) {
	f.calls++
	f.gotUser, f.gotIDs = user, ids
	return f.statuses, f.err
}
func (f *fakeLibrary) List(_ context.Context, user int64, filter library.ListFilter) ([]db.ListLibraryRow, int64, error) {
	f.calls++
	f.gotUser, f.gotFilter = user, &filter
	return f.rows, f.total, f.err
}

func libraryRouter(fake *fakeLibrary) http.Handler {
	return userRouter(deps{library: fake, books: fakeBooks{}})
}

func TestPutLibraryEntry(t *testing.T) {
	added := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

	t.Run("geçerli durum → 200, kullanıcı token'dan", func(t *testing.T) {
		fake := &fakeLibrary{entry: db.LibraryEntry{UserID: 7, BookID: 5, Status: "READ", CreatedAt: added, UpdatedAt: added}}
		rec := do(t, libraryRouter(fake), http.MethodPut, "/api/me/library/5", `{"status":"READ"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body)
		}
		if fake.gotUser != 7 || fake.gotBook != 5 || fake.gotStatus != "READ" {
			t.Errorf("depoya giden: user %d book %d status %q", fake.gotUser, fake.gotBook, fake.gotStatus)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"book_id", "status", "added_at", "updated_at"} {
			if _, ok := body[k]; !ok {
				t.Errorf("cevapta %q alanı yok: %s", k, rec.Body)
			}
		}
		if _, ok := body["user_id"]; ok {
			t.Errorf("cevapta user_id olmamalı: %s", rec.Body)
		}
	})

	tests := []struct {
		name, url, body string
		err             error
		wantStatus      int
		wantCode        string
	}{
		{"geçersiz durum", "/api/me/library/5", `{"status":"DONE"}`, nil, http.StatusBadRequest, "validation_error"},
		{"durum yok", "/api/me/library/5", `{}`, nil, http.StatusBadRequest, "validation_error"},
		{"bilinmeyen alan", "/api/me/library/5", `{"status":"READ","user_id":9}`, nil, http.StatusBadRequest, "invalid_json"},
		{"bozuk kimlik", "/api/me/library/abc", `{"status":"READ"}`, nil, http.StatusBadRequest, "invalid_id"},
		{"yayında olmayan kitap", "/api/me/library/5", `{"status":"READ"}`, books.ErrNotFound, http.StatusNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeLibrary{err: tt.err}
			rec := do(t, libraryRouter(fake), http.MethodPut, tt.url, tt.body)
			if rec.Code != tt.wantStatus || errorCode(t, rec) != tt.wantCode {
				t.Errorf("status %d kod %q, istenen %d %q (%s)", rec.Code, errorCode(t, rec), tt.wantStatus, tt.wantCode, rec.Body)
			}
		})
	}

	t.Run("token yok → 401", func(t *testing.T) {
		fake := &fakeLibrary{}
		rec := do(t, newRouter(deps{library: fake, tokens: fakeVerifier{}}), http.MethodPut, "/api/me/library/5", `{"status":"READ"}`)
		if rec.Code != http.StatusUnauthorized || fake.calls != 0 {
			t.Errorf("status %d, depo çağrısı %d; istenen 401 ve 0", rec.Code, fake.calls)
		}
	})
}

func TestRemoveLibraryEntry(t *testing.T) {
	fake := &fakeLibrary{}
	rec := do(t, libraryRouter(fake), http.MethodDelete, "/api/me/library/5", "")
	if rec.Code != http.StatusNoContent || fake.gotUser != 7 || fake.gotBook != 5 {
		t.Errorf("status %d, user %d, book %d", rec.Code, fake.gotUser, fake.gotBook)
	}
}

func TestLibraryStatuses(t *testing.T) {
	t.Run("boş parametre → boş nesne, depo çağrılmaz", func(t *testing.T) {
		fake := &fakeLibrary{}
		rec := do(t, libraryRouter(fake), http.MethodGet, "/api/me/library/statuses?book_ids=", "")
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"statuses":{}}` || fake.calls != 0 {
			t.Errorf("status %d gövde %s çağrı %d", rec.Code, rec.Body, fake.calls)
		}
	})

	t.Run("tekrarlar ayıklanır, kimlik anahtarlı nesne döner", func(t *testing.T) {
		fake := &fakeLibrary{statuses: map[int64]string{3: "READ"}}
		rec := do(t, libraryRouter(fake), http.MethodGet, "/api/me/library/statuses?book_ids=3,3,4", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body)
		}
		if !slices.Equal(fake.gotIDs, []int64{3, 4}) || fake.gotUser != 7 {
			t.Errorf("depoya giden kimlikler %v, kullanıcı %d", fake.gotIDs, fake.gotUser)
		}
		if strings.TrimSpace(rec.Body.String()) != `{"statuses":{"3":"READ"}}` {
			t.Errorf("gövde %s", rec.Body)
		}
	})

	many := make([]string, 101)
	for i := range many {
		many[i] = strconv.Itoa(i + 1)
	}
	for _, raw := range []string{"1,,2", "0", "-1", "x", "1.5", strings.Join(many, ",")} {
		t.Run("bozuk book_ids "+raw[:min(len(raw), 10)], func(t *testing.T) {
			fake := &fakeLibrary{}
			rec := do(t, libraryRouter(fake), http.MethodGet, "/api/me/library/statuses?book_ids="+raw, "")
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_id" || fake.calls != 0 {
				t.Errorf("status %d kod %q çağrı %d", rec.Code, errorCode(t, rec), fake.calls)
			}
		})
	}
}

func TestListLibrary(t *testing.T) {
	t.Run("geçerli parametreler süzgece aynen gider", func(t *testing.T) {
		fake := &fakeLibrary{}
		rec := do(t, libraryRouter(fake), http.MethodGet,
			"/api/me/library?status=READ&q=dune&genre=bilim-kurgu&rated=no&sort=title&limit=12&offset=24", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body)
		}
		want := library.ListFilter{Status: "READ", Query: "dune", Genre: "bilim-kurgu", Rated: "no", Sort: "title", Limit: 12, Offset: 24}
		if fake.gotFilter == nil || *fake.gotFilter != want || fake.gotUser != 7 {
			t.Errorf("süzgeç %+v, kullanıcı %d; istenen %+v", fake.gotFilter, fake.gotUser, want)
		}
	})

	for _, q := range []string{"status=DONE", "rated=maybe", "sort=year", "limit=0", "genre=Bilim%20Kurgu", "q=" + strings.Repeat("a", 101)} {
		t.Run("geçersiz "+q[:min(len(q), 12)], func(t *testing.T) {
			fake := &fakeLibrary{}
			rec := do(t, libraryRouter(fake), http.MethodGet, "/api/me/library?"+q, "")
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_error" || fake.calls != 0 {
				t.Errorf("status %d kod %q çağrı %d", rec.Code, errorCode(t, rec), fake.calls)
			}
		})
	}

	t.Run("öğe: kitap, durum, puanım ve katalog bilgisi", func(t *testing.T) {
		added := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
		fake := &fakeLibrary{total: 2, rows: []db.ListLibraryRow{
			{Book: db.Book{ID: 1, Title: "Dune"}, Status: "READ", AddedAt: added, MyRating: ptr(int16(8)), InCatalog: true},
			{Book: db.Book{ID: 2, Title: "Vakıf"}, Status: "READING", AddedAt: added, InCatalog: false},
		}}
		rec := do(t, libraryRouter(fake), http.MethodGet, "/api/me/library", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body)
		}
		var page struct {
			Items []struct {
				Book      struct{ ID int64 } `json:"book"`
				Status    string             `json:"status"`
				MyRating  *int               `json:"my_rating"`
				InCatalog bool               `json:"in_catalog"`
			} `json:"items"`
			Total  int64 `json:"total"`
			Limit  int32 `json:"limit"`
			Offset int32 `json:"offset"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if page.Total != 2 || page.Limit != defaultPageLimit || len(page.Items) != 2 {
			t.Fatalf("sayfa %+v", page)
		}
		first, second := page.Items[0], page.Items[1]
		if first.Book.ID != 1 || first.Status != "READ" || first.MyRating == nil || *first.MyRating != 8 || !first.InCatalog {
			t.Errorf("ilk öğe %+v", first)
		}
		if second.MyRating != nil || second.InCatalog {
			t.Errorf("ikinci öğe puansız ve katalog dışı olmalı: %+v", second)
		}
		if !strings.Contains(rec.Body.String(), `"my_rating":null`) {
			t.Errorf("puansız öğede my_rating null olmalı: %s", rec.Body)
		}
	})

	t.Run("boş kitaplık → items boş dizi", func(t *testing.T) {
		rec := do(t, libraryRouter(&fakeLibrary{}), http.MethodGet, "/api/me/library", "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items":[]`) {
			t.Errorf("status %d gövde %s", rec.Code, rec.Body)
		}
	})
}

func TestLibraryStoreErrors(t *testing.T) {
	dbErr := errors.New("db")
	tests := []struct {
		name, method, url, body string
		books                   fakeBooks
		fake                    *fakeLibrary
	}{
		{"silme depo hatası", http.MethodDelete, "/api/me/library/5", "", fakeBooks{}, &fakeLibrary{err: dbErr}},
		{"ekleme depo hatası 404 değil", http.MethodPut, "/api/me/library/5", `{"status":"READ"}`, fakeBooks{}, &fakeLibrary{err: dbErr}},
		{"durum sorgusu depo hatası", http.MethodGet, "/api/me/library/statuses?book_ids=3", "", fakeBooks{}, &fakeLibrary{err: dbErr}},
		{"liste depo hatası", http.MethodGet, "/api/me/library", "", fakeBooks{}, &fakeLibrary{err: dbErr}},
		{"liste tür yüklemesi hatası", http.MethodGet, "/api/me/library", "", fakeBooks{genreErr: dbErr}, &fakeLibrary{total: 1, rows: []db.ListLibraryRow{{Book: db.Book{ID: 1, Title: "Dune"}, Status: "READ"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, userRouter(deps{library: tt.fake, books: tt.books}), tt.method, tt.url, tt.body)
			if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "internal" {
				t.Errorf("500 internal bekleniyordu: %d %s", rec.Code, rec.Body)
			}
		})
	}
}
