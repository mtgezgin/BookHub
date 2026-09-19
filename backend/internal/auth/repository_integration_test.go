package auth

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// uniqueEmail, testler arası çakışmayan bir e-posta üretir ve test bitince kullanıcıyı
// siler (refresh token'lar CASCADE ile gider).
func uniqueEmail(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	email := testdb.Unique("user") + "@example.com"
	t.Cleanup(func() { testdb.Exec(t, pool, "DELETE FROM users WHERE email = $1", email) })
	return email
}

func TestRepositoryUsersRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	email := uniqueEmail(t, pool)

	u, err := repo.CreateUser(ctx, email, "Ali", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Rol kolonu INSERT'te yok → DB varsayılanı: en dar yetki.
	if u.Role != RoleUser {
		t.Errorf("yeni kullanıcı USER doğmalı, gelen %q", u.Role)
	}

	if _, err := repo.CreateUser(ctx, email, "İkiz", "hash"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("ErrEmailTaken bekleniyordu, gelen %v", err)
	}
	if _, err := repo.UserByEmail(ctx, "yok-"+email); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("ErrUserNotFound bekleniyordu, gelen %v", err)
	}
	got, err := repo.UserByID(ctx, u.ID)
	if err != nil || got.Email != email {
		t.Errorf("UserByID: %+v, %v", got, err)
	}
}

// TestRepositoryEmailMustBeLowercase: normalizasyonu unutan bir kod yolu DB'de durdurulur.
func TestRepositoryEmailMustBeLowercase(t *testing.T) {
	pool := testdb.Pool(t)
	email := uniqueEmail(t, pool)
	_, err := NewRepository(pool).CreateUser(context.Background(), "ALI-"+email, "Ali", "hash")
	if err == nil {
		testdb.Exec(t, pool, "DELETE FROM users WHERE email = $1", "ALI-"+email)
		t.Fatal("büyük harfli e-posta CHECK kısıtını geçti")
	}
}

func TestRepositoryEnsureAdminRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	email := uniqueEmail(t, pool)

	u, created, err := repo.EnsureAdmin(ctx, email, "Admin", "ilk-hash")
	if err != nil || !created || u.Role != RoleAdmin {
		t.Fatalf("yeni admin: %+v created=%v err=%v", u, created, err)
	}

	u2, created, err := repo.EnsureAdmin(ctx, email, "Admin", "ikinci-hash")
	if err != nil || created || u2.ID != u.ID {
		t.Fatalf("var olan yükseltilmeli: %+v created=%v err=%v", u2, created, err)
	}
	stored, err := repo.UserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if stored.Role != RoleAdmin || stored.PasswordHash != "ilk-hash" {
		t.Errorf("rol ADMIN, parola DEĞİŞMEMİŞ olmalı: role=%q hash=%q", stored.Role, stored.PasswordHash)
	}
}

// ---------------------------------------------------------------------------------------------
// Refresh token aileleri (Faz 13) — gerçek DB'de sözleşme ve EŞZAMANLILIK.
//
// Kilit testlerinin kuralları (hepsi deterministik):
//   - time.Sleep YOK. "Diğer işlem şimdi bekliyor mu?" sorusunu Postgres'e soruyoruz:
//     pg_blocking_pids(pid), o backend'i bekleten kilitlerin sahiplerini döner. Böylece
//     "300 ms yeter herhalde" tahmini yerine "rotasyon ŞU AN testin kilidinde bekliyor"
//     gerçeğine göre ilerliyoruz.
//   - Her bekleme context.WithTimeout(lockWait) ile sınırlı — kurulum/fixture sorguları da:
//     bir kilitlenme testi go test'in 10 dakikalık zaman aşımına kadar asmaz, lockWait'te düşürür.
//   - Testin KENDİ transaction'ı havuzun dışında, pgx.Connect ile açılan ayrı bir bağlantıda:
//     pid'i kesin bilinir ve üretim kodunun havuzuyla karışmaz.
// ---------------------------------------------------------------------------------------------

// lockWait, kilit testlerindeki her beklemenin üst sınırı.
const lockWait = 10 * time.Second

