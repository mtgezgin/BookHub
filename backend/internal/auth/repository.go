package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mtgezgin/bookhub/backend/internal/database"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// Repository hataları: pgx/Postgres dilini domain diline çevirir; üst katmanlar sürücüyü
// tanımaz (books/genres ile aynı desen).
var (
	ErrEmailTaken   = errors.New("bu e-posta zaten kayıtlı")
	ErrUserNotFound = errors.New("kullanıcı bulunamadı")
	// errRefreshNotFound dışarı açılmaz: servis onu ErrInvalidToken'a çevirir. "Bulunamadı"
	// geniş anlamlı: token yok, süresi dolmuş, ailesi iptal edilmiş ya da mutlak tavanı geçmiş —
	// istemcinin yapacağı hepsinde aynı (yeniden giriş), nedeni ona söylemiyoruz.
	errRefreshNotFound = errors.New("refresh token bulunamadı")
)

// ReuseError, KULLANILMIŞ bir refresh token'ın yeniden sunulduğunu söyler (reuse = çalınma
// belirtisi). Döndüğü anda ailenin tamamı zaten iptal edilmiş ve COMMIT edilmiştir.
//
// Neden ayrı bir tip: handler bu olayı güvenlik uyarısı olarak loglamak ister (errors.As ile
// kimin, hangi ailesi). Ama istemciye giden cevap düz "geçersiz token" ile BİREBİR aynı olmalı —
// farklı bir kod/mesaj, saldırgana "bu token'ı biri zaten kullanmış" diye bilgi sızdıran bir
// kâhin (oracle) olurdu. Unwrap → ErrInvalidToken sayesinde errors.Is(err, ErrInvalidToken)
// kontrolü yapan her yer onu kendiliğinden 401'e çevirir.
type ReuseError struct {
	UserID   int64
	FamilyID int64
}

func (e *ReuseError) Error() string {
	return fmt.Sprintf("kullanılmış refresh token yeniden sunuldu: kullanıcı %d, aile %d iptal edildi", e.UserID, e.FamilyID)
}

func (e *ReuseError) Unwrap() error { return ErrInvalidToken }

// usersEmailKey, email UNIQUE kısıtının Postgres'in verdiği adı.
const usersEmailKey = "users_email_key"

// Repository, kullanıcı ve refresh token kalıcılığı.
type Repository struct {
	conn db.DBTX
	q    *db.Queries
}

func NewRepository(conn db.DBTX) *Repository {
	return &Repository{conn: conn, q: db.New(conn)}
}

func (r *Repository) CreateUser(ctx context.Context, email, displayName, passwordHash string) (db.User, error) {
	u, err := r.q.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
	})
	if c, ok := database.UniqueViolation(err); ok && c == usersEmailKey {
		return db.User{}, ErrEmailTaken
	}
	if err != nil {
		return db.User{}, fmt.Errorf("kullanıcı oluşturulamadı: %w", err)
	}
	return u, nil
}

func (r *Repository) UserByEmail(ctx context.Context, email string) (db.User, error) {
	return r.userOrNotFound(r.q.GetUserByEmail(ctx, email))
}

func (r *Repository) UserByID(ctx context.Context, id int64) (db.User, error) {
	return r.userOrNotFound(r.q.GetUserByID(ctx, id))
}

func (r *Repository) userOrNotFound(u db.User, err error) (db.User, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, ErrUserNotFound
	}
	if err != nil {
		return db.User{}, fmt.Errorf("kullanıcı okunamadı: %w", err)
	}
	return u, nil
}

