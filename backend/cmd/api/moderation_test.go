package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/moderation"
)

// fakeModeration, moderationService sahtesi: dönecek değerler + çağrı kaydı.
type fakeModeration struct {
	book  db.Book
	list  []db.Book
	total int64
	logs  []db.AuditLog
	err   error

	calls     []string
	gotActor  int64
	gotID     int64
	gotCreate db.CreateBookParams
	gotUpdate db.UpdateBookDetailsParams
	gotReason string
	gotStatus string
	gotLimit  int32
}

func (f *fakeModeration) rec(call string, actor, id int64) {
	f.calls = append(f.calls, call)
	f.gotActor, f.gotID = actor, id
}

func (f *fakeModeration) Suggest(_ context.Context, user int64, p db.CreateBookParams) (db.Book, error) {
	f.rec("Suggest", user, 0)
	f.gotCreate = p
	return f.book, f.err
}
func (f *fakeModeration) CreateApproved(_ context.Context, actor int64, p db.CreateBookParams) (db.Book, error) {
	f.rec("CreateApproved", actor, 0)
	f.gotCreate = p
	return f.book, f.err
}
func (f *fakeModeration) Update(_ context.Context, actor, id int64, p db.UpdateBookDetailsParams) (db.Book, error) {
	f.rec("Update", actor, id)
	f.gotUpdate = p
	return f.book, f.err
}
func (f *fakeModeration) Approve(_ context.Context, actor, id int64) (db.Book, error) {
	f.rec("Approve", actor, id)
	return f.book, f.err
}
func (f *fakeModeration) Reject(_ context.Context, actor, id int64, reason string) (db.Book, error) {
	f.rec("Reject", actor, id)
	f.gotReason = reason
	return f.book, f.err
}
func (f *fakeModeration) Archive(_ context.Context, actor, id int64) (db.Book, error) {
	f.rec("Archive", actor, id)
	return f.book, f.err
}
func (f *fakeModeration) Restore(_ context.Context, actor, id int64) (db.Book, error) {
	f.rec("Restore", actor, id)
	return f.book, f.err
}
func (f *fakeModeration) Get(_ context.Context, id int64) (db.Book, error) {
	f.rec("Get", 0, id)
	return f.book, f.err
}
func (f *fakeModeration) List(_ context.Context, status string, limit, _ int32) ([]db.Book, int64, error) {
	f.rec("List", 0, 0)
	f.gotStatus, f.gotLimit = status, limit
	return f.list, f.total, f.err
}
func (f *fakeModeration) History(_ context.Context, id int64) ([]db.AuditLog, error) {
	f.rec("History", 0, id)
	return f.logs, f.err
}

// userRouter, her isteğe USER (id 7) token'ı ekler.
func userRouter(d deps) http.Handler {
	d.tokens = fakeVerifier{}
	h := newRouter(d)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer good-user")
		h.ServeHTTP(w, r)
	})
}

