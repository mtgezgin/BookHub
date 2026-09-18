package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// memStore, store'un bellek içi sahtesi: gerçek DB'nin SÖZLEŞMESİNİ taklit eder
// (tekil e-posta, tek kullanımlık token) ama SQL'siz. Sözleşmenin gerçek DB'de de
// tuttuğunu repository_integration_test.go kanıtlıyor.
type memStore struct {
	mu      sync.Mutex
	nextID  int64
	users   map[int64]db.User
	byEmail map[string]int64
	tokens  map[string]memToken // anahtar: string(tokenHash)
}

type memToken struct {
	userID    int64
	expiresAt time.Time
}

func newMemStore() *memStore {
	return &memStore{users: map[int64]db.User{}, byEmail: map[string]int64{}, tokens: map[string]memToken{}}
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

func (m *memStore) SaveRefreshToken(_ context.Context, userID int64, h []byte, exp time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[string(h)] = memToken{userID: userID, expiresAt: exp}
	return nil
}

func (m *memStore) ConsumeRefreshToken(_ context.Context, h []byte) (int64, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tok, ok := m.tokens[string(h)]
	if !ok {
		return 0, time.Time{}, errRefreshNotFound
	}
	delete(m.tokens, string(h))
	return tok.userID, tok.expiresAt, nil
}

func (m *memStore) DeleteRefreshToken(_ context.Context, h []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, string(h))
	return nil
}

func (m *memStore) DeleteExpiredRefreshTokens(context.Context, int64) error { return nil }

// newTestService: hızlı bcrypt (MinCost) ve kontrol edilebilir saat.
func newTestService(t *testing.T) (*Service, *memStore, *time.Time) {
	t.Helper()
	tokens, err := NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	st := newMemStore()
	now := time.Now()
	s := NewService(st, tokens, 24*time.Hour)
	s.hashCost = bcrypt.MinCost
	s.now = func() time.Time { return now }
	return s, st, &now
}

const goodPassword = "yeterince-uzun-parola"

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
	s, _, _ := newTestService(t)
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

	// Eski token artık tüketildi: ikinci kullanım reddedilir.
	if _, err := s.Refresh(ctx, first.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("eski token reddedilmeli, gelen %v", err)
	}
	if _, err := s.Refresh(ctx, second.RefreshToken); err != nil {
		t.Errorf("yeni token çalışmalı: %v", err)
	}
	if _, err := s.Refresh(ctx, "hic-verilmemis-token"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("bilinmeyen token reddedilmeli, gelen %v", err)
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