// EnsureAdmin, e-postayla kullanıcıyı bulur (yoksa verilen hash'le oluşturur) ve ADMIN
// yapar — hepsi TEK transaction'da. created, kullanıcının yeni açılıp açılmadığını söyler.
// Var olan kullanıcının parolasına DOKUNULMAZ; yalnızca rolü yükselir.
func (r *Repository) EnsureAdmin(ctx context.Context, email, displayName, passwordHash string) (db.User, bool, error) {
	var (
		user    db.User
		created bool
	)
	err := database.WithTx(ctx, r.conn, func(tx pgx.Tx) error {
		txRepo := NewRepository(tx)
		u, err := txRepo.UserByEmail(ctx, email)
		if errors.Is(err, ErrUserNotFound) {
			u, err = txRepo.CreateUser(ctx, email, displayName, passwordHash)
			created = true
		}
		if err != nil {
			return err
		}
		if _, err := txRepo.q.SetUserRole(ctx, db.SetUserRoleParams{ID: u.ID, Role: RoleAdmin}); err != nil {
			return fmt.Errorf("rol atanamadı: %w", err)
		}
		u.Role = RoleAdmin
		user = u
		return nil
	})
	if err != nil {
		return db.User{}, false, err
	}
	return user, created, nil
}

// Rotation, başarılı bir rotasyonun sonucu: halef token kimin, hangi ailede, ne zaman ölüyor.
type Rotation struct {
	UserID   int64
	FamilyID int64
	// ExpiresAt, halefin GERÇEK bitişi: min(istenen bitiş, ailenin mutlak tavanı). Servis bunu
	// olduğu gibi istemciye verir — istemci tavanı aşan bir süre görüp yanılmasın.
	ExpiresAt time.Time
}

// StartRefreshFamily, yeni bir giriş için aile açar ve ilk token'ını kaydeder (tek tx: token'sız
// aile ya da ailesiz token kalmaz). familyExp ailenin MUTLAK tavanıdır; tokenExp'i servis
// min(now+refreshTTL, familyExp) olarak hesaplar.
//
// verifiedHash: servisin bcrypt ile doğruladığı hash. Aileyi açmadan ÖNCE users satırını FOR
// SHARE ile kilitleyip hash'in hâlâ bu olduğunu denetliyoruz. NEDEN: doğrulama tx'in dışında
// yapıldı; o arada bir parola değişimi (ChangePassword) tüm aileleri öldürüp commit etmiş
// olabilir — bizim ailemiz iptalden SONRA doğar ve ESKİ parolayla yaşardı. FOR SHARE, parola
// değişiminin FOR NO KEY UPDATE'iyle çakışır (ikisi sıraya girer: değişim önce bittiyse burada
// YENİ hash'i okuruz ve girişi reddederiz), rotasyonların FK için aldığı FOR KEY SHARE ile
// çakışmaz (girişler rotasyonları bekletmez).
func (r *Repository) StartRefreshFamily(ctx context.Context, userID int64, verifiedHash string, tokenHash []byte, tokenExp, familyExp time.Time) error {
	return database.WithTx(ctx, r.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		current, err := q.LockUserPasswordForLogin(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound // doğrulama ile aile arasında hesap silindi
		}
		if err != nil {
			return fmt.Errorf("kullanıcı kilitlenemedi: %w", err)
		}
		if current != verifiedHash {
			return ErrInvalidCredentials // parola bu arada değişti: doğrulanan parola artık geçersiz
		}
		return startFamily(ctx, q, userID, tokenHash, tokenExp, familyExp)
	})
}

// startFamily, aile + ilk token INSERT'i (StartRefreshFamily ve ChangePassword'ün ortak parçası;
// çağıran tx'i ve ön koşulları — kilitler, hash denetimi — kendi kurar).
func startFamily(ctx context.Context, q *db.Queries, userID int64, tokenHash []byte, tokenExp, familyExp time.Time) error {
	familyID, err := q.CreateRefreshFamily(ctx, db.CreateRefreshFamilyParams{UserID: userID, AbsoluteExpiresAt: familyExp})
	if err != nil {
		return fmt.Errorf("refresh token ailesi açılamadı: %w", err)
	}
	err = q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    userID,
		FamilyID:  familyID,
		TokenHash: tokenHash,
		ExpiresAt: tokenExp,
	})
	if err != nil {
		return fmt.Errorf("refresh token kaydedilemedi: %w", err)
	}
	return nil
}

