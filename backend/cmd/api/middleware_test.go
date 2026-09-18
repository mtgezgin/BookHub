package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/ratelimit"
)

// adminRouter, admin rotalarını test eden (handler davranışına odaklı) testler için:
// gerçek router + her isteğe geçerli bir ADMIN token'ı ekleyen ince sarmalayıcı.
// Koruma zincirinin KENDİSİ aşağıdaki TestAdminRoutesAreGuarded'da test ediliyor.
func adminRouter(d deps) http.Handler {
	d.tokens = fakeVerifier{}
	h := newRouter(d)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer good-admin")
		h.ServeHTTP(w, r)
	})
}

// adminRoutes, /api/admin altındaki TÜM rotalar. Yeni admin rotası eklenince buraya da
// eklenmeli — eklenmezse korumasız kalma riski testsiz kalır.
var adminRoutes = []struct{ method, path string }{
	{http.MethodGet, "/api/admin/books"},
	{http.MethodPost, "/api/admin/books"},
	{http.MethodGet, "/api/admin/books/1"},
	{http.MethodPatch, "/api/admin/books/1"},
	{http.MethodDelete, "/api/admin/books/1"},
	{http.MethodPost, "/api/admin/books/1/approve"},
	{http.MethodPost, "/api/admin/books/1/reject"},
	{http.MethodPost, "/api/admin/books/1/restore"},
	{http.MethodGet, "/api/admin/books/1/audit"},
	{http.MethodPut, "/api/admin/books/1/genres"},
	{http.MethodDelete, "/api/admin/comments/1"},
	{http.MethodPost, "/api/admin/genres"},
	{http.MethodPatch, "/api/admin/genres/1"},
	{http.MethodDelete, "/api/admin/genres/1"},
}

// TestAdminRoutesAreGuarded: her admin rotası token'sız 401, USER token'ıyla 403 döner.
// deps'teki store'lar nil: koruma delinseydi handler nil'e dokunur, panik → 500 olurdu —
// yani 401/403 dışındaki her sonuç bir sızıntıdır.
func TestAdminRoutesAreGuarded(t *testing.T) {
	h := newRouter(deps{tokens: fakeVerifier{}})
	for _, rt := range adminRoutes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			cases := []struct {
				name       string
				token      string
				wantStatus int
				wantCode   string
			}{
				{"token yok", "", http.StatusUnauthorized, "missing_token"},
				{"geçersiz token", "sahte", http.StatusUnauthorized, "invalid_token"},
				{"USER token", "good-user", http.StatusForbidden, "forbidden"},
			}
			for _, c := range cases {
				req := httptest.NewRequest(rt.method, rt.path, nil)
				if c.token != "" {
					req.Header.Set("Authorization", "Bearer "+c.token)
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != c.wantStatus {
					t.Errorf("%s: status istenen %d, gelen %d (%s)", c.name, c.wantStatus, rec.Code, rec.Body)
					continue
				}
				if got := errorCode(t, rec); got != c.wantCode {
					t.Errorf("%s: code istenen %q, gelen %q", c.name, c.wantCode, got)
				}
			}
		})
	}
}

// TestAdminRouteAllowsAdmin: koruma, ADMIN'i geçirir (handler'a ulaşılır).
func TestAdminRouteAllowsAdmin(t *testing.T) {
	fake := &fakeGenres{}
	rec := do(t, adminRouter(deps{genres: fake}), http.MethodPost, "/api/admin/genres", `{"name":"Tarih"}`)
	if rec.Code != http.StatusCreated || fake.gotName != "Tarih" {
		t.Errorf("admin handler'a ulaşmalı: status %d, gotName %q", rec.Code, fake.gotName)
	}
}

// TestRequireAdminWithoutRequireAuth: requireAdmin tek başına (yanlış zincir) kapalı
// başarısız olmalı — kimlik yoksa geçirmemeli.
func TestRequireAdminWithoutRequireAuth(t *testing.T) {
	called := false
	h := requireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if called || rec.Code != http.StatusUnauthorized {
		t.Errorf("kimliksiz istek geçmemeli: called=%v status=%d", called, rec.Code)
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		header, want string
		ok           bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"BEARER  abc ", "abc", true},
		{"", "", false},
		{"Bearer", "", false},
		{"Bearer ", "", false},
		{"Basic abc", "", false},
		{"Token abc", "", false},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tt.header != "" {
			req.Header.Set("Authorization", tt.header)
		}
		got, ok := bearerToken(req)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%q: istenen (%q,%v), gelen (%q,%v)", tt.header, tt.want, tt.ok, got, ok)
		}
	}
}

// TestPrincipalRoundTrip: context'e konan kimlik aynen okunur; boş context'te ok=false.
func TestPrincipalRoundTrip(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := principalFrom(req.Context()); ok {
		t.Error("boş context'te kimlik olmamalı")
	}
	want := auth.Principal{UserID: 3, Role: auth.RoleAdmin}
	got, ok := principalFrom(withPrincipal(req.Context(), want))
	if !ok || got != want {
		t.Errorf("istenen %+v, gelen %+v (ok=%v)", want, got, ok)
	}
}

// TestLoginRateLimit: aynı IP'den sınırı aşan login denemesi 429 + Retry-After alır ve
// servise HİÇ ulaşmaz; başka bir IP etkilenmez; register kotası ayrıdır.
func TestLoginRateLimit(t *testing.T) {
	svc := &fakeAuth{err: auth.ErrInvalidCredentials}
	h := newRouter(deps{auth: svc, tokens: fakeVerifier{}, limiter: ratelimit.NewMemory(3, time.Minute)})
	login := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.co","password":"x"}`))
		req.RemoteAddr = ip + ":51234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	for i := 1; i <= 3; i++ {
		if rec := login("10.0.0.1"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%d. deneme sınır içinde (401 beklenir): %d", i, rec.Code)
		}
	}
	svc.gotEmail = ""
	rec := login("10.0.0.1")
	if rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "rate_limited" || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("4. deneme 429 + Retry-After olmalı: %d %v", rec.Code, rec.Header())
	}
	if svc.gotEmail != "" {
		t.Error("sınırı aşan istek servise ulaşmamalı")
	}
	if rec := login("10.0.0.2"); rec.Code != http.StatusUnauthorized {
		t.Errorf("başka IP etkilenmemeli: %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{}`))
	req.RemoteAddr = "10.0.0.1:51234"
	regRec := httptest.NewRecorder()
	h.ServeHTTP(regRec, req)
	if regRec.Code == http.StatusTooManyRequests {
		t.Error("login denemeleri register kotasını tüketmemeli")
	}
}

type failingLimiter struct{}

func (failingLimiter) Allow(context.Context, string) (ratelimit.Decision, error) {
	return ratelimit.Decision{}, errors.New("redis: connection refused")
}

// TestRateLimitFailsOpen: sınırlayıcı arızası girişleri kilitlemez (bilinçli ödünleşim).
func TestRateLimitFailsOpen(t *testing.T) {
	h := newRouter(deps{auth: &fakeAuth{err: auth.ErrInvalidCredentials}, tokens: fakeVerifier{}, limiter: failingLimiter{}})
	rec := do(t, h, http.MethodPost, "/api/auth/login", `{"email":"a@b.co","password":"x"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("arızada istek geçmeli (401 servis cevabı): %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[2001:db8::1]:443"
	req.Header.Set("X-Forwarded-For", "6.6.6.6") // istemci yazabilir → YOK SAYILMALI
	if got := clientIP(req); got != "2001:db8::1" {
		t.Errorf("IPv6 host bekleniyordu, gelen %q", got)
	}
}
