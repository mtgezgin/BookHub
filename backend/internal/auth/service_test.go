package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// memStore, store'un bellek içi sahtesi: gerçek DB'nin SÖZLEŞMESİNİ taklit eder
// (tekil e-posta, token aileleri, tek kullanımlık token + reuse'da ailenin ölümü, mutlak
// tavan) ama SQL'siz. Sözleşmenin gerçek DB'de — eşzamanlılık altında da — tuttuğunu
// repository_integration_test.go kanıtlıyor. Tek mutex = her metot atomik (DB'deki tx gibi).
type memStore struct {
	mu       sync.Mutex
	nextID   int64
	users    map[int64]db.User
	byEmail  map[string]int64
	tokens   map[string]memToken // anahtar: string(tokenHash)
	families map[int64]*memFamily
	nextFam  int64
	// userByIDErr: UserByID'nin AYRI çağrısını patlatır (rotasyonun kendi okuması etkilenmez).
	// Refresh'in commit'ten SONRA kullanıcı okumadığını kanıtlamak için (bulgu F02).
	userByIDErr error
}

type memToken struct {
	userID    int64
	familyID  int64
	expiresAt time.Time
	used      bool
}

type memFamily struct {
	userID      int64
	absoluteExp time.Time
	revoked     bool
}

func newMemStore() *memStore {
	return &memStore{
		users:    map[int64]db.User{},
		byEmail:  map[string]int64{},
		tokens:   map[string]memToken{},
		families: map[int64]*memFamily{},
	}
}

func (m *memStore) CreateUser(_ context.Context, email, name, hash string) (db.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[email]; ok {
		return db.User{}, ErrEmailTaken
	}
	m.nextID++
	u := db.User{ID: m.nextID, Email: email, DisplayName: name, PasswordHash: hash, Role: RoleUser}
	m.users[u.ID] = u
	m.byEmail[email] = u.ID
	return u, nil
}

func (m *memStore) UserByEmail(_ context.Context, email string) (db.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byEmail[email]
	if !ok {
		return db.User{}, ErrUserNotFound
	}
	return m.users[id], nil
}

func (m *memStore) UserByID(_ context.Context, id int64) (db.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.userByIDErr != nil {
		return db.User{}, m.userByIDErr
	}
	u, ok := m.users[id]
	if !ok {
		return db.User{}, ErrUserNotFound
	}
	return u, nil
}

func (m *memStore) EnsureAdmin(ctx context.Context, email, name, hash string) (db.User, bool, error) {
	u, err := m.UserByEmail(ctx, email)
	created := false
	if errors.Is(err, ErrUserNotFound) {
		u, err = m.CreateUser(ctx, email, name, hash)
		created = true
	}
	if err != nil {
		return db.User{}, false, err
	}
	m.setRole(u.ID, RoleAdmin)
	u.Role = RoleAdmin
	return u, created, nil
}

func (m *memStore) setRole(id int64, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.users[id]
	u.Role = role
	m.users[id] = u
}

func (m *memStore) StartRefreshFamily(_ context.Context, userID int64, verifiedHash string, h []byte, tokenExp, familyExp time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Repository ile aynı sözleşme: doğrulanan hash artık kullanıcının hash'i değilse (parola
	// bu arada değişti) aile AÇILMAZ.
	u, ok := m.users[userID]
	if !ok {
		return ErrUserNotFound
	}
	if u.PasswordHash != verifiedHash {
		return ErrInvalidCredentials
	}
	m.startFamilyLocked(userID, h, tokenExp, familyExp)
	return nil
}

// startFamilyLocked: yeni aile + ilk token (m.mu tutuluyorken çağrılır).
func (m *memStore) startFamilyLocked(userID int64, h []byte, tokenExp, familyExp time.Time) {
	m.nextFam++
	m.families[m.nextFam] = &memFamily{userID: userID, absoluteExp: familyExp}
	m.tokens[string(h)] = memToken{userID: userID, familyID: m.nextFam, expiresAt: tokenExp}
}

// ChangePassword, Repository.ChangePassword'ün adımlarını taklit eder: hash denetimi → yeni
// hash → kullanıcının TÜM canlı aileleri ölür (token'larıyla) → bu cihaz için yeni aile.
// Hepsi tek mutex altında = DB'deki tek tx.
func (m *memStore) ChangePassword(_ context.Context, c PasswordChange) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[c.UserID]
	if !ok {
		return 0, ErrUserNotFound
	}
	if u.PasswordHash != c.VerifiedHash {
		return 0, ErrWrongPassword
	}
	u.PasswordHash = c.NewHash
	m.users[c.UserID] = u
	var revoked int64
	for id, fam := range m.families {
		if fam.userID == c.UserID && !fam.revoked {
			m.revokeLocked(id)
			revoked++
		}
	}
	for k, tok := range m.tokens {
		if tok.userID == c.UserID {
			delete(m.tokens, k)
		}
	}
	m.startFamilyLocked(c.UserID, c.TokenHash, c.TokenExp, c.FamilyExp)
	return revoked, nil
}

// RotateRefreshToken, Repository.RotateRefreshToken'ın adımlarını AYNI sırayla taklit eder:
// aile kontrolleri (iptal, tavan) → reuse → süre → halef (bitişi tavana kırpılır).
func (m *memStore) RotateRefreshToken(_ context.Context, oldHash, newHash []byte, now, newExp time.Time) (Rotation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tok, ok := m.tokens[string(oldHash)]
	if !ok {
		return Rotation{}, errRefreshNotFound
	}
	fam, ok := m.families[tok.familyID]
	if !ok || fam.revoked || !now.Before(fam.absoluteExp) {
		return Rotation{}, errRefreshNotFound
	}
	if tok.used {
		m.revokeLocked(tok.familyID)
		return Rotation{}, &ReuseError{UserID: fam.userID, FamilyID: tok.familyID}
	}
	if !now.Before(tok.expiresAt) {
		return Rotation{}, errRefreshNotFound
	}
	tok.used = true
	m.tokens[string(oldHash)] = tok
	exp := earliest(newExp, fam.absoluteExp)
	m.tokens[string(newHash)] = memToken{userID: tok.userID, familyID: tok.familyID, expiresAt: exp}
	// Kullanıcı, gerçek depoda olduğu gibi rotasyonun İÇİNDE okunur (Rotation.User): hesap
	// silinmişse rotasyon da başarısızdır.
	u, ok := m.users[tok.userID]
	if !ok {
		return Rotation{}, errRefreshNotFound
	}
	return Rotation{UserID: tok.userID, FamilyID: tok.familyID, ExpiresAt: exp, User: u}, nil
}

