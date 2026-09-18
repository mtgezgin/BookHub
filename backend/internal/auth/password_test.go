package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestHashPassword: üretim hash'i cost 12 ile üretilir, parolanın kendisini içermez,
// aynı parola iki kez hash'lenince FARKLI sonuç verir (salt) ve doğrulanabilir.
func TestHashPassword(t *testing.T) {
	const pw = "dogru-at-pil-zimba"

	h1, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if cost, err := bcrypt.Cost([]byte(h1)); err != nil || cost != bcryptCost {
		t.Errorf("cost: istenen %d, gelen %d (err %v)", bcryptCost, cost, err)
	}
	if strings.Contains(h1, pw) {
		t.Error("hash parolanın kendisini içeriyor")
	}

	h2, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if h1 == h2 {
		t.Error("aynı parola aynı hash'i verdi — salt kullanılmıyor")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(h1), []byte(pw)); err != nil {
		t.Errorf("doğru parola doğrulanamadı: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(h1), []byte(pw+"x")); err == nil {
		t.Error("yanlış parola kabul edildi")
	}
}

// TestHashPasswordTooLong: 72 baytı aşan parola SESSİZCE kırpılmaz, reddedilir.
func TestHashPasswordTooLong(t *testing.T) {
	if _, err := hashPassword(strings.Repeat("a", MaxPasswordBytes), bcrypt.MinCost); err != nil {
		t.Errorf("tam 72 bayt kabul edilmeli: %v", err)
	}
	_, err := hashPassword(strings.Repeat("a", MaxPasswordBytes+1), bcrypt.MinCost)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Errorf("istenen ErrPasswordTooLong, gelen %v", err)
	}
}