func TestSuggestBook(t *testing.T) {
	t.Run("giriş yapmış kullanıcı → 201 + PENDING, öneren token'dan", func(t *testing.T) {
		fake := &fakeModeration{book: db.Book{ID: 9, Title: "Öneri", Status: books.StatusPending}}
		rec := do(t, userRouter(deps{moderation: fake}), http.MethodPost, "/api/books/suggestions", `{"title":"Öneri","author":"Yazar"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status: istenen 201, gelen %d (%s)", rec.Code, rec.Body)
		}
		if fake.calls[0] != "Suggest" || fake.gotActor != 7 {
			t.Errorf("Suggest kullanıcı 7 ile çağrılmalı: %v aktör=%d", fake.calls, fake.gotActor)
		}
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("JSON: %v", err)
		}
		if got["status"] != books.StatusPending {
			t.Errorf("öneren statüyü görmeli: %v", got["status"])
		}
		for _, internal := range []string{"submitted_by", "rejection_reason", "moderated_by", "moderated_at"} {
			if _, leaked := got[internal]; leaked {
				t.Errorf("iç moderasyon alanı %q öneri cevabına sızdı", internal)
			}
		}
	})
	t.Run("token yok → 401", func(t *testing.T) {
		rec := do(t, newRouter(deps{moderation: &fakeModeration{}, tokens: fakeVerifier{}}), http.MethodPost, "/api/books/suggestions", `{"title":"T","author":"A"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("401 bekleniyordu, gelen %d", rec.Code)
		}
	})
	t.Run("status alanı (mass-assignment) → 400", func(t *testing.T) {
		fake := &fakeModeration{}
		rec := do(t, userRouter(deps{moderation: fake}), http.MethodPost, "/api/books/suggestions", `{"title":"T","author":"A","status":"APPROVED"}`)
		if rec.Code != http.StatusBadRequest || len(fake.calls) != 0 {
			t.Errorf("400 ve servis çağrısı YOK bekleniyordu: %d %v", rec.Code, fake.calls)
		}
	})
}

func TestAdminListBooks(t *testing.T) {
	t.Run("varsayılan statü PENDING, admin alanları görünür", func(t *testing.T) {
		reason := "kaynak yok"
		fake := &fakeModeration{list: []db.Book{{ID: 3, Status: books.StatusRejected, RejectionReason: &reason}}, total: 1}
		rec := do(t, adminRouter(deps{moderation: fake, books: fakeBooks{}}), http.MethodGet, "/api/admin/books", "")
		if rec.Code != http.StatusOK || fake.gotStatus != books.StatusPending || fake.gotLimit != defaultPageLimit {
			t.Fatalf("status %d, statü %q, limit %d", rec.Code, fake.gotStatus, fake.gotLimit)
		}
		var page adminBookPage
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("JSON: %v", err)
		}
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].RejectionReason == nil || page.Items[0].Status != books.StatusRejected {
			t.Errorf("admin sayfası yanlış: %+v", page)
		}
	})
	t.Run("küçük harf statü kabul", func(t *testing.T) {
		fake := &fakeModeration{}
		rec := do(t, adminRouter(deps{moderation: fake, books: fakeBooks{}}), http.MethodGet, "/api/admin/books?status=archived", "")
		if rec.Code != http.StatusOK || fake.gotStatus != books.StatusArchived {
			t.Errorf("ARCHIVED bekleniyordu: %d %q", rec.Code, fake.gotStatus)
		}
	})
	t.Run("geçersiz statü → 400, servise gidilmez", func(t *testing.T) {
		fake := &fakeModeration{}
		rec := do(t, adminRouter(deps{moderation: fake}), http.MethodGet, "/api/admin/books?status=DELETED", "")
		if rec.Code != http.StatusBadRequest || len(fake.calls) != 0 {
			t.Errorf("400 bekleniyordu: %d %v", rec.Code, fake.calls)
		}
	})
}

// TestModerationActions: her geçiş ucu doğru servis metodunu, token'daki admin ve URL'deki
// id ile çağırmalı; servis hataları doğru HTTP koduna çevrilmeli.
func TestModerationActions(t *testing.T) {
	actions := []struct {
		method, path, body, wantCall string
	}{
		{http.MethodPost, "/api/admin/books/5/approve", "", "Approve"},
		{http.MethodPost, "/api/admin/books/5/reject", `{"reason":"  eksik kaynak  "}`, "Reject"},
		{http.MethodPost, "/api/admin/books/5/reject", "", "Reject"},
		{http.MethodPost, "/api/admin/books/5/restore", "", "Restore"},
		{http.MethodDelete, "/api/admin/books/5", "", "Archive"},
		{http.MethodPatch, "/api/admin/books/5", `{"title":"Yeni"}`, "Update"},
	}
	errCases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"başarı", nil, http.StatusOK, ""},
		{"yok", books.ErrNotFound, http.StatusNotFound, "not_found"},
		{"geçersiz geçiş", fmt.Errorf("%w: APPROVED ✗ reject", moderation.ErrInvalidTransition), http.StatusConflict, "invalid_transition"},
		{"beklenmeyen", errors.New("db"), http.StatusInternalServerError, "internal"},
	}
	for _, a := range actions {
		for _, c := range errCases {
			t.Run(a.method+" "+a.path+" "+a.body+"/"+c.name, func(t *testing.T) {
				fake := &fakeModeration{book: db.Book{ID: 5, Status: books.StatusApproved}, err: c.err}
				rec := do(t, adminRouter(deps{moderation: fake, books: fakeBooks{}}), a.method, a.path, a.body)
				if rec.Code != c.wantStatus {
					t.Fatalf("status: istenen %d, gelen %d (%s)", c.wantStatus, rec.Code, rec.Body)
				}
				if len(fake.calls) != 1 || fake.calls[0] != a.wantCall || fake.gotActor != 1 || fake.gotID != 5 {
					t.Errorf("çağrı: istenen %s(aktör 1, id 5), gelen %v aktör=%d id=%d", a.wantCall, fake.calls, fake.gotActor, fake.gotID)
				}
				if c.wantCode != "" {
					if got := errorCode(t, rec); got != c.wantCode {
						t.Errorf("code: istenen %q, gelen %q", c.wantCode, got)
					}
				}
			})
		}
	}
}

