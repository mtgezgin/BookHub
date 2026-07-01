package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// newRouter, uygulamanın HTTP router'ını (yönlendiricisini) kurar.
// Chi, net/http ile uyumludur: handler imzaları aynı, sadece daha zengin.
func newRouter() http.Handler {
	r := chi.NewRouter()

	// Middleware: her isteğe, yukarıdan aşağı SIRAYLA uygulanan ara katmanlar.
	r.Use(middleware.RequestID) // her isteğe benzersiz bir ID ekle (log korelasyonu)
	r.Use(middleware.Logger)    // method/path/status/süreyi stdout'a logla (12-factor XI)
	r.Use(middleware.Recoverer) // bir handler panik ederse 500 dön, sunucu çökmesin

	// Bilinmeyen yol / desteklenmeyen method'ta da DÜZ HTML değil, tutarlı JSON zarfı dön.
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "kaynak bulunamadı")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "bu method desteklenmiyor")
	})

	// Rotalar. İleride buraya /api/books, /api/admin/... grupları eklenecek.
	r.Get("/health", handleHealth)

	return r
}

// handleHealth, servisin ayakta olduğunu söyleyen basit bir uç nokta.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
