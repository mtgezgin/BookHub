package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
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

// ErrWrongPassword, parola değiştirirken sunulan MEVCUT parolanın tutmadığını söyler.
// ErrInvalidCredentials'tan AYRI bir sentinel çünkü HTTP karşılığı farklı: login'de 401
// doğal, burada ise 401 yanlış olurdu — istemci 401'i "access token bitti" sayıp yenilemeye
// gider ve kullanıcı gerçek nedeni (parolayı yanlış yazdı) hiç görmezdi. Handler 403 verir.
// Burada enumeration kaygısı yok: istek zaten kimliği doğrulanmış kullanıcının kendi hesabına.
var ErrWrongPassword = errors.New("mevcut parola hatalı")

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
	// StartRefreshFamily'nin verifiedHash'i: servisin bcrypt ile DOĞRULADIĞI hash. Depo, aileyi
	// açmadan önce kullanıcının hash'inin kilit altında hâlâ bu olduğunu denetler (bkz. Login).
	StartRefreshFamily(ctx context.Context, userID int64, verifiedHash string, tokenHash []byte, tokenExp, familyExp time.Time) error
	RotateRefreshToken(ctx context.Context, oldHash, newHash []byte, now, newExp time.Time) (Rotation, error)
	RevokeRefreshFamily(ctx context.Context, tokenHash []byte, now time.Time) error
	CleanupRefreshTokens(ctx context.Context, userID int64, now time.Time) error
	// ChangePassword: yeni hash'i yazar, kullanıcının TÜM canlı ailelerini öldürür ve bu cihaz
	// için yeni bir aile açar — hepsi TEK transaction'da. Dönüş: iptal edilen aile sayısı.
	ChangePassword(ctx context.Context, c PasswordChange) (revokedFamilies int64, err error)
}

