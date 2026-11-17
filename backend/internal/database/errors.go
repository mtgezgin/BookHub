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

// InvalidInput, err Postgres'in "veri istisnası" sınıfındansa (SQLSTATE 22xxx) true döner:
// geçersiz bayt dizisi (22021), metinde NUL (22P05), aralık dışı sayı (22003) gibi. Bunlar
// sunucunun değil gönderilen değerin hatasıdır.
func InvalidInput(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && len(pgErr.Code) == 5 && pgErr.Code[:2] == "22"
}

// TimedOut, err veritabanının süre sınırlarından biriyse true döner: statement_timeout sorguyu
// iptal etti (57014) ya da lock_timeout doldu (55P03). İsteğin değil o anki yükün hatasıdır;
// yeniden denenebilir.
func TimedOut(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "57014" || pgErr.Code == "55P03")
}