// createUser, test kullanıcısı açar (test bitince CASCADE ile aileleri/token'larıyla silinir).
func createUser(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	u, err := NewRepository(pool).CreateUser(ctx, uniqueEmail(t, pool), "Ali", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u.ID
}

// liveFamily, üretim yollarıyla kurulmuş bir aile: A harcanmış (döndürülmüş), B canlı halefi.
type liveFamily struct {
	userID, familyID int64
	rawA, rawB       string
}

func newLiveFamily(t *testing.T, pool *pgxpool.Pool) liveFamily {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	repo := NewRepository(pool)
	f := liveFamily{userID: createUser(t, pool), rawA: testdb.Unique("tok-a"), rawB: testdb.Unique("tok-b")}
	now := time.Now()
	if err := repo.StartRefreshFamily(ctx, f.userID, "hash", hashRefreshToken(f.rawA), now.Add(time.Hour), now.Add(24*time.Hour)); err != nil {
		t.Fatalf("StartRefreshFamily: %v", err)
	}
	rot, err := repo.RotateRefreshToken(ctx, hashRefreshToken(f.rawA), hashRefreshToken(f.rawB), now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateRefreshToken (A→B): %v", err)
	}
	f.familyID = rot.FamilyID
	return f
}

// fixtureToken / insertFamily: üretim yollarının bilerek üretmediği durumları (iptal edilmiş
// ama token'lı aile, tavanı geçmiş aile...) DOĞRUDAN SQL ile kurar — temizlik testleri için.
type fixtureToken struct {
	hash []byte
	exp  time.Time
	used bool
}

func insertFamily(t *testing.T, pool *pgxpool.Pool, userID int64, absExp time.Time, revoked bool, toks ...fixtureToken) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	var revokedAt *time.Time
	if revoked {
		revokedAt = &absExp
	}
	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO refresh_token_families (user_id, absolute_expires_at, revoked_at) VALUES ($1, $2, $3) RETURNING id`,
		userID, absExp, revokedAt).Scan(&id)
	if err != nil {
		t.Fatalf("aile eklenemedi: %v", err)
	}
	for _, tok := range toks {
		var usedAt *time.Time
		if tok.used {
			usedAt = &tok.exp
		}
		_, err := pool.Exec(ctx,
			`INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, used_at) VALUES ($1, $2, $3, $4, $5)`,
			userID, id, tok.hash, tok.exp, usedAt)
		if err != nil {
			t.Fatalf("token eklenemedi: %v", err)
		}
	}
	return id
}

// familyState, aileyi havuzdan TAZE okur (üretim kodunun tx'inden bağımsız bir bağlantıyla).
func familyState(t *testing.T, pool *pgxpool.Pool, familyID int64) (exists, revoked bool, tokens int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	err := pool.QueryRow(ctx, `
		SELECT true, f.revoked_at IS NOT NULL,
		       (SELECT count(*) FROM refresh_tokens t WHERE t.family_id = f.id)::int
		FROM refresh_token_families f WHERE f.id = $1`, familyID).Scan(&exists, &revoked, &tokens)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, 0
	}
	if err != nil {
		t.Fatalf("aile okunamadı: %v", err)
	}
	return exists, revoked, tokens
}

func tokenExists(t *testing.T, pool *pgxpool.Pool, hash []byte) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM refresh_tokens WHERE token_hash = $1", hash).Scan(&n); err != nil {
		t.Fatalf("sayım sorgusu: %v", err)
	}
	return n == 1
}

// assertFamilyDead: aile iptal edilmiş VE hiç token'ı kalmamış olmalı.
func assertFamilyDead(t *testing.T, pool *pgxpool.Pool, familyID int64) {
	t.Helper()
	exists, revoked, n := familyState(t, pool, familyID)
	if !exists || !revoked || n != 0 {
		t.Errorf("aile %d ölü olmalı (var, iptal edilmiş, 0 token): var=%v iptal=%v token=%d", familyID, exists, revoked, n)
	}
}

// dedicatedTx, testin kendi transaction'ını havuzun DIŞINDA, ayrı bir bağlantıda açar ve
// backend pid'ini döner. Temizlik (LIFO) önce bu bağlantıyı kapatır → tutulan kilitler, test
// yarıda düşse bile kullanıcı silme temizliğinden ÖNCE bırakılır.
func dedicatedTx(t *testing.T) (pgx.Tx, int32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	conn, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("ayrı bağlantı açılamadı: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	return tx, int32(conn.PgConn().PID())
}

// async, bir üretim çağrısını goroutine'de koşturur; bitince done kapanır ve err okunur.
// (err, close(done)'dan ÖNCE yazılır; <-done'dan sonra okumak yarışsızdır.)
type async struct {
	done chan struct{}
	err  error
}

// goAsync: çağrının kendi context'i de lockWait ile sınırlı — bir mutation kilitlenmeye yol
// açsa bile goroutine sonsuza dek asılı kalmaz (pgx beklenen sorguyu iptal eder).
func goAsync(fn func(ctx context.Context) error) *async {
	a := &async{done: make(chan struct{})}
	go func() {
		defer close(a.done)
		ctx, cancel := context.WithTimeout(context.Background(), lockWait)
		defer cancel()
		a.err = fn(ctx)
	}()
	return a
}

func (a *async) wait(t *testing.T, what string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	select {
	case <-a.done:
		return a.err
	case <-ctx.Done():
		t.Fatalf("%s %s içinde bitmedi", what, lockWait)
		return nil
	}
}

// blockedBy, holder pid'inin tuttuğu bir kilidi BEKLEYEN backend'i yoklar (pg_blocking_pids)
// ve pid'ini döner. op bu arada bloklanmadan biterse (0, false). Zaman aşımı testi düşürür.
// Ticker bir uyku değil: her tur ya sonucu, ya op'un bitişini ya da context'in iptalini bekler.
func blockedBy(t *testing.T, pool *pgxpool.Pool, holder int32, op *async) (int32, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var pid int32
		err := pool.QueryRow(ctx,
			`SELECT pid FROM pg_stat_activity WHERE $1::integer = ANY(pg_blocking_pids(pid)) LIMIT 1`,
			holder).Scan(&pid)
		switch {
		case err == nil:
			return pid, true
		case !errors.Is(err, pgx.ErrNoRows):
			t.Fatalf("pg_blocking_pids yoklaması (holder %d): %v", holder, err)
		}
		select {
		case <-op.done:
			return 0, false
		case <-ctx.Done():
			t.Fatalf("%s içinde pid %d'nin blokladığı bir işlem görünmedi", lockWait, holder)
		case <-tick.C:
		}
	}
}

// mustBlockedBy: op'un holder'ın kilidinde beklediğini GÖRMEDEN ilerleme; bloklanmadan
// bittiyse senaryo kurulamamış demektir → testi düşür (hatasıyla birlikte).
func mustBlockedBy(t *testing.T, pool *pgxpool.Pool, holder int32, op *async, what string) int32 {
	t.Helper()
	pid, ok := blockedBy(t, pool, holder, op)
	if !ok {
		t.Fatalf("%s, pid %d'nin kilidini beklemeden bitti (err=%v) — senaryo kurulamadı", what, holder, op.err)
	}
	return pid
}

func isDeadlock(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40P01"
}

// TestRotateRefreshTokenReuseRevokesFamilyRealDB (T0): X→X2 döndürülür, X tekrar gelir →
// *ReuseError (ErrInvalidToken'ı sarar) ve aile ÖLÜR — bunu üretim tx'inden bağımsız, taze bir
// sorguyla doğruluyoruz. Reuse dalı hatayı WithTx'in closure'ından dönseydi iptal ROLLBACK
// olurdu: hata yine dönerdi ama aile yaşardı. Bu test tam o tuzağı yakalar.
func TestRotateRefreshTokenReuseRevokesFamilyRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	userID := createUser(t, pool)
	x, x2 := hashRefreshToken(testdb.Unique("x")), hashRefreshToken(testdb.Unique("x2"))
	now := time.Now()
	if err := repo.StartRefreshFamily(ctx, userID, "hash", x, now.Add(time.Hour), now.Add(24*time.Hour)); err != nil {
		t.Fatalf("StartRefreshFamily: %v", err)
	}
	rot, err := repo.RotateRefreshToken(ctx, x, x2, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateRefreshToken X→X2: %v", err)
	}
	if rot.UserID != userID {
		t.Errorf("Rotation.UserID: istenen %d, gelen %d", userID, rot.UserID)
	}

	_, err = repo.RotateRefreshToken(ctx, x, hashRefreshToken(testdb.Unique("x3")), now, now.Add(time.Hour))
	var reuse *ReuseError
	if !errors.As(err, &reuse) || !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("*ReuseError (ErrInvalidToken'ı saran) bekleniyordu, gelen %v", err)
	}
	if reuse.UserID != userID || reuse.FamilyID != rot.FamilyID {
		t.Errorf("ReuseError kimliği: istenen user=%d family=%d, gelen %+v", userID, rot.FamilyID, reuse)
	}
	assertFamilyDead(t, pool, rot.FamilyID)

	_, err = repo.RotateRefreshToken(ctx, x2, hashRefreshToken(testdb.Unique("x4")), now, now.Add(time.Hour))
	if !errors.Is(err, errRefreshNotFound) || errors.As(err, &reuse) {
		t.Errorf("meşru halef X2 artık düz 'bulunamadı' olmalı, gelen %v", err)
	}
}

// TestRevokeWaitsForInFlightRotationRealDB (T1): iptal, UÇUŞTAKİ bir rotasyonun halef
// INSERT'inden SONRA gelir ve o halefi de öldürmelidir.
//
// Kurgu: F ailesinde A (harcanmış) + B (canlı). Test, ayrı bağlantıdaki tx'inde halefle AYNI
// hash'li commit'lenmemiş bir satır ekler (geçici başka bir aileye). Üretim rotasyonu B→halef
// goroutine'de koşar ve INSERT'te UNIQUE index'te bekler — F'nin ve B'nin kilidi ELİNDEYKEN.
// İkinci goroutine üretim iptalini başlatır (A'nın reuse'u ya da B ile çıkış) ve rotasyonun
// kilidinde bekler. Sonra test tx'i ROLLBACK eder: rotasyon halefi yazıp commit eder, iptal
// devam eder.
//
// Deterministik olarak kırılır: rotasyonda aile FOR UPDATE ile kilitlenmezse ya da iptal token
// DELETE'ini aile UPDATE'inden ÖNCE koşturursa, DELETE'in anlık görüntüsü henüz commit
// edilmemiş halefi görmez → halef hayatta kalır ve "iptal edilmiş" aile yaşar.
func TestRevokeWaitsForInFlightRotationRealDB(t *testing.T) {
	pool := testdb.Pool(t) // DB yoksa alt-testler değil testin kendisi atlansın
	repo := NewRepository(pool)
	tests := []struct {
		name   string
		revoke func(ctx context.Context, repo *Repository, f liveFamily) error
		check  func(t *testing.T, err error)
	}{
		{
			name: "reuse: harcanmış A yeniden geliyor",
			revoke: func(ctx context.Context, repo *Repository, f liveFamily) error {
				now := time.Now()
				_, err := repo.RotateRefreshToken(ctx, hashRefreshToken(f.rawA), hashRefreshToken(testdb.Unique("z")), now, now.Add(time.Hour))
				return err
			},
			check: func(t *testing.T, err error) {
				var reuse *ReuseError
				if !errors.As(err, &reuse) {
					t.Errorf("A'nın reuse'u *ReuseError dönmeli, gelen %v", err)
				}
			},
		},
		{
			name: "çıkış: B ile Logout",
			revoke: func(ctx context.Context, repo *Repository, f liveFamily) error {
				return repo.RevokeRefreshFamily(ctx, hashRefreshToken(f.rawB), time.Now())
			},
			check: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("RevokeRefreshFamily: %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newLiveFamily(t, pool)
			successor := hashRefreshToken(testdb.Unique("halef"))
			decoy := insertFamily(t, pool, f.userID, time.Now().Add(time.Hour), false)

			tx, testPid := dedicatedTx(t)
			ctx, cancel := context.WithTimeout(context.Background(), lockWait)
			defer cancel()
			if _, err := tx.Exec(ctx,
				`INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at) VALUES ($1, $2, $3, now() + interval '1 hour')`,
				f.userID, decoy, successor); err != nil {
				t.Fatalf("engelleyici satır: %v", err)
			}

			rotation := goAsync(func(ctx context.Context) error {
				now := time.Now()
				_, err := repo.RotateRefreshToken(ctx, hashRefreshToken(f.rawB), successor, now, now.Add(time.Hour))
				return err
			})
			rotPid := mustBlockedBy(t, pool, testPid, rotation, "rotasyon")

			revocation := goAsync(func(ctx context.Context) error { return tt.revoke(ctx, repo, f) })
			mustBlockedBy(t, pool, rotPid, revocation, "iptal")

			if err := tx.Rollback(ctx); err != nil {
				t.Fatalf("Rollback: %v", err)
			}
			if err := rotation.wait(t, "rotasyon"); err != nil {
				t.Errorf("rotasyon iptalden ÖNCE kilidi aldı, başarmalı: %v", err)
			}
			tt.check(t, revocation.wait(t, "iptal"))

			assertFamilyDead(t, pool, f.familyID)
			if tokenExists(t, pool, successor) {
				t.Error("uçuştaki rotasyonun halefi iptalden sağ çıktı (bayat anlık görüntü)")
			}
		})
	}
}

// TestRotationWaitsForRevocationHoldingFamilyRealDB (T2): iptal aileyi ÖNCE tutar. Rotasyon
// aile kilidinde bekler, HİÇBİR token satırını kilitlemeden — iptal token'ları rahatça silip
// commit eder; rotasyon uyanınca aileyi iptal edilmiş görür → ErrInvalidToken.
//
// Rotasyon ters sırayla (önce token'ı işaretleyip SONRA aileyi kilitleyerek) çalışsaydı: rotasyon
// B'yi tutup aileyi, iptal aileyi tutup B'yi beklerdi → deadlock (40P01). Kilit sırası
// değişmezinin gerçek DB'deki kanıtı.
func TestRotationWaitsForRevocationHoldingFamilyRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	tokens, err := NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	svc := NewService(NewRepository(pool), tokens, time.Hour, 24*time.Hour)
	f := newLiveFamily(t, pool)

	tx, testPid := dedicatedTx(t)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	// İptalin ilk adımı, üretimdeki sırayla: ÖNCE aile.
	if _, err := tx.Exec(ctx, `UPDATE refresh_token_families SET revoked_at = now() WHERE id = $1`, f.familyID); err != nil {
		t.Fatalf("aile UPDATE: %v", err)
	}

	rotation := goAsync(func(ctx context.Context) error {
		_, err := svc.Refresh(ctx, f.rawB)
		return err
	})
	mustBlockedBy(t, pool, testPid, rotation, "rotasyon")

	// İptalin ikinci adımı: token'lar. Rotasyon hiçbir token'ı tutmadığı için BEKLEMEDEN geçmeli.
	if _, err := tx.Exec(ctx, `DELETE FROM refresh_tokens WHERE family_id = $1`, f.familyID); err != nil {
		t.Fatalf("iptalin token DELETE'i başarısız (40P01 = kilit sırası ters): %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	err = rotation.wait(t, "rotasyon")
	if isDeadlock(err) {
		t.Fatalf("deadlock (40P01): rotasyon kilit sırasını bozuyor: %v", err)
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("iptal edilmiş ailede rotasyon ErrInvalidToken vermeli, gelen %v", err)
	}
	assertFamilyDead(t, pool, f.familyID)
}

// TestRefreshConcurrentReuseRealDB (T3, eski TestConsumeRefreshTokenConcurrent'in yerine):
// aynı token'la 20 eşzamanlı yenileme. Aile kilidi hepsini sıraya sokar: TAM BİRİ kazanır,
// kilidi ondan sonra alan harcanmış token'ı görür → reuse → aile ölür; kalanlar iptal edilmiş
// aileyi (ya da silinmiş token'ı) görür → düz ErrInvalidToken. Sonunda kazananın halefi de
// ölüdür: eşzamanlı iki kullanım, tanım gereği token'ın kopyalandığını gösterir.
func TestRefreshConcurrentReuseRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	tokens, err := NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	s := NewService(NewRepository(pool), tokens, time.Hour, 24*time.Hour)
	s.hashCost = bcrypt.MinCost
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	email := uniqueEmail(t, pool)
	if _, err := s.Register(ctx, email, "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	sess, err := s.Login(ctx, email, goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	var familyID int64
	if err := pool.QueryRow(ctx, "SELECT family_id FROM refresh_tokens WHERE token_hash = $1",
		hashRefreshToken(sess.RefreshToken)).Scan(&familyID); err != nil {
		t.Fatalf("aile okunamadı: %v", err)
	}

	// goAsync: her yenilemenin context'i de, onu beklememiz de lockWait ile sınırlı — bir
	// kilitlenme (havuz ya da bağlantılar arası) testi go test'in 10 dakikasına kadar asmaz.
	const n = 20
	start := make(chan struct{})
	calls := make([]*async, n)
	for i := range calls {
		calls[i] = goAsync(func(ctx context.Context) error {
			<-start // hepsi aynı anda başlasın
			_, err := s.Refresh(ctx, sess.RefreshToken)
			return err
		})
	}
	close(start)

	var (
		wins, reuses, invalid int
		unknown               []error
	)
	for _, c := range calls {
		err := c.wait(t, "eşzamanlı yenileme")
		var reuse *ReuseError
		switch {
		case err == nil:
			wins++
		case errors.As(err, &reuse):
			reuses++
		case errors.Is(err, ErrInvalidToken):
			invalid++
		default:
			unknown = append(unknown, err)
		}
	}

	if len(unknown) > 0 {
		t.Fatalf("beklenmeyen hatalar (deadlock/sürücü): %v", unknown)
	}
	if wins != 1 || reuses < 1 || wins+reuses+invalid != n {
		t.Errorf("tam 1 kazanan, ≥1 reuse, kalanı ErrInvalidToken bekleniyordu: kazanan=%d reuse=%d geçersiz=%d", wins, reuses, invalid)
	}
	assertFamilyDead(t, pool, familyID)
}

// TestRotateRefreshTokenCapAndExpiryRealDB: mutlak tavan ve süre kontrolleri gerçek DB'de.
// Saat parametre olduğu için zamanı SQL'le oynamadan "ileri sarıyoruz".
func TestRotateRefreshTokenCapAndExpiryRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	// Postgres mikro saniye saklar; karşılaştırmalar kesin olsun diye o hassasiyete kırp.
	t0 := time.Now().Truncate(time.Microsecond)

	t.Run("halefin bitişi ailenin tavanına kırpılır, tavanda aile biter", func(t *testing.T) {
		userID := createUser(t, pool)
		capAt := t0.Add(10 * time.Hour)
		h1, h2, h3 := hashRefreshToken(testdb.Unique("c1")), hashRefreshToken(testdb.Unique("c2")), hashRefreshToken(testdb.Unique("c3"))
		if err := repo.StartRefreshFamily(ctx, userID, "hash", h1, t0.Add(4*time.Hour), capAt); err != nil {
			t.Fatalf("StartRefreshFamily: %v", err)
		}

		rot, err := repo.RotateRefreshToken(ctx, h1, h2, t0.Add(3*time.Hour), t0.Add(7*time.Hour))
		if err != nil || !rot.ExpiresAt.Equal(t0.Add(7*time.Hour)) {
			t.Fatalf("tavanın altında pencere aynen uzamalı: %v, %v", rot.ExpiresAt.Sub(t0), err)
		}
		rot, err = repo.RotateRefreshToken(ctx, h2, h3, t0.Add(6*time.Hour), t0.Add(11*time.Hour))
		if err != nil || !rot.ExpiresAt.Equal(capAt) {
			t.Fatalf("pencere tavanı aşıyorsa tavana kırpılmalı: t0+%v, %v", rot.ExpiresAt.Sub(t0), err)
		}
		var stored time.Time
		if err := pool.QueryRow(ctx, "SELECT expires_at FROM refresh_tokens WHERE token_hash = $1", h3).Scan(&stored); err != nil || !stored.Equal(capAt) {
			t.Errorf("DB'deki bitiş de tavan olmalı: t0+%v, %v", stored.Sub(t0), err)
		}

		_, err = repo.RotateRefreshToken(ctx, h3, hashRefreshToken(testdb.Unique("c4")), capAt, capAt.Add(4*time.Hour))
		if !errors.Is(err, errRefreshNotFound) {
			t.Errorf("tavan anında errRefreshNotFound bekleniyordu, gelen %v", err)
		}
	})

	t.Run("tavanı geçmiş aile, token'ın süresi dolmasa da reddedilir ve reuse sayılmaz", func(t *testing.T) {
		// Aile kontrolünü (adım b) token süresinden (adım d) AYRI sınamak için tutarsız bir
		// durum kuruyoruz: token t0+5 saate kadar geçerli, ama aile t0+1 saatte bitti.
		userID := createUser(t, pool)
		h := hashRefreshToken(testdb.Unique("capped"))
		fam := insertFamily(t, pool, userID, t0.Add(time.Hour), false, fixtureToken{hash: h, exp: t0.Add(5 * time.Hour)})

		_, err := repo.RotateRefreshToken(ctx, h, hashRefreshToken(testdb.Unique("n")), t0.Add(2*time.Hour), t0.Add(3*time.Hour))
		if !errors.Is(err, errRefreshNotFound) {
			t.Fatalf("errRefreshNotFound bekleniyordu, gelen %v", err)
		}
		// Tavan bir güvenlik olayı değil: aile iptal EDİLMEZ, token harcanmış sayılmaz.
		if exists, revoked, n := familyState(t, pool, fam); !exists || revoked || n != 1 {
			t.Errorf("aile dokunulmadan kalmalı: var=%v iptal=%v token=%d", exists, revoked, n)
		}
	})

	t.Run("süresi dolmuş token reddedilir, harcanmış işareti geri alınır", func(t *testing.T) {
		userID := createUser(t, pool)
		h := hashRefreshToken(testdb.Unique("expired"))
		if err := repo.StartRefreshFamily(ctx, userID, "hash", h, t0.Add(time.Hour), t0.Add(24*time.Hour)); err != nil {
			t.Fatalf("StartRefreshFamily: %v", err)
		}
		_, err := repo.RotateRefreshToken(ctx, h, hashRefreshToken(testdb.Unique("n")), t0.Add(time.Hour), t0.Add(2*time.Hour))
		if !errors.Is(err, errRefreshNotFound) {
			t.Fatalf("tam bitiş anında errRefreshNotFound bekleniyordu, gelen %v", err)
		}
		// Rollback used_at'i de geri almalı: aksi hâlde aynı (süresi dolmuş) token'ın bir
		// sonraki sunumu REUSE sanılır ve aile sebepsiz ölürdü.
		var used bool
		if err := pool.QueryRow(ctx, "SELECT used_at IS NOT NULL FROM refresh_tokens WHERE token_hash = $1", h).Scan(&used); err != nil || used {
			t.Errorf("süresi dolmuş token harcanmış işaretlenmemeli: used=%v err=%v", used, err)
		}
	})
}

// TestServiceFlowRealDB: register → login → refresh → logout zinciri gerçek DB'de.
func TestServiceFlowRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	tokens, err := NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	s := NewService(NewRepository(pool), tokens, time.Hour, 24*time.Hour)
	s.hashCost = bcrypt.MinCost
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	email := uniqueEmail(t, pool)

	if _, err := s.Register(ctx, email, "Ali", goodPassword); err != nil {
		t.Fatalf("Register: %v", err)
	}
	sess, err := s.Login(ctx, email, goodPassword)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	next, err := s.Refresh(ctx, sess.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	// DAVRANIŞ DEĞİŞTİ (Faz 13): eskiden burada önce eski token'ı yeniden deniyorduk; artık o
	// bir REUSE olur ve ailenin tamamını (next dahil) öldürür — sonraki "çıkış" adımı anlamsız
	// kalırdı. Sıra: önce çıkış (aileyi öldürür), sonra iki token'ın da reddi.
	if err := s.Logout(ctx, next.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := s.Refresh(ctx, next.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("çıkıştan sonra reddedilmeli, gelen %v", err)
	}
	if _, err := s.Refresh(ctx, sess.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("eski refresh token reddedilmeli, gelen %v", err)
	}
}

// TestCleanupRefreshTokensRealDB (eski TestDeleteExpiredRefreshTokensRealDB'nin genişletilmişi):
// girişteki süpürme yalnızca VERİLEN kullanıcının ÖLÜ ailelerini ve canlı ailelerdeki süresi
// dolmuş token'ları siler. Yanlış bir WHERE (user_id'yi unutmak, süreyi ters karşılaştırmak,
// canlı aileyi ölü saymak) her girişte kullanıcıları başka cihazlardaki oturumlarından atardı.
// TestRotateRejectsRevokedFamilyRealDB: adım (b)'deki "aile iptal edilmişse reddet" kontrolü.
// Normalde iptal ailenin token'larını da siler, yani iptal edilmiş bir ailede token kalmaz ve
// adım (a) zaten "bulunamadı" der. Kontrol bir savunma hattı: iptal ile token silme arasında
// bir tutarsızlık olsa bile (ör. elle düzenlenmiş satır, yarım kalmış bir bakım işi) iptal
// edilmiş ailenin token'ı ASLA yeni bir token üretmemeli. Bu test o tutarsız durumu kurar.
func TestRotateRejectsRevokedFamilyRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	user := createUser(t, pool)
	now := time.Now()
	old, successor := hashRefreshToken(testdb.Unique("revoked-live")), hashRefreshToken(testdb.Unique("revoked-succ"))
	fam := insertFamily(t, pool, user, now.Add(24*time.Hour), true, fixtureToken{hash: old, exp: now.Add(time.Hour)})

	_, err := repo.RotateRefreshToken(ctx, old, successor, now, now.Add(time.Hour))
	if !errors.Is(err, errRefreshNotFound) {
		t.Fatalf("iptal edilmiş aile: istenen errRefreshNotFound, gelen %v", err)
	}
	if tokenExists(t, pool, successor) {
		t.Error("iptal edilmiş ailede halef token oluşmamalı")
	}
	var usedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT used_at FROM refresh_tokens WHERE token_hash = $1`, old).Scan(&usedAt); err != nil {
		t.Fatalf("token okunamadı: %v", err)
	}
	if usedAt != nil {
		t.Error("reddedilen rotasyon token'ı harcanmış işaretlememeli (rollback)")
	}
	if exists, revoked, _ := familyState(t, pool, fam); !exists || !revoked {
		t.Errorf("aile iptal edilmiş olarak kalmalı: var=%v iptal=%v", exists, revoked)
	}
}

func TestCleanupRefreshTokensRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	a, b := createUser(t, pool), createUser(t, pool)
	now := time.Now()
	future, past := now.Add(24*time.Hour), now.Add(-time.Hour)
	tok := func(name string) []byte { return hashRefreshToken(testdb.Unique(name)) }

	aExpired, aSpent, aLive := tok("a-expired"), tok("a-spent"), tok("a-live")
	live := insertFamily(t, pool, a, future, false,
		fixtureToken{hash: aExpired, exp: past, used: true}, // harcanmış + süresi dolmuş: tespit değeri yok
		// harcanmış ama süresi DOLMAMIŞ: reuse tespiti tam da bu satıra dayanır (ikinci kez
		// gelirse aileyi öldürmek için onu tanımalıyız) — temizlik bunu ASLA silmemeli.
		fixtureToken{hash: aSpent, exp: now.Add(time.Hour), used: true},
		fixtureToken{hash: aLive, exp: now.Add(time.Hour)},
	)
	dead := map[string]int64{
		"iptal edilmiş":        insertFamily(t, pool, a, future, true, fixtureToken{hash: tok("r"), exp: now.Add(time.Hour)}),
		"tavanı geçmiş":        insertFamily(t, pool, a, now, false, fixtureToken{hash: tok("c"), exp: now.Add(time.Hour)}),
		"yalnız harcanmış":     insertFamily(t, pool, a, future, false, fixtureToken{hash: tok("u"), exp: now.Add(time.Hour), used: true}),
		"yalnız süresi dolmuş": insertFamily(t, pool, a, future, false, fixtureToken{hash: tok("e"), exp: now}),
	}
	bExpired := tok("b-expired")
	bDead := insertFamily(t, pool, b, future, true, fixtureToken{hash: bExpired, exp: past})

	if err := repo.CleanupRefreshTokens(ctx, a, now); err != nil {
		t.Fatalf("CleanupRefreshTokens: %v", err)
	}

	if exists, _, _ := familyState(t, pool, live); !exists {
		t.Error("A'nın canlı ailesi silinmemeli")
	}
	if tokenExists(t, pool, aExpired) {
		t.Error("A'nın canlı ailesindeki süresi dolmuş token silinmeliydi")
	}
	if !tokenExists(t, pool, aLive) {
		t.Error("A'nın geçerli token'ı silinmemeli")
	}
	if !tokenExists(t, pool, aSpent) {
		t.Error("A'nın harcanmış ama süresi dolmamış token'ı silinmemeli (reuse tespiti ona dayanır)")
	}
	for name, id := range dead {
		if exists, _, _ := familyState(t, pool, id); exists {
			t.Errorf("A'nın %s ailesi silinmeliydi", name)
		}
	}
	if exists, _, n := familyState(t, pool, bDead); !exists || n != 1 || !tokenExists(t, pool, bExpired) {
		t.Errorf("B'nin ailesi/token'ı A için yapılan süpürmede silinmemeli: var=%v token=%d", exists, n)
	}
}

