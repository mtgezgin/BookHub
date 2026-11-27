package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/ratelimit"
	"github.com/mtgezgin/bookhub/backend/internal/reviews"
)

// fakeReviews, inceleme servisinin sahtesi: handler'ın servise NE gönderdiğini kaydeder, verilen
// incelemeyi ya da hatayı döner. Servisin kendisi gerçek veritabanıyla internal/reviews'te test edilir.
type fakeReviews struct {
	review db.Review
	rows   []db.ListBookReviewsRow
	err    error

	calls             int
	gotActor, gotID   int64
	gotDraft          *reviews.Draft
	gotReason         string
	gotLimit, gotSkip int32
}

func (f *fakeReviews) List(_ context.Context, book int64, limit, offset int32) ([]db.ListBookReviewsRow, int64, error) {
	f.calls++
	f.gotID, f.gotLimit, f.gotSkip = book, limit, offset
	return f.rows, int64(len(f.rows)), f.err
}
func (f *fakeReviews) Mine(_ context.Context, user, book int64) (db.Review, error) {
	f.calls++
	f.gotActor, f.gotID = user, book
	return f.review, f.err
}
func (f *fakeReviews) Save(_ context.Context, user, book int64, d reviews.Draft) (db.Review, error) {
	f.calls++
	f.gotActor, f.gotID, f.gotDraft = user, book, &d
	return f.review, f.err
}
func (f *fakeReviews) Delete(_ context.Context, user, book int64) error {
	f.calls++
	f.gotActor, f.gotID = user, book
	return f.err
}
func (f *fakeReviews) Hide(_ context.Context, admin, id int64, reason string) (db.Review, error) {
	f.calls++
	f.gotActor, f.gotID, f.gotReason = admin, id, reason
	return f.review, f.err
}
func (f *fakeReviews) Unhide(_ context.Context, admin, id int64) (db.Review, error) {
	f.calls++
	f.gotActor, f.gotID = admin, id
	return f.review, f.err
}

var reviewTime = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

// reviewCase, bir inceleme ucunun tek senaryosu: istek, servisin hatası, beklenen durum ve hata kodu,
// servisin çağrılıp çağrılmadığı. as: "user" (good-user, id 7), "admin" (good-admin, id 1), "" (kimliksiz).
type reviewCase struct {
	name, as, method, url, body string
	svcErr                      error
	wantStatus                  int
	wantCode                    string // "" = başarı
	wantCall                    bool
}

func runReviewCases(t *testing.T, cases []reviewCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeReviews{err: c.svcErr, review: db.Review{ID: 3, BookID: 5, Body: "Metin", Version: 1, CreatedAt: reviewTime}}
			rec := do(t, reviewRouter(fake, c.as), c.method, c.url, c.body)
			if rec.Code != c.wantStatus {
				t.Fatalf("status %d, beklenen %d (%s)", rec.Code, c.wantStatus, rec.Body)
			}
			if c.wantCode != "" && errorCode(t, rec) != c.wantCode {
				t.Errorf("hata kodu %q, beklenen %q", errorCode(t, rec), c.wantCode)
			}
			if (fake.calls > 0) != c.wantCall {
				t.Errorf("servis çağrısı %d, beklenen çağrı=%v", fake.calls, c.wantCall)
			}
		})
	}
}

func reviewRouter(fake *fakeReviews, as string) http.Handler {
	d := deps{reviews: fake, tokens: fakeVerifier{}}
	switch as {
	case "user":
		return userRouter(d)
	case "admin":
		return adminRouter(d)
	}
	return newRouter(d)
}

const reviewURL = "/api/books/5/review"

