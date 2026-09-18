package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// Servis hataları. ErrInvalidCredentials BİLEREK tek: "e-posta yok" ile "parola yanlış"
// aynı cevabı verir — aksi hâlde login formu, hangi e-postaların kayıtlı olduğunu
// sorgulayan bedava bir API'ye dönüşür (user enumeration).
var ErrInvalidCredentials = errors.New("e-posta veya parola hatalı")

// ValidationError, istemcinin düzeltebileceği girdi problemleri (hepsi birden).
//
// Doğrulama NEDEN burada, handler'da değil: parola politikası bir İŞ KURALI. Onu HTTP
// katmanına koysaydık, aynı servisi çağıran ikinci giriş noktası (cmd/manage'in
// create-admin komutu) politikayı atlayabilirdi. Kural servisteyse her kapı onu uygular.
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string { return strings.Join(e.Problems, "; ") }

const (
	maxEmailLen       = 254 // RFC 5321 yol sınırı
	maxDisplayNameLen = 100
)

// User, dışarıya verilen kullanıcı. db.User'dan farkı: PasswordHash alanı YOK.
// Hash'i taşımayan bir tip, onu yanlışlıkla bir JSON cevabına koymayı İMKÂNSIZ kılar —
// "dikkat etmeyi" değil, derleyiciyi güvenceye alıyoruz.
type User struct {
	ID          int64
	Email       string
	DisplayName string
	Role        string
	CreatedAt   time.Time
}

func toUser(u db.User) User {
	return User{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Role: u.Role, CreatedAt: u.CreatedAt}
}

// Session, başarılı bir giriş/yenilemenin çıktısı.
type Session struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             User
}

// store, servisin kalıcılıktan beklediği davranışlar (tüketen tarafta tanımlı →
// testlerde bellek içi sahte takılır). *Repository bunu sağlar.
type store interface {
	CreateUser(ctx context.Context, email, displayName, passwordHash string) (db.User, error)
	UserByEmail(ctx context.Context, email string) (db.User, error)
	UserByID(ctx context.Context, id int64) (db.User, error)
	EnsureAdmin(ctx context.Context, email, displayName, passwordHash string) (db.User, bool, error)
	SaveRefreshToken(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error
	ConsumeRefreshToken(ctx context.Context, tokenHash []byte) (int64, time.Time, error)
	DeleteRefreshToken(ctx context.Context, tokenHash []byte) error
	DeleteExpiredRefreshTokens(ctx context.Context, userID int64) error
}

// Service, projenin İLK servis katmanı. Kitap/tür CRUD'unda handler → repository yeterliydi
// (araya servis koymak boş bir geçiş katmanı olurdu). Auth'ta ise gerçek iş mantığı var:
// parola politikası, zamanlama eşitleme, token üretimi + saklama + rotasyon — bunların
// hiçbiri ne HTTP'nin ne de SQL'in işi.
type Service struct {
	store      store
	tokens     *TokenManager
	refreshTTL time.Duration
	now        func() time.Time

	// Testler için enjekte edilebilir: hashCost testlerde bcrypt.MinCost olur (hız);
	// compare, "kullanıcı yokken de karşılaştırma yapıldı mı" iddiasını ölçmek için.
	hashCost int
	compare  func(hash, password []byte) error
}

// NewService, üretim ayarlarıyla bir Service kurar ve zamanlama-eşitleme hash'ini ısıtır.
func NewService(s store, tokens *TokenManager, refreshTTL time.Duration) *Service {
	dummyHash() // ilk "kullanıcı yok" girişi fazladan 250 ms beklemesin
	return &Service{
		store:      s,
		tokens:     tokens,
		refreshTTL: refreshTTL,
		now:        time.Now,
		hashCost:   bcryptCost,
		compare:    bcrypt.CompareHashAndPassword,
	}
}

// dummyHash, var olmayan bir e-postayla giriş denendiğinde karşılaştırılacak sahte hash.
//
// NEDEN: kullanıcı yoksa hemen "hatalı" dönersek cevap ~1 ms'de gelir; kullanıcı varsa
// bcrypt ~250 ms sürer. Saldırgan yalnızca SÜREYE bakarak hangi e-postaların kayıtlı
// olduğunu çıkarabilir (timing-based user enumeration) — mesajı generic yapmak yetmez.
// Kullanıcı yoksa da aynı maliyette bir karşılaştırma yapınca iki yol aynı sürede biter.
// sync.OnceValue: süreç başına BİR kez üretilir.
var dummyHash = sync.OnceValue(func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("bookhub-timing-equaliser"), bcryptCost)
	if err != nil {
		panic(fmt.Sprintf("dummy hash üretilemedi: %v", err)) // yalnızca bellek/RNG arızası
	}
	return h
})

