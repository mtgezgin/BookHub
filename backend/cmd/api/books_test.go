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

// fakeBooks, bookStore'un SAHTE uygulaması: ne dönmesi söylendiyse onu döner.
// Gerçek DB yok → saf birim testi. (Interface'i tüketen tarafta tanımlamanın meyvesi.)
type fakeBooks struct {
	book     db.Book
	list     []db.Book
	total    int64
	filter   *books.ListFilter // List'e gelen filtreyi yakalamak için (nil = yakalama)
	genres   map[int64][]db.Genre
	err      error
	setErr   error
	genreErr error
}

func (f fakeBooks) Get(context.Context, int64) (db.Book, error) { return f.book, f.err }
func (f fakeBooks) List(_ context.Context, lf books.ListFilter) ([]db.Book, int64, error) {
	if f.filter != nil {
		*f.filter = lf
	}
	return f.list, f.total, f.err
}
func (f fakeBooks) Create(context.Context, db.CreateBookParams) (db.Book, error) {
	return f.book, f.err
}
func (f fakeBooks) GenresForBook(_ context.Context, id int64) ([]db.Genre, error) {
	return f.genres[id], f.genreErr
}
func (f fakeBooks) GenresForBooks(context.Context, []int64) (map[int64][]db.Genre, error) {
	return f.genres, f.genreErr
}
func (f fakeBooks) SetGenres(context.Context, int64, []int64) error { return f.setErr }

// capturingBooks, Create'e GEÇİLEN params'ı kaydeden sahte (pointer receiver: duruma
// yazması gerekiyor). "Handler DB'ye ne yazıyor" iddiası ancak böyle kanıtlanır.
type capturingBooks struct {
	fakeBooks
	got db.CreateBookParams
}

func (f *capturingBooks) Create(_ context.Context, p db.CreateBookParams) (db.Book, error) {
	f.got = p
	return f.book, f.err
}

func do(t *testing.T, h http.Handler, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("hata cevabı geçerli JSON değil: %v (gövde: %s)", err, rec.Body)
	}
	return env.Error.Code
}

// TestHandleGetBook, GET /api/books/{id}'nin hata HARİTALAMASINI doğrular:
// geçersiz girdi → 400, yok → 404, repo hatası → 500, var → 200 + DTO.
func TestHandleGetBook(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		fake       fakeBooks
		wantStatus int
		wantCode   string // "" = başarı beklenir
	}{
		{name: "geçerli id → 200", url: "/api/books/1", fake: fakeBooks{book: db.Book{ID: 1, Title: "Clean Code"}}, wantStatus: http.StatusOK},
		{name: "olmayan id → 404", url: "/api/books/999", fake: fakeBooks{err: books.ErrNotFound}, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "sayı olmayan id → 400 (sorguya gitmeden)", url: "/api/books/abc", wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "sıfır id → 400", url: "/api/books/0", wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "repo hatası → 500", url: "/api/books/1", fake: fakeBooks{err: errors.New("x")}, wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newRouter(deps{books: tt.fake}), http.MethodGet, tt.url, "")
			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Errorf("error.code: istenen %q, gelen %q", tt.wantCode, got)
				}
				return
			}
			var got bookResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("kitap cevabı geçerli JSON değil: %v", err)
			}
			if got.ID != tt.fake.book.ID || got.Title != tt.fake.book.Title {
				t.Errorf("kitap yanlış: istenen %+v, gelen %+v", tt.fake.book, got)
			}
		})
	}
}