// TestSaveReviewRequests: PUT gövdesinin doğrulaması ve servis hatalarının HTTP karşılığı.
func TestSaveReviewRequests(t *testing.T) {
	put := func(name, body string, svcErr error, status int, code string, call bool) reviewCase {
		return reviewCase{name, "user", http.MethodPut, reviewURL, body, svcErr, status, code, call}
	}
	runReviewCases(t, []reviewCase{
		put("ilk yazım → 201", `{"body":"Güzel"}`, nil, http.StatusCreated, "", true),
		put("düzenleme → 200", `{"body":"Güzel","version":1}`, nil, http.StatusOK, "", true),
		put("5000 çok baytlı karakter geçer", `{"body":"`+strings.Repeat("ş", reviews.MaxBodyLen)+`"}`, nil, http.StatusCreated, "", true),
		put("5001 karakter → 400", `{"body":"`+strings.Repeat("a", reviews.MaxBodyLen+1)+`"}`, nil, http.StatusBadRequest, "validation_error", false),
		put("boş metin → 400", `{"body":""}`, nil, http.StatusBadRequest, "validation_error", false),
		put("yalnız boşluk → 400", `{"body":"  \n "}`, nil, http.StatusBadRequest, "validation_error", false),
		put("NUL karakter → 400 (500 değil; readJSON)", `{"body":"a\u0000b"}`, nil, http.StatusBadRequest, "invalid_json", false),
		put("sürüm 0 → 400", `{"body":"a","version":0}`, nil, http.StatusBadRequest, "validation_error", false),
		put("bilinmeyen alan (user_id) → 400", `{"body":"a","user_id":9}`, nil, http.StatusBadRequest, "invalid_json", false),
		put("sürüm uyuşmazlığı → 409", `{"body":"a","version":2}`, reviews.ErrVersionConflict, http.StatusConflict, "version_conflict", true),
		put("yayında olmayan kitap → 404", `{"body":"a"}`, books.ErrNotFound, http.StatusNotFound, "not_found", true),
		put("beklenmeyen hata → 500", `{"body":"a"}`, errors.New("db"), http.StatusInternalServerError, "internal", true),
		{"kimliksiz → 401", "", http.MethodPut, reviewURL, `{"body":"a"}`, nil, http.StatusUnauthorized, "missing_token", false},

		// Faz 17C: belge gövdesi (spec karar 7). Biçimin ayrıntılı ret tablosu internal/richtext'te;
		// burada uç ile doğrulayıcının bağlandığı sınanır.
		put("belge: ilk yazım → 201", `{"doc":`+docOf(`{"type":"text","text":"Güzel"}`)+`}`, nil, http.StatusCreated, "", true),
		put("belge: düzenleme → 200", `{"doc":`+docOf(`{"type":"text","text":"Güzel"}`)+`,"version":1}`, nil, http.StatusOK, "", true),
		put("belge: HTML metindir, geçer", `{"doc":`+docOf(`{"type":"text","text":"<script>alert(1)</script>"}`)+`}`, nil, http.StatusCreated, "", true),
		put("belge: bilinmeyen düğüm → 400", `{"doc":{"type":"doc","content":[{"type":"heading","content":[{"type":"text","text":"a"}]}]}}`, nil, http.StatusBadRequest, "validation_error", false),
		put("belge: bağlantı işareti → 400", `{"doc":`+docOf(`{"type":"text","text":"a","marks":[{"type":"link","attrs":{"href":"javascript:alert(1)"}}]}`)+`}`, nil, http.StatusBadRequest, "validation_error", false),
		put("belge: attrs → 400", `{"doc":{"type":"doc","content":[{"type":"paragraph","attrs":{"style":"x"},"content":[{"type":"text","text":"a"}]}]}}`, nil, http.StatusBadRequest, "validation_error", false),
		put("belge: metin yok → 400", `{"doc":{"type":"doc","content":[{"type":"paragraph"}]}}`, nil, http.StatusBadRequest, "validation_error", false),
		put("belge: 5001 karakter → 400", `{"doc":`+docOf(`{"type":"text","text":"`+strings.Repeat("ş", reviews.MaxBodyLen+1)+`"}`)+`}`, nil, http.StatusBadRequest, "validation_error", false),
		put("belge: NUL → 400 (500 değil)", `{"doc":`+docOf(`{"type":"text","text":"a\u0000b"}`)+`}`, nil, http.StatusBadRequest, "validation_error", false),
		put("belge: nesne değil (HTML metni) → 400", `{"doc":"<p>merhaba</p>"}`, nil, http.StatusBadRequest, "validation_error", false),
		put("belge ile body birlikte → 400", `{"doc":`+docOf(`{"type":"text","text":"a"}`)+`,"body":"b"}`, nil, http.StatusBadRequest, "validation_error", false),
		put("belge ile has_spoiler birlikte → 400", `{"doc":`+docOf(`{"type":"text","text":"a"}`)+`,"has_spoiler":true}`, nil, http.StatusBadRequest, "validation_error", false),
		put("doc null, body dolu: eski gövde sayılır → 201", `{"doc":null,"body":"Güzel"}`, nil, http.StatusCreated, "", true),
		put("ikisi de yok → 400", `{}`, nil, http.StatusBadRequest, "validation_error", false),
		put("eski gövde: 2000'den çok satır → 400 (düğüm sınırı)", `{"body":"`+strings.Repeat(`a\n`, 1200)+`a"}`, nil, http.StatusBadRequest, "validation_error", false),
	})
}

