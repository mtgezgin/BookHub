package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/config"
	"github.com/mtgezgin/bookhub/backend/internal/ratelimit"
)

// fakeAuth, authService'in sahtesi: dönecek değerler + son çağrının argümanları.
//
// logoutErrs: Logout'un SIRAYLA döneceği hatalar (biten yerde err'e düşer). handleLogout
// iptali yeniden denediği için "ilk deneme patlar, ikincisi tutar" gibi senaryoları ancak
// çağrı başına ayrı cevap verebilen bir sahte sınayabilir.
type fakeAuth struct {
	user       auth.User
	session    auth.Session
	err        error
	logoutErrs []error

	gotEmail, gotPassword, gotToken string
	gotNewPassword                  string
	gotUserID                       int64
	logoutCalls                     int

	// logoutCtxErr / logoutCtxDeadline: Logout'a GELEN context'in durumu. handleLogout,
	// isteğin context'ini context.WithoutCancel ile kopardığı ve kendi 5 sn'lik bütçesini
	// taktığı için çağrıya iptal EDİLMEMİŞ ama SÜRESİ SINIRLI bir context ulaşmalı. Sahte
	// context'i çöpe atsaydı, o satır silindiğinde hiçbir test kırılmazdı.
	logoutCtxErr      error
	logoutCtxDeadline bool
}

func (f *fakeAuth) Register(_ context.Context, email, _, password string) (auth.User, error) {
	f.gotEmail, f.gotPassword = email, password
	return f.user, f.err
}
func (f *fakeAuth) Login(_ context.Context, email, password string) (auth.Session, error) {
	f.gotEmail, f.gotPassword = email, password
	return f.session, f.err
}
func (f *fakeAuth) Refresh(_ context.Context, token string) (auth.Session, error) {
	f.gotToken = token
	return f.session, f.err
}
func (f *fakeAuth) Logout(ctx context.Context, token string) error {
	f.logoutCtxErr = ctx.Err()
	_, f.logoutCtxDeadline = ctx.Deadline()
	f.gotToken = token
	f.logoutCalls++
	if len(f.logoutErrs) > 0 {
		err := f.logoutErrs[0]
		f.logoutErrs = f.logoutErrs[1:]
		return err
	}
	return f.err
}
func (f *fakeAuth) Me(_ context.Context, id int64) (auth.User, error) {
	f.gotUserID = id
	return f.user, f.err
}
func (f *fakeAuth) ChangePassword(_ context.Context, id int64, current, next string) (auth.Session, error) {
	f.gotUserID = id
	f.gotPassword, f.gotNewPassword = current, next
	return f.session, f.err
}

// fakeVerifier, yalnızca "good-<rol>" token'larını kabul eder.
type fakeVerifier struct{}

func (fakeVerifier) Verify(raw string) (auth.Principal, error) {
	switch raw {
	case "good-user":
		return auth.Principal{UserID: 7, Role: auth.RoleUser}, nil
	case "good-admin":
		return auth.Principal{UserID: 1, Role: auth.RoleAdmin}, nil
	}
	return auth.Principal{}, auth.ErrInvalidToken
}

var sampleUser = auth.User{
	ID: 7, Email: "ali@example.com", DisplayName: "Ali", Role: auth.RoleUser,
	CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
}

// testCookies, handler testlerinin varsayılan çerez ayarı (üretim varsayılanıyla aynı).
var testCookies = cookieSettings{name: "bookhub_refresh", secure: true}

// doAuth, do'nun çerezli sürümü: isteğe Cookie başlığı ekleyip handler'ı çalıştırır.
func doAuth(t *testing.T, h http.Handler, method, url, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// refreshCookie, isteğe eklenecek refresh çerezi (yalnız ad + değer; öznitelikler
// tarayıcıdan sunucuya GİTMEZ — istek başlığı sadece "ad=değer" çiftleri taşır).
func refreshCookie(value string) *http.Cookie {
	return &http.Cookie{Name: testCookies.name, Value: value}
}

// noLogoutBackoff, handleLogout'un yeniden deneme beklemelerini bu test boyunca sıfırlar:
// gerçek süreler (200 ms + 1 sn) başarısız iptali sınayan her teste boşuna saniyeler eklerdi.
// Bu paketteki testler t.Parallel kullanmadığı için genel değişkeni geçici olarak değiştirmek
// güvenli; t.Cleanup eski değeri geri koyar.
func noLogoutBackoff(t *testing.T) {
	t.Helper()
	old := logoutRevokeBackoff
	logoutRevokeBackoff = make([]time.Duration, len(old)) // aynı sayıda deneme, sıfır bekleme
	t.Cleanup(func() { logoutRevokeBackoff = old })
}

// responseCookie, cevabın Set-Cookie başlıkları arasından verilen adlıyı bulur (yoksa nil).
func responseCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// assertClearingCookie, cevabın çerezi SİLEN bir Set-Cookie taşıdığını doğrular: boş değer +
// Max-Age=0 ve silme için şart olan aynı Path/HttpOnly/SameSite/Secure üçlüsü (farklı olsaydı
// tarayıcı ikinci bir çerez yazar, asıl çerez yaşamaya devam ederdi).
func assertClearingCookie(t *testing.T, rec *httptest.ResponseRecorder, c cookieSettings) {
	t.Helper()
	ck := responseCookie(rec, c.name)
	if ck == nil {
		t.Fatalf("silme çerezi bekleniyordu, Set-Cookie: %v", rec.Header().Values("Set-Cookie"))
	}
	if ck.Value != "" || ck.MaxAge != -1 {
		t.Errorf("silme çerezi boş değer + Max-Age=0 olmalı: %q", ck)
	}
	// Path DEĞERİYLE yazılır, refreshCookiePath sabitiyle DEĞİL: sabitle karşılaştırmak
	// totoloji olurdu (sabit "/" olsa beklenti de "/" olur, test yeşil kalırdı) — oysa dar
	// kapsam güvenlik gereği, bkz. auth.go'daki refreshCookiePath yorumu.
	if ck.Path != "/api/auth" || !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode || ck.Secure != c.secure {
		t.Errorf("silme çerezi set edilenle aynı öznitelikleri taşımalı: %q", ck)
	}
}

func TestHandleRegister(t *testing.T) {
	const valid = `{"email":"ali@example.com","display_name":"Ali","password":"yeterince-uzun"}`
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "geçerli → 201", body: valid, wantStatus: http.StatusCreated},
		// Mass-assignment: istemci kendini admin yapmaya çalışıyor.
		{name: "role alanı → 400", body: `{"email":"a@b.co","display_name":"A","password":"12345678","role":"ADMIN"}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "doğrulama hatası → 400", body: valid, svcErr: &auth.ValidationError{Problems: []string{"password en az 8 karakter olmalı"}},
			wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "kayıtlı e-posta → 409", body: valid, svcErr: auth.ErrEmailTaken, wantStatus: http.StatusConflict, wantCode: "email_taken"},
		{name: "beklenmeyen → 500", body: valid, svcErr: errors.New("db çöktü"), wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAuth{user: sampleUser, err: tt.svcErr}
			rec := do(t, handleRegister(svc), http.MethodPost, "/api/auth/register", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Errorf("code: istenen %q, gelen %q", tt.wantCode, got)
				}
				return
			}
			assertUserKeys(t, rec.Body.Bytes())
		})
	}
}

// assertUserKeys, kullanıcı cevabının TAM alan listesini sabitler: password_hash (ya da
// ileride eklenecek herhangi bir iç alan) sessizce sızamaz.
func assertUserKeys(t *testing.T, body []byte) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("cevap JSON değil: %v", err)
	}
	want := []string{"id", "email", "display_name", "role", "created_at"}
	if len(got) != len(want) {
		t.Errorf("alan sayısı: istenen %d %v, gelen %d %v", len(want), want, len(got), got)
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("eksik alan %q", k)
		}
	}
}

func TestHandleLogin(t *testing.T) {
	sess := auth.Session{
		AccessToken: "acc", AccessExpiresAt: time.Date(2026, 9, 1, 0, 15, 0, 0, time.UTC),
		RefreshToken: "ref", RefreshExpiresAt: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
		User: sampleUser,
	}
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "geçerli → 200", body: `{"email":"ali@example.com","password":"x"}`, wantStatus: http.StatusOK},
		{name: "eksik alan → 400", body: `{"email":"ali@example.com"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "hatalı kimlik → 401 generic", body: `{"email":"ali@example.com","password":"x"}`, svcErr: auth.ErrInvalidCredentials,
			wantStatus: http.StatusUnauthorized, wantCode: "invalid_credentials"},
		{name: "beklenmeyen → 500", body: `{"email":"a@b.co","password":"x"}`, svcErr: errors.New("db"), wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAuth{session: sess, err: tt.svcErr}
			rec := do(t, handleLogin(svc, testCookies), http.MethodPost, "/api/auth/login", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Errorf("code: istenen %q, gelen %q", tt.wantCode, got)
				}
				return
			}
			var got sessionResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("cevap JSON değil: %v", err)
			}
			if got.AccessToken != "acc" || got.TokenType != "Bearer" || got.User.ID != 7 {
				t.Errorf("oturum cevabı yanlış: %+v", got)
			}
		})
	}
}

