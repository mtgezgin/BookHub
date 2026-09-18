package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// Kapsam analizinin işaret ettiği sınır durumları: her biri "istemci hatası sunucuya
// (store'a) hiç ulaşmadan 400 olur" iddiasını kilitler.

// TestSlugCannotBeDerived: yalnız emoji içeren başlık/ad slug üretemez → 400, store'a gidilmez.
func TestSlugCannotBeDerived(t *testing.T) {
	t.Run("kitap: başlıktan boş slug", func(t *testing.T) {
		fake := &capturingBooks{fakeBooks: fakeBooks{book: db.Book{ID: 1}}}
		rec := do(t, adminRouter(deps{books: fake}), http.MethodPost, "/api/admin/books", `{"title":"🙂🙂","author":"a"}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_error" || !strings.Contains(rec.Body.String(), "slug zorunlu") {
			t.Errorf("400 'slug zorunlu' bekleniyordu: %d %s", rec.Code, rec.Body)
		}
		if fake.got.Title != "" {
			t.Error("geçersiz istek store'a ulaşmamalı")
		}
	})
	t.Run("tür: addan boş slug", func(t *testing.T) {
		fake := &fakeGenres{}
		rec := do(t, adminRouter(deps{genres: fake}), http.MethodPost, "/api/admin/genres", `{"name":"🙂"}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_error" || fake.gotName != "" {
			t.Errorf("400 ve store çağrısı yok bekleniyordu: %d %s (gotName=%q)", rec.Code, rec.Body, fake.gotName)
		}
	})
	// PATCH'te boş slug bilerek FARKLI: create'te "boş = türet" demek, PATCH'te ise
	// "slug'ı boşalt" isteği olur — slug zorunlu alan, boşaltılamaz.
	t.Run("tür PATCH: boş slug", func(t *testing.T) {
		fake := &fakeGenres{}
		rec := do(t, adminRouter(deps{genres: fake}), http.MethodPatch, "/api/admin/genres/3", `{"slug":""}`)
		if rec.Code != http.StatusBadRequest || fake.gotUpdate != [2]*string{} {
			t.Errorf("400 ve store çağrısı yok bekleniyordu: %d %s", rec.Code, rec.Body)
		}
	})
}

// TestCoverURLNeedsHost: şeması doğru ama host'u olmayan URL ("https://") de reddedilir —
// httpURL'nin u.Host == "" yarısı.
func TestCoverURLNeedsHost(t *testing.T) {
	for _, u := range []string{"https://", "http:///resim.jpg"} {
		fake := &capturingBooks{fakeBooks: fakeBooks{book: db.Book{ID: 1}}}
		rec := do(t, adminRouter(deps{books: fake}), http.MethodPost, "/api/admin/books", `{"title":"T","slug":"t","author":"a","cover_url":"`+u+`"}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_error" {
			t.Errorf("%q → 400 validation_error bekleniyordu: %d %s", u, rec.Code, rec.Body)
		}
	}
}

// TestSessionResponseCarriesExpiry: istemci token'ı ne zaman yenileyeceğini cevaptaki
// bitiş zamanlarından öğrenir — ikisi de aynen aktarılmalı.
func TestSessionResponseCarriesExpiry(t *testing.T) {
	access := time.Date(2026, 9, 1, 12, 15, 0, 0, time.UTC)
	refresh := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc := &fakeAuth{session: auth.Session{AccessToken: "a", AccessExpiresAt: access, RefreshToken: "r", RefreshExpiresAt: refresh, User: sampleUser}}
	rec := do(t, handleLogin(svc), http.MethodPost, "/api/auth/login", `{"email":"a@b.co","password":"x"}`)
	var got sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if !got.AccessExpiresAt.Equal(access) || !got.RefreshExpiresAt.Equal(refresh) {
		t.Errorf("bitiş zamanları aktarılmalı: access=%v refresh=%v", got.AccessExpiresAt, got.RefreshExpiresAt)
	}
}
