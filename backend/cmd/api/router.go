package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mtgezgin/bookhub/backend/internal/ratelimit"
)

// deps, HTTP katmanının ihtiyaç duyduğu tüm bağımlılıklar.
//
// Neden struct: router büyüdükçe newRouter(books, genres, auth, ...) gibi uzayan bir
// parametre listesi hem okunmaz hem de her yeni bağımlılıkta TÜM test çağrılarını kırar.
// Struct ile testler yalnızca ihtiyaç duydukları alanı doldurur: deps{books: fake}.
// Yine de elle DI: grafın tamamı main.go'da tek yerde kuruluyor (GO_CONVENTIONS §3).
type deps struct {
	books      bookStore
	genres     genreStore
	moderation moderationService
	comments   commentService
	ratings    ratingService
	auth       authService
	tokens     tokenVerifier
	limiter    ratelimit.Limiter // nil → sınırsız (yalnız bir alanı dolduran testler)
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
	r.Get("/api/books/top-rated", handleTopRated(d.books, d.ratings)) // statik yol {id}'den önce eşleşir
	r.Get("/api/books/{id}", handleGetBook(d.books))                  // {id} = Chi URL parametresi
	r.Get("/api/genres", handleListGenres(d.genres))

	// Giriş yapmış herkes kitap önerebilir; öneri PENDING doğar (public listede görünmez).
	// Statik "suggestions" segmenti Chi'de {id}'den önce eşleşir.
	r.With(requireAuth(d.tokens)).Post("/api/books/suggestions", handleSuggestBook(d.moderation))

	// Yorumlar ve puanlar (Faz 5). Okuma herkese açık; yazma giriş ister. Puan uçlarında
	// URL'de KULLANICI YOK: "benim puanım" = token'daki kullanıcı → başkasının puanına
	// dokunmak ifade bile edilemez (IDOR'a yapısal kapı kapatma).
	r.Get("/api/books/{id}/comments", handleListComments(d.books, d.comments))
	r.Get("/api/books/{id}/ratings/summary", handleRatingSummary(d.ratings))
	r.Group(func(r chi.Router) {
		r.Use(requireAuth(d.tokens))
		r.Post("/api/books/{id}/comments", handleCreateComment(d.comments))
		r.Get("/api/books/{id}/rating", handleMyRating(d.ratings))
		r.Put("/api/books/{id}/rating", handleRateBook(d.ratings))
		r.Delete("/api/books/{id}/rating", handleRemoveRating(d.ratings))
	})

	limiter := d.limiter
	if limiter == nil {
		limiter = ratelimit.Unlimited{}
	}

	// Kimlik uçları. /me dışındakiler bilerek açık: token'ı zaten bunlar veriyor.
	// login/register IP başına sınırlı (Faz 7): parola denemesi ve toplu hesap açma.
	// Ayrı anahtar adları: login denemeleri register kotasını tüketmesin.
	r.Route("/api/auth", func(r chi.Router) {
		r.With(rateLimit(limiter, "register")).Post("/register", handleRegister(d.auth))
		r.With(rateLimit(limiter, "login")).Post("/login", handleLogin(d.auth))
		r.Post("/refresh", handleRefresh(d.auth))
		r.Post("/logout", handleLogout(d.auth))
		// r.With(mw): middleware'i YALNIZCA bu rotaya uygula.
		r.With(requireAuth(d.tokens)).Get("/me", handleMe(d.auth))
	})

	// /api/admin altındaki HER rota: önce kimlik (401), sonra rol (403).
	// r.Use grubun TAMAMINA uygulanır — yeni bir admin rotası eklemek onu otomatik
	// korur; tek tek "bunu da korudum mu?" diye hatırlamak gerekmez.
	// Ayrı yol öneki bilinçli: yetki kararı routing tablosunda görünsün, handler
	// içindeki bir "if user.IsAdmin" dalında değil.
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(requireAuth(d.tokens), requireAdmin)

		// Kitap yaşam döngüsü (Faz 4). Her yazım bir denetim kaydıyla aynı transaction'da.
		r.Get("/books", handleAdminListBooks(d.moderation, d.books))
		r.Post("/books", handleCreateBook(d.moderation))
		r.Get("/books/{id}", handleAdminGetBook(d.moderation, d.books))
		r.Patch("/books/{id}", handleAdminUpdateBook(d.moderation, d.books))
		r.Delete("/books/{id}", handleTransition("ArchiveBook", d.moderation, moderationService.Archive, d.books))
		r.Post("/books/{id}/approve", handleTransition("ApproveBook", d.moderation, moderationService.Approve, d.books))
		r.Post("/books/{id}/reject", handleRejectBook(d.moderation, d.books))
		r.Post("/books/{id}/restore", handleTransition("RestoreBook", d.moderation, moderationService.Restore, d.books))
		r.Get("/books/{id}/audit", handleBookHistory(d.moderation))
		r.Put("/books/{id}/genres", handleSetBookGenres(d.books))

		r.Delete("/comments/{id}", handleDeleteComment(d.comments))

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
