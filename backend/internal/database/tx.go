package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Beginner, transaction açabilen her şey: *pgxpool.Pool da pgx.Tx de bunu sağlar
// (Tx üzerinde Begin → SAVEPOINT, yani iç içe transaction'lar kendiliğinden çalışır).
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ErrNoTx, repository'ye transaction açamayan bir bağlantı verildiğinde döner
// (ör. birim testlerindeki sahte DBTX). Programlama hatasıdır, istemci hatası değil.
var ErrNoTx = errors.New("bu bağlantı transaction açamıyor")

// WithTx, fn'i tek bir transaction içinde çalıştırır: fn hata dönerse (ya da panik
// ederse) HER ŞEY geri alınır, başarılıysa commit edilir.
//
// Neden: "önce sil, sonra ekle" gibi çok adımlı yazımlarda ara adımda hata olursa
// yarım kalmış bir durum (türleri silinmiş ama yenileri eklenmemiş kitap) kalmamalı.
// Ya hepsi ya hiçbiri — atomiklik.
func WithTx(ctx context.Context, conn any, fn func(tx pgx.Tx) error) (err error) {
	b, ok := conn.(Beginner)
	if !ok {
		return ErrNoTx
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	// Commit başarılıysa Rollback bir no-op'tur; her çıkış yolunda güvenle çağrılır.
	// (defer + adlandırılmış dönüş değeri: fn panik etse bile tx açık kalmaz.)
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) && err == nil {
			err = fmt.Errorf("rollback başarısız: %w", rbErr)
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit başarısız: %w", err)
	}
	return nil
}
