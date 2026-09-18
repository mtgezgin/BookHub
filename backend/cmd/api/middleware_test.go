package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
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
	trusted := []netip.Prefix{netip.MustParsePrefix("172.28.0.10/32")}
	tests := []struct {
		name, remote, realIP, want string
	}{
		{"doğrudan istek, başlık yok", "203.0.113.5:4000", "", "203.0.113.5"},
		// İstemcinin kendisi yazdığı başlık YOK SAYILMALI — yoksa her istekte sahte IP ile
		// rate limit atlatılırdı.
		{"doğrudan istek, sahte X-Real-IP", "203.0.113.5:4000", "6.6.6.6", "203.0.113.5"},
		{"güvenilen proxy'den, X-Real-IP kullanılır", "172.28.0.10:5000", "198.51.100.7", "198.51.100.7"},
		{"güvenilen proxy'den, bozuk başlık → proxy IP'si", "172.28.0.10:5000", "bozuk", "172.28.0.10"},
		{"güvenilmeyen iç adres", "172.28.0.11:5000", "198.51.100.7", "172.28.0.11"},
		{"IPv6", "[2001:db8::1]:443", "", "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := resolveClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = clientIP(r) }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tt.want {
				t.Errorf("istenen %q, gelen %q", tt.want, got)
			}
		})
	}
}

// TestRateLimitBehindProxy: NGINX arkasında iki farklı istemci AYRI kovalara düşmeli
// (güvenilen proxy'nin X-Real-IP'si sayesinde); aynı istemci sınıra takılmalı.
func TestRateLimitBehindProxy(t *testing.T) {
	h := newRouter(deps{
		auth: &fakeAuth{err: auth.ErrInvalidCredentials}, tokens: fakeVerifier{},
		limiter:        ratelimit.NewMemory(1, time.Minute),
		trustedProxies: []netip.Prefix{netip.MustParsePrefix("172.28.0.10/32")},
	})
	login := func(client string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.co","password":"x"}`))
		req.RemoteAddr = "172.28.0.10:5000" // hepsi aynı proxy'den gelir
		req.Header.Set("X-Real-IP", client)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if login("198.51.100.1") != http.StatusUnauthorized || login("198.51.100.2") != http.StatusUnauthorized {
		t.Fatal("farklı istemciler ayrı kovada olmalı (ilk denemeleri geçmeli)")
	}
	if code := login("198.51.100.1"); code != http.StatusTooManyRequests {
		t.Errorf("aynı istemcinin ikinci denemesi 429 olmalı: %d", code)
	}
}

func TestRemoteHostWithoutPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5"
	if got := remoteHost(req); got != "10.0.0.5" {
		t.Errorf("portsuz RemoteAddr olduğu gibi dönmeli: %q", got)
	}
}

func TestClientIPWithoutResolver(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:4321"
	req.Header.Set("X-Real-IP", "6.6.6.6")
	if got := clientIP(req); got != "203.0.113.9" {
		t.Errorf("resolveClientIP yoksa RemoteAddr host'u dönmeli: %q", got)
	}
}

// TestRequestLoggerDefaultsStatus200: hiç WriteHeader/Write çağırmayan handler 200 loglanmalı (0 değil).
func TestRequestLoggerDefaultsStatus200(t *testing.T) {
	buf := captureLogs(t)
	h := requestLogger(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/sessiz", nil))
	lines := logLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("tek erişim satırı bekleniyordu: %v", lines)
	}
	if status, _ := lines[0]["status"].(float64); status != http.StatusOK {
		t.Errorf("status=200 loglanmalı, gelen %v", lines[0]["status"])
	}
}