// TestHandleRefreshAndLogout (Faz 13'te ÇEREZE taşındı): token artık gövdede değil,
// httpOnly çerezde gelir. Gövde hiç okunmaz.
func TestHandleRefreshAndLogout(t *testing.T) {
	t.Run("refresh geçerli → 200, çerezdeki token servise iletilir", func(t *testing.T) {
		svc := &fakeAuth{session: auth.Session{AccessToken: "yeni", User: sampleUser}}
		rec := doAuth(t, handleRefresh(svc, testCookies), http.MethodPost, "/", "", refreshCookie("eski"))
		if rec.Code != http.StatusOK || svc.gotToken != "eski" {
			t.Errorf("status %d, iletilen token %q", rec.Code, svc.gotToken)
		}
	})
	t.Run("refresh geçersiz → 401 + silme çerezi", func(t *testing.T) {
		rec := doAuth(t, handleRefresh(&fakeAuth{err: auth.ErrInvalidToken}, testCookies), http.MethodPost, "/", "", refreshCookie("x"))
		if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "invalid_token" {
			t.Errorf("401 invalid_token bekleniyordu: %d %s", rec.Code, rec.Body)
		}
		assertClearingCookie(t, rec, testCookies)
	})
	// Gövdedeki token ARTIK GEÇERSİZ bir kanal: kabul etseydik httpOnly'nin anlamı kalmazdı
	// (XSS'li betik çaldığı değeri gövdede yollayabilirdi).
	t.Run("çerez yok, token gövdede → 401 + servis çağrılmaz", func(t *testing.T) {
		svc := &fakeAuth{}
		rec := do(t, handleRefresh(svc, testCookies), http.MethodPost, "/", `{"refresh_token":"gövdeden"}`)
		if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "invalid_token" {
			t.Fatalf("401 invalid_token bekleniyordu: %d %s", rec.Code, rec.Body)
		}
		if svc.gotToken != "" {
			t.Errorf("servis çağrılmamalıydı, iletilen token %q", svc.gotToken)
		}
		assertClearingCookie(t, rec, testCookies)
	})
	t.Run("logout → 204, gövdesiz + silme çerezi", func(t *testing.T) {
		svc := &fakeAuth{}
		rec := doAuth(t, handleLogout(svc, testCookies), http.MethodPost, "/", "", refreshCookie("tok"))
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || svc.gotToken != "tok" {
			t.Errorf("204 + boş gövde bekleniyordu: %d %q (token %q)", rec.Code, rec.Body, svc.gotToken)
		}
		assertClearingCookie(t, rec, testCookies)
	})
	t.Run("logout çerezsiz → 204, servis çağrılmaz", func(t *testing.T) {
		svc := &fakeAuth{}
		rec := do(t, handleLogout(svc, testCookies), http.MethodPost, "/", "")
		if rec.Code != http.StatusNoContent || svc.gotToken != "" {
			t.Errorf("204 + servis çağrısı YOK bekleniyordu: %d (token %q)", rec.Code, svc.gotToken)
		}
		assertClearingCookie(t, rec, testCookies)
	})
	// DEĞİŞTİ (inceleme bulgusu): eskiden bu alt test 500'de de silme çerezi bekliyordu.
	// O kural, "çıkış = aile ölür" değişmezini sunucunun cevap verdiği TEK başarısızlık
	// türünde sessizce bozuyordu: çerez silinince istemcinin LOGOUT_PENDING yeniden denemesi
	// ÇEREZSİZ gider, sunucu hiçbir şey iptal etmeden 204 der, istemci bayrağı siler ve aile
	// mutlak tavana kadar yaşar. Artık kural handleRefresh'inkiyle aynı: 500'de çerez KORUNUR
	// (iptalin tek anahtarı odur), yalnız BAŞARIDA ve çerezsiz istekte silinir.
	t.Run("logout servis hatası → 500 ve çerez KORUNUR (yeniden deneme anahtarsız kalmasın)", func(t *testing.T) {
		noLogoutBackoff(t)
		svc := &fakeAuth{err: errors.New("db")}
		rec := doAuth(t, handleLogout(svc, testCookies), http.MethodPost, "/", "", refreshCookie("tok"))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("500 bekleniyordu: %d", rec.Code)
		}
		if ck := responseCookie(rec, testCookies.name); ck != nil {
			t.Errorf("500'de çerez SİLİNMEMELİ, gelen Set-Cookie: %q", ck)
		}
		// Sunucu pes etmeden önce kendi kendini onarmayı dener (ilk deneme + 2 tekrar).
		if svc.logoutCalls != len(logoutRevokeBackoff)+1 {
			t.Errorf("iptal %d kez denenmeliydi, %d kez denendi", len(logoutRevokeBackoff)+1, svc.logoutCalls)
		}
	})

	// SUNUCU KENDİ KENDİNİ ONARIR: token'ı bu an yalnız biz görebiliyoruz (istemcideki çerez
	// httpOnly), üstelik context.WithoutCancel ile koşuyoruz — DB'nin bir anlık sarsıntısında
	// ikinci deneme çoğu zaman tutar ve kullanıcı hiçbir şey fark etmez.
	t.Run("iptal ilk denemede patlarsa yeniden denenir, tutunca 204 + silme çerezi", func(t *testing.T) {
		noLogoutBackoff(t)
		svc := &fakeAuth{logoutErrs: []error{errors.New("db geçici")}} // 2. çağrı nil döner
		rec := doAuth(t, handleLogout(svc, testCookies), http.MethodPost, "/", "", refreshCookie("tok"))
		if rec.Code != http.StatusNoContent {
			t.Errorf("204 bekleniyordu: %d %s", rec.Code, rec.Body)
		}
		if svc.logoutCalls != 2 {
			t.Errorf("iptal 2 kez denenmeliydi, %d", svc.logoutCalls)
		}
		// Aile gerçekten öldüğü için çerez artık ölü bir sır: silinir.
		assertClearingCookie(t, rec, testCookies)
	})

	// İSTEMCİ KOPSA DA AİLE ÖLMELİ: sekme kapanırken atılan çıkış isteğinde (istemci onu
	// keepalive ile yolluyor) bağlantı cevap dönmeden kopabilir. handleLogout iptali
	// context.WithoutCancel ile koştuğu için iş yarıda kalmaz; o satır düşerse r.Context()
	// iptal olur, DB'deki iptal transaction'ı kesilir ve "çıkış yaptım" diyen kullanıcının
	// ailesi mutlak tavana kadar yaşar. İddianın mutantı öldüren kısmı ctx.Err()==nil:
	// sahte servis context'i umursamadığı için durum kodu/çerez tek başına fark etmez.
	t.Run("istemci bağlantıyı koparsa iptal yine de tamamlanır", func(t *testing.T) {
		svc := &fakeAuth{}
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		req.AddCookie(refreshCookie("tok"))
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // istemci koptu
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		handleLogout(svc, testCookies).ServeHTTP(rec, req)

		if svc.logoutCalls != 1 {
			t.Fatalf("Logout 1 kez çağrılmalıydı, %d", svc.logoutCalls)
		}
		if svc.logoutCtxErr != nil {
			t.Errorf("Logout iptal EDİLMEMİŞ bir context ile çağrılmalıydı, gelen: %v", svc.logoutCtxErr)
		}
		// Kopma bağımsızlığı context'i SÜRESİZ bırakmak pahasına alınmamalı: 5 sn'lik bütçe
		// (ve onunla birlikte "sızan goroutine yok" güvencesi) de yerinde durmalı.
		if !svc.logoutCtxDeadline {
			t.Error("Logout context'i zaman sınırlı olmalıydı (5 sn'lik bütçe)")
		}
		if rec.Code != http.StatusNoContent {
			t.Errorf("204 bekleniyordu: %d", rec.Code)
		}
		assertClearingCookie(t, rec, testCookies)
	})
}