func (m *memStore) RevokeRefreshFamily(_ context.Context, h []byte, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tok, ok := m.tokens[string(h)]; ok {
		m.revokeLocked(tok.familyID)
	}
	return nil
}

// revokeLocked: aile ölür, token'larının hepsi silinir (m.mu tutuluyorken çağrılır).
func (m *memStore) revokeLocked(familyID int64) {
	if fam, ok := m.families[familyID]; ok {
		fam.revoked = true
	}
	for k, tok := range m.tokens {
		if tok.familyID == familyID {
			delete(m.tokens, k)
		}
	}
}

// CleanupRefreshTokens: ölü aileler (iptal / tavan geçmiş / canlı token'sız) token'larıyla
// gider; canlı ailelerdeki süresi dolmuş token'lar silinir (users.sql'deki iki DELETE).
func (m *memStore) CleanupRefreshTokens(_ context.Context, userID int64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	live := map[int64]bool{}
	for _, tok := range m.tokens {
		if !tok.used && now.Before(tok.expiresAt) {
			live[tok.familyID] = true
		}
	}
	for id, fam := range m.families {
		if fam.userID == userID && (fam.revoked || !now.Before(fam.absoluteExp) || !live[id]) {
			m.revokeLocked(id)
			delete(m.families, id)
		}
	}
	for k, tok := range m.tokens {
		if tok.userID == userID && !now.Before(tok.expiresAt) {
			delete(m.tokens, k)
		}
	}
	return nil
}

// familyOf, token'ın ailesini döner (testlerin "aile öldü mü" iddiaları için).
func (m *memStore) familyOf(t *testing.T, raw string) (int64, memFamily) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	tok, ok := m.tokens[string(hashRefreshToken(raw))]
	if !ok {
		t.Fatalf("token deposunda yok")
	}
	return tok.familyID, *m.families[tok.familyID]
}

// familyTokens, ailede kalan token sayısı.
func (m *memStore) familyTokens(familyID int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, tok := range m.tokens {
		if tok.familyID == familyID {
			n++
		}
	}
	return n
}

// newTestService: hızlı bcrypt (MinCost) ve kontrol edilebilir saat.
func newTestService(t *testing.T) (*Service, *memStore, *time.Time) {
	t.Helper()
	tokens, err := NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	st := newMemStore()
	now := time.Now()
	s := NewService(st, tokens, 24*time.Hour, testSessionMaxAge)
	s.hashCost = bcrypt.MinCost
	s.now = func() time.Time { return now }
	return s, st, &now
}

const goodPassword = "yeterince-uzun-parola"

// testSessionMaxAge, testlerdeki oturum tavanı (refreshTTL 24 saat → tavan 7 gün).
const testSessionMaxAge = 7 * 24 * time.Hour

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name, email, displayName, password string
		wantProblem                        string // boşsa hata beklenmez
	}{
		{"geçerli", "ali@example.com", "Ali", goodPassword, ""},
		{"e-posta yok", "", "Ali", goodPassword, "email zorunlu"},
		{"@ yok", "ali.example.com", "Ali", goodPassword, "email geçerli"},
		{"görünen-ad biçimi", "Ali <ali@example.com>", "Ali", goodPassword, "email geçerli"},
		{"noktasız alan adı", "ali@localhost", "Ali", goodPassword, "email geçerli"},
		{"ASCII olmayan", "ayşe@example.com", "Ayşe", goodPassword, "email geçerli"},
		{"çok uzun e-posta", strings.Repeat("a", 250) + "@x.com", "Ali", goodPassword, "email geçerli"},
		{"ad yok", "ali@example.com", "   ", goodPassword, "display_name zorunlu"},
		{"ad çok uzun", "ali@example.com", strings.Repeat("ğ", maxDisplayNameLen+1), goodPassword, "display_name en fazla"},
		{"kısa parola", "ali@example.com", "Ali", "1234567", "password en az"},
		// 8 karakter ama 16 bayt: sınır KARAKTER cinsinden, Türkçe parola haksızca reddedilmesin.
		{"8 Türkçe karakter yeter", "ali@example.com", "Ali", "ğüşıöçğü", ""},
		{"72 bayttan uzun", "ali@example.com", "Ali", strings.Repeat("ş", 37), "password en fazla 72 bayt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, _ := newTestService(t)
			_, err := s.Register(context.Background(), tt.email, tt.displayName, tt.password)
			if tt.wantProblem == "" {
				if err != nil {
					t.Fatalf("beklenmeyen hata: %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("ValidationError bekleniyordu, gelen %v", err)
			}
			if !strings.Contains(ve.Error(), tt.wantProblem) {
				t.Errorf("mesaj %q içermeli, gelen %q", tt.wantProblem, ve.Error())
			}
		})
	}
}

func TestRegister(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()

	u, err := s.Register(ctx, "  Ali@Example.COM ", " Ali ", goodPassword)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.Email != "ali@example.com" || u.DisplayName != "Ali" || u.Role != RoleUser {
		t.Errorf("normalize edilmiş kullanıcı bekleniyordu, gelen %+v", u)
	}
	stored := st.users[u.ID]
	if stored.PasswordHash == goodPassword || bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(goodPassword)) != nil {
		t.Error("parola hash'lenmiş olarak saklanmalı")
	}

	// Aynı e-posta, farklı büyük/küçük harf → yine çakışma.
	if _, err := s.Register(ctx, "ALI@example.com", "Ali 2", goodPassword); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("ErrEmailTaken bekleniyordu, gelen %v", err)
	}
}

