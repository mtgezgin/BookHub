package main

import (
	"context"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/ratelimit"
)

// tokenVerifier, middleware'in token doğrulayıcıdan beklediği tek davranış.
// *auth.TokenManager bunu sağlar; testlerde sahte takılır.
type tokenVerifier interface {
	Verify(raw string) (auth.Principal, error)
}

// principalKey, context'e koyduğumuz değerin anahtarı. Tipi DIŞARI AÇIK DEĞİL: başka bir
// paket aynı anahtarı üretip context'teki kimliği ezemez/okuyamaz. (Düz string anahtar —
// ör. "user" — her paketin çarpışabileceği ortak bir ad alanı olurdu.)
type principalKey struct{}

func withPrincipal(ctx context.Context, p auth.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// principalFrom, requireAuth'un koyduğu kimliği okur. ok=false → rota korunmuyor demektir.
func principalFrom(ctx context.Context) (auth.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(auth.Principal)
	return p, ok
}

// bearerToken, "Authorization: Bearer <token>" başlığından token'ı çıkarır.
// Şema adı büyük/küçük harf duyarsız (RFC 7235), token boş olamaz.
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	token = strings.TrimSpace(token)
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

// unauthorized, 401 yazar. WWW-Authenticate başlığı standardın gereği (RFC 6750):
// istemciye "hangi şemayla kimlik göstermelisin" der.
func unauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="bookhub"`)
	writeError(w, http.StatusUnauthorized, code, message)
}

// requireAuth, geçerli bir access token ister; kimliği context'e koyar.
//
// MIDDLEWARE NEDİR: handler'ı saran bir handler — isteği önce o görür, uygun değilse
// zinciri KESER (next'i çağırmaz). Express'teki app.use((req,res,next) => ...) ile aynı
// fikir; Go'da imza func(http.Handler) http.Handler.
//
// 401 vs 403: 401 = "seni tanımıyorum" (token yok/geçersiz), 403 = "seni tanıyorum ama
// buna yetkin yok" (requireAdmin).
func requireAuth(v tokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				unauthorized(w, "missing_token", "kimlik doğrulaması gerekli (Authorization: Bearer başlığı)")
				return
			}
			p, err := v.Verify(raw)
			if err != nil {
				// Nedeni (süre mi, imza mı) istemciye söylemiyoruz; her iki durumda da
				// istemcinin yapacağı aynı: yeniden giriş ya da /refresh.
				unauthorized(w, "invalid_token", "geçersiz ya da süresi dolmuş token")
				return
			}
			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), p)))
		})
	}
}

// requireAdmin, requireAuth'tan SONRA zincirlenir: kimlik belli, şimdi rol kontrolü.
// Kaba (rol bazlı) yetki burada, routing tablosunda görünür yerde yaşar; kaynak
// SAHİPLİĞİ kontrolleri (bu puan senin mi?) servis katmanında (GO_CONVENTIONS §12).
func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			// requireAuth'suz zincirlenmiş = yanlış kurulum. AÇIK değil KAPALI başarısız ol.
			unauthorized(w, "missing_token", "kimlik doğrulaması gerekli")
			return
		}
		if !p.IsAdmin() {
			writeError(w, http.StatusForbidden, "forbidden", "bu işlem için admin yetkisi gerekli")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIPKey, resolveClientIP'in çözdüğü istemci IP'sinin context anahtarı.
type clientIPKey struct{}

// resolveClientIP, isteğin GERÇEK istemci IP'sini bir kez çözer ve context'e koyar.
//
// Varsayılan: RemoteAddr (TCP bağlantısının karşı ucu). Bir reverse proxy (NGINX, Faz 11)
// arkasında bu HER ZAMAN proxy'nin adresidir — rate limit tüm kullanıcıları tek kovaya
// atardı. Bu yüzden: bağlantı GÜVENİLEN bir proxy'den geliyorsa, onun koyduğu X-Real-IP
// başlığı kullanılır.
//
// Neden yalnız güvenilen proxy'den: X-Real-IP / X-Forwarded-For'u İSTEMCİ de yazabilir.
// Her istekte sahte bir IP yollayıp rate limit'i atlatmasın diye, doğrudan gelen isteklerde
// bu başlıklar YOK SAYILIR. NGINX yapılandırması başlığı kendi $remote_addr'ıyla EZER
// (istemcinin gönderdiği değer proxy'den geçemez).
// X-Forwarded-For yerine X-Real-IP: XFF bir listedir ve istemci başına ekleme yapabilir;
// doğru okumak "sağdan ilk güvenilmeyen" mantığı ister. Tek hop'lu yapımızda gerek yok.
func resolveClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteHost(r)
			if addr, err := netip.ParseAddr(ip); err == nil && isTrusted(addr, trusted) {
				if realIP, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
					ip = realIP.String()
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip)))
		})
	}
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// clientIP, resolveClientIP'in çözdüğü adres; middleware zincirde yoksa RemoteAddr.
func clientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPKey{}).(string); ok {
		return ip
	}
	return remoteHost(r)
}

// rateLimit, kaba kuvvet (brute force) ve toplu hesap açmaya karşı IP başına sınır koyar.
// Aşılınca 429 + Retry-After (istemci ne kadar beklemesi gerektiğini bilsin).
//
// Sınırlayıcı arızası (Redis düştü) → İSTEĞE İZİN VER (fail-open) ve logla. Ödünleşim bilinçli:
// fail-closed, Redis'in her kesintisini TÜM kullanıcılar için "giriş yapılamıyor"a çevirirdi.
// Parola kaba kuvvetine karşı asıl duvar zaten bcrypt'in yavaşlığı; rate limit ikinci hat.
func rateLimit(l ratelimit.Limiter, name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			d, err := l.Allow(r.Context(), name+":"+clientIP(r))
			if err != nil {
				slog.Warn("rate limit kontrol edilemedi (izin veriliyor)", "err", err, "request_id", w.Header().Get(requestIDHeader))
				next.ServeHTTP(w, r)
				return
			}
			if !d.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(d.RetryAfter.Seconds()))))
				writeError(w, http.StatusTooManyRequests, "rate_limited", "çok fazla deneme; biraz bekleyip tekrar dene")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requestIDHeader, isteğin kimliğini istemciye de döndürdüğümüz başlık. Kullanıcı bir hata
// bildirdiğinde bu değeri verirse, log'da o isteğin bütün satırları bulunur.
const requestIDHeader = "X-Request-Id"

// requestLogger, chi'nin düz metin Logger'ının yerine: her istek için TEK yapılandırılmış
// satır (slog) — method, yol, status, süre, boyut, request_id. JSON modunda log toplayıcılar
// "status>=500 olanlar" gibi sorguları metin ayrıştırmadan yapabilir.
// middleware.RequestID'den SONRA zincirlenmeli (id'yi o üretir).
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := middleware.GetReqID(r.Context())
		w.Header().Set(requestIDHeader, id)

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor) // status/boyutu yakalamak için
		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK // handler hiç WriteHeader çağırmadıysa net/http 200 yazar
		}
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		}
		slog.Log(r.Context(), level, "http",
			"method", r.Method,
			"path", r.URL.Path, // query BİLEREK yok: arama terimleri kişisel veri olabilir
			"status", status,
			"bytes", ww.BytesWritten(),
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
			"remote", clientIP(r),
			"request_id", id,
		)
	})
}
