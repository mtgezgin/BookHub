package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// failingDBTX, her sorguda verilen hatayı döndüren db.DBTX sahtesi: repository'nin HATA
// ÇEVİRİSİNİ (sürücü dili → domain dili) DB'siz kanıtlamak için.
type failingDBTX struct{ err error }

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
		if _, _, err := r.ConsumeRefreshToken(ctx, []byte("h")); !errors.Is(err, errRefreshNotFound) {
			t.Errorf("ConsumeRefreshToken: errRefreshNotFound bekleniyordu, gelen %v", err)
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
		_, _, err = r.ConsumeRefreshToken(ctx, []byte("h"))
		if !errors.Is(err, driverErr) || errors.Is(err, errRefreshNotFound) {
			t.Errorf("ConsumeRefreshToken: sarılı sürücü hatası bekleniyordu, gelen %v", err)
		}
	})
}
