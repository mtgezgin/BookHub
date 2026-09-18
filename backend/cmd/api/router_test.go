package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
)

// TestRouter_NotFound, bilinmeyen bir yola gidince TUTARLI JSON hata zarfının
// döndüğünü doğrular. Böylece newRouter + writeError + writeJSON + zarf birlikte test edilir.
func TestRouter_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/bilinmeyen-yol", nil)
	rec := httptest.NewRecorder()

	// Bu test kitap rotasına hiç girmiyor → repository yerine nil geçmek güvenli.
	newRouter(deps{}).ServeHTTP(rec, req) // router'ı gerçek bir istekle çağır

	if rec.Code != http.StatusNotFound {
		t.Errorf("durum kodu: istenen %d, gelen %d", http.StatusNotFound, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: istenen application/json, gelen %q", ct)
	}

	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("cevap geçerli JSON değil: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("error.code: istenen \"not_found\", gelen %q", body.Error.Code)
	}
}

// TestRouter_MethodNotAllowed, VAR OLAN bir yola desteklenmeyen method'la
// gidildiğinde (POST /api/books/{id}) Chi'nin varsayılan düz metin cevabı yerine
// bizim tutarlı JSON zarfımızın döndüğünü doğrular.
func TestRouter_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/books/1", nil)
	rec := httptest.NewRecorder()

	// Handler'a hiç girilmiyor (router method'u önce eliyor) → repository nil güvenli.
	newRouter(deps{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", http.StatusMethodNotAllowed, rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: istenen application/json, gelen %q", ct)
	}

	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("cevap geçerli JSON değil: %v", err)
	}
	if body.Error.Code != "method_not_allowed" {
		t.Errorf("error.code: istenen \"method_not_allowed\", gelen %q", body.Error.Code)
	}
}

// TestRouter_AuthRoutes: /api/auth/{register,login,refresh,logout} rota tablosunda POST
// olarak bağlı ve HERKESE açık (token gerekmez); GET ise 405 alır. Handler testleri bu
// handler'ları doğrudan çağırdığı için yanlış bir bağlama onlarda görünmezdi.
func TestRouter_AuthRoutes(t *testing.T) {
	tests := []struct {
		path    string
		body    string
		reached func(*fakeAuth) bool // handler'a ulaşıldı mı (sahte servis çağrıldı mı)
	}{
		{"/api/auth/register", `{"email":"ali@example.com","display_name":"Ali","password":"yeterince-uzun"}`, func(f *fakeAuth) bool { return f.gotEmail != "" }},
		{"/api/auth/login", `{"email":"ali@example.com","password":"yeterince-uzun"}`, func(f *fakeAuth) bool { return f.gotEmail != "" }},
		{"/api/auth/refresh", `{"refresh_token":"tok"}`, func(f *fakeAuth) bool { return f.gotToken == "tok" }},
		{"/api/auth/logout", `{"refresh_token":"tok"}`, func(f *fakeAuth) bool { return f.gotToken == "tok" }},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			svc := &fakeAuth{user: sampleUser, session: auth.Session{AccessToken: "a", User: sampleUser}}
			h := newRouter(deps{auth: svc, tokens: fakeVerifier{}})

			// Authorization başlığı YOK: public rota token istememeli.
			rec := do(t, h, http.MethodPost, tt.path, tt.body)
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
				t.Fatalf("POST %s public olmalı, gelen %d (%s)", tt.path, rec.Code, rec.Body)
			}
			if !tt.reached(svc) {
				t.Errorf("POST %s handler'a ulaşmadı (status %d)", tt.path, rec.Code)
			}

			get := do(t, h, http.MethodGet, tt.path, "")
			if get.Code != http.StatusMethodNotAllowed {
				t.Errorf("GET %s: istenen 405, gelen %d", tt.path, get.Code)
			}
		})
	}
}
