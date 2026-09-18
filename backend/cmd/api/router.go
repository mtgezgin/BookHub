package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// deps, HTTP katmanının ihtiyaç duyduğu tüm bağımlılıklar.
//
// Neden struct: router büyüdükçe newRouter(books, genres, auth, ...) gibi uzayan bir
// parametre listesi hem okunmaz hem de her yeni bağımlılıkta TÜM test çağrılarını kırar.
// Struct ile testler yalnızca ihtiyaç duydukları alanı doldurur: deps{books: fake}.
// Yine de elle DI: grafın tamamı main.go'da tek yerde kuruluyor (GO_CONVENTIONS §3).
type deps struct {
	books  bookStore
	genres genreStore
	auth   authService
	tokens tokenVerifier
}

// newRouter, uygulamanın HTTP router'ını (yönlendiricisini) kurar.
// Chi, net/http ile uyumludur: handler imzaları aynı, sadece daha zengin.
func newRouter(d deps) http.Handler {
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

	r.Get("/health", handleHealth)

	// Herkese açık okuma uçları.
	r.Get("/api/books", handleListBooks(d.books))
	r.Get("/api/books/{id}", handleGetBook(d.books)) // {id} = Chi URL parametresi
	r.Get("/api/genres", handleListGenres(d.genres))

	// Kimlik uçları. /me dışındakiler bilerek açık: token'ı zaten bunlar veriyor.
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", handleRegister(d.auth))
		r.Post("/login", handleLogin(d.auth))
		r.Post("/refresh", handleRefresh(d.auth))
		r.Post("/logout", handleLogout(d.auth))
		// r.With(mw): middleware'i YALNIZCA bu rotaya uygula.
		r.With(requireAuth(d.tokens)).Get("/me", handleMe(d.auth))
	})

	// /api/admin altı Faz 3'te auth middleware'iyle korunacak — şimdilik açık.
	// Ayrı yol öneki bilinçli: yetki kararı routing tablosunda görünsün, handler
	// içindeki bir "if user.IsAdmin" dalında değil.
	r.Route("/api/admin", func(r chi.Router) {
		r.Post("/books", handleCreateBook(d.books))
		r.Put("/books/{id}/genres", handleSetBookGenres(d.books))

		r.Post("/genres", handleCreateGenre(d.genres))
		r.Patch("/genres/{id}", handleUpdateGenre(d.genres))
		r.Delete("/genres/{id}", handleDeleteGenre(d.genres))
	})

	return r
}

// handleHealth, servisin ayakta olduğunu söyleyen basit bir uç nokta.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