func TestRejectReasonHandling(t *testing.T) {
	fake := &fakeModeration{book: db.Book{ID: 5}}
	do(t, adminRouter(deps{moderation: fake, books: fakeBooks{}}), http.MethodPost, "/api/admin/books/5/reject", `{"reason":"  eksik kaynak  "}`)
	if fake.gotReason != "eksik kaynak" {
		t.Errorf("gerekçe kırpılmış iletilmeli, gelen %q", fake.gotReason)
	}

	long := `{"reason":"` + strings.Repeat("x", maxRejectionReasonLen+1) + `"}`
	fake = &fakeModeration{}
	rec := do(t, adminRouter(deps{moderation: fake}), http.MethodPost, "/api/admin/books/5/reject", long)
	if rec.Code != http.StatusBadRequest || len(fake.calls) != 0 {
		t.Errorf("uzun gerekçe 400 olmalı ve servise gitmemeli: %d %v", rec.Code, fake.calls)
	}
}

func TestAdminUpdateBookValidation(t *testing.T) {
	tests := []struct {
		name, body string
		wantStatus int
	}{
		{"boş gövde nesnesi → 400", `{}`, http.StatusBadRequest},
		{"status değiştirilemez → 400", `{"status":"APPROVED"}`, http.StatusBadRequest},
		{"boş başlık → 400", `{"title":"  "}`, http.StatusBadRequest},
		{"geçersiz slug → 400", `{"slug":"Kötü Slug"}`, http.StatusBadRequest},
		{"javascript: URL → 400", `{"cover_url":"javascript:alert(1)"}`, http.StatusBadRequest},
		{"geçerli kısmi → 200", `{"page_count":250}`, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeModeration{book: db.Book{ID: 5}}
			rec := do(t, adminRouter(deps{moderation: fake, books: fakeBooks{}}), http.MethodPatch, "/api/admin/books/5", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantStatus == http.StatusOK && (fake.gotUpdate.PageCount == nil || *fake.gotUpdate.PageCount != 250 || fake.gotUpdate.Title != nil) {
				t.Errorf("yalnız page_count iletilmeli: %+v", fake.gotUpdate)
			}
		})
	}
}

func TestBookHistory(t *testing.T) {
	actor := int64(1)
	fake := &fakeModeration{logs: []db.AuditLog{{ID: 1, ActorID: &actor, Action: "book.approve", Details: json.RawMessage(`{"from":"PENDING","to":"APPROVED"}`)}}}
	rec := do(t, adminRouter(deps{moderation: fake}), http.MethodGet, "/api/admin/books/5/audit", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	// details JSON olarak GÖMÜLÜ gelmeli (base64 string değil).
	if !strings.Contains(rec.Body.String(), `"details":{"from":"PENDING","to":"APPROVED"}`) {
		t.Errorf("details ham JSON olarak gömülmeli: %s", rec.Body)
	}

	empty := do(t, adminRouter(deps{moderation: &fakeModeration{}}), http.MethodGet, "/api/admin/books/5/audit", "")
	if strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Errorf("boş geçmiş [] olmalı: %q", empty.Body)
	}
}

