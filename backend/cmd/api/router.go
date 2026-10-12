package main

import (
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mtgezgin/bookhub/backend/internal/config"
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
	// ratingMinVotes: ağırlıklı puandaki m (RATING_MIN_VOTES); katalogun ?sort=rating sırası için.
	ratingMinVotes int64
	auth           authService
	tokens         tokenVerifier
	limiter        ratelimit.Limiter // nil → sınırsız (yalnız bir alanı dolduran testler)
	// trustedProxies: X-Real-IP başlığına güvenilecek proxy adresleri (NGINX). Boş → hiçbiri.
	trustedProxies []netip.Prefix
	// cookies: refresh çerezinin adı + Secure özniteliği (config'ten gelir). Adı boşsa
	// newRouter varsayılana düşürür.
	cookies cookieSettings
}

// newRouter, uygulamanın HTTP router'ını (yönlendiricisini) kurar.
// Chi, net/http ile uyumludur: handler imzaları aynı, sadece daha zengin.
func newRouter(d deps) http.Handler {
	r := chi.NewRouter()

	// Middleware: her isteğe, yukarıdan aşağı SIRAYLA uygulanan ara katmanlar.
	r.Use(middleware.RequestID)              // her isteğe benzersiz bir ID ekle (log korelasyonu)
	r.Use(resolveClientIP(d.trustedProxies)) // gerçek istemci IP'si (proxy arkasında da)
	r.Use(requestLogger)                     // her istek için tek yapılandırılmış log satırı + X-Request-Id (12-factor XI)
	r.Use(middleware.Recoverer)              // bir handler panik ederse 500 dön, sunucu çökmesin
	// CSRF koruması (bkz. crossOriginProtection). SIRA ÖNEMLİ: RequestID + requestLogger'dan
	// SONRA → reddedilen istek de erişim log'una düşer ve cevabı X-Request-Id taşır. Zincirin
	// başında olsaydı ret, log'a hiç ulaşmadan zinciri keserdi: "neden 403 aldım?" sorusu izsiz kalırdı.
	r.Use(crossOriginProtection())

	// Bilinmeyen yol / desteklenmeyen method'ta da DÜZ HTML değil, tutarlı JSON zarfı dön.
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "kaynak bulunamadı")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "bu method desteklenmiyor")
	})

	r.Get("/health", handleHealth)

	// Herkese açık okuma uçları.
	r.Get("/api/books", handleListBooks(d.books, d.ratingMinVotes))
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

	// Çerez adı boşsa varsayılana düş. NEDEN: net/http, adı geçersiz (boş dahil) bir çerezi
	// HATA DÖNMEDEN düşürür — yalnız bir log satırı basar. Yalnızca tek bir alanı dolduran
	// bir test (deps{auth: fake}) sessizce "çerezsiz" bir dünyada koşar ve iddiaları hep
	// boşa çıkardı. Üretimde config zaten boş adı boot'ta reddediyor (fail-fast).
	cookies := d.cookies
	if cookies.name == "" {
		cookies.name = config.DefaultCookieName
	}

	// Kimlik uçları. /me dışındakiler bilerek açık: token'ı zaten bunlar veriyor.
	// login/register IP başına sınırlı (Faz 7): parola denemesi ve toplu hesap açma.
	// Ayrı anahtar adları: login denemeleri register kotasını tüketmesin.
	r.Route("/api/auth", func(r chi.Router) {
		r.With(rateLimit(limiter, "register")).Post("/register", handleRegister(d.auth))
		r.With(rateLimit(limiter, "login")).Post("/login", handleLogin(d.auth, cookies))
		r.Post("/refresh", handleRefresh(d.auth, cookies))
		r.Post("/logout", handleLogout(d.auth, cookies))
		// r.With(mw): middleware'i YALNIZCA bu rotaya uygula.
		r.With(requireAuth(d.tokens)).Get("/me", handleMe(d.auth))
		// Parola değiştirme (Faz 13): önce kimlik (kimliksiz istek kotayı yemesin), sonra IP
		// başına AYRI "password" kovası — çalınmış bir access token'la mevcut parolayı kaba
		// kuvvetle deneyen biri login kotasını değil bunu tüketir (NGINX'te de ayrı zone).
		r.With(requireAuth(d.tokens), rateLimit(limiter, "password")).Post("/password", handleChangePassword(d.auth, cookies))
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

// crossOriginProtection, tarayıcıdan gelen CROSS-ORIGIN yazma isteklerini (POST/PUT/PATCH/DELETE)
// reddeden CSRF korumasıdır: Go 1.25'te stdlib'e giren http.CrossOriginProtection + bizim JSON
// hata zarfımız (varsayılan ret cevabı düz metin "403 Forbidden" olurdu; frontend `code`a göre dallanır).
//
// CSRF (Cross-Site Request Forgery) NEDİR: kötü niyetli bir sayfa (evil.example), ziyaretçinin
// tarayıcısına BİZİM API'mize bir istek attırır. Tarayıcı, bizim sitemize ait çerezleri isteğe
// kendiliğinden ekler → istek kullanıcının kimliğiyle çalışır, kullanıcı ise habersizdir.
// Saldırgan cevabı OKUYAMAZ (same-origin politikası) ama yazma isteğinin YAN ETKİSİ olmuştur bile.
// (readJSON Content-Type'a bakmıyor: text/plain gövdeli bir form ya da fetch, CORS ön kontrolüne
// — preflight — takılmadan buraya ulaşabilir; asıl kapı bu yüzden burası.)
//
// Karar nasıl verilir (go doc net/http.CrossOriginProtection):
//  1. GET/HEAD/OPTIONS HER ZAMAN geçer ("güvenli" method'lar). Bu yüzden hiçbir GET ucu durum
//     DEĞİŞTİRMEMELİ — değiştirseydi bu koruma onu kapsamazdı.
//  2. Sec-Fetch-Site: isteği hangi bağlamın başlattığını TARAYICININ KENDİSİ yazar; sayfadaki JS bu
//     başlığı ne ekleyebilir ne değiştirebilir (yasaklı başlık). 2023'ten beri tüm tarayıcılarda var —
//     ama YALNIZ güvenli bağlamda: tarayıcılar Fetch Metadata başlıklarını sadece HTTPS'e ve
//     localhost'a yollar.
//     "same-origin" ya da "none" (adres çubuğu, yer imi) → geçer; "cross-site" VE "same-site" → 403.
//  3. Sec-Fetch-Site yoksa — eski tarayıcı YA DA güvenli olmayan bir origin (düz HTTP ile açılmış,
//     localhost olmayan bir sunucu, ör. http://192.168.1.5:3000: orada modern tarayıcı da bu başlığı
//     göndermez, yani bu yedek kontrol o kurulumun ASIL savunmasıdır) — Origin başlığındaki host[:port], Host
//     başlığıyla karşılaştırılır; eşitse geçer, farklıysa 403. PORT da karşılaştırmaya dahil → NGINX,
//     API'ye Host'u portuyla iletmeli (bkz. frontend/nginx/api-proxy.conf, $http_host).
//  4. İkisi de yoksa geçer: bu ya tarayıcı dışı bir istemcidir (curl, backend/scripts/mvp_smoke.sh,
//     Go testleri) ya da Origin göndermeyen çok eski bir tarayıcı. CSRF bir TARAYICI saldırısıdır:
//     başlıklarını kendisi yazan bir istemci zaten kendi adına konuşuyordur, araya giren kurban yok.
//
// "same-site" de REDDEDİLİR: same-site = aynı kayıtlı alan adının KARDEŞ alt alanı (ör.
// blog.bookhub.example → bookhub.example). Tarayıcı için aynı SİTE ama aynı ORIGIN değil — ve
// SameSite çerez özniteliği bu isteklere çerezi YİNE ekler. Ele geçirilmiş ya da XSS'li bir kardeş
// alt alan bizim adımıza yazma yapamasın. Frontend ile API TEK origin'de (NGINX; geliştirmede vite
// proxy'si) → izin verilecek başka bir origin yok, AddTrustedOrigin gerekmiyor.
//
// Neden ŞİMDİ: bugün kimlik Authorization başlığında (tarayıcı onu kendiliğinden EKLEMEZ) ve refresh
// token JSON gövdesinde → CSRF'in tutunacağı bir çerez yok. Sonraki adımda refresh token httpOnly
// bir ÇEREZE taşınıyor: o andan itibaren login cevabı oturumu tarayıcıya kendiliğinden yazar ve başka
// bir sayfa kurbanın tarayıcısında SALDIRGANIN hesabıyla oturum açtırabilirdi (login-CSRF: kurban
// yazdıklarını farkında olmadan saldırganın hesabına kaydeder). Bu koruma o kapıyı çerez gelmeden
// ÖNCE kapatır.
//
// Neden klasik CSRF token'ı (gizli form alanı / double-submit çerezi) değil: sunucuda durum ya da
// istemcide ek bir başlık/çerez dansı ister; Sec-Fetch-Site aynı güvenceyi durumsuz ve tek yerde verir.
func crossOriginProtection() func(http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Erişim satırı yalnız "403" der; NEDEN'i operatör için burada. Origin/Host ikilisi, ör.
		// Host'u portsuz ileten bir proxy'yi (Origin "localhost:3000", Host "localhost") ilk bakışta ele verir.
		slog.Warn("cross-origin istek reddedildi",
			"sec_fetch_site", r.Header.Get("Sec-Fetch-Site"),
			"origin", r.Header.Get("Origin"),
			"host", r.Host,
			"request_id", w.Header().Get(requestIDHeader),
		)
		writeError(w, http.StatusForbidden, "cross_origin_request", "başka bir siteden gelen yazma isteği reddedildi")
	}))
	return cop.Handler
}

// handleHealth, servisin ayakta olduğunu söyleyen basit bir uç nokta.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
