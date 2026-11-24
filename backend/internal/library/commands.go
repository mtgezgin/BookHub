package library

import (
	"errors"
	"fmt"
)

// StatusNone, "kitaplıkta değil": satır yok. Veritabanında hiç yazılmaz; durum makinesinin girdisi.
const StatusNone Status = ""

// İlerleme birimleri (spec § Kararlar "İlerleme birimi"): tek birim, dönüşüm yok.
const (
	UnitPage    = "page"
	UnitPercent = "percent"
)

// Günlük satırının sonucu; reading_log'daki CHECK'in Go karşılığı.
const (
	OutcomeFinished  = "FINISHED"
	OutcomeAbandoned = "ABANDONED"
)

// Command, kullanıcının raf üzerindeki NİYETİ (spec kavram 2). İstemci "durumu X yap" demez; "başla",
// "bitir" der. Her komut her durumda tanımlıdır: ya bir geçiş yapar ya da zararsızca hiçbir şey
// yapmaz. Kullanıcı bir niyeti için hata görmez (çift tık, yeniden deneme, iki sekme).
type Command string

const (
	CmdWant     Command = "want"
	CmdStart    Command = "start"
	CmdFinish   Command = "finish"
	CmdAbandon  Command = "abandon"
	CmdPostpone Command = "postpone"
	CmdMarkRead Command = "mark-read"
	CmdRemove   Command = "remove"
)

// StartMode, yarım bırakılmış bir kitaba "başla" denince ne olacağı (kullanıcı kararı, 2026-11-16).
type StartMode string

const (
	// ModeAuto: yarım bırakılandan "Devam et", başka her durumdan yeni okuma.
	ModeAuto StartMode = ""
	// ModeResume: son yarım kalan okuma geri açılır (sayfa ve başlangıç günü korunur).
	ModeResume StartMode = "resume"
	// ModeRestart: yeni okuma; eski kayıt geçmişte kalır.
	ModeRestart StartMode = "restart"
)

// ErrInvalidCommand, bilinmeyen bir komut ya da başlama biçimi: istemcinin hatası (400). Durumdan
// doğan bir hata DEĞİLDİR; o tür hata bu makinede yok.
var ErrInvalidCommand = errors.New("bilinmeyen raf komutu")

// action, bir komutun bulunduğu durumdaki ETKİSİ. Komut (kullanıcının dili) ile etki (servisin yapacağı
// iş) ayrı: dokuz komut × beş durum, sekiz etkiye iner.
type action int

const (
	actNone      action = iota // hiçbir şey yazılmaz (zaten hedef durumda)
	actSetStatus               // yalnız durum değişir; okuma alanları boşalır, günlük yazılmaz
	actStart                   // yeni okuma: varsayılan (ya da seçilen) baskı, sayfa toplamının görüntüsü
	actResume                  // son yarım bırakılan okuma günlükten geri açılır, günlük satırı silinir
	actClose                   // okuyorken kapat: günlüğe yazılır, durum READ ya da ABANDONED
	actPostpone                // okuyorken ertele: ilerleme varsa günlüğe "yarım bırakıldı", durum WANT_TO_READ
	actMarkRead                // okumuyorken "Okudum": günlüğe tarihsiz (ya da verilen tarihli) FINISHED
	actRemove                  // satır silinir; okuyorken ve ilerleme varsa önce günlüğe "yarım bırakıldı"
)

// step, resolve'un çıktısı: yapılacak iş ve (gerekiyorsa) hedef durum.
type step struct {
	act    action
	status Status // actSetStatus ve actClose için hedef
}

// resolve, DURUM MAKİNESİ'nin tamamı (spec 17A § 3): mevcut durum + komut → etki. Saf fonksiyon:
// veritabanını, saati, HTTP'yi bilmez → bütün (durum, komut) çiftleri tabloyla test edilir
// (commands_test.go). Spec'in tablosunda olmayan çiftler hedef durumdaysa hiçbir şey yapmaz,
// değilse en yakın tanımlı komuta çevrilir; hiçbiri hata değildir.
//
//	               ┌──────── want / postpone ────────┐
//	yok ─want─▶ WANT_TO_READ ─start─▶ READING ─finish─▶ READ ─start─▶ READING (yeniden okuma)
//	                                   │   ▲
//	                             abandon   start (resume: devam et · restart: baştan)
//	                                   ▼   │
//	                                 ABANDONED
func resolve(current Status, c Command, m StartMode) (step, error) {
	if !knownStatus(current) {
		return step{}, fmt.Errorf("%w: durum %q", ErrInvalidCommand, current)
	}
	if m != ModeAuto && (c != CmdStart || (m != ModeResume && m != ModeRestart)) {
		return step{}, fmt.Errorf("%w: %s için biçim %q", ErrInvalidCommand, c, m)
	}
	switch c {
	case CmdWant, CmdPostpone:
		// "Ertele" okumuyorken "okumak istiyorum"un aynısıdır; okurken de "istiyorum" ertelemedir.
		switch current {
		case StatusWantToRead:
			return step{act: actNone}, nil
		case StatusReading:
			return step{act: actPostpone}, nil
		}
		return step{act: actSetStatus, status: StatusWantToRead}, nil
	case CmdStart:
		switch {
		case current == StatusReading:
			return step{act: actNone}, nil
		case current == StatusAbandoned && m != ModeRestart:
			return step{act: actResume}, nil
		}
		return step{act: actStart}, nil // yok, WANT_TO_READ, READ (yeniden okuma), ABANDONED + restart
	case CmdFinish, CmdMarkRead:
		switch current {
		case StatusRead:
			return step{act: actNone}, nil
		case StatusReading:
			return step{act: actClose, status: StatusRead}, nil
		}
		return step{act: actMarkRead}, nil
	case CmdAbandon:
		switch current {
		case StatusAbandoned:
			return step{act: actNone}, nil
		case StatusReading:
			return step{act: actClose, status: StatusAbandoned}, nil
		}
		// Okumuyorken "yarım bıraktım": yalnız durum (eski PUT {status: ABANDONED}'un davranışı);
		// kapatılacak bir okuma olmadığı için günlüğe bir şey yazılmaz.
		return step{act: actSetStatus, status: StatusAbandoned}, nil
	case CmdRemove:
		if current == StatusNone {
			return step{act: actNone}, nil
		}
		return step{act: actRemove}, nil
	}
	return step{}, fmt.Errorf("%w: %q", ErrInvalidCommand, c)
}

func knownStatus(s Status) bool { return s == StatusNone || ValidStatus(s) }

// CommandForStatus, eski `PUT /api/me/library/{id} {status}` ucunun komut karşılığı (spec 17A § 3):
// yayın penceresindeki eski arayüz de aynı servisten geçer, ikinci bir yazma yolu kalmaz. READING
// "başla"dır (yarım bırakılmışsa ModeAuto onu sürdürür), READ "zaten okudum"dur (okurken tarihsiz
// bitirir). Bilinmeyen durum için ok=false.
func CommandForStatus(s Status) (c Command, ok bool) {
	switch s {
	case StatusWantToRead:
		return CmdWant, true
	case StatusReading:
		return CmdStart, true
	case StatusRead:
		return CmdMarkRead, true
	case StatusAbandoned:
		return CmdAbandon, true
	}
	return "", false
}

// ValidCommand, dışarıdan gelen komut adını sınırda doğrular (handler'ın 400'ü).
func ValidCommand(c string) bool {
	switch Command(c) {
	case CmdWant, CmdStart, CmdFinish, CmdAbandon, CmdPostpone, CmdMarkRead, CmdRemove:
		return true
	}
	return false
}
