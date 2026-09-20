package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/config"
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
		path string
		body string
		// cookies: refresh/logout token'ı Faz 13'ten beri gövdede değil çerezde taşıyor.
		cookies []*http.Cookie
		reached func(*fakeAuth) bool // handler'a ulaşıldı mı (sahte servis çağrıldı mı)
	}{
		{path: "/api/auth/register", body: `{"email":"ali@example.com","display_name":"Ali","password":"yeterince-uzun"}`, reached: func(f *fakeAuth) bool { return f.gotEmail != "" }},
		{path: "/api/auth/login", body: `{"email":"ali@example.com","password":"yeterince-uzun"}`, reached: func(f *fakeAuth) bool { return f.gotEmail != "" }},
		{path: "/api/auth/refresh", cookies: []*http.Cookie{refreshCookie("tok")}, reached: func(f *fakeAuth) bool { return f.gotToken == "tok" }},
		{path: "/api/auth/logout", cookies: []*http.Cookie{refreshCookie("tok")}, reached: func(f *fakeAuth) bool { return f.gotToken == "tok" }},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			svc := &fakeAuth{user: sampleUser, session: auth.Session{AccessToken: "a", User: sampleUser}}
			h := newRouter(deps{auth: svc, tokens: fakeVerifier{}, cookies: testCookies})

			// Authorization başlığı YOK: public rota token istememeli.
			rec := doAuth(t, h, http.MethodPost, tt.path, tt.body, tt.cookies...)
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

// TestRouter_CookieNameDefaultsWhenUnset: deps.cookies HİÇ doldurulmadığında (yalnız bir alanı
// dolduran testlerin yaptığı gibi: deps{auth: fake}) newRouter'ın çerez adını
// config.DefaultCookieName'e düşürdüğünü doğrular — hem YAZARKEN (login'in Set-Cookie'si) hem
// OKURKEN (refresh'in istekten okuduğu çerez).
//
// NEDEN: net/http, adı boş bir çerezi HATA DÖNMEDEN düşürür. Bu düşüş olmasaydı router
// "çerezsiz" bir dünyada koşar, oturum yazan/okuyan her iddia sessizce boşa çıkardı.
func TestRouter_CookieNameDefaultsWhenUnset(t *testing.T) {
	t.Run("login çerezi varsayılan adla YAZILIR", func(t *testing.T) {
		svc := &fakeAuth{user: sampleUser, session: auth.Session{AccessToken: "a", RefreshToken: "r", User: sampleUser}}

		// cookies alanı BİLEREK yok: düşüşü sınayan şey bu.
		rec := doAuth(t, newRouter(deps{auth: svc}), http.MethodPost, "/api/auth/login", crossOriginLoginBody)

		if rec.Code != http.StatusOK {
			t.Fatalf("durum kodu: istenen 200, gelen %d (%s)", rec.Code, rec.Body)
		}
		ck := responseCookie(rec, config.DefaultCookieName)
		if ck == nil {
			t.Fatalf("Set-Cookie %q bekleniyordu, gelen başlıklar: %v", config.DefaultCookieName, rec.Header()["Set-Cookie"])
		}
		if ck.Value != "r" {
			t.Errorf("çerez değeri: istenen %q, gelen %q", "r", ck.Value)
		}
	})

	t.Run("refresh çerezi varsayılan adla OKUNUR", func(t *testing.T) {
		svc := &fakeAuth{user: sampleUser, session: auth.Session{AccessToken: "a", RefreshToken: "yeni", User: sampleUser}}

		rec := doAuth(t, newRouter(deps{auth: svc}), http.MethodPost, "/api/auth/refresh", "",
			&http.Cookie{Name: config.DefaultCookieName, Value: "eski"})

		if rec.Code != http.StatusOK {
			t.Fatalf("durum kodu: istenen 200, gelen %d (%s)", rec.Code, rec.Body)
		}
		if svc.gotToken != "eski" {
			t.Errorf("servise iletilen token: istenen %q, gelen %q", "eski", svc.gotToken)
		}
	})
}

// crossOriginLoginBody, cross-origin testlerinde geçen isteğin handler'a ulaşıp 200 alacağı gövde.
const crossOriginLoginBody = `{"email":"ali@example.com","password":"yeterince-uzun"}`

// doWithHeaders, do'nun başlık alan eşi: tarayıcının KENDİLİĞİNDEN yazdığı başlıkları
// (Sec-Fetch-Site, Origin) ve proxy'nin ilettiği Host'u taklit eden testler için.
// host "" → httptest varsayılanı ("example.com").
func doWithHeaders(t *testing.T, h http.Handler, method, host, url, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	if host != "" {
		req.Host = host
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestRouter_CrossOriginProtection: tarayıcıdan gelen cross-origin YAZMA istekleri router'da,
// handler'a ULAŞMADAN 403 + JSON zarfı (cross_origin_request) alır; aynı origin, güvenli
// method'lar ve başlıksız (tarayıcı dışı) istemciler geçer.
//
// Handler testleri handler'ı doğrudan çağırdığı için bu koruma onlarda görünmez → burada gerçek
// router'la, tarayıcının yazacağı başlıklarla sınanıyor.
func TestRouter_CrossOriginProtection(t *testing.T) {
	crossSite := map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}
	tests := []struct {
		name          string
		method, path  string // "" → POST /api/auth/login (geçerse sahte servis 200 döner)
		host          string // "" → example.com
		headers       map[string]string
		wantForbidden bool
	}{
		// Sec-Fetch-Site: tüm güncel tarayıcılar yazar; karar önce buna göre verilir.
		{name: "Sec-Fetch-Site cross-site POST → 403", headers: crossSite, wantForbidden: true},
		{name: "Sec-Fetch-Site same-site (kardeş alt alan) POST → 403",
			headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "http://blog.example.com"}, wantForbidden: true},
		{name: "Sec-Fetch-Site same-origin POST → geçer",
			headers: map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://example.com"}},
		{name: "Sec-Fetch-Site none (adres çubuğu/yer imi) POST → geçer",
			headers: map[string]string{"Sec-Fetch-Site": "none"}},
		// Aşağıdakiler kararın GERÇEKTEN Sec-Fetch-Site'a göre verildiğini ayırt eder: Origin yedeği
		// tek başına bu sonuçları üretemezdi (Origin yok ya da Host'la eşit).
		{name: "Sec-Fetch-Site cross-site, Origin yok → 403",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantForbidden: true},
		{name: "Sec-Fetch-Site cross-site, Origin = Host → 403 (tarayıcının beyanı önceliklidir)",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "http://example.com"}, wantForbidden: true},
		{name: "Sec-Fetch-Site same-site, Origin yok → 403",
			headers: map[string]string{"Sec-Fetch-Site": "same-site"}, wantForbidden: true},
		{name: "Sec-Fetch-Site same-origin, Origin ≠ Host → geçer (Origin'e bakılmaz)",
			headers: map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "https://evil.example"}},

		// Sec-Fetch-Site yok (eski tarayıcı ya da düz HTTP'deki, localhost olmayan bir origin):
		// Origin'in host[:port]'u Host ile karşılaştırılır.
		{name: "Origin = Host → geçer", headers: map[string]string{"Origin": "http://example.com"}},
		{name: "Origin ≠ Host → 403", headers: map[string]string{"Origin": "https://evil.example"}, wantForbidden: true},
		// NGINX'in api-proxy.conf'u neden $http_host iletiyor: $host portu düşürür → tarayıcı
		// localhost:3000'deyken API "localhost" görür ve MEŞRU istek 403 alırdı.
		{name: "Origin portlu, Host portsuz (NGINX $host) → 403", host: "localhost",
			headers: map[string]string{"Origin": "http://localhost:3000"}, wantForbidden: true},
		{name: "Origin portlu, Host portlu (NGINX $http_host) → geçer", host: "localhost:3000",
			headers: map[string]string{"Origin": "http://localhost:3000"}},

		// Güvenli method'lar HER ZAMAN geçer; başlıksız istemciler (curl, mvp_smoke.sh) de.
		{name: "GET cross-site → geçer", method: http.MethodGet, path: "/api/books", headers: crossSite},
		{name: "başlıksız POST (curl, smoke betiği) → geçer"},

		// Yalnız POST değil tüm güvensiz method'lar; ve kimlik kontrolünden ÖNCE (401 değil 403).
		{name: "DELETE cross-site → 403 (kimlik kontrolünden önce)", method: http.MethodDelete,
			path: "/api/books/1/rating", headers: crossSite, wantForbidden: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, path, body := tt.method, tt.path, ""
			if method == "" {
				method, path, body = http.MethodPost, "/api/auth/login", crossOriginLoginBody
			}
			svc := &fakeAuth{user: sampleUser, session: auth.Session{AccessToken: "a", User: sampleUser}}
			h := newRouter(deps{auth: svc, books: fakeBooks{}, tokens: fakeVerifier{}})

			rec := doWithHeaders(t, h, method, tt.host, path, body, tt.headers)

			if !tt.wantForbidden {
				if rec.Code != http.StatusOK {
					t.Fatalf("%s %s geçmeliydi: istenen 200, gelen %d (%s)", method, path, rec.Code, rec.Body)
				}
				return
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("durum kodu: istenen 403, gelen %d (%s)", rec.Code, rec.Body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type: istenen application/json, gelen %q", ct)
			}
			if code := errorCode(t, rec); code != "cross_origin_request" {
				t.Errorf("error.code: istenen \"cross_origin_request\", gelen %q", code)
			}
			if svc.gotEmail != "" {
				t.Error("reddedilen istek handler'a (servise) ulaşmamalı")
			}
		})
	}
}

