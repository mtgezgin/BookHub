package slug

import "testing"

func TestMake(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Clean Code", "clean-code"},
		{"Bilim Kurgu & Fantastik", "bilim-kurgu-fantastik"},
		{"Şeker Portakalı", "seker-portakali"},
		{"İNCE MEMED", "ince-memed"}, // İ → i (Türkçe büyük İ)
		{"Çağdaş Türk Öyküsü", "cagdas-turk-oykusu"},
		{"  --Go   101!!  ", "go-101"}, // baş/son ayırıcı atılır, ardışıklar tekleşir
		{"The Go Programming Language (2nd ed.)", "the-go-programming-language-2nd-ed"},
		{"Dune", "dune"},
		{"🙂🙂", ""}, // yalnızca desteklenmeyen karakter → boş (çağıran doğrular)
	}
	for _, tt := range tests {
		if got := Make(tt.in); got != tt.want {
			t.Errorf("Make(%q) = %q, istenen %q", tt.in, got, tt.want)
		}
	}
}