func TestLogin(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()
	u, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	t.Run("yanlış parola → generic hata", func(t *testing.T) {
		if _, err := s.Login(ctx, "ali@example.com", "yanlis-parola"); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("ErrInvalidCredentials bekleniyordu, gelen %v", err)
		}
	})

	t.Run("doğru parola, farklı harf büyüklüğü → oturum", func(t *testing.T) {
		sess, err := s.Login(ctx, " ALI@example.com", goodPassword)
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		p, err := s.tokens.Verify(sess.AccessToken)
		if err != nil || p.UserID != u.ID || p.Role != RoleUser {
			t.Errorf("access token kullanıcıyı taşımalı: %+v, %v", p, err)
		}
		if sess.RefreshToken == "" || len(sess.RefreshToken) < 26 {
			t.Errorf("refresh token en az 128 bit olmalı, gelen %q", sess.RefreshToken)
		}
		// DB'de token'ın kendisi değil özeti durmalı.
		if _, raw := st.tokens[sess.RefreshToken]; raw {
			t.Error("refresh token düz metin saklanmış")
		}
		if _, hashed := st.tokens[string(hashRefreshToken(sess.RefreshToken))]; !hashed {
			t.Error("refresh token'ın özeti saklanmamış")
		}
	})
}

// TestLoginRejectsTruncatedPassword: bcrypt karşılaştırması 72 baytta kırptığı için
// "72 baytlık parola + herhangi bir sonek" eskiden girişi açıyordu.
func TestLoginRejectsTruncatedPassword(t *testing.T) {
	s, _, _ := newTestService(t)
	ctx := context.Background()
	pw := strings.Repeat("p", MaxPasswordBytes)
	if _, err := s.Register(ctx, "ali@example.com", "Ali", pw); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := s.Login(ctx, "ali@example.com", pw); err != nil {
		t.Fatalf("tam parola çalışmalı: %v", err)
	}
	if _, err := s.Login(ctx, "ali@example.com", pw+"SONEK"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("72 bayt + sonek reddedilmeli, gelen %v", err)
	}
}

// TestLoginUnknownEmailStillCompares: e-posta yokken de bcrypt karşılaştırması yapılmalı
// (zamanlama ile kullanıcı keşfine karşı). Süre ölçmek kırılgan olurdu; bunun yerine
// karşılaştırmanın ÇAĞRILDIĞINI sayıyoruz.
func TestLoginUnknownEmailStillCompares(t *testing.T) {
	s, _, _ := newTestService(t)
	calls := 0
	s.compare = func(hash, pw []byte) error {
		calls++
		return bcrypt.CompareHashAndPassword(hash, pw)
	}

	_, err := s.Login(context.Background(), "yok@example.com", goodPassword)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ErrInvalidCredentials bekleniyordu (e-posta yok ile parola yanlış AYNI olmalı), gelen %v", err)
	}
	if calls != 1 {
		t.Errorf("kullanıcı yokken de 1 karşılaştırma bekleniyordu, yapılan %d", calls)
	}
}

func TestRefreshRotation(t *testing.T) {
	s, _, now := newTestService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	first, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	second, err := s.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh token döndürülmeli (rotasyon)")
	}
	if second.User.ID != first.User.ID {
		t.Errorf("yenilenen oturum aynı kullanıcının olmalı: %d ≠ %d", second.User.ID, first.User.ID)
	}
	// Tavan (7 gün) uzakta → halef tam refreshTTL (24 saat) yaşamalı, tavana kadar değil.
	if want := now.Add(24 * time.Hour); !second.RefreshExpiresAt.Equal(want) {
		t.Errorf("halef now+24s'te bitmeli, gelen now+%v", second.RefreshExpiresAt.Sub(*now))
	}

	// DAVRANIŞ DEĞİŞTİ (Faz 13, token aileleri): eskiden "eski token reddedilir, yenisi
	// çalışmaya devam eder"i sırayla deniyorduk. Artık eski (harcanmış) token'ın ikinci
	// kullanımı REUSE sayılır ve ailenin TAMAMINI öldürür — yenisi de ölür (bkz.
	// TestRefreshReuseRevokesFamily). Bu yüzden önce yeni token'ı kullanıyoruz.
	if _, err := s.Refresh(ctx, second.RefreshToken); err != nil {
		t.Errorf("yeni token çalışmalı: %v", err)
	}
	if _, err := s.Refresh(ctx, first.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("eski token reddedilmeli, gelen %v", err)
	}
	_, err = s.Refresh(ctx, "hic-verilmemis-token")
	var reuse *ReuseError
	if !errors.Is(err, ErrInvalidToken) || errors.As(err, &reuse) {
		t.Errorf("bilinmeyen token düz ErrInvalidToken ile reddedilmeli (reuse değil), gelen %v", err)
	}
}

// TestRefreshReuseRevokesFamily: harcanmış bir token'ın İKİNCİ kullanımı *ReuseError döner
// (ErrInvalidToken'ı sarar → istemci için düz 401) ve ailenin tamamını öldürür: meşru halef
// de artık geçersiz. Hangisinin hırsız olduğunu bilemeyiz; ikisi de yeniden parola girer.
// Aynı kullanıcının BAŞKA bir girişi (başka cihaz = başka aile) etkilenmez.
func TestRefreshReuseRevokesFamily(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()
	u, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	first, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	otherDevice, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("ikinci cihaz Login: %v", err)
	}
	familyID, _ := st.familyOf(t, first.RefreshToken)

	second, err := s.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	// Saldırgan (ya da sahibi) eski kopyayı yeniden sunuyor.
	_, err = s.Refresh(ctx, first.RefreshToken)
	var reuse *ReuseError
	if !errors.As(err, &reuse) {
		t.Fatalf("*ReuseError bekleniyordu, gelen %v", err)
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("ReuseError ErrInvalidToken'ı sarmalı (istemci için aynı 401), gelen %v", err)
	}
	if reuse.UserID != u.ID || reuse.FamilyID != familyID {
		t.Errorf("ReuseError kimliği: istenen user=%d family=%d, gelen %+v", u.ID, familyID, reuse)
	}

	if _, err := s.Refresh(ctx, second.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("reuse'dan sonra meşru halef de ölmeli, gelen %v", err)
	}
	if n := st.familyTokens(familyID); n != 0 {
		t.Errorf("iptal edilen ailede token kalmamalı, %d var", n)
	}
	if !st.families[familyID].revoked {
		t.Error("aile iptal edilmiş olarak işaretlenmeli")
	}
	if _, err := s.Refresh(ctx, otherDevice.RefreshToken); err != nil {
		t.Errorf("başka bir giriş (başka aile) etkilenmemeli: %v", err)
	}
}

