package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// failingDBTX, her sorguda verilen hatayı döndüren db.DBTX sahtesi: repository'nin HATA
// ÇEVİRİSİNİ (sürücü dili → domain dili) DB'siz kanıtlamak için. Begin de sağlar (Faz 13:
// refresh token yolları WithTx içinde); açılan "transaction" da aynı hatayı döndürür.
type failingDBTX struct{ err error }

func (f failingDBTX) Begin(context.Context) (pgx.Tx, error) { return failingTx{err: f.err}, nil }

// failingTx, WithTx'in içini süren sahte pgx.Tx. Gömülü nil arayüz: burada tanımlanmayan bir
// metot (CopyFrom, Prepare...) çağrılırsa panik — test beklenmeyen bir yolu hemen yakalar.
type failingTx struct {
	pgx.Tx
	err error
}

func (f failingTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.err
}
func (f failingTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, f.err }
func (f failingTx) QueryRow(context.Context, string, ...any) pgx.Row        { return errRow{f.err} }
func (failingTx) Commit(context.Context) error                              { return nil }
func (failingTx) Rollback(context.Context) error                            { return nil }

type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }

func (f failingDBTX) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.err
}
func (f failingDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, f.err
}
func (f failingDBTX) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return errRow(f)
}

// TestRepositoryErrorTranslation: yalnızca pgx.ErrNoRows "yok" demektir. Başka her sürücü
// hatası SARILARAK yukarı çıkmalı — "kullanıcı yok"a dönüşseydi, bir DB kesintisinde Login
// "e-posta bulunamadı" yoluna sapardı; "token yok"a dönüşseydi kullanıcılar oturumdan düşerdi.
func TestRepositoryErrorTranslation(t *testing.T) {
	ctx := context.Background()
	driverErr := errors.New("connection reset by peer")

	t.Run("ErrNoRows → domain 'yok' hataları", func(t *testing.T) {
		r := NewRepository(failingDBTX{err: pgx.ErrNoRows})
		if _, err := r.UserByEmail(ctx, "a@b.co"); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("UserByEmail: ErrUserNotFound bekleniyordu, gelen %v", err)
		}
		if _, err := r.UserByID(ctx, 1); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("UserByID: ErrUserNotFound bekleniyordu, gelen %v", err)
		}
		if _, err := r.RotateRefreshToken(ctx, []byte("h"), []byte("h2"), time.Now(), time.Now()); !errors.Is(err, errRefreshNotFound) {
			t.Errorf("RotateRefreshToken: errRefreshNotFound bekleniyordu, gelen %v", err)
		}
		// Çıkış idempotent: bilinmeyen token hata değil.
		if err := r.RevokeRefreshFamily(ctx, []byte("h"), time.Now()); err != nil {
			t.Errorf("RevokeRefreshFamily: bilinmeyen token nil dönmeli, gelen %v", err)
		}
	})

	t.Run("başka sürücü hatası sarılır, 'yok' olmaz", func(t *testing.T) {
		r := NewRepository(failingDBTX{err: driverErr})
		_, err := r.UserByEmail(ctx, "a@b.co")
		if !errors.Is(err, driverErr) || errors.Is(err, ErrUserNotFound) {
			t.Errorf("UserByEmail: sarılı sürücü hatası bekleniyordu, gelen %v", err)
		}
		_, err = r.UserByID(ctx, 1)
		if !errors.Is(err, driverErr) || errors.Is(err, ErrUserNotFound) {
			t.Errorf("UserByID: sarılı sürücü hatası bekleniyordu, gelen %v", err)
		}
		_, err = r.RotateRefreshToken(ctx, []byte("h"), []byte("h2"), time.Now(), time.Now())
		if !errors.Is(err, driverErr) || errors.Is(err, errRefreshNotFound) {
			t.Errorf("RotateRefreshToken: sarılı sürücü hatası bekleniyordu, gelen %v", err)
		}
		// Çıkışta DB kesintisi yutulmamalı: "çıktın" deyip aileyi yaşatmak olurdu.
		if err := r.RevokeRefreshFamily(ctx, []byte("h"), time.Now()); !errors.Is(err, driverErr) {
			t.Errorf("RevokeRefreshFamily: sarılı sürücü hatası bekleniyordu, gelen %v", err)
		}
		if err := r.StartRefreshFamily(ctx, 1, []byte("h"), time.Now(), time.Now()); !errors.Is(err, driverErr) {
			t.Errorf("StartRefreshFamily: sarılı sürücü hatası bekleniyordu, gelen %v", err)
		}
		if err := r.CleanupRefreshTokens(ctx, 1, time.Now()); !errors.Is(err, driverErr) {
			t.Errorf("CleanupRefreshTokens: sarılı sürücü hatası bekleniyordu, gelen %v", err)
		}
	})
}
