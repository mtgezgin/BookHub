package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/reviews"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// roleVerifier, idVerifier'ın yönetici de tanıyan hâli: "admin-<id>" o kimlikteki ADMIN'dir.
type roleVerifier struct{}

func (roleVerifier) Verify(raw string) (auth.Principal, error) {
	if id, ok := strings.CutPrefix(raw, "admin-"); ok {
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return auth.Principal{}, auth.ErrInvalidToken
		}
		return auth.Principal{UserID: n, Role: auth.RoleAdmin}, nil
	}
	return idVerifier{}.Verify(raw)
}

// reviewAPI, inceleme uçlarını gerçek servisle çağıran küçük istemci.
type reviewAPI struct {
	t       *testing.T
	router  http.Handler
	bookURL string
}

// call, token verilen kişi adına bir istek ("" = kimliksiz).
func (a reviewAPI) call(token, method, url, body string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

// review, beklenen durumla dönen incelemeyi çözer.
func (a reviewAPI) review(rec *httptest.ResponseRecorder, want int) reviewResponse {
	a.t.Helper()
	if rec.Code != want {
		a.t.Fatalf("status %d, beklenen %d (%s)", rec.Code, want, rec.Body)
	}
	var r reviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		a.t.Fatal(err)
	}
	return r
}

func (a reviewAPI) save(token, body string, want int) reviewResponse {
	a.t.Helper()
	return a.review(a.call(token, http.MethodPut, a.bookURL+"/review", body), want)
}

func (a reviewAPI) moderate(token string, id int64, action, body string) reviewResponse {
	a.t.Helper()
	url := "/api/admin/reviews/" + strconv.FormatInt(id, 10) + "/" + action
	return a.review(a.call(token, http.MethodPost, url, body), http.StatusOK)
}

// listed, herkese açık listedeki inceleme sayısı.
func (a reviewAPI) listed() int64 {
	a.t.Helper()
	rec := a.call("", http.MethodGet, a.bookURL+"/reviews", "")
	var page reviewPage
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
		a.t.Fatalf("liste: %d %s", rec.Code, rec.Body)
	}
	return page.Total
}

func userToken(id int64) string  { return "user-" + strconv.FormatInt(id, 10) }
func adminToken(id int64) string { return "admin-" + strconv.FormatInt(id, 10) }

// TestReviewEndpointsRealDB, inceleme uçlarını UÇTAN UCA doğrular: router → handler → gerçek servis →
// Postgres (Doğrulama 12, 13, 15'in API yarısı). Birim testleri sahte servisle koşar; servisin
// hatalarının doğru koda çevrildiğini ve gizlemenin listeye yansıdığını yalnız bu dikey kesit kanıtlar.
func TestReviewEndpointsRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	alice, bob := testdb.CreateUser(t, pool, "Okur"), testdb.CreateUser(t, pool, "İkinci")
	admin := testdb.CreateUser(t, pool, "Yönetici")
	book := testdb.CreateBook(t, pool, "", books.StatusApproved)
	api := reviewAPI{t: t, bookURL: "/api/books/" + strconv.FormatInt(book, 10),
		router: newRouter(deps{reviews: reviews.NewService(pool), tokens: roleVerifier{}})}

	first := api.save(userToken(alice), `{"body":"İlk hâl"}`, http.StatusCreated)
	api.save(userToken(bob), `{"body":"Bence de","has_spoiler":true}`, http.StatusCreated)
	if rec := api.call(userToken(alice), http.MethodPut, api.bookURL+"/review", `{"body":"Ezme"}`); rec.Code != http.StatusConflict {
		t.Errorf("sürümsüz ikinci yazım 409 olmalı (Doğrulama 12): %d %s", rec.Code, rec.Body)
	}
	edited := api.save(userToken(alice), `{"body":"Düzeltilmiş","version":1}`, http.StatusOK)
	if edited.ID != first.ID || edited.Version != 2 || edited.EditedAt == nil {
		t.Errorf("düzenleme aynı incelemeyi güncellemeli: %+v", edited)
	}

	hidden := api.moderate(adminToken(admin), first.ID, "hide", `{"reason":"Kişisel bilgi"}`)
	if hidden.Hidden == nil || hidden.Hidden.Reason != "Kişisel bilgi" || api.listed() != 1 {
		t.Errorf("gizlenen inceleme listede olmamalı (Doğrulama 15): %+v", hidden)
	}
	still := api.save(userToken(alice), `{"body":"Yine düzelttim","version":2}`, http.StatusOK)
	if still.Hidden == nil {
		t.Error("sahibi düzenleyince inceleme gizli kalmalı (REV-01)")
	}
	if shown := api.moderate(adminToken(admin), first.ID, "unhide", ""); shown.Hidden != nil || api.listed() != 2 {
		t.Errorf("geri açılan inceleme listede olmalı: %+v", shown)
	}

	if rec := api.call(userToken(bob), http.MethodDelete, api.bookURL+"/review", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("silme: %d %s", rec.Code, rec.Body)
	}
	if rec := api.call(userToken(bob), http.MethodGet, api.bookURL+"/review", ""); rec.Code != http.StatusNotFound || errorCode(t, rec) != "no_review" {
		t.Errorf("silinen inceleme 404 no_review: %d %s", rec.Code, rec.Body)
	}
	if mine := api.review(api.call(userToken(alice), http.MethodGet, api.bookURL+"/review", ""), http.StatusOK); mine.Body != "Yine düzelttim" {
		t.Errorf("başkasının silmesi Alice'in incelemesine dokunmamalı (Doğrulama 13): %+v", mine)
	}
}