// TestHandlersDoNotLeakInternalErrors, "detay log'a, istemciye değil" kuralını ÜÇ uç
// için GÖVDE üzerinden kanıtlar: error.code'un "internal" olması yetmez — sürücü/şema
// bilgisi taşıyan metnin cevabın hiçbir yerinde geçmemesi gerekir.
func TestHandlersDoNotLeakInternalErrors(t *testing.T) {
	const driverDetail = `ERROR: relation "books" does not exist (SQLSTATE 42P01) @ 10.0.0.7:5432`
	fake := fakeBooks{err: fmt.Errorf("kitap sorgusu (id=1): %w", errors.New(driverDetail))}
	router := newRouter(deps{books: fake})

	for _, c := range []struct{ method, url, body string }{
		{http.MethodGet, "/api/books/1", ""},
		{http.MethodGet, "/api/books", ""},
		{http.MethodPost, "/api/admin/books", `{"title":"T","slug":"t","author":"A"}`},
	} {
		t.Run(c.method+" "+c.url, func(t *testing.T) {
			rec := do(t, router, c.method, c.url, c.body)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("durum kodu: istenen 500, gelen %d", rec.Code)
			}
			body := rec.Body.String()
			for _, leak := range []string{"relation", "SQLSTATE", "42P01", "10.0.0.7", "5432", "kitap sorgusu"} {
				if strings.Contains(body, leak) {
					t.Errorf("iç detay gövdeye sızdı (%q): %s", leak, body)
				}
			}
		})
	}
}