// TestRefreshAbsoluteCap: kayan pencere her rotasyonda uzasa da oturum ailenin MUTLAK
// tavanını (sessionMaxAge) aşamaz. Gerçek değerlerle: 7 günlük pencere, 30 günlük tavan;
// kullanıcı her 6 günde bir yeniliyor → pencere hiç kapanmıyor, ama tavanda oturum biter.
func TestRefreshAbsoluteCap(t *testing.T) {
	s, _, now := newTestService(t)
	const day = 24 * time.Hour
	s.refreshTTL, s.sessionMaxAge = 7*day, 30*day
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t0 := *now
	capAt := t0.Add(30 * day)

	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !sess.RefreshExpiresAt.Equal(t0.Add(7 * day)) {
		t.Errorf("ilk token t0+7g'de bitmeli, gelen %v", sess.RefreshExpiresAt.Sub(t0))
	}

	for d := 6; d < 30; d += 6 { // 6., 12., 18., 24. gün
		*now = t0.Add(time.Duration(d) * day)
		sess, err = s.Refresh(ctx, sess.RefreshToken)
		if err != nil {
			t.Fatalf("%d. gün Refresh: %v", d, err)
		}
		if sess.RefreshExpiresAt.After(capAt) {
			t.Fatalf("%d. gün: bitiş tavanı aştı (%v > 30g)", d, sess.RefreshExpiresAt.Sub(t0))
		}
		// Tavanın altındayken halef TAM now+refreshTTL yaşamalı: pencere now+sessionMaxAge'e
		// kaysa her token 30 gün yaşardı ve yukarıdaki "tavanı aşmadı" kontrolü bunu yakalamazdı.
		if d < 24 && !sess.RefreshExpiresAt.Equal((*now).Add(7*day)) {
			t.Errorf("%d. gün: kayan pencere now+7g olmalı, gelen now+%v", d, sess.RefreshExpiresAt.Sub(*now))
		}
	}
	// 24. günde pencere 31. güne uzardı → tavana (30. gün) kırpılmış olmalı.
	if !sess.RefreshExpiresAt.Equal(capAt) {
		t.Errorf("son bitiş tam tavan olmalı, gelen t0+%v", sess.RefreshExpiresAt.Sub(t0))
	}

	*now = capAt // tavan anı: artık geçersiz (mutlak tavan tek başına yeter)
	if _, err := s.Refresh(ctx, sess.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("tavandan sonra ErrInvalidToken bekleniyordu, gelen %v", err)
	}
}

// TestLoginCapsFirstTokenAtSessionMaxAge: tavan pencereden kısaysa (config bunu reddeder;
// servis yine de kendini korur) ilk token'ın bitişi de tavana kırpılır.
func TestLoginCapsFirstTokenAtSessionMaxAge(t *testing.T) {
	s, _, now := newTestService(t)
	s.sessionMaxAge = time.Hour // refreshTTL 24 saat
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if want := now.Add(time.Hour); !sess.RefreshExpiresAt.Equal(want) {
		t.Errorf("ilk token tavanda (now+1 saat) bitmeli, gelen now+%v", sess.RefreshExpiresAt.Sub(*now))
	}
}

func TestRefreshExpired(t *testing.T) {
	s, _, now := newTestService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	*now = now.Add(24 * time.Hour) // tam bitiş anı: artık geçersiz
	if _, err := s.Refresh(ctx, sess.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("süresi dolmuş token reddedilmeli, gelen %v", err)
	}
}

