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
)

// fakeAuth, authService'in sahtesi: dönecek değerler + son çağrının argümanları.
type fakeAuth struct {
	user    auth.User
	session auth.Session
	err     error

	gotEmail, gotPassword, gotToken string
	gotUserID                       int64
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
func (f *fakeAuth) Logout(_ context.Context, token string) error {
	f.gotToken = token
	return f.err
}
func (f *fakeAuth) Me(_ context.Context, id int64) (auth.User, error) {
	f.gotUserID = id
	return f.user, f.err
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
			rec := do(t, handleLogin(svc), http.MethodPost, "/api/auth/login", tt.body)
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
			if got.AccessToken != "acc" || got.RefreshToken != "ref" || got.TokenType != "Bearer" || got.User.ID != 7 {
				t.Errorf("oturum cevabı yanlış: %+v", got)
			}
		})
	}
}

func TestHandleRefreshAndLogout(t *testing.T) {
	t.Run("refresh geçerli → 200, token servise iletilir", func(t *testing.T) {
		svc := &fakeAuth{session: auth.Session{AccessToken: "yeni", User: sampleUser}}
		rec := do(t, handleRefresh(svc), http.MethodPost, "/", `{"refresh_token":"eski"}`)
		if rec.Code != http.StatusOK || svc.gotToken != "eski" {
			t.Errorf("status %d, iletilen token %q", rec.Code, svc.gotToken)
		}
	})
	t.Run("refresh geçersiz → 401", func(t *testing.T) {
		rec := do(t, handleRefresh(&fakeAuth{err: auth.ErrInvalidToken}), http.MethodPost, "/", `{"refresh_token":"x"}`)
		if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "invalid_token" {
			t.Errorf("401 invalid_token bekleniyordu: %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("refresh token yok → 400", func(t *testing.T) {
		rec := do(t, handleRefresh(&fakeAuth{}), http.MethodPost, "/", `{}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("400 bekleniyordu: %d", rec.Code)
		}
	})
	t.Run("logout → 204, gövdesiz", func(t *testing.T) {
		svc := &fakeAuth{}
		rec := do(t, handleLogout(svc), http.MethodPost, "/", `{"refresh_token":"tok"}`)
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || svc.gotToken != "tok" {
			t.Errorf("204 + boş gövde bekleniyordu: %d %q (token %q)", rec.Code, rec.Body, svc.gotToken)
		}
	})
	t.Run("logout servis hatası → 500", func(t *testing.T) {
		rec := do(t, handleLogout(&fakeAuth{err: errors.New("db")}), http.MethodPost, "/", `{"refresh_token":"tok"}`)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("500 bekleniyordu: %d", rec.Code)
		}
	})
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
		return do(t, newRouter(deps{auth: &fakeAuth{err: err}}), http.MethodPost, "/api/auth/refresh", `{"refresh_token":"`+token+`"}`)
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
	rec := do(t, handleRefresh(&fakeAuth{err: errors.New("pq: gizli-sürücü-detayı")}), http.MethodPost, "/", `{"refresh_token":"x"}`)
	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "internal" {
		t.Fatalf("500 internal bekleniyordu: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "gizli-sürücü-detayı") {
		t.Errorf("cevap gövdesi iç hata detayını sızdırıyor: %s", rec.Body)
	}
}

// TestReadRefreshTokenTooLong: 256 karakterden uzun refresh_token → 400 validation_error
// ve servis HİÇ çağrılmaz.
func TestReadRefreshTokenTooLong(t *testing.T) {
	svc := &fakeAuth{}
	body := `{"refresh_token":"` + strings.Repeat("a", 257) + `"}`
	rec := do(t, handleRefresh(svc), http.MethodPost, "/", body)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_error" {
		t.Fatalf("400 validation_error bekleniyordu: %d %s", rec.Code, rec.Body)
	}
	if svc.gotToken != "" {
		t.Errorf("servis çağrılmamalıydı, iletilen token uzunluğu %d", len(svc.gotToken))
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