// docOf, tek paragraflı belge JSON'u; içerik paragrafın content dizisinin içidir.
func docOf(content string) string {
	return `{"type":"doc","content":[{"type":"paragraph","content":[` + content + `]}]}`
}

// TestSaveReviewErrorHidesInput: doğrulama mesajı istemcinin gönderdiği değeri geri yazmaz.
func TestSaveReviewErrorHidesInput(t *testing.T) {
	rec := do(t, reviewRouter(&fakeReviews{}, "user"), http.MethodPut, reviewURL,
		`{"doc":{"type":"doc","content":[{"type":"<img src=x onerror=alert(1)>"}]}}`)
	if rec.Code != http.StatusBadRequest || strings.Contains(rec.Body.String(), "onerror") {
		t.Fatalf("status %d, gövde girdiyi taşımamalı: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "doc.content[0]") {
		t.Errorf("mesaj sorunun yerini söylemeli: %s", rec.Body)
	}
}

// TestReviewReadDeleteRequests: okurun GET ve DELETE'i, herkese açık liste.
func TestReviewReadDeleteRequests(t *testing.T) {
	runReviewCases(t, []reviewCase{
		{"incelemem → 200", "user", http.MethodGet, reviewURL, "", nil, http.StatusOK, "", true},
		{"incelemem yok → 404 no_review", "user", http.MethodGet, reviewURL, "", reviews.ErrNotFound, http.StatusNotFound, "no_review", true},
		{"kitap yok → 404 not_found", "user", http.MethodGet, reviewURL, "", books.ErrNotFound, http.StatusNotFound, "not_found", true},
		{"kimliksiz GET → 401", "", http.MethodGet, reviewURL, "", nil, http.StatusUnauthorized, "missing_token", false},
		{"silme → 204", "user", http.MethodDelete, reviewURL, "", nil, http.StatusNoContent, "", true},
		{"silinecek inceleme yok → 404", "user", http.MethodDelete, reviewURL, "", reviews.ErrNotFound, http.StatusNotFound, "no_review", true},
		{"kimliksiz DELETE → 401", "", http.MethodDelete, reviewURL, "", nil, http.StatusUnauthorized, "missing_token", false},
		{"liste kimliksiz → 200", "", http.MethodGet, "/api/books/5/reviews", "", nil, http.StatusOK, "", true},
		{"liste: geçersiz limit → 400", "", http.MethodGet, "/api/books/5/reviews?limit=0", "", nil, http.StatusBadRequest, "validation_error", false},
		{"liste: kitap yok → 404", "", http.MethodGet, "/api/books/5/reviews", "", books.ErrNotFound, http.StatusNotFound, "not_found", true},
	})
}

// TestModerateReviewRequests: yönetici uçları. Kullanıcı rolü 403 (TestAdminRoutesAreGuarded da sınar).
func TestModerateReviewRequests(t *testing.T) {
	const hide, unhide = "/api/admin/reviews/3/hide", "/api/admin/reviews/3/unhide"
	runReviewCases(t, []reviewCase{
		{"gizleme → 200", "admin", http.MethodPost, hide, `{"reason":"Kişisel bilgi"}`, nil, http.StatusOK, "", true},
		{"gerekçesiz gizleme → 400", "admin", http.MethodPost, hide, `{"reason":"  "}`, nil, http.StatusBadRequest, "validation_error", false},
		{"501 karakter gerekçe → 400", "admin", http.MethodPost, hide, `{"reason":"` + strings.Repeat("g", reviews.MaxReasonLen+1) + `"}`, nil, http.StatusBadRequest, "validation_error", false},
		{"gövdesiz gizleme → 400", "admin", http.MethodPost, hide, "", nil, http.StatusBadRequest, "invalid_json", false},
		{"olmayan inceleme → 404", "admin", http.MethodPost, hide, `{"reason":"x"}`, reviews.ErrNotFound, http.StatusNotFound, "not_found", true},
		{"geri açma → 200", "admin", http.MethodPost, unhide, "", nil, http.StatusOK, "", true},
		{"geri açma: olmayan inceleme → 404", "admin", http.MethodPost, unhide, "", reviews.ErrNotFound, http.StatusNotFound, "not_found", true},
		{"geri açma: beklenmeyen hata → 500", "admin", http.MethodPost, unhide, "", errors.New("db"), http.StatusInternalServerError, "internal", true},
		{"kullanıcı gizleyemez → 403", "user", http.MethodPost, hide, `{"reason":"x"}`, nil, http.StatusForbidden, "forbidden", false},
	})
}

// TestReviewInputsReachService: kullanıcı token'dan, kitap adresten; metin ve gerekçe kırpılmış,
// sürüm ve spoiler bayrağı olduğu gibi; liste sayfası sorgudan.
func TestReviewInputsReachService(t *testing.T) {
	fake := &fakeReviews{}
	do(t, reviewRouter(fake, "user"), http.MethodPut, reviewURL, `{"body":"  Güzel \n","has_spoiler":true,"version":4}`)
	if d := fake.gotDraft; fake.gotActor != 7 || fake.gotID != 5 || d == nil || d.Doc.Text() != "Güzel" || !d.Doc.HasSpoiler() || d.Version == nil || *d.Version != 4 {
		t.Errorf("servise giden: user %d kitap %d taslak %+v", fake.gotActor, fake.gotID, d)
	}

	// Yeni gövde: belge doğrulanmış ve kanonik hâliyle gider; spoiler belgeden okunur.
	fake = &fakeReviews{}
	do(t, reviewRouter(fake, "user"), http.MethodPut, reviewURL,
		`{"doc":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Katil ","marks":[{"type":"spoiler"}]},{"type":"text","text":"uşak","marks":[{"type":"spoiler"}]}]}]},"version":2}`)
	const canonical = `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","marks":[{"type":"spoiler"}],"text":"Katil uşak"}]}]}`
	if d := fake.gotDraft; d == nil || string(d.Doc.JSON()) != canonical || !d.Doc.HasSpoiler() || d.Version == nil || *d.Version != 2 {
		t.Errorf("servise giden belge: %+v", d)
	}

	fake = &fakeReviews{}
	do(t, reviewRouter(fake, "admin"), http.MethodPost, "/api/admin/reviews/3/hide", `{"reason":" Spam "}`)
	if fake.gotActor != 1 || fake.gotID != 3 || fake.gotReason != "Spam" {
		t.Errorf("gizlemede servise giden: yönetici %d inceleme %d gerekçe %q", fake.gotActor, fake.gotID, fake.gotReason)
	}

	fake = &fakeReviews{}
	do(t, reviewRouter(fake, ""), http.MethodGet, "/api/books/5/reviews?limit=5&offset=10", "")
	if fake.gotID != 5 || fake.gotLimit != 5 || fake.gotSkip != 10 {
		t.Errorf("listede servise giden: kitap %d limit %d offset %d", fake.gotID, fake.gotLimit, fake.gotSkip)
	}
}

// TestReviewContracts: cevapların alanları sabit. Herkese açık listede sürüm, gizleme ve e-posta yok;
// sahibinin gördüğünde gizleyen yöneticinin kimliği yok.
func TestReviewContracts(t *testing.T) {
	fake := &fakeReviews{rows: []db.ListBookReviewsRow{{
		ID: 3, UserID: 7, AuthorName: "Ali", AuthorScore: ptr(int16(8)), Body: "Metin", CreatedAt: reviewTime,
	}}}
	var page struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	decode(t, do(t, reviewRouter(fake, ""), http.MethodGet, "/api/books/5/reviews", "").Body.Bytes(), &page)
	if len(page.Items) != 1 {
		t.Fatalf("liste öğesi %d", len(page.Items))
	}
	checkContract(t, page.Items[0], []string{"id", "author", "author_score", "body", "has_spoiler", "doc", "created_at", "edited_at"})
	// Belgesi olmayan eski inceleme: doc düz metinden çevrilir, hiçbir zaman null değildir.
	if got := string(page.Items[0]["doc"]); got != docOf(`{"type":"text","text":"Metin"}`) {
		t.Errorf("eski incelemenin doc alanı: %s", got)
	}

	hiddenAt, admin := reviewTime.Add(time.Hour), int64(1)
	stored := json.RawMessage(docOf(`{"type":"text","marks":[{"type":"bold"}],"text":"Metin"}`))
	fake = &fakeReviews{review: db.Review{ID: 3, BookID: 5, Body: "Metin", BodyDoc: stored, Version: 2, CreatedAt: reviewTime,
		HiddenAt: &hiddenAt, HiddenBy: &admin, HiddenReason: "Spam"}}
	var mine map[string]json.RawMessage
	decode(t, do(t, reviewRouter(fake, "user"), http.MethodGet, reviewURL, "").Body.Bytes(), &mine)
	checkContract(t, mine, []string{"id", "book_id", "body", "has_spoiler", "doc", "version", "created_at", "edited_at", "hidden"})
	if string(mine["doc"]) != string(stored) {
		t.Errorf("saklanan belge aynen dönmeli: %s", mine["doc"])
	}
	var hidden map[string]json.RawMessage
	decode(t, mine["hidden"], &hidden)
	checkContract(t, hidden, []string{"at", "reason"})
}

func decode(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("cevap çözülemedi: %v (%s)", err, raw)
	}
}

