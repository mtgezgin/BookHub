package books

import "testing"

func TestEscapeLike(t *testing.T) {
	tests := []struct{ in, want string }{
		{"dune", "dune"},
		{"%50", `\%50`},
		{"a_b", `a\_b`},
		{`c:\yol`, `c:\\yol`},
	}
	for _, tt := range tests {
		if got := EscapeLike(tt.in); got != tt.want {
			t.Errorf("EscapeLike(%q) = %q, istenen %q", tt.in, got, tt.want)
		}
	}
}
