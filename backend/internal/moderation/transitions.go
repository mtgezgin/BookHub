// Package moderation, kitabın yaşam döngüsünü yönetir: öneri, onay/red, arşiv, düzenleme
// ve bunların her birinin denetim kaydı (audit log). Faz 4'ün servis katmanı.
package moderation

import (
	"errors"
	"fmt"

	"github.com/mtgezgin/bookhub/backend/internal/books"
)

// Action, bir kitaba uygulanabilecek moderasyon işlemi.
type Action string

const (
	ActionApprove Action = "approve"
	ActionReject  Action = "reject"
	ActionArchive Action = "archive"
	ActionRestore Action = "restore"
)

// ErrInvalidTransition, mevcut statüden istenen işleme geçilemediğinde döner
// (ör. REJECTED bir kitabı "approve" etmek). Handler bunu 409'a çevirir: istek biçimce
// doğru ama kaynağın ŞU ANKİ durumuyla çelişiyor.
var ErrInvalidTransition = errors.New("bu statüdeki kitaba bu işlem uygulanamaz")

// transitions, DURUM MAKİNESİ'nin tamamı: işlem → (mevcut statü → yeni statü).
// Burada olmayan her çift yasaktır. Kural VERİ olarak duruyor, if/else ağacı olarak değil:
// tüm makineyi tek bakışta okursun ve testi 4×4'lük tabloyu gezerek yazarsın.
//
//	PENDING ──approve──▶ APPROVED ──archive──▶ ARCHIVED
//	   │                    ▲                      │
//	   └──reject──▶ REJECTED └───────restore───────┘
//
// REJECTED son durak: reddedilen öneri yeniden açılmaz (kullanıcı yeniden önerebilir).
var transitions = map[Action]map[string]string{
	ActionApprove: {books.StatusPending: books.StatusApproved},
	ActionReject:  {books.StatusPending: books.StatusRejected},
	ActionArchive: {books.StatusApproved: books.StatusArchived},
	ActionRestore: {books.StatusArchived: books.StatusApproved},
}

// Next, saf fonksiyon: mevcut statü + işlem → yeni statü ya da ErrInvalidTransition.
// DB'yi, HTTP'yi, saati bilmez → tablo testleriyle eksiksiz doğrulanabilir.
func Next(current string, a Action) (string, error) {
	if to, ok := transitions[a][current]; ok {
		return to, nil
	}
	return "", fmt.Errorf("%w: %s ✗ %s", ErrInvalidTransition, current, a)
}