// TestRefreshUsesFreshRole: rol değişince yenilenen access token YENİ rolü taşımalı —
// eski token'dan kopyalanmamalı.
func TestRefreshUsesFreshRole(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()
	u, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	st.setRole(u.ID, RoleAdmin)
	next, err := s.Refresh(ctx, sess.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if p, err := s.tokens.Verify(next.AccessToken); err != nil || p.Role != RoleAdmin {
		t.Errorf("yeni token ADMIN rolü taşımalı: %+v, %v", p, err)
	}
}

func TestLogout(t *testing.T) {
	s, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := s.Logout(ctx, sess.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := s.Refresh(ctx, sess.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("çıkıştan sonra refresh reddedilmeli, gelen %v", err)
	}
	if err := s.Logout(ctx, "bilinmeyen"); err != nil {
		t.Errorf("logout idempotent olmalı, gelen %v", err)
	}
}

// TestLogoutRevokesWholeFamily: çıkış yalnız sunulan token'ı değil AİLEYİ öldürür. Burada
// çıkış, zinciri çoktan ilerlemiş bir oturumun ESKİ (harcanmış) token'ıyla yapılıyor — güncel
// halef de ölmeli; aksi hâlde eski bir kopyayla "çıkış yaptım" diyen kullanıcının oturumu
// başka bir yerde yaşamaya devam ederdi.
func TestLogoutRevokesWholeFamily(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	first, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	familyID, _ := st.familyOf(t, first.RefreshToken)
	latest, err := s.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if err := s.Logout(ctx, first.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := s.Refresh(ctx, latest.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("ailenin güncel token'ı da ölmeli, gelen %v", err)
	}
	if n := st.familyTokens(familyID); n != 0 || !st.families[familyID].revoked {
		t.Errorf("aile iptal edilmeli ve token'sız kalmalı: kalan=%d revoked=%v", n, st.families[familyID].revoked)
	}
}

func TestEnsureAdmin(t *testing.T) {
	s, _, _ := newTestService(t)
	ctx := context.Background()

	if _, _, err := s.EnsureAdmin(ctx, "admin@example.com", "Admin", "kisa"); err == nil {
		t.Error("CLI yolu da parola politikasına tabi olmalı")
	}

	u, created, err := s.EnsureAdmin(ctx, "admin@example.com", "Admin", goodPassword)
	if err != nil || !created || u.Role != RoleAdmin {
		t.Fatalf("yeni admin bekleniyordu: %+v created=%v err=%v", u, created, err)
	}
	u2, created, err := s.EnsureAdmin(ctx, "ADMIN@example.com", "Admin", goodPassword)
	if err != nil || created || u2.ID != u.ID {
		t.Errorf("var olan hesap yükseltilmeli, yenisi açılmamalı: %+v created=%v err=%v", u2, created, err)
	}
}

// TestLoginCorruptHash: compare, "eşleşmedi" dışında bir hata dönerse (bozuk hash)
// Login sarılı bir hata döner — ErrInvalidCredentials DEĞİL (kullanıcının hatası değil).
func TestLoginCorruptHash(t *testing.T) {
	s, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	s.compare = func(_, _ []byte) error { return bcrypt.ErrHashTooShort }

	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err == nil {
		t.Fatal("bozuk hash için hata bekleniyordu")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("bozuk hash ErrInvalidCredentials olarak maskelenmemeli: %v", err)
	}
	if !errors.Is(err, bcrypt.ErrHashTooShort) {
		t.Errorf("alttaki hata sarılı olmalı, gelen %v", err)
	}
	if sess.AccessToken != "" || sess.RefreshToken != "" {
		t.Errorf("hata durumunda oturum dönmemeli: %+v", sess)
	}
}

// sweepStore, CleanupRefreshTokens çağrısını kaydeden (ve isteğe bağlı hata döndüren)
// memStore sarmalayıcısı.
type sweepStore struct {
	*memStore
	sweptFor []int64
	sweptAt  []time.Time
	sweepErr error
}

func (s *sweepStore) CleanupRefreshTokens(ctx context.Context, userID int64, now time.Time) error {
	s.sweptFor = append(s.sweptFor, userID)
	s.sweptAt = append(s.sweptAt, now)
	if s.sweepErr != nil {
		return s.sweepErr
	}
	return s.memStore.CleanupRefreshTokens(ctx, userID, now)
}

func TestLoginSweepsExpiredTokens(t *testing.T) {
	ctx := context.Background()

	t.Run("başarılı giriş kullanıcının ölü ailelerini ve süresi dolmuş token'larını süpürür", func(t *testing.T) {
		s, st, now := newTestService(t)
		u, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword)
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		// Çıkış yapılmış (iptal edilmiş) eski bir oturum: bir sonraki girişte süpürülmeli.
		old, err := s.Login(ctx, "ali@example.com", goodPassword)
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		oldFamily, _ := st.familyOf(t, old.RefreshToken)
		if err := s.Logout(ctx, old.RefreshToken); err != nil {
			t.Fatalf("Logout: %v", err)
		}
		sw := &sweepStore{memStore: st}
		s.store = sw

		if _, err := s.Login(ctx, "ali@example.com", goodPassword); err != nil {
			t.Fatalf("Login: %v", err)
		}
		if len(sw.sweptFor) != 1 || sw.sweptFor[0] != u.ID {
			t.Errorf("CleanupRefreshTokens(%d) bir kez çağrılmalıydı, çağrılar %v", u.ID, sw.sweptFor)
		}
		// Tek saat: temizlik servisin saatiyle (s.now) koşar, DB'nin now()'ıyla değil.
		if len(sw.sweptAt) != 1 || !sw.sweptAt[0].Equal(*now) {
			t.Errorf("temizlik servisin saatini almalı: istenen %v, gelen %v", *now, sw.sweptAt)
		}
		if _, ok := st.families[oldFamily]; ok {
			t.Error("iptal edilmiş eski aile girişte silinmeliydi")
		}
	})

	t.Run("süpürme hatası oturumsuz döner", func(t *testing.T) {
		s, st, _ := newTestService(t)
		if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
			t.Fatalf("Register: %v", err)
		}
		sweepErr := errors.New("db down")
		sw := &sweepStore{memStore: st, sweepErr: sweepErr}
		s.store = sw

		sess, err := s.Login(ctx, "ali@example.com", goodPassword)
		if !errors.Is(err, sweepErr) {
			t.Fatalf("süpürme hatası bekleniyordu, gelen %v", err)
		}
		if sess.AccessToken != "" || sess.RefreshToken != "" {
			t.Errorf("hata durumunda oturum dönmemeli: %+v", sess)
		}
		if len(st.tokens) != 0 || len(st.families) != 0 {
			t.Errorf("hata durumunda aile/token kaydedilmemeli: %d token, %d aile", len(st.tokens), len(st.families))
		}
	})
}

// TestRefreshDeletedUser: refresh token verildikten sonra kullanıcı silindiyse
// UserByID ErrUserNotFound döner → Refresh ErrInvalidToken dönmeli.
func TestRefreshDeletedUser(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()
	u, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	st.mu.Lock()
	delete(st.users, u.ID)
	delete(st.byEmail, u.Email)
	st.mu.Unlock()

	if _, err := s.Refresh(ctx, sess.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("silinmiş kullanıcı için ErrInvalidToken bekleniyordu, gelen %v", err)
	}
}

func TestMe(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()

	t.Run("db.User → User eşlemesi", func(t *testing.T) {
		created := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
		st.mu.Lock()
		st.users[42] = db.User{ID: 42, Email: "veli@example.com", DisplayName: "Veli", PasswordHash: "gizli-hash", Role: RoleAdmin, CreatedAt: created}
		st.mu.Unlock()

		got, err := s.Me(ctx, 42)
		if err != nil {
			t.Fatalf("Me: %v", err)
		}
		want := User{ID: 42, Email: "veli@example.com", DisplayName: "Veli", Role: RoleAdmin, CreatedAt: created}
		if got != want {
			t.Errorf("Me = %+v, beklenen %+v", got, want)
		}
	})

	t.Run("ErrUserNotFound olduğu gibi geçer", func(t *testing.T) {
		if _, err := s.Me(ctx, 9999); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("ErrUserNotFound bekleniyordu, gelen %v", err)
		}
	})
}

// errStore, memStore'u sarar ve seçilen metotlarda ALTYAPI hatası döndürür (DB kesintisi).
// Bu hataların hiçbiri "kimlik bilgisi yanlış" ya da "token geçersiz"e dönüşmemeli:
// aksi hâlde bir DB kesintisi kullanıcıya 401 olarak görünür ve oturumlar düşer.
type errStore struct {
	*memStore
	byEmailErr, byIDErr, rotateErr, startErr, revokeErr, changeErr error
}

