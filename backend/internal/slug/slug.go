// Package slug, insan metninden URL'de kimlik olarak kullanılabilecek slug üretir:
// "Bilim Kurgu & Fantastik" → "bilim-kurgu-fantastik".
//
// Neden sunucuda: öneri formundaki kullanıcıdan (Faz 4, Faz 9) slug uydurmasını
// beklemek hem kötü deneyim hem de tutarsız veri demek. İstemci slug gönderirse o
// kullanılır; göndermezse başlıktan türetilir.
package slug

import "strings"

// Türkçe (ve yaygın Latin) harfleri ASCII karşılığına çevir. Go'nun strings.ToLower'ı
// "I"yı Türkçe kurala göre değil evrensel kurala göre küçültür ("ı" değil "i"), bu da
// slug için tam istediğimiz şey.
var transliterate = map[rune]string{
	'ç': "c", 'Ç': "c", 'ğ': "g", 'Ğ': "g", 'ı': "i", 'İ': "i",
	'ö': "o", 'Ö': "o", 'ş': "s", 'Ş': "s", 'ü': "u", 'Ü': "u",
	'â': "a", 'î': "i", 'û': "u", 'é': "e", 'è': "e", 'ê': "e",
	'à': "a", 'á': "a", 'ä': "a", 'ë': "e", 'ï': "i", 'ñ': "n",
}

// Make, s'den slug üretir. Harf/rakam dışı her şey tek bir tireye dönüşür; baştaki ve
// sondaki tireler atılır. Sonuç boş olabilir (ör. yalnızca emoji) — çağıran doğrular.
func Make(s string) string {
	var b strings.Builder
	pendingDash := false
	for _, r := range s {
		var chunk string
		switch {
		case transliterate[r] != "":
			chunk = transliterate[r]
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			chunk = string(r)
		case r >= 'A' && r <= 'Z':
			chunk = string(r + ('a' - 'A'))
		default:
			// Ayırıcı: ardışık ayırıcılar tek tireye iner, en başta tire açılmaz.
			pendingDash = b.Len() > 0
			continue
		}
		if pendingDash {
			b.WriteByte('-')
			pendingDash = false
		}
		b.WriteString(chunk)
	}
	return b.String()
}