func TestAdminGetBook(t *testing.T) {
	for _, status := range []string{books.StatusPending, books.StatusApproved, books.StatusRejected, books.StatusArchived} {
		t.Run(status, func(t *testing.T) {
			fake := &fakeModeration{book: db.Book{ID: 5, Status: status}}
			store := fakeBooks{genres: map[int64][]db.Genre{5: {{ID: 1, Name: "Roman", Slug: "roman"}}}}
			rec := do(t, adminRouter(deps{moderation: fake, books: store}), http.MethodGet, "/api/admin/books/5", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d (%s)", rec.Code, rec.Body)
			}
			var got adminBookResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("JSON: %v", err)
			}
			if got.Status != status || len(got.Genres) != 1 || got.Genres[0].Slug != "roman" {
				t.Errorf("admin DTO statü + türleri taşımalı: %+v", got)
			}
			if len(fake.calls) != 1 || fake.calls[0] != "Get" || fake.gotID != 5 {
				t.Errorf("svc.Get path id (5) ile çağrılmalı: %v id=%d", fake.calls, fake.gotID)
			}
		})
	}
}

func TestAdminGetBookNotFound(t *testing.T) {
	rec := do(t, adminRouter(deps{moderation: &fakeModeration{err: books.ErrNotFound}, books: fakeBooks{}}), http.MethodGet, "/api/admin/books/5", "")
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
		t.Errorf("404 not_found bekleniyordu: %d %s", rec.Code, rec.Body)
	}
}

// TestActorFromWithoutPrincipal: requireAuth dışında bağlanmış bir handler kapalı başarısız olur.
func TestActorFromWithoutPrincipal(t *testing.T) {
	var ok = true
	rec := do(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, ok = actorFrom(w, r) }), http.MethodGet, "/", "")
	if ok || rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "missing_token" {
		t.Errorf("401 missing_token + false bekleniyordu: ok=%v %d %s", ok, rec.Code, rec.Body)
	}

	fake := &fakeModeration{}
	rec = do(t, handleSuggestBook(fake), http.MethodPost, "/", `{"title":"T","author":"A"}`)
	if rec.Code != http.StatusUnauthorized || len(fake.calls) != 0 {
		t.Errorf("principal'sız handler 401 vermeli ve servisi çağırmamalı: %d %v", rec.Code, fake.calls)
	}
}

func TestSuggestBookDuplicateSlug(t *testing.T) {
	fake := &fakeModeration{err: books.ErrDuplicateSlug}
	rec := do(t, userRouter(deps{moderation: fake}), http.MethodPost, "/api/books/suggestions", `{"title":"T","author":"A"}`)
	if rec.Code != http.StatusConflict || errorCode(t, rec) != "slug_conflict" {
		t.Errorf("409 slug_conflict bekleniyordu: %d %s", rec.Code, rec.Body)
	}
}

func TestAdminListBooksErrors(t *testing.T) {
	t.Run("svc.List hatası → 500", func(t *testing.T) {
		rec := do(t, adminRouter(deps{moderation: &fakeModeration{err: errors.New("db")}, books: fakeBooks{}}), http.MethodGet, "/api/admin/books", "")
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("500 bekleniyordu: %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("GenresForBooks hatası → 500", func(t *testing.T) {
		fake := &fakeModeration{list: []db.Book{{ID: 3}}, total: 1}
		rec := do(t, adminRouter(deps{moderation: fake, books: fakeBooks{genreErr: errors.New("db")}}), http.MethodGet, "/api/admin/books", "")
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("500 bekleniyordu: %d %s", rec.Code, rec.Body)
		}
	})
}

// TestWriteAdminBookGenresError: geçiş/güncelleme başarılı ama türler okunamazsa 500.
func TestWriteAdminBookGenresError(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/admin/books/5/approve", ""},
		{http.MethodPatch, "/api/admin/books/5", `{"title":"Yeni"}`},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			fake := &fakeModeration{book: db.Book{ID: 5}}
			rec := do(t, adminRouter(deps{moderation: fake, books: fakeBooks{genreErr: errors.New("db")}}), tc.method, tc.path, tc.body)
			if rec.Code != http.StatusInternalServerError || len(fake.calls) != 1 {
				t.Errorf("500 bekleniyordu (servis bir kez çağrılmış): %d %v %s", rec.Code, fake.calls, rec.Body)
			}
		})
	}
}

func TestBookHistoryNotFound(t *testing.T) {
	rec := do(t, adminRouter(deps{moderation: &fakeModeration{err: books.ErrNotFound}}), http.MethodGet, "/api/admin/books/5/audit", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("bilinmeyen kitap 404 olmalı: %d %s", rec.Code, rec.Body)
	}
}