// TestSleepCtxRespectsBudget: sleepCtx'in İKİ dalı da doğrudan sınanır.
//
// NEDEN AYRI BİR TEST: handleLogout testleri noLogoutBackoff ile bütün beklemeleri sıfırlıyor,
// bu yüzden handler üzerinden sleepCtx her zaman <-t.C dalını seçiyor; <-ctx.Done() dalı
// hiç çalışmıyor. O dal ölü kaldığı sürece sleepCtx'i düz time.Sleep'e çeviren bir değişiklik
// (bütçe dolduktan sonra bile beklemeye devam eden, isteği 5 sn'nin ötesine taşıyan sürüm)
// tek bir testi bile kırmazdı.
func TestSleepCtxRespectsBudget(t *testing.T) {
	t.Run("bütçe dolmuşsa beklemeden false döner", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // bütçe doldu (handler'da: 5 sn'lik timeout'un sona ermesi)
		start := time.Now()
		// Süre BİLEREK bir saat: fonksiyon timer'ı bekleseydi test orada asılı kalırdı, yani
		// hızlı dönüşün kendisi ctx dalının seçildiğinin kanıtı.
		if sleepCtx(ctx, time.Hour) {
			t.Error("bütçesi dolmuş context'te false dönmeliydi (yeniden denemenin anlamı yok)")
		}
		if d := time.Since(start); d > time.Second {
			t.Errorf("erken dönmeliydi, %s bekledi", d)
		}
	})
	t.Run("bütçe varken beklemeyi tamamlar ve true döner", func(t *testing.T) {
		if !sleepCtx(context.Background(), time.Millisecond) {
			t.Error("beklemesi tamamlanan çağrı true dönmeli (döngü yeniden denemeli)")
		}
	})
}

// TestHandleLogoutStopsWhenBudgetExhausted: bütçe bir backoff beklemesi sırasında dolarsa
// döngü ERKEN durur — kalan denemeleri yapmaz — ama cevap yine 500 ve çerez yine KORUNUR.
//
// sleepCtx'in dönüşünü yok sayan bir döngü (ör. `sleepCtx(...)` çağrısının sonucunu atan
// hâli) bu testte kalan denemeleri de yapar ve çağrı sayısı tutmaz. Diğer logout testleri
// bunu göremez: onlarda bekleme sıfır olduğu için bütçe hiç dolmuyor.
func TestHandleLogoutStopsWhenBudgetExhausted(t *testing.T) {
	// Beklemeler bütçeden (handler'daki sabit 5 sn) UZUN: ilk deneme patlar, sleepCtx bütçe
	// dolana dek bekler ve false döner. noLogoutBackoff'un tersi bir kurulum.
	//
	// Bu alt test gerçekten ~5 sn sürer ve kısaltılamaz: bütçe handler'da sabit kodlu, üstelik
	// context.WithoutCancel üst context'in deadline'ını da sildiği için isteğe daha kısa bir
	// süre takıp bütçeyi dışarıdan daraltmak mümkün değil.
	old := logoutRevokeBackoff
	logoutRevokeBackoff = []time.Duration{time.Hour, time.Hour}
	t.Cleanup(func() { logoutRevokeBackoff = old })

	svc := &fakeAuth{err: errors.New("db")}
	rec := doAuth(t, handleLogout(svc, testCookies), http.MethodPost, "/", "", refreshCookie("tok"))

	// Bütçe dolduğu an duruldu: ilk deneme + hiç. Tam sayı yazılı, çünkü asıl iddia
	// "len(backoff)+1 denemenin HEPSİ yapılmadı".
	if svc.logoutCalls != 1 {
		t.Errorf("bütçe dolunca döngü ilk denemeden sonra durmalıydı, %d kez denendi", svc.logoutCalls)
	}
	if svc.logoutCalls >= len(logoutRevokeBackoff)+1 {
		t.Errorf("tüm denemeler yapılmış: bütçe hiç dikkate alınmamış (%d çağrı)", svc.logoutCalls)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("iptal edilemedi: 500 bekleniyordu, gelen %d %s", rec.Code, rec.Body)
	}
	// Erken çıkış da bir BAŞARISIZLIK: aile DB'de canlı kaldı, onu öldürebilecek tek anahtar
	// tarayıcıdaki bu çerez — silinmemeli.
	if ck := responseCookie(rec, testCookies.name); ck != nil {
		t.Errorf("bütçe dolduğunda da çerez SİLİNMEMELİ, gelen Set-Cookie: %q", ck)
	}
}

