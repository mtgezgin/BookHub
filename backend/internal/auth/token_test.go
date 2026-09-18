package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret-that-is-at-least-32-bytes!")

// frozen, saati sabit bir TokenManager üretir: süre testleri gerçek zamana bağlı kalmasın.
func frozen(t *testing.T, at time.Time) *TokenManager {
	t.Helper()
	m, err := NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	m.now = func() time.Time { return at }
	return m
}

// sign, testte ELLE token üretir — saldırganın yapabileceği gibi başlığı/gövdeyi
// istediğimiz gibi kurmak için.
func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("test token'ı imzalanamadı: %v", err)
	}
	return s
}

func validClaims(now time.Time) accessClaims {
	return accessClaims{
		Role: RoleUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "7",
			Issuer:    tokenIssuer,
			Audience:  jwt.ClaimStrings{tokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	}
}

func TestNewTokenManagerRejectsWeakConfig(t *testing.T) {
	if _, err := NewTokenManager([]byte("kisa"), time.Minute); err == nil {
		t.Error("32 bayttan kısa secret kabul edildi")
	}
	if _, err := NewTokenManager(testSecret, 0); err == nil {
		t.Error("sıfır süre kabul edildi")
	}
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	m := frozen(t, now)

	tok, exp, err := m.Issue(42, RoleAdmin)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if want := now.Add(15 * time.Minute); !exp.Equal(want) {
		t.Errorf("bitiş: istenen %v, gelen %v", want, exp)
	}

	p, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if p.UserID != 42 || p.Role != RoleAdmin || !p.IsAdmin() {
		t.Errorf("principal yanlış: %+v", p)
	}
}

// TestVerifyRejects, doğrulayıcının reddetmesi gereken her token biçimi. Her satır bir
// saldırı ya da bozulma senaryosu; hepsi AYNI hatayı (ErrInvalidToken) vermeli.
func TestVerifyRejects(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	m := frozen(t, now)

	good, _, err := m.Issue(7, RoleUser)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	tests := []struct {
		name  string
		token func() string
	}{
		{"boş", func() string { return "" }},
		{"çöp", func() string { return "bu.bir.token-degil" }},
		{"süresi dolmuş", func() string {
			// Aynı token'ı 16 dk sonrasının saatiyle doğrulamak yerine geçmişte üret.
			old := frozen(t, now.Add(-time.Hour))
			tok, _, _ := old.Issue(7, RoleUser)
			return tok
		}},
		{"başka secret ile imzalı", func() string {
			return sign(t, jwt.SigningMethodHS256, []byte("baska-bir-secret-ama-o-da-32-bayt!!"), validClaims(now))
		}},
		// alg=none: imzasız token. Doğrulayıcı başlığa güvenseydi imzayı hiç kontrol etmezdi.
		{"alg=none", func() string {
			return sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims(now))
		}},
		// Algoritma karışıklığı: AYNI secret, farklı HMAC. WithValidMethods olmasa bu
		// token geçerdi — keyfunc algoritmaya bakmadan secret'ı döndürüyor.
		{"HS512 (sabitlenmemiş algoritma)", func() string {
			return sign(t, jwt.SigningMethodHS512, testSecret, validClaims(now))
		}},
		{"yanlış issuer", func() string {
			c := validClaims(now)
			c.Issuer = "kotu-servis"
			return sign(t, jwt.SigningMethodHS256, testSecret, c)
		}},
		{"yanlış audience", func() string {
			c := validClaims(now)
			c.Audience = jwt.ClaimStrings{"baska-api"}
			return sign(t, jwt.SigningMethodHS256, testSecret, c)
		}},
		{"exp yok", func() string {
			c := validClaims(now)
			c.ExpiresAt = nil
			return sign(t, jwt.SigningMethodHS256, testSecret, c)
		}},
		{"sayısal olmayan sub", func() string {
			c := validClaims(now)
			c.Subject = "admin"
			return sign(t, jwt.SigningMethodHS256, testSecret, c)
		}},
		{"bilinmeyen rol", func() string {
			c := validClaims(now)
			c.Role = "ROOT"
			return sign(t, jwt.SigningMethodHS256, testSecret, c)
		}},
		// Gövdeyle oynanmış token: imza artık içerikle eşleşmez.
		{"gövdesi değiştirilmiş", func() string {
			parts := strings.Split(good, ".")
			forged := sign(t, jwt.SigningMethodHS256, []byte("saldirganin-kendi-secreti-32-bayt!!"), func() accessClaims {
				c := validClaims(now)
				c.Role = RoleAdmin
				return c
			}())
			return parts[0] + "." + strings.Split(forged, ".")[1] + "." + parts[2]
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.Verify(tt.token())
			if !errors.Is(err, ErrInvalidToken) {
				t.Errorf("istenen ErrInvalidToken, gelen %v", err)
			}
		})
	}
}
