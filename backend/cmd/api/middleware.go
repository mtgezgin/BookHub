package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
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