// TestHandleListBooks: boş sayfada items "[]" (null DEĞİL); dolu sayfada İÇERİK, SIRA ve
// sayfalama bilgisi korunur (yalnızca eleman sayısına bakmak, boş kayıtlar dönen bir
// hatayı kaçırırdı).
func TestHandleListBooks(t *testing.T) {
	t.Run("boş sayfa → items [] (null değil)", func(t *testing.T) {
		rec := do(t, newRouter(deps{books: fakeBooks{}}), http.MethodGet, "/api/books", "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items":[]`) {
			t.Fatalf("istenen 200 + items:[], gelen %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("dolu sayfa → içerik, sıra ve toplam korunur", func(t *testing.T) {
		fake := fakeBooks{total: 42, list: []db.Book{
			{ID: 7, Title: "The Go Programming Language"},
			{ID: 3, Title: "Clean Code"},
		}}
		rec := do(t, newRouter(deps{books: fake}), http.MethodGet, "/api/books", "")
		var got bookPage
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("istenen 200 + JSON, gelen %d %v", rec.Code, err)
		}
		if len(got.Items) != 2 || got.Items[0].ID != 7 || got.Items[1].ID != 3 {
			t.Errorf("içerik/sıra bozuldu: %+v", got.Items)
		}
		if got.Total != 42 || got.Limit != defaultPageLimit || got.Offset != 0 {
			t.Errorf("sayfa bilgisi yanlış: total=%d limit=%d offset=%d", got.Total, got.Limit, got.Offset)
		}
	})
}

// TestListBooksQueryParams, sorgu parametrelerinin filtreye DOĞRU aktarıldığını ve
// geçersiz değerlerin sorguya hiç gitmeden 400 aldığını doğrular.
func TestListBooksQueryParams(t *testing.T) {
	valid := []struct {
		url  string
		want books.ListFilter
	}{
		{"/api/books", books.ListFilter{Limit: 20}},
		{"/api/books?genre=bilim-kurgu&q=%20dune%20&limit=5&offset=10", books.ListFilter{Genre: "bilim-kurgu", Query: "dune", Limit: 5, Offset: 10}},
		{"/api/books?limit=100", books.ListFilter{Limit: 100}},
	}
	for _, tc := range valid {
		t.Run(tc.url, func(t *testing.T) {
			var got books.ListFilter
			rec := do(t, newRouter(deps{books: fakeBooks{filter: &got}}), http.MethodGet, tc.url, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("durum kodu: istenen 200, gelen %d (%s)", rec.Code, rec.Body)
			}
			if got != tc.want {
				t.Errorf("filtre: istenen %+v, gelen %+v", tc.want, got)
			}
		})
	}

	for _, url := range []string{
		"/api/books?limit=0", "/api/books?limit=101", "/api/books?limit=abc",
		"/api/books?offset=-1", "/api/books?offset=10001",
		"/api/books?genre=Kötü%20Slug", "/api/books?q=" + strings.Repeat("a", maxQueryLen+1),
	} {
		t.Run("400: "+url[:min(len(url), 40)], func(t *testing.T) {
			rec := do(t, newRouter(deps{books: fakeBooks{}}), http.MethodGet, url, "")
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_error" {
				t.Errorf("istenen 400 validation_error, gelen %d %s", rec.Code, rec.Body)
			}
		})
	}
}

// TestHandleCreateBook, create ucunun girdi doğrulamasını ve hata haritalamasını doğrular.
// "status gönderilirse 400" bir REGRESYON testidir: bu uç bir ara istemcinin status
// yollayıp kitabını kendi yayına almasına izin veriyordu (mass-assignment).
func TestHandleCreateBook(t *testing.T) {
	const valid = `{"title":"The Go Programming Language","slug":"gopl","author":"Donovan"}`
	long := strings.Repeat("ş", maxTitleLen+1) // rune sayar: "ş" 2 bayt ama 1 karakter

	tests := []struct {
		name       string
		body       string
		fake       fakeBooks
		wantStatus int
		wantCode   string // "" = başarı
	}{
		{name: "geçerli gövde → 201", body: valid, fake: fakeBooks{book: db.Book{ID: 7}}, wantStatus: http.StatusCreated},
		{name: "yayın yılı alt sınırı 1450 kabul", body: `{"title":"T","slug":"t","author":"a","publication_year":1450}`, wantStatus: http.StatusCreated},
		{name: "tam sınırda başlık (300 karakter) kabul", body: `{"title":"` + strings.Repeat("ş", maxTitleLen) + `","slug":"t","author":"a"}`, wantStatus: http.StatusCreated},

		{name: "REGRESYON: istemci status yollarsa → 400", body: `{"title":"S","slug":"s","author":"X","status":"APPROVED"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "bilinmeyen alan (typo) → 400", body: `{"titel":"Y","slug":"y","author":"X"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "yarım JSON → 400", body: `{"title": "yarım`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "söz dizimi hatası → 400", body: `{"title":,}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "yanlış tip → 400", body: `{"title":123,"slug":"s","author":"a"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "boş gövde → 400", body: ``, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "iki JSON nesnesi → 400", body: valid + `{"title":"ikinci"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},

		{name: "zorunlu alanlar eksik → 400", body: `{"description":"x"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "yalnız boşluk başlık → 400", body: `{"title":"   ","slug":"s","author":"a"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "301 karakterlik başlık → 400", body: `{"title":"` + long + `","slug":"s","author":"a"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "geçersiz slug → 400", body: `{"title":"T","slug":"Kötü Slug!","author":"a"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "javascript: kapak URL'si → 400 (XSS)", body: `{"title":"T","slug":"t","author":"a","cover_url":"javascript:alert(1)"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "geçersiz dil kodu → 400", body: `{"title":"T","slug":"t","author":"a","language":"türkçe"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "page_count 0 → 400", body: `{"title":"T","slug":"t","author":"a","page_count":0}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "yayın yılı 1449 → 400", body: `{"title":"T","slug":"t","author":"a","publication_year":1449}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "yayın yılı 9999 → 400", body: `{"title":"T","slug":"t","author":"a","publication_year":9999}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},

		{name: "aynı slug → 409 (500 değil)", body: valid, fake: fakeBooks{err: books.ErrDuplicateSlug}, wantStatus: http.StatusConflict, wantCode: "slug_conflict"},
		{name: "repo hatası → 500", body: valid, fake: fakeBooks{err: errors.New("x")}, wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newRouter(deps{books: tt.fake}), http.MethodPost, "/api/admin/books", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Errorf("error.code: istenen %q, gelen %q", tt.wantCode, got)
				}
			}
		})
	}
}

// TestHandleCreateBookWritesExactParams, handler'ın DB'ye NE YAZDIĞINI alan alan
// doğrular (cevaba değil). Bir alanı karıştırmak (slug↔author), düşürmek (yıl=nil)
// ya da istemcinin değerini ezmek (language hep "tr") bu testi kırar.
func TestHandleCreateBookWritesExactParams(t *testing.T) {
	year, pages := int32(2015), int32(380)

	t.Run("tüm alanlar aynen, trim uygulanmış, statü sunucudan", func(t *testing.T) {
		fake := &capturingBooks{fakeBooks: fakeBooks{book: db.Book{ID: 1}}}
		body := `{"title":"  The Go Programming Language  ","slug":"gopl","author":" Donovan ",
			"description":" Go'nun kutsal kitabı ","cover_url":"https://example.test/gopl.jpg",
			"publication_year":2015,"page_count":380,"language":"en"}`
		rec := do(t, newRouter(deps{books: fake}), http.MethodPost, "/api/admin/books", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("durum kodu: istenen 201, gelen %d (gövde: %s)", rec.Code, rec.Body)
		}
		want := db.CreateBookParams{
			Title: "The Go Programming Language", Slug: "gopl", Author: "Donovan",
			Description: "Go'nun kutsal kitabı", CoverUrl: "https://example.test/gopl.jpg",
			PublicationYear: &year, PageCount: &pages, Language: "en", Status: books.StatusApproved,
		}
		assertParams(t, fake.got, want)
	})

	t.Run("slug gönderilmezse başlıktan türetilir (Türkçe karakterler dahil)", func(t *testing.T) {
		fake := &capturingBooks{fakeBooks: fakeBooks{book: db.Book{ID: 1}}}
		rec := do(t, newRouter(deps{books: fake}), http.MethodPost, "/api/admin/books", `{"title":"Şeker Portakalı","author":"José Mauro"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("durum kodu: istenen 201, gelen %d (%s)", rec.Code, rec.Body)
		}
		if fake.got.Slug != "seker-portakali" {
			t.Errorf("türetilen slug: istenen %q, gelen %q", "seker-portakali", fake.got.Slug)
		}
	})

	t.Run("language gönderilmezse varsayılan tr (DB DEFAULT devreye girmez)", func(t *testing.T) {
		fake := &capturingBooks{fakeBooks: fakeBooks{book: db.Book{ID: 1}}}
		rec := do(t, newRouter(deps{books: fake}), http.MethodPost, "/api/admin/books", `{"title":"T","slug":"t","author":"a"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("durum kodu: istenen 201, gelen %d", rec.Code)
		}
		if fake.got.Language != defaultLanguage || fake.got.PublicationYear != nil || fake.got.PageCount != nil {
			t.Errorf("varsayılanlar yanlış: %+v", fake.got)
		}
	})
}

// assertParams, pointer alanlar yüzünden struct'lar == ile karşılaştırılamadığı için
// alan alan kıyaslar.
func assertParams(t *testing.T, got, want db.CreateBookParams) {
	t.Helper()
	deref := func(p *int32) string {
		if p == nil {
			return "nil"
		}
		return fmt.Sprint(*p)
	}
	type row struct{ field, got, want string }
	for _, r := range []row{
		{"Title", got.Title, want.Title},
		{"Slug", got.Slug, want.Slug},
		{"Author", got.Author, want.Author},
		{"Description", got.Description, want.Description},
		{"CoverUrl", got.CoverUrl, want.CoverUrl},
		{"PublicationYear", deref(got.PublicationYear), deref(want.PublicationYear)},
		{"PageCount", deref(got.PageCount), deref(want.PageCount)},
		{"Language", got.Language, want.Language},
		{"Status", got.Status, want.Status},
	} {
		if r.got != r.want {
			t.Errorf("%s: istenen %q, gelen %q", r.field, r.want, r.got)
		}
	}
}
