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
	// errRefreshNotFound dışarı açılmaz: servis onu ErrInvalidToken'a çevirir.
	errRefreshNotFound = errors.New("refresh token bulunamadı")
)

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

func (r *Repository) SaveRefreshToken(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	err := r.q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return fmt.Errorf("refresh token kaydedilemedi: %w", err)
	}
	return nil
}

// ConsumeRefreshToken, token'ı atomik olarak siler ve sahibini + bitişini döner.
// Aynı token'la gelen ikinci çağrı errRefreshNotFound alır (tek kullanımlık).
func (r *Repository) ConsumeRefreshToken(ctx context.Context, tokenHash []byte) (int64, time.Time, error) {
	row, err := r.q.ConsumeRefreshToken(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, time.Time{}, errRefreshNotFound
	}
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("refresh token kullanılamadı: %w", err)
	}
	return row.UserID, row.ExpiresAt, nil
}

func (r *Repository) DeleteRefreshToken(ctx context.Context, tokenHash []byte) error {
	if err := r.q.DeleteRefreshToken(ctx, tokenHash); err != nil {
		return fmt.Errorf("refresh token silinemedi: %w", err)
	}
	return nil
}

func (r *Repository) DeleteExpiredRefreshTokens(ctx context.Context, userID int64) error {
	if err := r.q.DeleteExpiredRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("süresi dolmuş token'lar silinemedi: %w", err)
	}
	return nil
}
