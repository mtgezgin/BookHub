package books

import (
	"errors"
	"testing"
)

// TestParseStatus: sınırda yalnızca dört kanonik değer geçer; küçük harf / yazım
// hatası sessizce boş listeye dönüşmez, hata olur.
func TestParseStatus(t *testing.T) {
	for _, ok := range []string{StatusPending, StatusApproved, StatusRejected, StatusArchived} {
		if got, err := ParseStatus(ok); err != nil || got != ok {
			t.Errorf("%q kabul edilmeli: %q, %v", ok, got, err)
		}
	}
	for _, bad := range []string{"", "pending", "APROVED", "DELETED", " PENDING"} {
		if _, err := ParseStatus(bad); !errors.Is(err, ErrUnknownStatus) {
			t.Errorf("%q reddedilmeli, gelen %v", bad, err)
		}
	}
}