// ChangePassword, parolayı değiştirir ve kullanıcının TÜM canlı ailelerini öldürür, sonra bu
// cihaz için yeni bir aile açar — hepsi TEK transaction'da. Yarım kalmış hiçbir durum yok:
// ya parola değişti + herkes düştü + bu cihaz içeride, ya da hiçbiri.
//
// KİLİT SIRASI (users.sql'deki değişmezin bir kademe yukarıdan başlayan hâli):
//
//	a. users satırı, FOR NO KEY UPDATE (FOR UPDATE DEĞİL — nedeni users.sql'de: rotasyonun
//	   halef INSERT'i users'ta FOR KEY SHARE alır, FOR UPDATE onunla çakışıp deadlock üretirdi).
//	   Hash, servisin doğruladığı hash'ten farklıysa → ErrWrongPassword, rollback: araya giren
//	   eşzamanlı bir değişim varsa doğrulanmamış bir parolayla üzerine yazmayız.
//	b. UPDATE users (anahtar olmayan kolonlar: elimizdeki kilit yeter).
//	c. Canlı aileler id sırasıyla FOR UPDATE (uçuştaki rotasyon/iptal burada bekletir; onun
//	   commit'inden sonra taze anlık görüntüyle devam ederiz) → toplu iptal → kullanıcının
//	   tüm token'ları silinir (halefler dahil).
//	d. Bu cihaz için yeni aile + token.
func (r *Repository) ChangePassword(ctx context.Context, c PasswordChange) (int64, error) {
	var revoked int64
	err := database.WithTx(ctx, r.conn, func(tx pgx.Tx) error {
		q := db.New(tx)

		// (a)
		current, err := q.LockUserPasswordForChange(ctx, c.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("kullanıcı kilitlenemedi: %w", err)
		}
		if current != c.VerifiedHash {
			return ErrWrongPassword
		}

		// (b)
		if err := q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: c.UserID, PasswordHash: c.NewHash}); err != nil {
			return fmt.Errorf("parola güncellenemedi: %w", err)
		}

		// (c) ÖNCE aileler (deterministik sırayla kilit), SONRA token'lar.
		if _, err := q.LockUserLiveRefreshFamilies(ctx, c.UserID); err != nil {
			return fmt.Errorf("refresh token aileleri kilitlenemedi: %w", err)
		}
		revoked, err = q.RevokeUserRefreshFamilies(ctx, db.RevokeUserRefreshFamiliesParams{Now: c.Now, UserID: c.UserID})
		if err != nil {
			return fmt.Errorf("refresh token aileleri iptal edilemedi: %w", err)
		}
		if err := q.DeleteUserRefreshTokens(ctx, c.UserID); err != nil {
			return fmt.Errorf("kullanıcının refresh token'ları silinemedi: %w", err)
		}

		// (d)
		return startFamily(ctx, q, c.UserID, c.TokenHash, c.TokenExp, c.FamilyExp)
	})
	if err != nil {
		return 0, err
	}
	return revoked, nil
}

