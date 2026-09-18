package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Alan uzunluk sınırları — KARAKTER (rune) cinsinden, bayt değil: "ğ" UTF-8'de 2
// bayttır; len() ile saymak Türkçe metni haksız yere reddederdi.
//
// Neden sınır: tek koruma 1 MiB gövde sınırı olsaydı, ~1 MB'lık bir başlık kabul edilip
// ardından HER liste cevabında geri servis edilirdi. Sayısal alanlara sınır koyup metin
// alanlarını açık bırakmak, doğrulamanın yarım kalması demektir.
const (
	maxTitleLen       = 300
	maxAuthorLen      = 200
	maxSlugLen        = 120
	maxNameLen        = 100
	maxDescriptionLen = 5000
	maxURLLen         = 2048
)

var (
	// slugPattern: URL'de kimlik olarak kullanılacak — küçük harf, rakam, tek tire.
	slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	// languagePattern: ISO 639-1 dil kodu, opsiyonel bölge (tr, en, en-US).
	languagePattern = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)
)

// validator, doğrulama problemlerini BİRİKTİRİR ki istemci hepsini tek cevapta görsün
// ("title eksik" düzelt → "slug geçersiz" düzelt → ... döngüsü yerine).
type validator struct {
	problems []string
}

func (v *validator) add(format string, args ...any) {
	v.problems = append(v.problems, fmt.Sprintf(format, args...))
}

// required, alanın boş olmamasını ister. DB'deki NOT NULL bunu YAKALAMAZ: boş string
// bir değerdir, NULL değildir.
func (v *validator) required(field, value string) {
	if value == "" {
		v.add("%s zorunlu", field)
	}
}

func (v *validator) maxLen(field, value string, max int) {
	if utf8.RuneCountInString(value) > max {
		v.add("%s en fazla %d karakter olabilir", field, max)
	}
}

// match, boş olmayan değerin kalıba uymasını ister (boşluğu required denetler).
func (v *validator) match(field, value string, re *regexp.Regexp, hint string) {
	if value != "" && !re.MatchString(value) {
		v.add("%s geçersiz (%s)", field, hint)
	}
}

// httpURL, boş olmayan değerin http/https bir URL olmasını ister.
// NEDEN: bu değer ileride frontend'de <img src> / <a href> olarak basılacak;
// "javascript:..." gibi bir şema XSS kapısı olurdu.
func (v *validator) httpURL(field, value string) {
	if value == "" {
		return
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		v.add("%s http(s) ile başlayan geçerli bir URL olmalı", field)
	}
}

func (v *validator) check(ok bool, format string, args ...any) {
	if !ok {
		v.add(format, args...)
	}
}

// err, birikmiş problemleri tek bir 400 validation_error'a çevirir (yoksa nil).
func (v *validator) err() error {
	if len(v.problems) == 0 {
		return nil
	}
	return badRequest("validation_error", strings.Join(v.problems, "; "))
}