// TestRouter_CrossOriginDenialIsLogged: koruma zincirde RequestID + requestLogger'dan SONRA
// gelir → 403 cevabı X-Request-Id taşır; erişim satırı ve nedeni anlatan WARN satırı AYNI
// request_id'yi paylaşır. Koruma zincirin başına taşınırsa bu test kırılır (ret izsiz kalırdı).
func TestRouter_CrossOriginDenialIsLogged(t *testing.T) {
	buf := captureLogs(t)
	rec := doWithHeaders(t, newRouter(deps{auth: &fakeAuth{}}), http.MethodPost, "", "/api/auth/login", crossOriginLoginBody,
		map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("403 bekleniyordu: %d (%s)", rec.Code, rec.Body)
	}
	id := rec.Header().Get(requestIDHeader)
	if id == "" {
		t.Fatal("ret cevabı da X-Request-Id taşımalı")
	}

	var denyLine, accessLine map[string]any
	for _, l := range logLines(t, buf) {
		switch l["msg"] {
		case "cross-origin istek reddedildi":
			denyLine = l
		case "http":
			accessLine = l
		}
	}
	if denyLine == nil || accessLine == nil {
		t.Fatalf("ret ve erişim satırı bekleniyordu: %s", buf)
	}
	if accessLine["status"] != float64(http.StatusForbidden) || accessLine["request_id"] != id {
		t.Errorf("erişim satırı 403'ü aynı request_id ile yazmalı (başlık=%q): %v", id, accessLine)
	}
	if denyLine["level"] != "WARN" || denyLine["request_id"] != id {
		t.Errorf("ret satırı WARN seviyesinde ve aynı request_id ile olmalı (başlık=%q): %v", id, denyLine)
	}
	if denyLine["sec_fetch_site"] != "cross-site" || denyLine["origin"] != "https://evil.example" || denyLine["host"] != "example.com" {
		t.Errorf("ret satırı nedeni (sec_fetch_site/origin/host) taşımalı: %v", denyLine)
	}
}