// RotateRefreshToken, eski token'ı HARCAR (used_at) ve aynı ailede bir halef kaydeder — hepsi
// TEK transaction'da. Saat (now) parametre: süre kararları servisin tek saatiyle verilir.
//
// Adımlar ve NEDENLERİ (sıra önemli — bkz. users.sql'deki kilit sırası değişmezi):
//
//	a. token'ın ailesini KİLİTSİZ bul (family_id asla değişmez).
//	b. aileyi FOR UPDATE kilitle; iptal edilmiş ya da mutlak tavanı geçmişse → bulunamadı.
//	   Aynı ailedeki her rotasyon/iptal burada sıraya girer: iki eşzamanlı rotasyondan ikincisi,
//	   birincinin commit'ini bekler ve token'ı "kullanılmış" görür.
//	c. token'ı işaretle (yalnız used_at NULL ise). 0 satır + token duruyor + used_at dolu =
//	   REUSE → aileyi öldür, COMMIT et, *ReuseError dön.
//	d. süresi dolmuşsa → bulunamadı (rollback: used_at işareti de geri alınır).
//	e. halefi kaydet; bitişi ailenin mutlak tavanını geçemez.
//
// Tek transaction neden: "harca" ile "halefi kaydet" arasında bir hata olursa ikisi birden geri
// alınır — ne halefsiz harcanmış bir token (kullanıcı sebepsiz düşer) ne de iki canlı token kalır.
func (r *Repository) RotateRefreshToken(ctx context.Context, oldHash, newHash []byte, now, newExp time.Time) (Rotation, error) {
	var (
		rot   Rotation
		reuse *ReuseError
	)
	err := database.WithTx(ctx, r.conn, func(tx pgx.Tx) error {
		q := db.New(tx)

		// (a)
		familyID, err := q.GetRefreshTokenFamily(ctx, oldHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return errRefreshNotFound
		}
		if err != nil {
			return fmt.Errorf("refresh token okunamadı: %w", err)
		}

		// (b) ÖNCE aile (ebeveyn) — token satırına ancak aile kilidi elimizdeyken dokunuyoruz.
		fam, err := q.LockRefreshFamily(ctx, familyID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errRefreshNotFound // aile bu arada silindi (temizlik / kullanıcı silme)
		}
		if err != nil {
			return fmt.Errorf("refresh token ailesi kilitlenemedi: %w", err)
		}
		if fam.RevokedAt != nil || !now.Before(fam.AbsoluteExpiresAt) {
			return errRefreshNotFound
		}

		// (c) SONRA token.
		tok, err := q.MarkRefreshTokenUsed(ctx, db.MarkRefreshTokenUsedParams{Now: now, TokenHash: oldHash})
		if errors.Is(err, pgx.ErrNoRows) {
			usedAt, err := q.GetRefreshTokenUsedAt(ctx, oldHash)
			if errors.Is(err, pgx.ErrNoRows) {
				return errRefreshNotFound
			}
			if err != nil {
				return fmt.Errorf("refresh token okunamadı: %w", err)
			}
			if usedAt == nil {
				// Aile kilidi elimizdeyken "duruyor, kullanılmamış ama işaretlenemedi" olamaz;
				// yine de olursa KAPALI başarısız ol (token'ı geçerli sayma).
				return errRefreshNotFound
			}
			// REUSE. Hangisinin hırsız olduğunu bilemeyiz → ailenin tamamı ölür.
			if err := revokeFamily(ctx, q, familyID, now); err != nil {
				return err
			}
			// TUZAK: burada hatayı DÖNMÜYORUZ. WithTx, closure hata dönerse ROLLBACK eder — iptal
			// de geri alınırdı ve "tespit ettik" dediğimiz aile yaşamaya devam ederdi. Sonucu
			// dıştaki değişkene yazıp nil dönüyoruz (COMMIT); hatayı WithTx'ten SONRA veriyoruz.
			reuse = &ReuseError{UserID: fam.UserID, FamilyID: familyID}
			return nil
		}
		if err != nil {
			return fmt.Errorf("refresh token kullanılamadı: %w", err)
		}

		// (d) Süre kontrolü işaretten SONRA ama aynı tx'te: süresi dolmuş token'ı geri çeviririz
		// ve rollback used_at'i de siler (süresi dolmuş token "harcanmış" sayılmaz).
		if !now.Before(tok.ExpiresAt) {
			return errRefreshNotFound
		}

		// (e) Halef. Kayan pencere (newExp) ailenin mutlak tavanını geçemez.
		exp := earliest(newExp, fam.AbsoluteExpiresAt)
		err = q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
			UserID:    tok.UserID,
			FamilyID:  familyID,
			TokenHash: newHash,
			ExpiresAt: exp,
		})
		if err != nil {
			return fmt.Errorf("yeni refresh token kaydedilemedi: %w", err)
		}
		rot = Rotation{UserID: tok.UserID, FamilyID: familyID, ExpiresAt: exp}
		return nil
	})
	if err != nil {
		return Rotation{}, err
	}
	if reuse != nil {
		return Rotation{}, reuse
	}
	return rot, nil
}