func (s *errStore) UserByEmail(ctx context.Context, email string) (db.User, error) {
	if s.byEmailErr != nil {
		return db.User{}, s.byEmailErr
	}
	return s.memStore.UserByEmail(ctx, email)
}

func (s *errStore) UserByID(ctx context.Context, id int64) (db.User, error) {
	if s.byIDErr != nil {
		return db.User{}, s.byIDErr
	}
	return s.memStore.UserByID(ctx, id)
}

func (s *errStore) RotateRefreshToken(ctx context.Context, oldHash, newHash []byte, now, newExp time.Time) (Rotation, error) {
	if s.rotateErr != nil {
		return Rotation{}, s.rotateErr
	}
	return s.memStore.RotateRefreshToken(ctx, oldHash, newHash, now, newExp)
}

func (s *errStore) StartRefreshFamily(ctx context.Context, userID int64, verifiedHash string, h []byte, tokenExp, familyExp time.Time) error {
	if s.startErr != nil {
		return s.startErr
	}
	return s.memStore.StartRefreshFamily(ctx, userID, verifiedHash, h, tokenExp, familyExp)
}

func (s *errStore) ChangePassword(ctx context.Context, c PasswordChange) (int64, error) {
	if s.changeErr != nil {
		return 0, s.changeErr
	}
	return s.memStore.ChangePassword(ctx, c)
}

func (s *errStore) RevokeRefreshFamily(ctx context.Context, h []byte, now time.Time) error {
	if s.revokeErr != nil {
		return s.revokeErr
	}
	return s.memStore.RevokeRefreshFamily(ctx, h, now)
}

// TestInfraErrorsAreNotAuthErrors: altyapı hatası olduğu gibi (sarılı) yukarı çıkar;
// ErrInvalidCredentials / ErrInvalidToken olarak MASKELENMEZ ve oturum üretilmez.
func TestInfraErrorsAreNotAuthErrors(t *testing.T) {
	ctx := context.Background()
	dbDown := errors.New("db down")

	setup := func(t *testing.T) (*Service, *errStore, Session) {
		t.Helper()
		s, st, _ := newTestService(t)
		if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
			t.Fatalf("Register: %v", err)
		}
		sess, err := s.Login(ctx, "ali@example.com", goodPassword)
		if err != nil {
			t.Fatalf("Login: %v", err)
		}
		es := &errStore{memStore: st}
		s.store = es
		return s, es, sess
	}
	assertInfra := func(t *testing.T, sess Session, err error, masked error) {
		t.Helper()
		if !errors.Is(err, dbDown) {
			t.Errorf("altyapı hatası olduğu gibi dönmeli, gelen %v", err)
		}
		if errors.Is(err, masked) {
			t.Errorf("altyapı hatası %v olarak maskelenmemeli", masked)
		}
		if sess.AccessToken != "" || sess.RefreshToken != "" {
			t.Errorf("hata durumunda oturum dönmemeli: %+v", sess)
		}
	}

	t.Run("Login: UserByEmail kesintisi", func(t *testing.T) {
		s, es, _ := setup(t)
		es.byEmailErr = dbDown
		sess, err := s.Login(ctx, "ali@example.com", goodPassword)
		assertInfra(t, sess, err, ErrInvalidCredentials)
	})
	t.Run("Login: refresh token ailesi kaydedilemedi", func(t *testing.T) {
		s, es, _ := setup(t)
		es.startErr = dbDown
		sess, err := s.Login(ctx, "ali@example.com", goodPassword)
		assertInfra(t, sess, err, ErrInvalidCredentials)
	})
	// Eskiden "tüketme kesintisi" ve "yeni token kaydedilemedi" iki ayrı alt-testti (iki ayrı
	// store çağrısı). Faz 13'te harcama + halefin kaydı TEK atomik çağrı (RotateRefreshToken,
	// tek tx) — ikisinin kesintisi servise aynı yoldan gelir; tek alt-test ikisini de kapsar.
	t.Run("Refresh: rotasyon (harcama + halef) kesintisi", func(t *testing.T) {
		s, es, old := setup(t)
		es.rotateErr = dbDown
		sess, err := s.Refresh(ctx, old.RefreshToken)
		assertInfra(t, sess, err, ErrInvalidToken)
	})
	// "Refresh: kullanıcı okunamadı" alt-testi KALDIRILDI (bulgu F02): kullanıcı artık
	// rotasyonun KENDİ transaction'ında okunuyor, servisin ayrı bir UserByID çağrısı yok.
	// O okumanın kesintisi yukarıdaki rotasyon kesintisiyle aynı yoldan gelir ve — asıl kazanç —
	// rollback sayesinde halef token hiç yazılmaz. Yerine geçen iddia:
	// TestRefreshDoesNotReadUserAfterCommit (UserByID patlasa bile yenileme çalışır).
	t.Run("Me: kullanıcı okunamadı", func(t *testing.T) {
		s, es, _ := setup(t)
		es.byIDErr = dbDown
		u, err := s.Me(ctx, 1)
		if !errors.Is(err, dbDown) {
			t.Errorf("altyapı hatası olduğu gibi dönmeli, gelen %v", err)
		}
		if u.ID != 0 {
			t.Errorf("hata durumunda kullanıcı dönmemeli: %+v", u)
		}
	})
	t.Run("Logout: iptal kesintisi olduğu gibi döner", func(t *testing.T) {
		s, es, old := setup(t)
		es.revokeErr = dbDown
		// Logout'un "idempotent, hep başarılı" sözü BİLİNMEYEN token içindir; DB kesintisi
		// yutulursa kullanıcı çıktığını sanır ama ailesi yaşamaya devam eder.
		if err := s.Logout(ctx, old.RefreshToken); !errors.Is(err, dbDown) {
			t.Errorf("iptal kesintisi yutulmamalı, gelen %v", err)
		}
	})
	// DB kesintisi "mevcut parola hatalı"ya dönüşmemeli: kullanıcı doğru parolayla 403 görürdü.
	t.Run("ChangePassword: kesinti olduğu gibi döner, ErrWrongPassword değil", func(t *testing.T) {
		s, es, _ := setup(t)
		es.changeErr = dbDown
		sess, err := s.ChangePassword(ctx, 1, goodPassword, "yepyeni-bir-parola")
		assertInfra(t, sess, err, ErrWrongPassword)
	})
}

