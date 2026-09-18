package auth

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// TestConsumeRefreshTokenConcurrent: aynı token'la 20 eşzamanlı tüketimden YALNIZCA biri
// kazanmalı. DELETE ... RETURNING tek atımlık olduğu için Postgres bunu garanti eder;
// SELECT-sonra-DELETE yazılsaydı birden fazla goroutine SELECT'i geçebilirdi.
func TestConsumeRefreshTokenConcurrent(t *testing.T) {
	pool := testdb.Pool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	u, err := repo.CreateUser(ctx, uniqueEmail(t, pool), "Ali", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	h := hashRefreshToken(testdb.Unique("tok"))
	if err := repo.SaveRefreshToken(ctx, u.ID, h, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveRefreshToken: %v", err)
	}

	const n = 20
	var (
		wg      sync.WaitGroup
		wins    atomic.Int32
		misses  atomic.Int32
		start   = make(chan struct{})
		errsMu  sync.Mutex
		unknown []error
	)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // hepsi aynı anda başlasın
			_, _, err := repo.ConsumeRefreshToken(ctx, h)
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, errRefreshNotFound):
				misses.Add(1)
			default:
				errsMu.Lock()
				unknown = append(unknown, err)
				errsMu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	if len(unknown) > 0 {
		t.Fatalf("beklenmeyen hatalar: %v", unknown)
	}
	if wins.Load() != 1 || misses.Load() != n-1 {
		t.Errorf("tek kazanan bekleniyordu: kazanan=%d, kaybeden=%d", wins.Load(), misses.Load())
	}
}

// TestServiceFlowRealDB: register → login → refresh → logout zinciri gerçek DB'de.
func TestServiceFlowRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	tokens, err := NewTokenManager(testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	s := NewService(NewRepository(pool), tokens, time.Hour)
	s.hashCost = bcrypt.MinCost
	ctx := context.Background()
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
	if _, err := s.Refresh(ctx, sess.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("eski refresh token reddedilmeli, gelen %v", err)
	}
	if err := s.Logout(ctx, next.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := s.Refresh(ctx, next.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("çıkıştan sonra reddedilmeli, gelen %v", err)
	}
}
