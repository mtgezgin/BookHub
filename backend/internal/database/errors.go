package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres hata kodları (SQLSTATE). Sayıları kod içine serpiştirmek yerine adlandırıyoruz.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
)

// UniqueViolation, err bir Postgres unique_violation ise İHLAL EDİLEN KISITIN ADINI döner.
// Kısıt adı önemli: bir tabloda birden fazla UNIQUE kolon varsa (genres: name VE slug)
// hangisinin çakıştığını ancak böyle söyleyebiliriz.
func UniqueViolation(err error) (constraint string, ok bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == codeUniqueViolation {
		return pgErr.ConstraintName, true
	}
	return "", false
}

// ForeignKeyViolation, err bir foreign_key_violation ise kısıt adını döner
// (ör. var olmayan bir türü kitaba bağlamaya çalışmak).
func ForeignKeyViolation(err error) (constraint string, ok bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == codeForeignKeyViolation {
		return pgErr.ConstraintName, true
	}
	return "", false
}