// captureLogs, testin süresince varsayılan slog'u JSON olarak bir tampona yönlendirir
// ("parola/token loga düşmedi" iddiaları için).
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// TestChangePassword (Faz 13): parola değişiminin servis kuralları.
func TestChangePassword(t *testing.T) {
	ctx := context.Background()
	const newPassword = "yepyeni-bir-parola"
	setup := func(t *testing.T) (*Service, *memStore, User) {
		t.Helper()
		s, st, _ := newTestService(t)
		u, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword)
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		return s, st, u
	}
	// assertUnchanged: eski parola hâlâ geçer, yenisi geçmez (değişim OLMADI).
	assertUnchanged := func(t *testing.T, s *Service) {
		t.Helper()
		if _, err := s.Login(ctx, "ali@example.com", goodPassword); err != nil {
			t.Errorf("başarısız değişimden sonra eski parola çalışmalı: %v", err)
		}
		if _, err := s.Login(ctx, "ali@example.com", newPassword); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("başarısız değişimden sonra yeni parola GEÇMEMELİ, gelen %v", err)
		}
	}

	t.Run("yanlış mevcut parola → ErrWrongPassword, hiçbir şey değişmez", func(t *testing.T) {
		s, st, u := setup(t)
		if _, err := s.Login(ctx, "ali@example.com", goodPassword); err != nil {
			t.Fatalf("Login: %v", err)
		}
		sess, err := s.ChangePassword(ctx, u.ID, "yanlis-parola", newPassword)
		if !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("ErrWrongPassword bekleniyordu, gelen %v", err)
		}
		if sess.AccessToken != "" || sess.RefreshToken != "" {
			t.Errorf("hata durumunda oturum dönmemeli: %+v", sess)
		}
		// Yanlış parola, var olan oturumları da ÖLDÜRMEMELİ (aksi hâlde çalınmış bir access
		// token'la yanlış parola yollayan biri kurbanı her cihazdan düşürebilirdi).
		for id, fam := range st.families {
			if fam.revoked {
				t.Errorf("aile %d yanlış parolayla iptal edilmemeliydi", id)
			}
		}
		assertUnchanged(t, s)
	})

	// bcrypt karşılaştırması 72 baytta KIRPAR: "72 baytlık gerçek parola + sonek" doğru sayılırdı.
	// Kayıt >72'yi reddettiği için böyle bir mevcut parola hiçbir hesabın olamaz → hatalı.
	t.Run("72 bayttan uzun mevcut parola → ErrWrongPassword (kırpma tuzağı)", func(t *testing.T) {
		s, _, _ := newTestService(t)
		pw := strings.Repeat("p", MaxPasswordBytes)
		u, err := s.Register(ctx, "ali@example.com", "Ali", pw)
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		if _, err := s.ChangePassword(ctx, u.ID, pw+"SONEK", newPassword); !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("72 bayt + sonek reddedilmeli, gelen %v", err)
		}
		if _, err := s.Login(ctx, "ali@example.com", newPassword); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("parola DEĞİŞMEMİŞ olmalı, gelen %v", err)
		}
		// Tam 72 bayt hâlâ meşru bir mevcut parola.
		if _, err := s.ChangePassword(ctx, u.ID, pw, newPassword); err != nil {
			t.Errorf("tam 72 baytlık mevcut parola çalışmalı: %v", err)
		}
	})

	t.Run("yeni parola kayıtla AYNI politikaya tabi", func(t *testing.T) {
		tests := []struct{ name, next, wantProblem string }{
			{"kısa", "1234567", "password en az"},
			{"72 bayttan uzun", strings.Repeat("ş", 37), "password en fazla 72 bayt"},
			{"mevcutla aynı", goodPassword, "farklı olmalı"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s, _, u := setup(t)
				_, err := s.ChangePassword(ctx, u.ID, goodPassword, tt.next)
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("ValidationError bekleniyordu, gelen %v", err)
				}
				if !strings.Contains(ve.Error(), tt.wantProblem) {
					t.Errorf("mesaj %q içermeli, gelen %q", tt.wantProblem, ve.Error())
				}
				if _, err := s.Login(ctx, "ali@example.com", goodPassword); err != nil {
					t.Errorf("reddedilen değişim parolayı değiştirmemeli: %v", err)
				}
			})
		}
	})

	// Yanlış mevcut parola + kusurlu yeni parola: cevap "mevcut parola hatalı" — yeni parolanın
	// kusuru, kimlik kanıtı verilmeden kullanıcıya politikayı "deneme" fırsatı sunmaz.
	t.Run("yanlış mevcut + kısa yeni → ErrWrongPassword önce gelir", func(t *testing.T) {
		s, _, u := setup(t)
		if _, err := s.ChangePassword(ctx, u.ID, "yanlis-parola", "kisa"); !errors.Is(err, ErrWrongPassword) {
			t.Errorf("ErrWrongPassword bekleniyordu, gelen %v", err)
		}
	})

	t.Run("başarı: tüm eski aileler ölür, bu cihaz yeni aileyle devam eder, eski parola geçmez", func(t *testing.T) {
		s, st, u := setup(t)
		deviceA, err := s.Login(ctx, "ali@example.com", goodPassword)
		if err != nil {
			t.Fatalf("Login A: %v", err)
		}
		deviceB, err := s.Login(ctx, "ali@example.com", goodPassword)
		if err != nil {
			t.Fatalf("Login B: %v", err)
		}
		famA, _ := st.familyOf(t, deviceA.RefreshToken)
		famB, _ := st.familyOf(t, deviceB.RefreshToken)
		logs := captureLogs(t)

		sess, err := s.ChangePassword(ctx, u.ID, goodPassword, newPassword)
		if err != nil {
			t.Fatalf("ChangePassword: %v", err)
		}
		// Bu cihaz: yeni bir oturum (access token kullanıcıyı taşır, refresh token canlı).
		if p, err := s.tokens.Verify(sess.AccessToken); err != nil || p.UserID != u.ID {
			t.Errorf("yeni access token kullanıcıyı taşımalı: %+v, %v", p, err)
		}
		if sess.RefreshToken == "" || sess.RefreshToken == deviceA.RefreshToken || sess.RefreshToken == deviceB.RefreshToken {
			t.Errorf("yeni bir refresh token bekleniyordu, gelen %q", sess.RefreshToken)
		}
		if _, err := s.Refresh(ctx, sess.RefreshToken); err != nil {
			t.Errorf("değişimin verdiği oturum yenilenebilmeli: %v", err)
		}
		// Diğer cihazlar: aileleri ölü, token'ları silinmiş, yenileme reddedilir.
		for name, fam := range map[string]int64{"A": famA, "B": famB} {
			if !st.families[fam].revoked || st.familyTokens(fam) != 0 {
				t.Errorf("cihaz %s'nin ailesi iptal edilmiş ve token'sız olmalı: revoked=%v token=%d", name, st.families[fam].revoked, st.familyTokens(fam))
			}
		}
		if _, err := s.Refresh(ctx, deviceA.RefreshToken); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("cihaz A'nın yenilemesi reddedilmeli, gelen %v", err)
		}
		if _, err := s.Refresh(ctx, deviceB.RefreshToken); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("cihaz B'nin yenilemesi reddedilmeli, gelen %v", err)
		}
		// Parola: eskisi ölü, yenisi geçerli.
		if _, err := s.Login(ctx, "ali@example.com", goodPassword); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("eski parola reddedilmeli, gelen %v", err)
		}
		if _, err := s.Login(ctx, "ali@example.com", newPassword); err != nil {
			t.Errorf("yeni parola çalışmalı: %v", err)
		}
		// Operatör izi: kim ve kaç aile; parola/hash/token ASLA.
		raw := logs.String()
		if !strings.Contains(raw, `"revoked_families":2`) || !strings.Contains(raw, fmt.Sprintf(`"user_id":%d`, u.ID)) {
			t.Errorf("INFO log user_id ve revoked_families=2 taşımalı: %s", raw)
		}
		for _, secret := range []string{goodPassword, newPassword, sess.RefreshToken, st.users[u.ID].PasswordHash} {
			if strings.Contains(raw, secret) {
				t.Errorf("sır loga düştü (%q): %s", secret, raw)
			}
		}
	})

	t.Run("kullanıcı silinmiş → ErrUserNotFound (maskelenmez)", func(t *testing.T) {
		s, st, u := setup(t)
		st.mu.Lock()
		delete(st.users, u.ID)
		st.mu.Unlock()
		if _, err := s.ChangePassword(ctx, u.ID, goodPassword, newPassword); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("ErrUserNotFound bekleniyordu, gelen %v", err)
		}
	})
}