// PasswordChange, store.ChangePassword'ün girdisi: tek bir transaction'da yapılacak her şey.
// Struct olması yalnız okunabilirlik için (yedi parametreli bir imza yerine adlı alanlar).
type PasswordChange struct {
	UserID int64
	// VerifiedHash: servisin mevcut parolayı bcrypt ile doğruladığı hash. Depo, users satırını
	// kilitledikten sonra hash'in hâlâ bu olduğunu denetler; değilse (eşzamanlı başka bir
	// değişim araya girdi) ErrWrongPassword ile geri alır — doğrulanmamış bir parolayla
	// hesabın parolası değiştirilemez.
	VerifiedHash string
	NewHash      string
	// Bu cihazın yeni ailesi (parola değişen cihaz oturumdan düşmesin): StartRefreshFamily'yle
	// aynı üçlü.
	TokenHash           []byte
	TokenExp, FamilyExp time.Time
	Now                 time.Time
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
	if p := passwordProblem(password); p != "" {
		problems = append(problems, p)
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// passwordProblem, parola POLİTİKASININ tek yeri: kayıt, admin oluşturma ve parola
// değiştirme aynı kuralı buradan alır. Boş dönüş = parola kabul edilebilir.
//
// NEDEN AYRI: kural validateRegistration'ın içinde gömülü kalsaydı parola değiştirme onu
// kopyalamak zorunda kalırdı; iki kopya, ilk değişiklikte sessizce ayrışır (ör. bir yerde 8,
// diğerinde 10 karakter) ve "kayıtta reddedilen parola değiştirmede geçer" olurdu.
func passwordProblem(password string) string {
	switch {
	case utf8.RuneCountInString(password) < MinPasswordLen:
		return fmt.Sprintf("password en az %d karakter olmalı", MinPasswordLen)
	case len(password) > MaxPasswordBytes:
		return fmt.Sprintf("password en fazla %d bayt olabilir", MaxPasswordBytes)
	}
	return ""
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
	// GİRİŞ YARIŞI: bcrypt karşılaştırması yukarıda, transaction DIŞINDA ve u.PasswordHash ile
	// yapıldı. O ~250 ms içinde kullanıcı (ya da hesabını ele geçiren biri, başka bir cihazda)
	// parolayı DEĞİŞTİRMİŞ olabilir; değişim tüm aileleri öldürür ama bizim açacağımız aile
	// iptalden SONRA doğar ve ESKİ parolayla yaşardı. Depo, aileyi açmadan önce hash'in kilit
	// altında hâlâ u.PasswordHash olduğunu denetler; değilse ErrInvalidCredentials (rollback).
	// Kullanıcı bu arada silindiyse de sonuç aynı: giriş yok.
	err = s.store.StartRefreshFamily(ctx, u.ID, u.PasswordHash, hashRefreshToken(refresh), refreshExp, familyExp)
	if errors.Is(err, ErrUserNotFound) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	return s.session(u, refresh, refreshExp)
}

// ChangePassword, mevcut parolayı doğrular, yenisini yazar ve kullanıcının TÜM oturum
// ailelerini (her cihaz) iptal eder; bu cihaz için yeni bir oturum açıp döner.
//
// NEDEN TÜM AİLELER: parola değiştirmenin en yaygın nedeni "hesabımı biri ele geçirmiş
// olabilir". Yalnız bu cihazın ailesi yenilense, saldırganın başka bir cihazdaki oturumu
// mutlak tavana (JWT_SESSION_MAX_AGE) kadar yaşamaya devam ederdi. Kullanıcı diğer
// cihazlarında yeniden giriş yapar — bilinçli bedel.
//
// BİLİNEN SINIR: değişimden önce verilmiş ACCESS token'lar (JWT, durumsuz) süreleri dolana
// kadar (≤ JWT_ACCESS_TTL, 15 dk) geçerli kalır. Kapatmak için token'a bir
// "password_changed_at"/"token_version" iddiası koyup hassas uçlarda DB'ye bakmak gerekir —
// bu fazın kapsamı dışında.
func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next string) (Session, error) {
	// bcrypt.CompareHashAndPassword 72 bayttan sonrasını SESSİZCE kırpar (bkz. Login): 72
	// baytlık gerçek parolanın sonuna her şey eklenebilirdi. Kayıt >72'yi reddettiği için
	// böyle bir "mevcut parola" hiçbir hesabın parolası olamaz → doğrudan "hatalı".
	if len(current) > MaxPasswordBytes {
		return Session{}, ErrWrongPassword
	}

	u, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return Session{}, err // ErrUserNotFound olduğu gibi: token geçerli ama hesap silinmiş
	}
	switch err := s.compare([]byte(u.PasswordHash), []byte(current)); {
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return Session{}, ErrWrongPassword
	case err != nil:
		return Session{}, fmt.Errorf("parola karşılaştırılamadı (user %d): %w", u.ID, err)
	}

	// Yeni parola için kayıtla AYNI politika. Mevcut parola doğrulandıktan SONRA: yanlış
	// mevcut parola her zaman "mevcut parola hatalı"dır, yeni parolanın kusuru onu gölgelemez.
	if p := passwordProblem(next); p != "" {
		return Session{}, &ValidationError{Problems: []string{p}}
	}
	if next == current {
		return Session{}, &ValidationError{Problems: []string{"yeni parola mevcut paroladan farklı olmalı"}}
	}
	newHash, err := hashPassword(next, s.hashCost)
	if err != nil {
		return Session{}, err
	}

	// Bu cihazın yeni ailesi, iptalle AYNI transaction'da açılır (Login ile aynı hesap).
	now := s.now()
	refresh := rand.Text()
	familyExp := now.Add(s.sessionMaxAge)
	refreshExp := earliest(now.Add(s.refreshTTL), familyExp)
	revoked, err := s.store.ChangePassword(ctx, PasswordChange{
		UserID:       u.ID,
		VerifiedHash: u.PasswordHash,
		NewHash:      newHash,
		TokenHash:    hashRefreshToken(refresh),
		TokenExp:     refreshExp,
		FamilyExp:    familyExp,
		Now:          now,
	})
	if err != nil {
		return Session{}, err // ErrWrongPassword dahil (eşzamanlı başka bir değişim araya girdi)
	}
	// Operatör izi: KİM ve KAÇ oturum. Parola, hash ya da token ASLA loglanmaz.
	slog.Info("parola değiştirildi, kullanıcının tüm oturum aileleri iptal edildi",
		"user_id", u.ID, "revoked_families", revoked)
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

	// Kullanıcı, rotasyonun KENDİ transaction'ında okundu (rot.User): rolü değiştiyse yeni
	// access token eskisini taşımaz. Burada AYRI bir UserByID çağrısı YOK — commit'ten sonraki
	// her adım, başarısız olduğunda halef token'ı "yazılmış ama teslim edilmemiş" bırakır ve
	// istemcinin eski çerezle yaptığı tekrar denemesi reuse sayılıp tüm aileyi öldürürdü
	// (inceleme bulgusu F02).
	//
	// GERİYE KALAN TEK RİSK (bilinçli politika): commit oldu ama CEVAP istemciye ulaşmadı
	// (ağ koptu, sekme kapandı). O zaman tarayıcıdaki çerez harcanmış token'ı taşır ve bir
	// sonraki yenileme reuse sayılır → aile iptal, kullanıcı yeniden giriş yapar. Bu KAPALI
	// başarısızlık bilinçli: alternatifi (harcanmış token'ı bir süre daha kabul eden tolerans
	// penceresi) çalınmış token'ı yarışta önde olan saldırganın fark edilmeden kullanmasına
	// izin verirdi. İstemci tarafındaki keepalive bu pencereyi daraltır (client.ts).
	// RefreshExpiresAt = halefin GERÇEK (tavanlı) bitişi, "now+refreshTTL" değil.
	return s.session(rot.User, next, rot.ExpiresAt)
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