// TestHandleLoginSetsRefreshCookie: başarılı giriş, refresh token'ı YALNIZCA çerezle verir.
// Öznitelikler tek tek sabitlenir — biri sessizce düşerse (ör. HttpOnly) Faz 13'ün tüm
// gerekçesi buharlaşır ve kimse fark etmez.
func TestHandleLoginSetsRefreshCookie(t *testing.T) {
	exp := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	sess := auth.Session{AccessToken: "acc", RefreshToken: "gizli-refresh", RefreshExpiresAt: exp, User: sampleUser}
	svc := &fakeAuth{session: sess}

	rec := do(t, handleLogin(svc, testCookies), http.MethodPost, "/api/auth/login", `{"email":"ali@example.com","password":"x"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("200 bekleniyordu: %d %s", rec.Code, rec.Body)
	}

	ck := responseCookie(rec, testCookies.name)
	if ck == nil {
		t.Fatalf("Set-Cookie bekleniyordu: %v", rec.Header().Values("Set-Cookie"))
	}
	if ck.Value != "gizli-refresh" {
		t.Errorf("çerez değeri: istenen %q, gelen %q", "gizli-refresh", ck.Value)
	}
	if !ck.HttpOnly {
		t.Error("HttpOnly şart: Faz 13'ün amacı JS'in bu değeri GÖREMEMESİ")
	}
	if !ck.Secure {
		t.Error("COOKIE_SECURE=true iken Secure şart")
	}
	if ck.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite=Strict şart (CSRF'e karşı ikinci katman), gelen %v", ck.SameSite)
	}
	// Beklenen Path SABİTTEN DEĞİL düz metinden gelir: refreshCookiePath ile karşılaştırsaydık
	// sabiti "/" yapan bir değişiklik beklentiyi de değiştirir ve test yeşil kalırdı. Oysa
	// kapsamın genişlemesi, 30 güne kadar yaşayan sırrın /api/books dahil HER isteğe eklenmesi
	// demek (ayrıca config'in "__Host-" reddi yalnız Path != "/" iken doğru).
	if ck.Path != "/api/auth" {
		t.Errorf("Path: istenen %q, gelen %q", "/api/auth", ck.Path)
	}
	if !ck.Expires.Equal(exp) {
		t.Errorf("Expires oturumun (tavanlı) bitişi olmalı: istenen %s, gelen %s", exp, ck.Expires)
	}

	// Gövde: refresh_token alanı ARTIK YOK, refresh_expires_at duruyor (sır değil).
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("cevap JSON değil: %v", err)
	}
	if _, ok := body["refresh_token"]; ok {
		t.Errorf("refresh_token cevap gövdesinde OLMAMALI: %s", rec.Body)
	}
	if _, ok := body["refresh_expires_at"]; !ok {
		t.Errorf("refresh_expires_at cevapta kalmalı: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "gizli-refresh") {
		t.Errorf("refresh token değeri gövdeye hiçbir alanda sızmamalı: %s", rec.Body)
	}
}

// TestHandleLoginRevokesOldCookie: istekte hâlâ eski bir refresh çerezi varsa, yeni giriş
// onu EZER → o token bir daha hiçbir istemciye ulaşmaz ama DB'de canlı kalırdı (yetim oturum).
// Giriş, eskisinin ailesini en iyi çabayla iptal eder; iptal patlarsa giriş YİNE başarılıdır.
func TestHandleLoginRevokesOldCookie(t *testing.T) {
	const body = `{"email":"ali@example.com","password":"x"}`
	sess := auth.Session{AccessToken: "acc", RefreshToken: "yeni-refresh", User: sampleUser}

	t.Run("eski çerezin ailesi iptal edilir", func(t *testing.T) {
		svc := &fakeAuth{session: sess}
		rec := doAuth(t, handleLogin(svc, testCookies), http.MethodPost, "/api/auth/login", body, refreshCookie("eski-refresh"))
		if rec.Code != http.StatusOK {
			t.Fatalf("200 bekleniyordu: %d %s", rec.Code, rec.Body)
		}
		if svc.gotToken != "eski-refresh" {
			t.Errorf("Logout eski çerez değeriyle çağrılmalıydı, gelen %q", svc.gotToken)
		}
		if ck := responseCookie(rec, testCookies.name); ck == nil || ck.Value != "yeni-refresh" {
			t.Errorf("cevap YENİ çerezi taşımalı: %v", ck)
		}
	})
	t.Run("çerezsiz girişte iptal denenmez", func(t *testing.T) {
		svc := &fakeAuth{session: sess}
		if rec := do(t, handleLogin(svc, testCookies), http.MethodPost, "/api/auth/login", body); rec.Code != http.StatusOK {
			t.Fatalf("200 bekleniyordu: %d %s", rec.Code, rec.Body)
		}
		if svc.gotToken != "" {
			t.Errorf("Logout çağrılmamalıydı, gelen %q", svc.gotToken)
		}
	})
	// Yanlış parola başkasının oturumunu ÖLDÜREMEMELİ: iptal yalnız başarılı girişte.
	t.Run("başarısız girişte iptal denenmez", func(t *testing.T) {
		svc := &fakeAuth{err: auth.ErrInvalidCredentials}
		rec := doAuth(t, handleLogin(svc, testCookies), http.MethodPost, "/api/auth/login", body, refreshCookie("eski-refresh"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("401 bekleniyordu: %d %s", rec.Code, rec.Body)
		}
		if svc.gotToken != "" {
			t.Errorf("Logout çağrılmamalıydı, gelen %q", svc.gotToken)
		}
		if ck := responseCookie(rec, testCookies.name); ck != nil {
			t.Errorf("başarısız giriş çerez YAZMAMALI: %v", ck)
		}
	})
	// EN İYİ ÇABA'nın sınandığı yer burası: iptal PATLASA da giriş başarılıdır. Bu dala
	// kazara bir return/writeInternalError girseydi (ör. "hatayı yutma, bildir" refleksiyle),
	// DB'nin bir anlık sarsıntısı, tarayıcısında eski bir çerez taşıyan HERKESİ girişten
	// men ederdi — üstelik mevcut alt testlerin (iptal tutar / çerez yok / giriş başarısız)
	// hiçbiri kırılmadan. Eski çerez kullanıcıya ait bile olmayabilir; onu iptal edememek
	// girişi reddetmek için bir gerekçe değil.
	t.Run("iptal patlasa da giriş başarılı: 200 + yeni çerez", func(t *testing.T) {
		// Hata logoutErrs ile veriliyor, fakeAuth.err ile DEĞİL: err olsaydı aynı hata
		// Login'den de dönerdi ve test, iptalin değil girişin hatasını sınardı.
		svc := &fakeAuth{session: sess, logoutErrs: []error{errors.New("db")}}
		rec := doAuth(t, handleLogin(svc, testCookies), http.MethodPost, "/api/auth/login", body, refreshCookie("eski-refresh"))
		if rec.Code != http.StatusOK {
			t.Fatalf("iptal patlasa da 200 bekleniyordu: %d %s", rec.Code, rec.Body)
		}
		if svc.logoutCalls != 1 {
			t.Errorf("iptal bir kez denenmeliydi, %d kez denendi", svc.logoutCalls)
		}
		// Asıl kırılma noktası: hata dalından erken dönülseydi Set-Cookie hiç yazılmazdı ve
		// kullanıcı 200 alsa bile bir sonraki yenilemede oturumunu kaybederdi.
		if ck := responseCookie(rec, testCookies.name); ck == nil || ck.Value != "yeni-refresh" {
			t.Errorf("cevap YENİ refresh çerezini taşımalı: %v", ck)
		}
	})
}

// TestReadRefreshCookieRejections: servise HİÇ gitmeden 401 verilen çerez biçimleri.
// Hepsinde cevap silme çerezi taşır (tarayıcıdaki bozuk çerez her açılışta aynı 401'i
// üretmesin) ve DB'ye dokunulmaz.
//
// Eski TestReadRefreshTokenTooLong'un yerini alır: uzunluk sınırı artık gövdedeki alanda
// değil çerezde uygulanıyor ve cevabı 400 validation_error değil 401 invalid_token —
// çerez, istemcinin "düzeltebileceği" bir girdi değil, kimliğin kendisi.
func TestReadRefreshCookieRejections(t *testing.T) {
	tests := []struct {
		name    string
		cookies []*http.Cookie
	}{
		{name: "çerez yok"},
		{name: "boş değer", cookies: []*http.Cookie{refreshCookie("")}},
		// 257 = sınırın (256) bir fazlası, SABİTLE DEĞİL SAYIYLA yazılı: maxRefreshTokenLen+1
		// yazsaydık sabiti 1 MB'a çıkaran bir değişiklikte beklenti de büyür, test yeşil kalırdı.
		{name: "257 karakter", cookies: []*http.Cookie{refreshCookie(strings.Repeat("a", 257))}},
		// Gölgeleme (cookie shadowing): saldırgan ikinci bir çerez yazdırır, tarayıcı ikisini
		// de gönderir ve sunucu hangisinin gerçek olduğunu ayırt EDEMEZ → tahmin etme, reddet.
		{name: "aynı adlı iki çerez", cookies: []*http.Cookie{refreshCookie("gercek"), refreshCookie("sahte")}},
		// Farklı ada sahip bir çerez bizimkini "doldurmaz".
		{name: "başka adlı çerez", cookies: []*http.Cookie{{Name: "baska", Value: "x"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAuth{session: auth.Session{AccessToken: "acc", User: sampleUser}}
			rec := doAuth(t, handleRefresh(svc, testCookies), http.MethodPost, "/", "", tt.cookies...)
			if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "invalid_token" {
				t.Fatalf("401 invalid_token bekleniyordu: %d %s", rec.Code, rec.Body)
			}
			if svc.gotToken != "" {
				t.Errorf("servis çağrılmamalıydı, iletilen token %q (uzunluk %d)", svc.gotToken, len(svc.gotToken))
			}
			assertClearingCookie(t, rec, testCookies)
		})
	}

	// Sınırın tam üstü hâlâ geçerli: 256 karakter kabul edilir (bizimki 26, pay bol).
	t.Run("256 karakter geçerli", func(t *testing.T) {
		svc := &fakeAuth{session: auth.Session{AccessToken: "acc", User: sampleUser}}
		token := strings.Repeat("a", 256)
		rec := doAuth(t, handleRefresh(svc, testCookies), http.MethodPost, "/", "", refreshCookie(token))
		if rec.Code != http.StatusOK || svc.gotToken != token {
			t.Errorf("200 bekleniyordu: %d (iletilen uzunluk %d)", rec.Code, len(svc.gotToken))
		}
	})
}

// TestDuplicateRefreshCookieWarns: gölgeleme girişimi operatöre görünür olmalı (WARN),
// ama çerez DEĞERLERİ loga ASLA düşmez.
func TestDuplicateRefreshCookieWarns(t *testing.T) {
	buf := captureLogs(t)
	svc := &fakeAuth{}
	rec := doAuth(t, newRouter(deps{auth: svc, cookies: testCookies}), http.MethodPost, "/api/auth/refresh", "",
		refreshCookie("gercek-deger"), refreshCookie("sahte-deger"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("401 bekleniyordu: %d %s", rec.Code, rec.Body)
	}
	raw := buf.String()
	var warn map[string]any
	for _, l := range logLines(t, buf) {
		if l["level"] == "WARN" {
			warn = l
		}
	}
	if warn == nil {
		t.Fatalf("çoklu çerez WARN üretmeli: %s", raw)
	}
	if warn["cookie_count"] != float64(2) || warn["request_id"] != rec.Header().Get(requestIDHeader) {
		t.Errorf("WARN sayıyı ve request_id'yi taşımalı: %v", warn)
	}
	if strings.Contains(raw, "gercek-deger") || strings.Contains(raw, "sahte-deger") {
		t.Errorf("çerez değerleri loga düşmemeli: %s", raw)
	}
}

// TestRefreshCookieSettings: COOKIE_NAME/COOKIE_SECURE gerçekten uygulanıyor mu.
// (Ad ayarlı olmasaydı aynı makinedeki compose ve kind yığınları tek "localhost" çerez
// kavanozunu paylaşıp birbirinin oturumunu ezerdi — çerezler PORT'a göre kapsanmaz.)
func TestRefreshCookieSettings(t *testing.T) {
	t.Run("COOKIE_NAME hem yazmada hem okumada kullanılır", func(t *testing.T) {
		cookies := cookieSettings{name: "bookhub_refresh_k8s", secure: true}
		svc := &fakeAuth{session: auth.Session{AccessToken: "acc", RefreshToken: "r1", User: sampleUser}}

		// Varsayılan adla gelen çerez artık "bizim" çerezimiz DEĞİL → 401.
		rec := doAuth(t, handleRefresh(svc, cookies), http.MethodPost, "/", "", &http.Cookie{Name: "bookhub_refresh", Value: "eski"})
		if rec.Code != http.StatusUnauthorized || svc.gotToken != "" {
			t.Errorf("başka adlı çerez okunmamalı: %d (token %q)", rec.Code, svc.gotToken)
		}
		if ck := responseCookie(rec, "bookhub_refresh_k8s"); ck == nil {
			t.Errorf("silme çerezi YAPILANDIRILMIŞ adla yazılmalı: %v", rec.Header().Values("Set-Cookie"))
		}

		rec = doAuth(t, handleRefresh(svc, cookies), http.MethodPost, "/", "", &http.Cookie{Name: "bookhub_refresh_k8s", Value: "dogru"})
		if rec.Code != http.StatusOK || svc.gotToken != "dogru" {
			t.Fatalf("yapılandırılmış adlı çerez okunmalı: %d (token %q)", rec.Code, svc.gotToken)
		}
		if ck := responseCookie(rec, "bookhub_refresh_k8s"); ck == nil || ck.Value != "r1" {
			t.Errorf("yeni çerez yapılandırılmış adla yazılmalı: %v", ck)
		}
	})

	// COOKIE_SECURE=false YALNIZ lokal içindir (ör. Safari'de düz http denemesi): Secure
	// özniteliği olmayan bir çerez, HTTPS olmayan bir bağlantıda düz metin gider.
	t.Run("COOKIE_SECURE=false → Secure yok (silme çerezinde de)", func(t *testing.T) {
		cookies := cookieSettings{name: "bookhub_refresh", secure: false}
		svc := &fakeAuth{session: auth.Session{AccessToken: "acc", RefreshToken: "r1", User: sampleUser}}

		rec := do(t, handleLogin(svc, cookies), http.MethodPost, "/api/auth/login", `{"email":"a@b.co","password":"x"}`)
		ck := responseCookie(rec, cookies.name)
		if ck == nil || ck.Secure {
			t.Errorf("Secure olmamalı: %v", ck)
		}
		if ck != nil && !ck.HttpOnly {
			t.Error("Secure kapalıyken bile HttpOnly durmalı")
		}
		assertClearingCookie(t, do(t, handleLogout(svc, cookies), http.MethodPost, "/", ""), cookies)
	})
}

// TestRouterWiresCookieSettings: config'ten gelen deps.cookies GERÇEKTEN Set-Cookie'ye
// ulaşıyor mu. NEDEN AYRI TEST: diğer çerez testleri handler'ı DOĞRUDAN çağırıyor; router
// alanı yolda düşürse (ya da boş adı varsayılana çeken yedek kural yüzünden sessizce
// "sağlıklı" görünse) suite yemyeşil kalır, üretim ise Secure'suz bir refresh çerezi yazardı
// — 30 güne kadar yaşayan bir sır, düz http isteğinde açıktan giderdi.
func TestRouterWiresCookieSettings(t *testing.T) {
	tests := []cookieSettings{
		{name: "bookhub_refresh_k8s", secure: false}, // kind ConfigMap + düz http
		{name: "bookhub_refresh", secure: true},      // üretim varsayılanı
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			svc := &fakeAuth{session: auth.Session{AccessToken: "acc", RefreshToken: "r1", User: sampleUser}}
			h := newRouter(deps{auth: svc, cookies: c})

			rec := doAuth(t, h, http.MethodPost, "/api/auth/login", `{"email":"a@b.co","password":"x"}`)
			ck := responseCookie(rec, c.name)
			if ck == nil {
				t.Fatalf("router yapılandırılmış adla çerez yazmalı: %v", rec.Header().Values("Set-Cookie"))
			}
			// Path yine sabitle değil değeriyle sınanır (bkz. TestHandleLoginSetsRefreshCookie).
			if ck.Secure != c.secure || !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode || ck.Path != "/api/auth" {
				t.Errorf("router çerez ayarlarını olduğu gibi geçirmeli: %q", ck)
			}

			// Okuma tarafı da aynı adı kullanmalı (yoksa yenileme hep 401 olurdu).
			rec = doAuth(t, h, http.MethodPost, "/api/auth/refresh", "", &http.Cookie{Name: c.name, Value: "tok"})
			if rec.Code != http.StatusOK || svc.gotToken != "tok" {
				t.Fatalf("yapılandırılmış adlı çerez okunmalı: %d (token %q)", rec.Code, svc.gotToken)
			}
			if ck := responseCookie(rec, c.name); ck == nil || ck.Secure != c.secure {
				t.Errorf("yenilenen çerez de aynı Secure değerini taşımalı: %v", ck)
			}
		})
	}
}

// TestCookieSettingsFromConfig: config → cookieSettings eşlemesi. main.go'yu hiçbir test
// çalıştırmıyor; bu iki satırlık eşleme yanlış bağlanırsa (ör. secure alanı hiç geçirilmezse)
// tek uyarı, üretimde Secure'suz yazılan çerez olurdu.
func TestCookieSettingsFromConfig(t *testing.T) {
	got := cookieSettingsFrom(config.Config{CookieName: "bookhub_refresh_k8s", CookieSecure: false})
	if got.name != "bookhub_refresh_k8s" || got.secure {
		t.Errorf("config alanları olduğu gibi taşınmalı: %+v", got)
	}
	if got = cookieSettingsFrom(config.Config{CookieName: "x", CookieSecure: true}); got.name != "x" || !got.secure {
		t.Errorf("config alanları olduğu gibi taşınmalı: %+v", got)
	}
}

// TestHandleRefreshReuse (Faz 13): servis *auth.ReuseError dönerse (harcanmış bir refresh
// token yeniden geldi, aile iptal edildi) istemci düz "geçersiz token" ile BİREBİR aynı cevabı
// alır — aynı status, aynı gövde; farklı olsaydı saldırgana "bu token'ı biri zaten kullanmış"
// diyen bir kâhin olurdu. Operatör ise user_id/family_id/request_id taşıyan bir WARN görür;
// token'ın kendisi loga ASLA düşmez. Düz geçersiz token WARN üretmez (gürültü olmasın).
func TestHandleRefreshReuse(t *testing.T) {
	const token = "calinmis-refresh-token-degeri"
	send := func(err error) *httptest.ResponseRecorder {
		t.Helper()
		// Gerçek router: X-Request-Id'yi RequestID + requestLogger zinciri koyar.
		return doAuth(t, newRouter(deps{auth: &fakeAuth{err: err}, cookies: testCookies}), http.MethodPost, "/api/auth/refresh", "", refreshCookie(token))
	}
	warnLines := func(buf *bytes.Buffer) []map[string]any {
		t.Helper()
		var out []map[string]any
		for _, l := range logLines(t, buf) {
			if l["level"] == "WARN" {
				out = append(out, l)
			}
		}
		return out
	}

	buf := captureLogs(t)
	plain := send(auth.ErrInvalidToken)
	if w := warnLines(buf); len(w) != 0 {
		t.Errorf("düz geçersiz token WARN üretmemeli: %v", w)
	}

	buf.Reset()
	reused := send(&auth.ReuseError{UserID: 7, FamilyID: 42})
	if reused.Code != http.StatusUnauthorized || errorCode(t, reused) != "invalid_token" {
		t.Fatalf("reuse → 401 invalid_token bekleniyordu: %d %s", reused.Code, reused.Body)
	}
	if reused.Body.String() != plain.Body.String() {
		t.Errorf("reuse cevabı düz geçersiz token'la aynı olmalı (kâhin yok):\n reuse: %s\n düz:   %s", reused.Body, plain.Body)
	}

	// Ham logun anlık görüntüsü ayrıştırmadan ÖNCE alınır; aşağıdaki "loga düşmedi" iddiaları
	// bu kopyaya bakar (tamponu okuyan bir yardımcıdan etkilenmesin).
	raw := buf.String()
	w := warnLines(buf)
	if len(w) != 1 {
		t.Fatalf("tam bir WARN satırı bekleniyordu, gelen %d: %s", len(w), raw)
	}
	id := reused.Header().Get(requestIDHeader)
	if id == "" || w[0]["request_id"] != id {
		t.Errorf("WARN, cevabın request_id'sini taşımalı: başlık=%q log=%v", id, w[0]["request_id"])
	}
	if w[0]["user_id"] != float64(7) || w[0]["family_id"] != float64(42) {
		t.Errorf("WARN kullanıcı ve aileyi taşımalı: %v", w[0])
	}
	if strings.Contains(raw, token) {
		t.Errorf("refresh token loga düşmemeli: %s", raw)
	}
	// Token'ın özeti (DB'deki token_hash) de loga düşmemeli — hangi kodlamayla yazılırsa yazılsın.
	sum := sha256.Sum256([]byte(token))
	for _, enc := range []string{
		hex.EncodeToString(sum[:]),
		base64.StdEncoding.EncodeToString(sum[:]),
		base64.RawURLEncoding.EncodeToString(sum[:]),
	} {
		if strings.Contains(raw, enc) {
			t.Errorf("refresh token özeti loga düşmemeli (%s): %s", enc, raw)
		}
	}
}

// TestMeThroughRouter: /api/auth/me gerçek router + requireAuth zinciriyle.
func TestMeThroughRouter(t *testing.T) {
	tests := []struct {
		name       string
		header     string // ham Authorization başlığı
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "başlık yok → 401", wantStatus: http.StatusUnauthorized, wantCode: "missing_token"},
		{name: "Basic şema → 401", header: "Basic YWxpOjEyMw==", wantStatus: http.StatusUnauthorized, wantCode: "missing_token"},
		{name: "boş Bearer → 401", header: "Bearer ", wantStatus: http.StatusUnauthorized, wantCode: "missing_token"},
		{name: "geçersiz token → 401", header: "Bearer sahte", wantStatus: http.StatusUnauthorized, wantCode: "invalid_token"},
		{name: "geçerli (küçük harf şema) → 200", header: "bearer good-user", wantStatus: http.StatusOK},
		{name: "kullanıcı silinmiş → 401", header: "Bearer good-user", svcErr: auth.ErrUserNotFound,
			wantStatus: http.StatusUnauthorized, wantCode: "invalid_token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAuth{user: sampleUser, err: tt.svcErr}
			h := newRouter(deps{auth: svc, tokens: fakeVerifier{}})
			req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantStatus == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("401 cevabı WWW-Authenticate başlığı taşımalı (RFC 6750)")
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Errorf("code: istenen %q, gelen %q", tt.wantCode, got)
				}
				return
			}
			if svc.gotUserID != 7 {
				t.Errorf("servis token'daki kullanıcıyla çağrılmalı, gelen %d", svc.gotUserID)
			}
			assertUserKeys(t, rec.Body.Bytes())
		})
	}
}

// TestHandleRefreshUnexpectedError: beklenmeyen servis hatası → 500 internal; hatanın
// ham metni cevap gövdesine SIZMAZ.
func TestHandleRefreshUnexpectedError(t *testing.T) {
	rec := doAuth(t, handleRefresh(&fakeAuth{err: errors.New("pq: gizli-sürücü-detayı")}, testCookies), http.MethodPost, "/", "", refreshCookie("x"))
	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "internal" {
		t.Fatalf("500 internal bekleniyordu: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "gizli-sürücü-detayı") {
		t.Errorf("cevap gövdesi iç hata detayını sızdırıyor: %s", rec.Body)
	}
	// 500'de çerez SİLİNMEZ: DB'nin geçici arızası, hâlâ geçerli olabilecek bir oturumu
	// kullanıcının tarayıcısından atmak için yeterli bir sebep değil.
	if ck := responseCookie(rec, testCookies.name); ck != nil {
		t.Errorf("beklenmeyen hatada çerez silinmemeli: %v", ck)
	}
}

// TestHandleMeUnexpectedError: Me'den beklenmeyen hata → 500 internal.
func TestHandleMeUnexpectedError(t *testing.T) {
	svc := &fakeAuth{err: errors.New("db down")}
	h := newRouter(deps{auth: svc, tokens: fakeVerifier{}})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer good-user")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "internal" {
		t.Errorf("500 internal bekleniyordu: %d %s", rec.Code, rec.Body)
	}
}

// TestHandleMeWithoutPrincipal: handleMe requireAuth'suz çağrılırsa (context'te principal
// yok) kapalı başarısız olur: 401 missing_token + WWW-Authenticate, servis çağrılmaz.
func TestHandleMeWithoutPrincipal(t *testing.T) {
	svc := &fakeAuth{user: sampleUser}
	rec := do(t, handleMe(svc), http.MethodGet, "/", "")

	if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "missing_token" {
		t.Fatalf("401 missing_token bekleniyordu: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("401 cevabı WWW-Authenticate başlığı taşımalı")
	}
	if svc.gotUserID != 0 {
		t.Errorf("servis çağrılmamalıydı, gelen userID %d", svc.gotUserID)
	}
}

// TestHandleChangePassword: uç, servisin her hata türünü DOĞRU HTTP koduna çevirmeli.
// En kritik satır "yanlış mevcut parola → 403": 401 verilseydi istemci (client.ts) onu
// "access token bitti" sayıp /refresh'e gider, isteği tekrarlar ve kullanıcı gerçek nedeni
// (parolayı yanlış yazdı) hiç görmezdi.
func TestHandleChangePassword(t *testing.T) {
	const valid = `{"current_password":"eski-parola","new_password":"yeni-parola"}`
	tests := []struct {
		name       string
		header     string
		body       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "kimliksiz → 401 (servise hiç gitmez)", body: valid,
			wantStatus: http.StatusUnauthorized, wantCode: "missing_token"},
		{name: "yanlış mevcut parola → 403", header: "Bearer good-user", body: valid, svcErr: auth.ErrWrongPassword,
			wantStatus: http.StatusForbidden, wantCode: "invalid_current_password"},
		{name: "yeni parola politikaya uymuyor → 400", header: "Bearer good-user", body: valid,
			svcErr:     &auth.ValidationError{Problems: []string{"password en az 8 karakter olmalı"}},
			wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "hesap silinmiş → 401", header: "Bearer good-user", body: valid, svcErr: auth.ErrUserNotFound,
			wantStatus: http.StatusUnauthorized, wantCode: "invalid_token"},
		{name: "beklenmeyen hata → 500", header: "Bearer good-user", body: valid, svcErr: errors.New("db çöktü"),
			wantStatus: http.StatusInternalServerError, wantCode: "internal"},
		{name: "mevcut parola boş → 400", header: "Bearer good-user", body: `{"current_password":"","new_password":"yeni-parola"}`,
			wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "yeni parola boş → 400", header: "Bearer good-user", body: `{"current_password":"eski-parola","new_password":""}`,
			wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		// Bilinmeyen alan: readJSON'ın DisallowUnknownFields'i. "user_id" gibi bir alanla
		// başkasının parolasını değiştirmeye çalışan istek daha gövdede ölür (mass-assignment).
		{name: "bilinmeyen alan → 400", header: "Bearer good-user",
			body:       `{"current_password":"eski-parola","new_password":"yeni-parola","user_id":1}`,
			wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAuth{user: sampleUser, err: tt.svcErr}
			h := newRouter(deps{auth: svc, tokens: fakeVerifier{}, cookies: testCookies})
			req := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(tt.body))
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Errorf("code: istenen %q, gelen %q", tt.wantCode, got)
				}
			}
			// Başarısız hiçbir yol çerez YAZMAMALI: yazsaydı, parolası değişmemiş bir
			// kullanıcının tarayıcısına sahte bir oturum çerezi konurdu.
			if ck := responseCookie(rec, testCookies.name); ck != nil {
				t.Errorf("hata yolunda Set-Cookie olmamalı: %q", ck)
			}
			if tt.header == "" && svc.gotNewPassword != "" {
				t.Error("kimliksiz istek servise ulaşmamalı")
			}
		})
	}
}

// TestHandleChangePasswordSuccess: başarıda 200 + YENİ refresh çerezi + gövdede sır YOK,
// ve servis token'daki kullanıcıyla (gövdeden gelen bir id'yle DEĞİL) çağrılır.
func TestHandleChangePasswordSuccess(t *testing.T) {
	svc := &fakeAuth{
		user: sampleUser,
		session: auth.Session{
			AccessToken: "acc", RefreshToken: "yeni-refresh", User: sampleUser,
			RefreshExpiresAt: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC),
		},
	}
	h := newRouter(deps{auth: svc, tokens: fakeVerifier{}, cookies: testCookies})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		strings.NewReader(`{"current_password":"eski-parola","new_password":"yeni-parola"}`))
	req.Header.Set("Authorization", "Bearer good-user")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: istenen 200, gelen %d (%s)", rec.Code, rec.Body)
	}
	if svc.gotUserID != 7 || svc.gotPassword != "eski-parola" || svc.gotNewPassword != "yeni-parola" {
		t.Errorf("servis çağrısı: gelen (id=%d, current=%q, next=%q)", svc.gotUserID, svc.gotPassword, svc.gotNewPassword)
	}
	// Parola değişimi kullanıcının TÜM ailelerini öldürüyor — bu cihazınki dahil. Yeni çerez
	// yazılmazsa tarayıcıda ölü bir çerez kalır ve ilk yenilemede oturum düşerdi.
	ck := responseCookie(rec, testCookies.name)
	if ck == nil || ck.Value != "yeni-refresh" {
		t.Fatalf("yeni refresh çerezi bekleniyordu, gelen %v (Set-Cookie: %v)", ck, rec.Header().Values("Set-Cookie"))
	}
	// Öznitelikler login'dekiyle aynı olmalı (değerler sabitlerle değil elle yazılır: totoloji olmasın).
	if ck.Path != "/api/auth" || !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode || !ck.Secure {
		t.Errorf("çerez öznitelikleri login'deki ile aynı olmalı: %q", ck)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("cevap JSON değil: %v", err)
	}
	if _, ok := body["refresh_token"]; ok {
		t.Errorf("refresh_token cevap gövdesinde OLMAMALI: %s", rec.Body)
	}
	if body["access_token"] != "acc" {
		t.Errorf("access_token cevapta olmalı: %s", rec.Body)
	}
}

// TestChangePasswordRateLimitIsSeparate: parola değiştirme kotası login/register'dan AYRI.
// NEDEN: çalınmış bir access token'la mevcut parolayı kaba kuvvetle deneyen biri, kurbanın
// GİRİŞ kotasını da tüketip onu kendi hesabından dışarıda bırakabilirdi (ve tersi).
func TestChangePasswordRateLimitIsSeparate(t *testing.T) {
	svc := &fakeAuth{user: sampleUser, err: auth.ErrWrongPassword}
	h := newRouter(deps{auth: svc, tokens: fakeVerifier{}, cookies: testCookies, limiter: ratelimit.NewMemory(1, time.Minute)})
	post := func(path, body, header string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.RemoteAddr = "10.0.0.9:51234"
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	const pwBody = `{"current_password":"eski-parola","new_password":"yeni-parola"}`

	if got := post("/api/auth/password", pwBody, "Bearer good-user"); got != http.StatusForbidden {
		t.Fatalf("ilk parola denemesi sınır içinde (403 beklenir): %d", got)
	}
	if got := post("/api/auth/password", pwBody, "Bearer good-user"); got != http.StatusTooManyRequests {
		t.Fatalf("ikinci parola denemesi 429 olmalı: %d", got)
	}
	// Parola kovası dolu ama LOGIN kovası dokunulmamış olmalı.
	if got := post("/api/auth/login", `{"email":"a@b.co","password":"x"}`, ""); got == http.StatusTooManyRequests {
		t.Error("login kotası parola denemelerinden etkilenmemeli")
	}
}

// TestChangePasswordAuthBeforeRateLimit: /api/auth/password'da middleware SIRASI —
// önce requireAuth, SONRA rateLimit("password").
//
// NEDEN ÖNEMLİ: sıra ters olsaydı, kimliği hiç doğrulanmamış istekler de kotayı yerdi.
// O zaman bir saldırgan tek bir token'a sahip olmadan, sadece boş POST'lar atarak
// kurbanın parola değiştirme hakkını tüketebilirdi (kendi hesabına erişimi olan birinin
// parolasını değiştirmesini engellemek = hesabı kurtarmasını engellemek).
func TestChangePasswordAuthBeforeRateLimit(t *testing.T) {
	svc := &fakeAuth{user: sampleUser, err: auth.ErrWrongPassword}
	// Kota 1: tek bir sayılan istek kovayı bitirir, böylece "sayıldı mı" sorusu net görünür.
	h := newRouter(deps{auth: svc, tokens: fakeVerifier{}, cookies: testCookies, limiter: ratelimit.NewMemory(1, time.Minute)})
	post := func(header string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
			strings.NewReader(`{"current_password":"eski-parola","new_password":"yeni-parola"}`))
		req.RemoteAddr = "10.0.0.7:51234"
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// Kimliksiz istekler: 401 ve kovaya DOKUNMAMALI.
	for i := 1; i <= 3; i++ {
		if got := post(""); got != http.StatusUnauthorized {
			t.Fatalf("%d. kimliksiz istek: istenen 401, gelen %d", i, got)
		}
	}
	// Kimlikli istek hâlâ sınır içinde olmalı — üstteki üç 401 kotayı yemediyse.
	if got := post("Bearer good-user"); got != http.StatusForbidden {
		t.Fatalf("kimlikli ilk istek sınır içinde olmalıydı (403 yanlış parola): gelen %d", got)
	}
	// Ve kota GERÇEKTEN çalışıyor: kimlikli ikinci istek 429. (Bu satır olmadan test,
	// rateLimit hiç bağlı olmasa da geçerdi.)
	if got := post("Bearer good-user"); got != http.StatusTooManyRequests {
		t.Fatalf("kimlikli ikinci istek 429 olmalıydı: gelen %d", got)
	}
}
