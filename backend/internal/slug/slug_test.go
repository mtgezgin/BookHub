package slug

import (
	"strings"
	"testing"
)

func TestMake(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Clean Code", "clean-code"},
		{"Bilim Kurgu & Fantastik", "bilim-kurgu-fantastik"},
		{"Şeker Portakalı", "seker-portakali"},
		{"İNCE MEMED", "ince-memed"}, // İ → I + birleşik nokta → i
		{"Çağdaş Türk Öyküsü", "cagdas-turk-oykusu"},
		{"  --Go   101!!  ", "go-101"}, // baş/son ayırıcı atılır, ardışıklar tekleşir
		{"The Go Programming Language (2nd ed.)", "the-go-programming-language-2nd-ed"},
		{"Dune", "dune"},
		{"🙂🙂", ""}, // yalnızca desteklenmeyen karakter → boş (çağıran doğrular)

		// REGRESYON (review bulgusu F1): büyük harfli aksanlılar eskiden AYIRICI sayılıp
		// atılıyordu — "Âşık Veysel" kaba bir kelimeye dönüşüyordu.
		{"Âşık Veysel", "asik-veysel"},
		{"KÂĞIT EV", "kagit-ev"},
		{"Émile Zola", "emile-zola"},
		{"Îmâr", "imar"},
		{"ÑANDÚ", "nandu"},
		// NFD'de ayrışmayan harfler (özel harita).
		{"Straße", "strasse"},
		{"Søren Kierkegaard", "soren-kierkegaard"},
		{"Łódź", "lodz"},
		{"KIŞ ıŞığı", "kis-isigi"},
	}
	for _, tt := range tests {
		if got := Make(tt.in); got != tt.want {
			t.Errorf("Make(%q) = %q, istenen %q", tt.in, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"kisa", 10, "kisa"},
		{"tam-sinir", 9, "tam-sinir"},
		{"bir-iki-uc", 7, "bir-iki"},  // s[7] == '-' → tam kelime
		{"bir-iki-uc", 6, "bir"},      // "bir-ik" → son tireden kes
		{"tekkelimeuzun", 5, "tekke"}, // tire yoksa sert kes
		{"ab-cdefghij", 4, "ab"},      // yarım kelime bırakılmaz
	}
	for _, tt := range tests {
		if got := Truncate(tt.in, tt.max); got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, istenen %q", tt.in, tt.max, got, tt.want)
		}
	}

	long := Make(strings.Repeat("Uzun Başlık ", 30)) // ~360 karakter
	got := Truncate(long, 120)
	if len(got) > 120 || strings.HasSuffix(got, "-") || !strings.HasPrefix(long, got) {
		t.Errorf("uzun slug: %d karakter, %q", len(got), got)
	}
}
