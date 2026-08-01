package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRouter_NotFound, bilinmeyen bir yola gidince TUTARLI JSON hata zarfının
// döndüğünü doğrular. Böylece newRouter + writeError + writeJSON + zarf birlikte test edilir.
func TestRouter_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/bilinmeyen-yol", nil)
	rec := httptest.NewRecorder()

	// Bu test kitap rotasına hiç girmiyor → repository yerine nil geçmek güvenli.
	newRouter(nil).ServeHTTP(rec, req) // router'ı gerçek bir istekle çağır

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