// raceStore, UserByEmail'den SONRA (giriş bcrypt'i eski hash'le doğruladıktan sonra) parolayı
// değiştirir: "giriş sürerken başka cihazda parola değişti" yarışının bellek içi kurgusu.
type raceStore struct {
	*memStore
	newHash string
}

func (r *raceStore) UserByEmail(ctx context.Context, email string) (db.User, error) {
	u, err := r.memStore.UserByEmail(ctx, email)
	if err == nil {
		r.mu.Lock()
		stored := r.users[u.ID]
		stored.PasswordHash = r.newHash
		r.users[u.ID] = stored
		r.mu.Unlock()
	}
	return u, nil // giriş, ESKİ hash'i taşıyan kopyayla devam eder
}

// TestLoginRejectsPasswordChangedDuringLogin: bcrypt tx dışında ESKİ hash'le doğrulandı; aile
// açılırken depo hash'in değiştiğini görür → giriş reddedilir, aile AÇILMAZ. Aksi hâlde bu
// giriş, "tüm oturumları kapat"an parola değişiminin arkasından eski parolayla yaşardı.
func TestLoginRejectsPasswordChangedDuringLogin(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	s.store = &raceStore{memStore: st, newHash: "degisti"}

	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ErrInvalidCredentials bekleniyordu, gelen %v", err)
	}
	if sess.AccessToken != "" || sess.RefreshToken != "" {
		t.Errorf("hata durumunda oturum dönmemeli: %+v", sess)
	}
	if len(st.families) != 0 || len(st.tokens) != 0 {
		t.Errorf("reddedilen giriş aile/token açmamalı: %d aile, %d token", len(st.families), len(st.tokens))
	}
}

// TestRefreshDoesNotReadUserAfterCommit (inceleme bulgusu F02): rotasyon COMMIT olduktan sonra
// başarısız olabilecek hiçbir adım kalmamalı.
//
// ESKİ HÂLİ: rotasyon commit edilir, sonra kullanıcı AYRI bir UserByID sorgusuyla okunurdu. O
// sorgu patlarsa istemci 500 alır; halef token ise DB'de yazılı kalır. İstemci (elindeki tek
// anahtar olan) ESKİ çerezle tekrar dener → sunucu bunu REUSE sayar → TÜM aile iptal edilir.
// Yani veritabanının bir saniyelik sarsıntısı, kullanıcıyı tüm cihazlarından atardı.
//
// Test bunu, UserByID'yi patlatıp rotasyonu sağlam bırakarak sabitler: yenileme YİNE çalışmalı.
func TestRefreshDoesNotReadUserAfterCommit(t *testing.T) {
	s, st, _ := newTestService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "ali@example.com", "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	sess, err := s.Login(ctx, "ali@example.com", goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	st.mu.Lock()
	st.userByIDErr = errors.New("db geçici olarak düştü")
	st.mu.Unlock()

	next, err := s.Refresh(ctx, sess.RefreshToken)
	if err != nil {
		t.Fatalf("rotasyon commit olduysa yenileme başarılı olmalı (kullanıcı tx İÇİNDE okunur): %v", err)
	}
	if next.User.Email != "ali@example.com" {
		t.Errorf("oturum rotasyonun okuduğu kullanıcıyı taşımalı, gelen %+v", next.User)
	}
	// Ve yeni token gerçekten kullanılabilir olmalı (halef teslim edildi).
	if _, err := s.Refresh(ctx, next.RefreshToken); err != nil {
		t.Errorf("halef token ile ikinci yenileme de çalışmalı: %v", err)
	}
}
