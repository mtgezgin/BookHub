package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Roller. DB'deki CHECK (role IN ('USER','ADMIN')) ile aynı küme.
const (
	RoleUser  = "USER"
	RoleAdmin = "ADMIN"
)

// Token'ın kimin tarafından, kimin için üretildiği. Doğrulamada ikisi de ŞART koşulur:
// aynı secret'ı paylaşan başka bir servisin (ör. ileride bir admin aracı) token'ı bu
// API'de geçerli sayılmasın.
const (
	tokenIssuer   = "bookhub"
	tokenAudience = "bookhub-api"
)

// MinSecretLen, HS256 anahtarının alt sınırı (bayt). HMAC-SHA256'nın güvenliği anahtarın
// entropisi kadardır; "secret" gibi kısa bir değer çevrimdışı kaba kuvvetle kırılır.
const MinSecretLen = 32

// ErrInvalidToken, imzası/süresi/iddiaları geçersiz her token için TEK hata. Nedenini
// (süre mi doldu, imza mı tutmadı) istemciye söylemiyoruz; detay sarılı hatada, log'da.
var ErrInvalidToken = errors.New("geçersiz ya da süresi dolmuş token")

// Principal, doğrulanmış bir isteğin "kim" bilgisi: middleware bunu context'e koyar,
// handler'lar oradan okur.
type Principal struct {
	UserID int64
	Role   string
}

// IsAdmin, kaba rol kontrolü. Kaynak SAHİPLİĞİ (bu yorum senin mi?) ayrı bir kontroldür
// ve servis katmanında yapılır — rol "ne yapabilirsin", sahiplik "neye yapabilirsin".
func (p Principal) IsAdmin() bool { return p.Role == RoleAdmin }

// accessClaims, JWT'nin gövdesi: standart alanlar (sub, iss, aud, exp, iat) + rolümüz.
type accessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager, kısa ömürlü access token'ları üretir ve doğrular (HS256).
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time // testlerde saati dondurmak için
}

// NewTokenManager, secret en az MinSecretLen bayt değilse hata döner. Config bunu zaten
// doğruluyor; burada tekrar etmek derinlemesine savunma — paketi başka bir giriş noktası
// (ör. CLI) kullanırsa zayıf anahtar yine içeri giremesin.
func NewTokenManager(secret []byte, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < MinSecretLen {
		return nil, fmt.Errorf("JWT secret en az %d bayt olmalı", MinSecretLen)
	}
	if ttl <= 0 {
		return nil, errors.New("access token süresi pozitif olmalı")
	}
	return &TokenManager{secret: secret, ttl: ttl, now: time.Now}, nil
}

// Issue, kullanıcı için imzalı bir access token ve bitiş zamanını döner.
func (m *TokenManager) Issue(userID int64, role string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.ttl)
	claims := accessClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    tokenIssuer,
			Audience:  jwt.ClaimStrings{tokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token imzalanamadı: %w", err)
	}
	return signed, exp, nil
}

// Verify, token'ı doğrular ve içindeki Principal'ı döner.
//
// Algoritma SABİTLEMESİ (WithValidMethods) bu fonksiyonun en önemli satırı. JWT başlığı
// hangi algoritmayla imzalandığını KENDİSİ söyler ("alg"); başlığa körü körüne güvenen
// bir doğrulayıcı iki klasik saldırıya açıktır:
//   - alg=none: "imzasız" token → imza kontrolü tamamen atlanır.
//   - algoritma karışıklığı: HS256 bekleyen yere HS512/RS256 ile oynanmış token.
//
// Biz "yalnızca HS256" diyoruz; başlık ne derse desin başka algoritma reddedilir.
func (m *TokenManager) Verify(raw string) (Principal, error) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(raw, &claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(tokenAudience),
		jwt.WithExpirationRequired(), // exp'siz token = sonsuza dek geçerli → reddet
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	// İmza geçerli olsa bile içeriği doğrula: bizim ürettiğimiz bir token bu alanları
	// hep doğru taşır; tutmuyorsa ya bir bug ya da secret sızmış demektir.
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id < 1 {
		return Principal{}, fmt.Errorf("%w: geçersiz sub %q", ErrInvalidToken, claims.Subject)
	}
	if claims.Role != RoleUser && claims.Role != RoleAdmin {
		return Principal{}, fmt.Errorf("%w: geçersiz rol %q", ErrInvalidToken, claims.Role)
	}
	return Principal{UserID: id, Role: claims.Role}, nil
}