// TestCleanupSkipsFamilyLockedByRotationRealDB: süpürme, o an başka bir işlemin kilitlediği
// aileyi ATLAR (FOR UPDATE SKIP LOCKED) — beklemez, silmez.
//
// Kurgu: aile, süpürmenin anlık görüntüsünde "ölü" görünüyor (tek token'ı harcanmış), ama
// ayrı bağlantıdaki test tx'i onu kilitleyip commit'lenmemiş CANLI bir halef yazmış (commit
// eşiğindeki bir işlem). SKIP LOCKED olmasa süpürme kilidi bekler; tx commit edince aileyi
// BAYAT görüntüye göre siler ve taze halef CASCADE ile gider — kullanıcı sebepsiz düşer.
func TestCleanupSkipsFamilyLockedByRotationRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	userID := createUser(t, pool)
	now := time.Now()
	locked := insertFamily(t, pool, userID, now.Add(24*time.Hour), false,
		fixtureToken{hash: hashRefreshToken(testdb.Unique("spent")), exp: now.Add(time.Hour), used: true})
	revoked := insertFamily(t, pool, userID, now.Add(24*time.Hour), true,
		fixtureToken{hash: hashRefreshToken(testdb.Unique("rev")), exp: now.Add(time.Hour)})
	successor := hashRefreshToken(testdb.Unique("halef"))

	tx, testPid := dedicatedTx(t)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	if _, err := tx.Exec(ctx, `SELECT id FROM refresh_token_families WHERE id = $1 FOR UPDATE`, locked); err != nil {
		t.Fatalf("aile kilidi: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		userID, locked, successor, now.Add(time.Hour)); err != nil {
		t.Fatalf("halef: %v", err)
	}

	cleanup := goAsync(func(ctx context.Context) error { return repo.CleanupRefreshTokens(ctx, userID, now) })
	if _, blocked := blockedBy(t, pool, testPid, cleanup); blocked {
		t.Error("süpürme, kilitli aileyi BEKLEDİ (SKIP LOCKED yok) — girişi uçuştaki bir işleme bağlıyor")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := cleanup.wait(t, "süpürme"); err != nil {
		t.Fatalf("CleanupRefreshTokens: %v", err)
	}

	if exists, _, _ := familyState(t, pool, locked); !exists || !tokenExists(t, pool, successor) {
		t.Errorf("kilitli aile ve taze halefi yaşamalı: aile=%v halef=%v", exists, tokenExists(t, pool, successor))
	}
	if exists, _, _ := familyState(t, pool, revoked); exists {
		t.Error("kilitsiz ölü aile yine de silinmeliydi (süpürme gerçekten koştu mu?)")
	}
}

// TestCleanupSkipsTokensOfLockedFamilyRealDB: süpürmenin ikinci adımı (canlı ailelerdeki süresi
// dolmuş token'lar) da kilit sırası değişmezine uyar: token'lara dokunmadan önce aileyi kilitler,
// kilitli aileyi ATLAR. Test tx'i üretimdeki iptal sırasını uçuşta tutuyor (aile UPDATE + token
// DELETE, commit'siz). Süpürme o ailenin token'larını beklerse ve iptal de süpürmenin kilitlediği
// bir token'ı beklerse döngü = deadlock; beklememek döngüyü baştan imkânsız kılar.
func TestCleanupSkipsTokensOfLockedFamilyRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	userID := createUser(t, pool)
	now := time.Now()
	spent := hashRefreshToken(testdb.Unique("spent"))
	fam := insertFamily(t, pool, userID, now.Add(24*time.Hour), false,
		fixtureToken{hash: spent, exp: now.Add(-time.Hour), used: true},
		fixtureToken{hash: hashRefreshToken(testdb.Unique("live")), exp: now.Add(time.Hour)})

	tx, testPid := dedicatedTx(t)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	if _, err := tx.Exec(ctx, `UPDATE refresh_token_families SET revoked_at = now() WHERE id = $1`, fam); err != nil {
		t.Fatalf("aile UPDATE: %v", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM refresh_tokens WHERE family_id = $1`, fam); err != nil {
		t.Fatalf("token DELETE: %v", err)
	}

	cleanup := goAsync(func(ctx context.Context) error { return repo.CleanupRefreshTokens(ctx, userID, now) })
	if _, blocked := blockedBy(t, pool, testPid, cleanup); blocked {
		t.Error("süpürme, uçuştaki iptalin kilitlediği token'ları BEKLEDİ (aile kilidini atlayarak token'a gitti)")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if err := cleanup.wait(t, "süpürme"); err != nil {
		t.Fatalf("CleanupRefreshTokens: %v", err)
	}
	if !tokenExists(t, pool, spent) {
		t.Error("kilitli ailenin süresi dolmuş token'ı bu turda ATLANMALIYDI (bir sonraki girişte gider)")
	}
}

// ---------------------------------------------------------------------------------------------
// Parola değiştirme (Faz 13 F) — tek tx, kilit sırası users → families → tokens.
// ---------------------------------------------------------------------------------------------

// passwordChange, ChangePassword'ün test girdisi: "hash" createUser'ın verdiği hash.
func passwordChange(userID int64, newToken []byte, now time.Time) PasswordChange {
	return PasswordChange{
		UserID:       userID,
		VerifiedHash: "hash",
		NewHash:      "yeni-hash",
		TokenHash:    newToken,
		TokenExp:     now.Add(time.Hour),
		FamilyExp:    now.Add(24 * time.Hour),
		Now:          now,
	}
}

// liveFamilyCount, kullanıcının iptal edilmemiş aile sayısı (havuzdan taze).
func liveFamilyCount(t *testing.T, pool *pgxpool.Pool, userID int64) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM refresh_token_families WHERE user_id = $1 AND revoked_at IS NULL`, userID).Scan(&n); err != nil {
		t.Fatalf("aile sayımı: %v", err)
	}
	return n
}

// TestChangePasswordRevokesAllFamiliesRealDB (P1): iki ailesi olan kullanıcı parolasını
// değiştirir → ikisi de iptal + 0 token, yeni hash yazılı, bu cihazın yeni ailesi canlı.
// Bayat doğrulanmış hash (eşzamanlı ikinci değişim) → ErrWrongPassword ve hiçbir şey değişmez.
func TestChangePasswordRevokesAllFamiliesRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	f := newLiveFamily(t, pool) // aile 1: A harcanmış + B canlı
	now := time.Now()
	otherDevice := hashRefreshToken(testdb.Unique("tok-c")) // aile 2: başka cihaz
	if err := repo.StartRefreshFamily(ctx, f.userID, "hash", otherDevice, now.Add(time.Hour), now.Add(24*time.Hour)); err != nil {
		t.Fatalf("ikinci aile: %v", err)
	}
	var otherFamily int64
	if err := pool.QueryRow(ctx, `SELECT family_id FROM refresh_tokens WHERE token_hash = $1`, otherDevice).Scan(&otherFamily); err != nil {
		t.Fatalf("ikinci aile okunamadı: %v", err)
	}

	thisDevice := hashRefreshToken(testdb.Unique("tok-yeni"))
	revoked, err := repo.ChangePassword(ctx, passwordChange(f.userID, thisDevice, now))
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if revoked != 2 {
		t.Errorf("iptal edilen aile sayısı: istenen 2, gelen %d", revoked)
	}
	assertFamilyDead(t, pool, f.familyID)
	assertFamilyDead(t, pool, otherFamily)
	if _, err := repo.RotateRefreshToken(ctx, hashRefreshToken(f.rawB), hashRefreshToken(testdb.Unique("z")), now, now.Add(time.Hour)); !errors.Is(err, errRefreshNotFound) {
		t.Errorf("eski cihazın token'ı artık geçersiz olmalı, gelen %v", err)
	}
	u, err := repo.UserByID(ctx, f.userID)
	if err != nil || u.PasswordHash != "yeni-hash" {
		t.Errorf("yeni hash yazılmalı: %q, %v", u.PasswordHash, err)
	}
	// Bu cihaz: yeni aile CANLI (rotasyon çalışır).
	if _, err := repo.RotateRefreshToken(ctx, thisDevice, hashRefreshToken(testdb.Unique("tok-yeni-2")), now, now.Add(time.Hour)); err != nil {
		t.Errorf("değişimin açtığı aile canlı olmalı: %v", err)
	}
	if n := liveFamilyCount(t, pool, f.userID); n != 1 {
		t.Errorf("değişimden sonra tam 1 canlı aile olmalı, gelen %d", n)
	}

	// Bayat doğrulama: servis "hash"i doğruladı ama arada parola "yeni-hash" oldu → ret, rollback.
	stale := hashRefreshToken(testdb.Unique("tok-bayat"))
	if _, err := repo.ChangePassword(ctx, passwordChange(f.userID, stale, now)); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("bayat doğrulanmış hash ErrWrongPassword vermeli, gelen %v", err)
	}
	if u, _ := repo.UserByID(ctx, f.userID); u.PasswordHash != "yeni-hash" {
		t.Errorf("reddedilen değişim hash'e dokunmamalı: %q", u.PasswordHash)
	}
	if tokenExists(t, pool, stale) || liveFamilyCount(t, pool, f.userID) != 1 {
		t.Error("reddedilen değişim aile açmamalı, canlı aileyi öldürmemeli (rollback)")
	}
}