// NormalizeEmail, e-postayı karşılaştırılabilir kanonik biçime getirir. DB'deki
// CHECK (email = lower(email)) bu fonksiyonun unutulmasına karşı son hattır.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validateRegistration, kayıt girdisini denetler; tüm problemleri birlikte döner.
func validateRegistration(email, displayName, password string) error {
	var problems []string
	switch {
	case email == "":
		problems = append(problems, "email zorunlu")
	case !validEmail(email):
		problems = append(problems, "email geçerli bir e-posta adresi olmalı")
	}
	switch {
	case displayName == "":
		problems = append(problems, "display_name zorunlu")
	case utf8.RuneCountInString(displayName) > maxDisplayNameLen:
		problems = append(problems, fmt.Sprintf("display_name en fazla %d karakter olabilir", maxDisplayNameLen))
	}
	switch {
	case utf8.RuneCountInString(password) < MinPasswordLen:
		problems = append(problems, fmt.Sprintf("password en az %d karakter olmalı", MinPasswordLen))
	case len(password) > MaxPasswordBytes:
		problems = append(problems, fmt.Sprintf("password en fazla %d bayt olabilir", MaxPasswordBytes))
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// validEmail: ASCII, makul uzunlukta ve net/mail'in ayrıştırdığı adresin TAM kendisi
// ("Ali <a@b.com>" gibi görünen-ad biçimlerini reddetmek için Address == girdi).
//
// ASCII şartı bilinçli bir sadeleştirme: Go'nun strings.ToLower'ı ile Postgres'in lower()'ı
// bazı Unicode harflerde (ör. Türkçe "İ") farklı sonuç verir; o zaman CHECK kısıtı
// patlar ve kullanıcı anlamsız bir 500 görür. Uluslararası e-posta adresleri nadir.
func validEmail(email string) bool {
	if len(email) > maxEmailLen {
		return false
	}
	for _, r := range email {
		if r > unicode.MaxASCII {
			return false
		}
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email && strings.Contains(email[strings.LastIndex(email, "@"):], ".")
}

// Register, yeni bir USER hesabı açar. Rol parametresi YOK: kayıt yolundan admin
// doğamaz (DB varsayılanı + bu imza, iki ayrı hat).
func (s *Service) Register(ctx context.Context, email, displayName, password string) (User, error) {
	email = NormalizeEmail(email)
	displayName = strings.TrimSpace(displayName)
	if err := validateRegistration(email, displayName, password); err != nil {
		return User{}, err
	}
	hash, err := hashPassword(password, s.hashCost)
	if err != nil {
		return User{}, err
	}
	u, err := s.store.CreateUser(ctx, email, displayName, hash)
	if err != nil {
		return User{}, err // ErrEmailTaken olduğu gibi yukarı çıkar → handler 409
	}
	return toUser(u), nil
}

// Login, kimlik bilgilerini doğrular ve yeni bir oturum (access + refresh) açar.
func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	u, err := s.store.UserByEmail(ctx, NormalizeEmail(email))
	if errors.Is(err, ErrUserNotFound) {
		// Kullanıcı yok — yine de bir bcrypt karşılaştırması yap (bkz. dummyHash).
		_ = s.compare(dummyHash(), []byte(password))
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}

	switch err := s.compare([]byte(u.PasswordHash), []byte(password)); {
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return Session{}, ErrInvalidCredentials
	case err != nil:
		// Hash bozuk (elle düzenlenmiş DB satırı gibi) — kullanıcının hatası değil.
		return Session{}, fmt.Errorf("parola karşılaştırılamadı (user %d): %w", u.ID, err)
	}

	// Sınırlı temizlik: tablo sınırsız büyümesin diye girişte kullanıcının süresi
	// dolmuş token'larını süpür.
	if err := s.store.DeleteExpiredRefreshTokens(ctx, u.ID); err != nil {
		return Session{}, err
	}
	return s.newSession(ctx, u)
}

// Refresh, refresh token'ı TEK KULLANIMLIK olarak tüketir ve yeni bir çift verir
// (rotasyon). Eski token artık geçersizdir: çalınmış bir refresh token'ın ömrü, gerçek
// sahibinin bir sonraki yenilemesiyle biter.
//
// Tüketme ve yenisini kaydetme aynı transaction'da değil: kayıt başarısız olursa
// kullanıcı yeniden giriş yapmak zorunda kalır. Bu KAPALI hata (fail-closed) — ters
// sıra (önce kaydet, sonra tüket) başarısızlıkta iki geçerli token bırakırdı.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	userID, expiresAt, err := s.store.ConsumeRefreshToken(ctx, hashRefreshToken(refreshToken))
	if errors.Is(err, errRefreshNotFound) {
		return Session{}, ErrInvalidToken
	}
	if err != nil {
		return Session{}, err
	}
	if !s.now().Before(expiresAt) {
		return Session{}, ErrInvalidToken
	}

	// Kullanıcıyı DB'den TAZE oku: rolü değiştiyse (admin'likten alındıysa) yeni access
	// token eski rolü taşımasın.
	u, err := s.store.UserByID(ctx, userID)
	if errors.Is(err, ErrUserNotFound) {
		return Session{}, ErrInvalidToken
	}
	if err != nil {
		return Session{}, err
	}
	return s.newSession(ctx, u)
}

// Logout, refresh token'ı siler. İdempotent: bilinmeyen token da "başarılı" sayılır —
// "çıkış yaptım mı?" sorusunun cevabı her durumda evet, ve cevap token'ın var olup
// olmadığını sızdırmaz. Access token ~15 dk içinde kendiliğinden ölür (stateless).
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.store.DeleteRefreshToken(ctx, hashRefreshToken(refreshToken))
}

