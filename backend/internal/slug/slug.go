// Package slug, insan metninden URL'de kimlik olarak kullanılabilecek slug üretir:
// "Bilim Kurgu & Fantastik" → "bilim-kurgu-fantastik".
//
// Neden sunucuda: öneri formundaki kullanıcıdan (Faz 4, Faz 9) slug uydurmasını
// beklemek hem kötü deneyim hem de tutarsız veri demek. İstemci slug gönderirse o
// kullanılır; göndermezse başlıktan türetilir.
package slug

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// special, Unicode ayrıştırmasıyla (NFD) ASCII'ye İNMEYEN harfler. Geri kalan her aksanlı
// harf (Â, É, Ş, Ğ, Ü, Ñ, İ...) NFD'de "temel harf + birleşik işaret"e ayrılır ve işaret
// atılınca kendiliğinden ASCII olur — elle harita tutmaya gerek kalmaz.
var special = map[rune]string{
	'ı': "i", // Türkçe noktasız ı: ayrıştırılamaz, kendi başına bir harf
	'ß': "ss", 'æ': "ae", 'Æ': "ae", 'œ': "oe", 'Œ': "oe",
	'ø': "o", 'Ø': "o", 'ł': "l", 'Ł': "l", 'đ': "d", 'Đ': "d",
}

// Make, s'den slug üretir. Harf/rakam dışı her şey tek bir tireye dönüşür; baştaki ve
// sondaki tireler atılır. Sonuç boş olabilir (ör. yalnızca emoji) — çağıran doğrular.
//
// Neden NFD: ilk sürüm küçük harfli bir aksan haritası kullanıyordu; haritada olmayan
// büyük harfler (Â, É, Î...) AYIRICI sayılıp atılıyordu → "Âşık Veysel" → "sik-veysel"
// (kaba bir kelime, üstelik kalıcı public URL). Harita tutmak, eksik harfi bulana kadar
// hata yapmak demek; ayrıştırma ise bütün aksanlı Latin harflerini tek kuralla çözer.
func Make(s string) string {
	var b strings.Builder
	pendingDash := false
	for _, r := range norm.NFD.String(s) {
		// Birleşik işaret (şapka, çengel, nokta...): harfin PARÇASI, ayırıcı değil → atla.
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		var chunk string
		switch {
		case special[r] != "":
			chunk = special[r]
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

// Truncate, slug'ı en fazla max karaktere indirir; kelimeyi ortadan bölmemek için son
// tireden keser. (Slug yalnızca ASCII içerdiği için bayt = karakter.)
//
// Neden: 300 karakterlik geçerli bir başlıktan türetilen slug, 120'lik slug sınırını
// aşıp isteği reddettiriyordu — istemci hiç slug göndermemişken "slug çok uzun" hatası.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if s[max] == '-' { // tam kelime sınırı
		return s[:max]
	}
	cut := s[:max]
	if i := strings.LastIndexByte(cut, '-'); i > 0 {
		cut = cut[:i]
	}
	return cut
}
