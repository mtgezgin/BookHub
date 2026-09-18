// Package auth, kimlik doğrulama ve yetkilendirmenin iş kurallarını taşır: parola
// hash'leme, access token (JWT) üretme/doğrulama, refresh token yaşam döngüsü ve bunları
// yöneten Service. HTTP'yi tanımaz — status kodları cmd/api'nin işi.
package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost, bcrypt'in iş faktörü: her +1 hesaplamayı İKİYE katlar. 12 ≈ 250 ms/hash —
// kullanıcı girişte fark etmez, ama çalınmış bir hash tablosuna kaba kuvvet saldıran
// biri için parola başına 250 ms, milyarlarca denemeyi yıllara çevirir.
//
// NEDEN SHA-256 DEĞİL: SHA hızlı olmak için tasarlandı (GPU'da saniyede milyarlarca).
// Parola hash'inde hız DÜŞMANDIR; bcrypt bilerek yavaş ve salt'ı kendisi üretir.
const bcryptCost = 12

// Parola politikası. Üst sınır BAYT cinsinden çünkü bcrypt yalnızca ilk 72 baytı işler:
// 73. bayttan sonrasını sessizce yok saysaydı "uzun-parola-A" ile "uzun-parola-B" aynı
// hash'i verirdi. x/crypto artık >72 baytta hata dönüyor; biz de kullanıcıya net söylüyoruz.
const (
	MinPasswordLen   = 8
	MaxPasswordBytes = 72
)

// ErrPasswordTooLong, bcrypt'in işleyemeyeceği uzunlukta parola.
var ErrPasswordTooLong = errors.New("parola en fazla 72 bayt olabilir")

// HashPassword, parolayı üretim maliyetiyle (cost 12) hash'ler.
func HashPassword(password string) (string, error) {
	return hashPassword(password, bcryptCost)
}

func hashPassword(password string, cost int) (string, error) {
	if len(password) > MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("parola hash'lenemedi: %w", err)
	}
	return string(h), nil
}