// RevokeRefreshFamily, token'ın AİLESİNİN tamamını öldürür (çıkış). İdempotent: bilinmeyen
// token nil döner. Yalnız sunulan token'ı değil aileyi öldürmek neden: aynı oturumun eski bir
// kopyası (ör. çalınmış ve zaten döndürülmüş) çıkıştan sonra yaşamasın.
func (r *Repository) RevokeRefreshFamily(ctx context.Context, tokenHash []byte, now time.Time) error {
	return database.WithTx(ctx, r.conn, func(tx pgx.Tx) error {
		q := db.New(tx)
		familyID, err := q.GetRefreshTokenFamily(ctx, tokenHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("refresh token okunamadı: %w", err)
		}
		return revokeFamily(ctx, q, familyID, now)
	})
}

// revokeFamily, aileyi öldürür: ÖNCE aile satırı (UPDATE onu kilitler), SONRA token'lar.
//
// Sıra neden hayati: aynı anda bir rotasyon aile kilidini tutuyor ve halef token'ını yazıyor
// olabilir. UPDATE o kilidi bekler; rotasyon commit edince DELETE YENİ bir anlık görüntüyle
// koşar ve halefi de görüp siler. Ters sırada DELETE, halefi henüz görmeyen eski bir anlık
// görüntüyle koşardı → halef hayatta kalır, "öldürülmüş" aile yaşamaya devam ederdi
// (repository_integration_test.go'daki kilit testi bunu deterministik olarak yakalar).
func revokeFamily(ctx context.Context, q *db.Queries, familyID int64, now time.Time) error {
	if err := q.RevokeRefreshFamily(ctx, db.RevokeRefreshFamilyParams{ID: familyID, Now: now}); err != nil {
		return fmt.Errorf("refresh token ailesi iptal edilemedi: %w", err)
	}
	if err := q.DeleteRefreshFamilyTokens(ctx, familyID); err != nil {
		return fmt.Errorf("ailenin refresh token'ları silinemedi: %w", err)
	}
	return nil
}

// CleanupRefreshTokens, girişte kullanıcının ölü ailelerini ve canlı ailelerdeki süresi dolmuş
// token'ları süpürür (ayrı bir zamanlanmış iş yerine bu bağlamda sınırlı, ucuz bir temizlik).
//
// İki deyim BİLEREK ayrı (transaction yok): her biri tek başına doğru ve tekrar çalıştırılabilir;
// tek tx'e koymak yalnızca kilitleri daha uzun tutardı. Kilitli (o an işlem gören) aileler iki
// adımda da ATLANIR — bkz. users.sql'deki SKIP LOCKED açıklamaları.
func (r *Repository) CleanupRefreshTokens(ctx context.Context, userID int64, now time.Time) error {
	if err := r.q.DeleteDeadRefreshFamilies(ctx, db.DeleteDeadRefreshFamiliesParams{UserID: userID, Now: now}); err != nil {
		return fmt.Errorf("ölü refresh token aileleri silinemedi: %w", err)
	}
	if err := r.q.DeleteExpiredRefreshTokens(ctx, db.DeleteExpiredRefreshTokensParams{UserID: userID, Now: now}); err != nil {
		return fmt.Errorf("süresi dolmuş refresh token'lar silinemedi: %w", err)
	}
	return nil
}
