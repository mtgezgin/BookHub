package moderation

import (
	"errors"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/books"
)

// TestNextExhaustive, 4 statü × 4 işlemin TAMAMINI gezer: izinli 4 geçiş dışındaki
// 12 çiftin hepsi reddedilmeli. Yeni bir geçiş eklemek = tabloya satır eklemek; bir
// geçişi yanlışlıkla açmak bu testi kırar.
func TestNextExhaustive(t *testing.T) {
	allowed := map[[2]string]string{
		{books.StatusPending, string(ActionApprove)}:  books.StatusApproved,
		{books.StatusPending, string(ActionReject)}:   books.StatusRejected,
		{books.StatusApproved, string(ActionArchive)}: books.StatusArchived,
		{books.StatusArchived, string(ActionRestore)}: books.StatusApproved,
	}
	statuses := []string{books.StatusPending, books.StatusApproved, books.StatusRejected, books.StatusArchived}
	actions := []Action{ActionApprove, ActionReject, ActionArchive, ActionRestore}

	for _, st := range statuses {
		for _, a := range actions {
			t.Run(st+"/"+string(a), func(t *testing.T) {
				got, err := Next(st, a)
				want, ok := allowed[[2]string{st, string(a)}]
				if ok {
					if err != nil || got != want {
						t.Errorf("istenen %s, gelen %q (err %v)", want, got, err)
					}
					return
				}
				if !errors.Is(err, ErrInvalidTransition) {
					t.Errorf("yasak geçiş kabul edildi: %s → %q (err %v)", st, got, err)
				}
			})
		}
	}
}

func TestNextUnknownInputs(t *testing.T) {
	if _, err := Next("DRAFT", ActionApprove); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("bilinmeyen statü reddedilmeli, gelen %v", err)
	}
	if _, err := Next(books.StatusPending, Action("publish")); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("bilinmeyen işlem reddedilmeli, gelen %v", err)
	}
}
