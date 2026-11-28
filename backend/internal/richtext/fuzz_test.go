package richtext

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzParse: doğrulayıcı hiçbir girdide panik atmaz ve kabul ettiği her belge için şu sözler tutar:
//   - kanonik JSON yeniden doğrulanınca yine kabul edilir ve aynı belgeyi verir (saklanan şey okununca
//     reddedilmez);
//   - kanonik JSON ikinci turda değişmez;
//   - düz metin sınırların içindedir, boş değildir ve denetim karakteri taşımaz (reviews.body'nin
//     CHECK'i ve Postgres'in NUL yasağı çiğnenmez).
//
// Normal `go test` yalnız tohumları koşturur; aramak için: go test -fuzz=FuzzParse ./internal/richtext
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		para(`{"type":"text","text":"Güzel bir kitap."}`),
		para(`{"type":"text","text":"uşak","marks":[{"type":"spoiler"},{"type":"bold"}]}`),
		`{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]},{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}]}]},{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"b","marks":[{"type":"italic"}]}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a\u0000"}]}]}`,
		`{"type":"doc","content":[{"type":"heading","attrs":{"level":1}}]}`,
		`null`, `[]`, `{}`, `{"type":"doc","content":[]}`, `{"type":"doc","type":"paragraph"}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		d, err := Parse(raw)
		if err != nil {
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("hata ErrInvalid'i sarmalı: %v", err)
			}
			return
		}
		canon := d.JSON()
		again, err := Parse(canon)
		if err != nil {
			t.Fatalf("kanonik belge yeniden doğrulanamadı: %v\n%s", err, canon)
		}
		if !Equal(d, again) || string(again.JSON()) != string(canon) {
			t.Fatalf("kanonik hâl kararlı değil:\n%s\n%s", canon, again.JSON())
		}
		text := d.Text()
		if n := utf8.RuneCountInString(text); n < 1 || n > MaxTextLen || strings.TrimSpace(text) == "" {
			t.Fatalf("düz metin sınır dışında: %d karakter", n)
		}
		if strings.ContainsFunc(text, func(r rune) bool { return r != '\n' && forbiddenRune(r) }) || !utf8.ValidString(text) {
			t.Fatalf("düz metin denetim karakteri ya da geçersiz UTF-8 taşıyor: %q", text)
		}
	})
}

// FuzzFromPlain: her düz metin bir belgeye çevrilebilir. Çevrilen belge ya doğrulamadan geçer ya da
// yalnız boş olduğu ya da bir sınırı aştığı için reddedilir; yapı hatası asla olmaz. Geçtiğinde
// kanoniktir ve spoiler bayrağı korunur.
func FuzzFromPlain(f *testing.F) {
	f.Add("Güzel bir kitap.", false)
	f.Add("bir\r\niki\n\n\tüç\x00", true)
	f.Add("", false)
	f.Add(strings.Repeat("a\n", 3000), true)
	f.Fuzz(func(t *testing.T, body string, spoiler bool) {
		d := FromPlain(body, spoiler)
		parsed, err := Parse(d.JSON())
		if err != nil {
			if !errors.Is(err, ErrEmpty) && !errors.Is(err, ErrLimit) {
				t.Fatalf("FromPlain yapı hatası olan belge üretti: %v", err)
			}
			return
		}
		if !Equal(parsed, d) {
			t.Fatalf("FromPlain kanonik olmayan belge üretti: %s", d.JSON())
		}
		if parsed.HasSpoiler() != spoiler {
			t.Fatalf("spoiler bayrağı korunmadı: %v", parsed.HasSpoiler())
		}
	})
}

// BenchmarkParse: sınırların dayandığı ölçüm. "tipik" 5000 karakterlik, biçimli bir inceleme;
// "en kötü" düğüm sınırına dayanan, her düğümü üç işaretli belge.
func BenchmarkParse(b *testing.B) {
	item := `{"type":"text","text":"` + strings.Repeat("ş", 40) + `"},{"type":"text","text":"` + strings.Repeat("a", 10) + `","marks":[{"type":"bold"},{"type":"spoiler"}]}`
	typical := `{"type":"doc","content":[` + strings.TrimSuffix(strings.Repeat(`{"type":"paragraph","content":[`+item+`]},`, 98), ",") + `]}`
	worstItem := `{"type":"text","text":"a","marks":[{"type":"bold"},{"type":"italic"},{"type":"spoiler"}]},{"type":"text","text":"b"},`
	worst := `{"type":"doc","content":[{"type":"paragraph","content":[` + strings.TrimSuffix(strings.Repeat(worstItem, (MaxNodes-2)/2), ",") + `]}]}`
	for name, doc := range map[string]string{"tipik": typical, "en kötü": worst} {
		b.Run(name, func(b *testing.B) {
			raw := []byte(doc)
			if _, err := Parse(raw); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for b.Loop() {
				_, _ = Parse(raw)
			}
		})
	}
}