// TestReviewRateLimit: yazım IP başına "review" kovasında; okuma ve silme kotaya takılmaz, kimliksiz
// istek kotayı yemez.
func TestReviewRateLimit(t *testing.T) {
	fake := &fakeReviews{}
	d := deps{reviews: fake, tokens: fakeVerifier{}, limiter: ratelimit.NewMemory(1, time.Minute)}
	anon, user := newRouter(d), userRouter(d)

	steps := []struct {
		name         string
		h            http.Handler
		method, body string
		want         int
	}{
		{"kimliksiz yazım kotayı yemez", anon, http.MethodPut, `{"body":"a"}`, http.StatusUnauthorized},
		{"ilk yazım", user, http.MethodPut, `{"body":"a"}`, http.StatusCreated},
		{"kota doldu", user, http.MethodPut, `{"body":"a","version":1}`, http.StatusTooManyRequests},
		{"okuma sınırsız", user, http.MethodGet, "", http.StatusOK},
		{"silme sınırsız", user, http.MethodDelete, "", http.StatusNoContent},
	}
	for _, s := range steps {
		if rec := do(t, s.h, s.method, reviewURL, s.body); rec.Code != s.want {
			t.Errorf("%s: status %d, beklenen %d", s.name, rec.Code, s.want)
		}
	}
}
