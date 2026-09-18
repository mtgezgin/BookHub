package database

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestWithTxNoBeginner: transaction açamayan bir bağlantı (Begin metodu yok) verilirse
// WithTx fn'i HİÇ çalıştırmadan ErrNoTx döner — adımlar transaction'sız, yarım kalabilecek
// biçimde koşmamalı.
func TestWithTxNoBeginner(t *testing.T) {
	called := false
	err := WithTx(context.Background(), struct{}{}, func(pgx.Tx) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrNoTx) {
		t.Errorf("ErrNoTx bekleniyordu, gelen %v", err)
	}
	if called {
		t.Error("transaction açılamadığı hâlde fn çalıştırıldı")
	}
}