// Me, token'daki kullanıcının güncel bilgisini döner.
func (s *Service) Me(ctx context.Context, userID int64) (User, error) {
	u, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return User{}, err
	}
	return toUser(u), nil
}

// EnsureAdmin, ilk admin'i oluşturur ya da var olan hesabı admin yapar (cmd/manage).
// Yeni hesap açılıyorsa kayıtla AYNI politika uygulanır.
func (s *Service) EnsureAdmin(ctx context.Context, email, displayName, password string) (User, bool, error) {
	email = NormalizeEmail(email)
	displayName = strings.TrimSpace(displayName)
	if err := validateRegistration(email, displayName, password); err != nil {
		return User{}, false, err
	}
	hash, err := hashPassword(password, s.hashCost)
	if err != nil {
		return User{}, false, err
	}
	u, created, err := s.store.EnsureAdmin(ctx, email, displayName, hash)
	if err != nil {
		return User{}, false, err
	}
	return toUser(u), created, nil
}

// newSession, access token üretir ve yeni bir refresh token kaydeder.
func (s *Service) newSession(ctx context.Context, u db.User) (Session, error) {
	access, accessExp, err := s.tokens.Issue(u.ID, u.Role)
	if err != nil {
		return Session{}, err
	}

	// rand.Text: crypto/rand'dan 128 bit (26 karakter base32) — tahmin edilemez.
	// Kendi "rastgele" string üretecini yazmak (math/rand, zaman damgası, uuid v1)
	// oturum çalmanın klasik kapısıdır.
	refresh := rand.Text()
	refreshExp := s.now().Add(s.refreshTTL)
	if err := s.store.SaveRefreshToken(ctx, u.ID, hashRefreshToken(refresh), refreshExp); err != nil {
		return Session{}, err
	}

	return Session{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     refresh,
		RefreshExpiresAt: refreshExp,
		User:             toUser(u),
	}, nil
}

// hashRefreshToken: DB'de token'ın kendisi değil SHA-256'sı durur (bkz. migration 0002).
func hashRefreshToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
