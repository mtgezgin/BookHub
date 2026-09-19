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
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
	// RefreshExpiresAt, refresh token'ın GERÇEK bitişi: kayan pencere (refreshTTL) ailenin
	// mutlak tavanına (sessionMaxAge) kırpılmış hâli.
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
	StartRefreshFamily(ctx context.Context, userID int64, tokenHash []byte, tokenExp, familyExp time.Time) error
	RotateRefreshToken(ctx context.Context, oldHash, newHash []byte, now, newExp time.Time) (Rotation, error)
	RevokeRefreshFamily(ctx context.Context, tokenHash []byte, now time.Time) error
	CleanupRefreshTokens(ctx context.Context, userID int64, now time.Time) error
}

// Service, projenin İLK servis katmanı. Kitap/tür CRUD'unda handler → repository yeterliydi
// (araya servis koymak boş bir geçiş katmanı olurdu). Auth'ta ise gerçek iş mantığı var:
// parola politikası, zamanlama eşitleme, token üretimi + saklama + rotasyon — bunların
// hiçbiri ne HTTP'nin ne de SQL'in işi.
type Service struct {
	store      store
	tokens     *TokenManager
	refreshTTL time.Duration
	// sessionMaxAge, bir girişin (refresh token AİLESİNİN) mutlak ömrü. refreshTTL kayan
	// penceredir (her rotasyon onu yeniden başlatır); sessionMaxAge ise pencerenin asla
	// geçemeyeceği tavan — çalınmış bir aile de en geç burada ölür.
	sessionMaxAge time.Duration
	now           func() time.Time

	// Testler için enjekte edilebilir: hashCost testlerde bcrypt.MinCost olur (hız);
	// compare, "kullanıcı yokken de karşılaştırma yapıldı mı" iddiasını ölçmek için.
	hashCost int
	compare  func(hash, password []byte) error
}

// NewService, üretim ayarlarıyla bir Service kurar ve zamanlama-eşitleme hash'ini ısıtır.
func NewService(s store, tokens *TokenManager, refreshTTL, sessionMaxAge time.Duration) *Service {
	dummyHash() // ilk "kullanıcı yok" girişi fazladan 250 ms beklemesin
	return &Service{
		store:         s,
		tokens:        tokens,
		refreshTTL:    refreshTTL,
		sessionMaxAge: sessionMaxAge,
		now:           time.Now,
		hashCost:      bcryptCost,
		compare:       bcrypt.CompareHashAndPassword,
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
	// bcrypt.CompareHashAndPassword 72 bayttan sonrasını SESSİZCE kırpar (Generate'in
	// aksine hata dönmez — denenerek doğrulandı): 72 baytlık parolanın sonuna ne
	// eklenirse eklensin giriş başarılı olurdu. Kayıt zaten >72'yi reddettiği için böyle
	// bir parola hiçbir hesabın parolası olamaz → doğrudan "hatalı".
	if len(password) > MaxPasswordBytes {
		return Session{}, ErrInvalidCredentials
	}

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

	// Sınırlı temizlik: tablo sınırsız büyümesin diye girişte kullanıcının ölü ailelerini ve
	// süresi dolmuş token'larını süpür.
	now := s.now()
	if err := s.store.CleanupRefreshTokens(ctx, u.ID, now); err != nil {
		return Session{}, err
	}

	// Her giriş YENİ bir aile açar (bir cihaz/tarayıcı = bir aile): bir cihazdaki reuse iptali
	// diğer cihazların oturumlarına dokunmaz. İlk token'ın bitişi de mutlak tavanı geçemez
	// (config sessionMaxAge >= refreshTTL'yi zorlar; min yine de derinlemesine savunma).
	//
	// rand.Text: crypto/rand'dan 128 bit (26 karakter base32) — tahmin edilemez.
	// Kendi "rastgele" string üretecini yazmak (math/rand, zaman damgası, uuid v1)
	// oturum çalmanın klasik kapısıdır.
	refresh := rand.Text()
	familyExp := now.Add(s.sessionMaxAge)
	refreshExp := earliest(now.Add(s.refreshTTL), familyExp)
	if err := s.store.StartRefreshFamily(ctx, u.ID, hashRefreshToken(refresh), refreshExp, familyExp); err != nil {
		return Session{}, err
	}
	return s.session(u, refresh, refreshExp)
}

// Refresh, refresh token'ı TEK KULLANIMLIK olarak harcar ve aynı ailede yeni bir çift verir
// (rotasyon). Eski token artık geçersizdir: çalınmış bir refresh token'ın ömrü, gerçek
// sahibinin bir sonraki yenilemesiyle biter.
//
// Harcanmış bir token İKİNCİ kez gelirse (reuse) repository ailenin tamamını öldürür ve
// *ReuseError döner. Onu OLDUĞU GİBİ yukarı veriyoruz: ErrInvalidToken'ı sardığı için istemci
// düz "geçersiz token" görür (kâhin yok), handler ise errors.As ile güvenlik uyarısı loglar.
//
// Harcama ile halefin kaydı TEK transaction'da (bkz. RotateRefreshToken): kayıt başarısız
// olursa harcama da geri alınır — ne iki canlı token ne de halefsiz harcanmış bir token kalır.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	now := s.now()
	next := rand.Text()
	rot, err := s.store.RotateRefreshToken(ctx, hashRefreshToken(refreshToken), hashRefreshToken(next), now, now.Add(s.refreshTTL))
	if errors.Is(err, errRefreshNotFound) {
		return Session{}, ErrInvalidToken
	}
	if err != nil {
		return Session{}, err // *ReuseError dahil: ErrInvalidToken'ı sarar
	}

	// Kullanıcıyı DB'den TAZE oku: rolü değiştiyse (admin'likten alındıysa) yeni access
	// token eski rolü taşımasın.
	u, err := s.store.UserByID(ctx, rot.UserID)
	if errors.Is(err, ErrUserNotFound) {
		return Session{}, ErrInvalidToken
	}
	if err != nil {
		return Session{}, err
	}
	// RefreshExpiresAt = halefin GERÇEK (tavanlı) bitişi, "now+refreshTTL" değil.
	return s.session(u, next, rot.ExpiresAt)
}

// Logout, refresh token'ın AİLESİNİ öldürür (yalnız sunulan token'ı değil: aynı girişin eski
// bir kopyası çıkıştan sonra yaşamasın). İdempotent: bilinmeyen token da "başarılı" sayılır —
// "çıkış yaptım mı?" sorusunun cevabı her durumda evet, ve cevap token'ın var olup olmadığını
// sızdırmaz. Access token ~15 dk içinde kendiliğinden ölür (stateless).
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.store.RevokeRefreshFamily(ctx, hashRefreshToken(refreshToken), s.now())
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

// session, KAYDEDİLMİŞ bir refresh token için access token üretir ve oturumu toplar.
func (s *Service) session(u db.User, refresh string, refreshExp time.Time) (Session, error) {
	access, accessExp, err := s.tokens.Issue(u.ID, u.Role)
	if err != nil {
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

// earliest, iki zamandan önce geleni döner (bitişi tavana kırpmak için).
func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// hashRefreshToken: DB'de token'ın kendisi değil SHA-256'sı durur (bkz. migration 0002).
func hashRefreshToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