// TestChangePasswordWaitsForInFlightRotationRealDB (P2): bir rotasyon F ailesinin kilidini
// tutarken (T1'deki UNIQUE index hilesi) parola değişir. Değişim users satırını kilitler, sonra
// F'de bekler; rotasyon halefi yazıp commit eder; değişim taze anlık görüntüyle devam eder →
// F iptal, halef silinmiş. DEADLOCK YOK: users kilidi FOR NO KEY UPDATE olduğu için rotasyonun
// halef INSERT'inin FK için aldığı FOR KEY SHARE onunla çakışmaz. Kilit FOR UPDATE yapılırsa
// (mutation) rotasyon users'ı, değişim F'yi bekler → 40P01 ile deterministik kırılır.
func TestChangePasswordWaitsForInFlightRotationRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	f := newLiveFamily(t, pool)
	successor := hashRefreshToken(testdb.Unique("halef"))
	decoy := insertFamily(t, pool, f.userID, time.Now().Add(time.Hour), false)

	tx, testPid := dedicatedTx(t)
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	if _, err := tx.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at) VALUES ($1, $2, $3, now() + interval '1 hour')`,
		f.userID, decoy, successor); err != nil {
		t.Fatalf("engelleyici satır: %v", err)
	}

	rotation := goAsync(func(ctx context.Context) error {
		now := time.Now()
		_, err := repo.RotateRefreshToken(ctx, hashRefreshToken(f.rawB), successor, now, now.Add(time.Hour))
		return err
	})
	rotPid := mustBlockedBy(t, pool, testPid, rotation, "rotasyon")

	thisDevice := hashRefreshToken(testdb.Unique("tok-yeni"))
	change := goAsync(func(ctx context.Context) error {
		_, err := repo.ChangePassword(ctx, passwordChange(f.userID, thisDevice, time.Now()))
		return err
	})
	// Değişim, rotasyonun tuttuğu F kilidinde beklemeli (users kilidini almış, ailelerde duruyor).
	mustBlockedBy(t, pool, rotPid, change, "parola değişimi")

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	rotErr := rotation.wait(t, "rotasyon")
	changeErr := change.wait(t, "parola değişimi")
	if isDeadlock(rotErr) || isDeadlock(changeErr) {
		t.Fatalf("deadlock (40P01): users kilidi rotasyonun FK kilidiyle çakışıyor: rotasyon=%v değişim=%v", rotErr, changeErr)
	}
	if rotErr != nil {
		t.Errorf("rotasyon değişimden ÖNCE kilidi aldı, başarmalı: %v", rotErr)
	}
	if changeErr != nil {
		t.Fatalf("ChangePassword: %v", changeErr)
	}

	assertFamilyDead(t, pool, f.familyID)
	if tokenExists(t, pool, successor) {
		t.Error("uçuştaki rotasyonun halefi parola değişiminden sağ çıktı (bayat anlık görüntü)")
	}
	if !tokenExists(t, pool, thisDevice) || liveFamilyCount(t, pool, f.userID) != 1 {
		t.Error("bu cihazın yeni ailesi tek canlı aile olmalı")
	}
}

// TestLoginRejectsPasswordChangedUnderneathRealDB (P3, GİRİŞ YARIŞI): giriş bcrypt'i tx dışında
// eski (commit'li) hash'le doğruladı; o sırada test tx'i parola değişiminin ilk adımını tutuyor
// (users FOR NO KEY UPDATE + yeni hash, commit'siz). StartRefreshFamily'nin FOR SHARE okuması
// orada BEKLER; commit'ten sonra yeni hash'i görür → ErrInvalidCredentials, aile AÇILMAZ.
// FOR SHARE kontrolü kaldırılırsa (mutation) giriş hiç beklemeden eski parolayla aile açar.
func TestLoginRejectsPasswordChangedUnderneathRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	tokens, err := NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	svc := NewService(NewRepository(pool), tokens, time.Hour, 24*time.Hour)
	svc.hashCost = bcrypt.MinCost
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	email := uniqueEmail(t, pool)
	u, err := svc.Register(ctx, email, "Ali", goodPassword)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	tx, testPid := dedicatedTx(t)
	// Parola değişiminin ilk iki adımı, üretimdeki kilitle (commit'siz).
	if _, err := tx.Exec(ctx, `SELECT password_hash FROM users WHERE id = $1 FOR NO KEY UPDATE`, u.ID); err != nil {
		t.Fatalf("users kilidi: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash = 'degisti', updated_at = now() WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("users UPDATE: %v", err)
	}

	login := goAsync(func(ctx context.Context) error {
		_, err := svc.Login(ctx, email, goodPassword) // eski hash'le doğrulanır (READ COMMITTED)
		return err
	})
	mustBlockedBy(t, pool, testPid, login, "giriş")

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := login.wait(t, "giriş"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("parola altından değişen giriş ErrInvalidCredentials vermeli, gelen %v", err)
	}
	if n := liveFamilyCount(t, pool, u.ID); n != 0 {
		t.Errorf("reddedilen giriş aile açmamalı, %d canlı aile var", n)
	}
}
