package library

import (
	"errors"
	"testing"
	"time"
)

// variant, testte bir komut ve (başla için) biçimi.
type variant struct {
	c Command
	m StartMode
}

func (v variant) String() string {
	if v.m == ModeAuto {
		return string(v.c)
	}
	return string(v.c) + "/" + string(v.m)
}

var (
	allStatuses = []Status{StatusNone, StatusWantToRead, StatusReading, StatusRead, StatusAbandoned}
	allVariants = []variant{
		{CmdWant, ModeAuto}, {CmdPostpone, ModeAuto},
		{CmdStart, ModeAuto}, {CmdStart, ModeResume}, {CmdStart, ModeRestart},
		{CmdFinish, ModeAuto}, {CmdMarkRead, ModeAuto}, {CmdAbandon, ModeAuto}, {CmdRemove, ModeAuto},
	}
)

// TestResolveExhaustive, 5 durum × 9 komutun TAMAMINI gezer (spec Doğrulama 1'in saf yarısı). Beklenen
// tablo spec 17A § 3'ün yazılı hâli; tabloda olmayan çiftler "hedefteyse hiçbir şey, değilse en yakın
// komut". Hiçbir çift hata dönmez. Bir geçişi değiştirmek bu tabloyu bilerek değiştirmeyi gerektirir.
func TestResolveExhaustive(t *testing.T) {
	none := step{act: actNone}
	setWant := step{act: actSetStatus, status: StatusWantToRead}
	start, resume := step{act: actStart}, step{act: actResume}
	closeRead := step{act: actClose, status: StatusRead}
	markRead := step{act: actMarkRead}

	want := map[Status]map[string]step{
		StatusNone: {
			"want": setWant, "postpone": setWant,
			"start": start, "start/resume": start, "start/restart": start,
			"finish": markRead, "mark-read": markRead,
			"abandon": {act: actSetStatus, status: StatusAbandoned}, "remove": none,
		},
		StatusWantToRead: {
			"want": none, "postpone": none,
			"start": start, "start/resume": start, "start/restart": start,
			"finish": markRead, "mark-read": markRead,
			"abandon": {act: actSetStatus, status: StatusAbandoned}, "remove": {act: actRemove},
		},
		StatusReading: {
			"want": {act: actPostpone}, "postpone": {act: actPostpone},
			"start": none, "start/resume": none, "start/restart": none,
			"finish": closeRead, "mark-read": closeRead,
			"abandon": {act: actClose, status: StatusAbandoned}, "remove": {act: actRemove},
		},
		StatusRead: {
			"want": setWant, "postpone": setWant,
			"start": start, "start/resume": start, "start/restart": start, // yeniden okuma
			"finish": none, "mark-read": none,
			"abandon": {act: actSetStatus, status: StatusAbandoned}, "remove": {act: actRemove},
		},
		StatusAbandoned: {
			"want": setWant, "postpone": setWant,
			"start": resume, "start/resume": resume, "start/restart": start,
			"finish": markRead, "mark-read": markRead,
			"abandon": none, "remove": {act: actRemove},
		},
	}

	for _, st := range allStatuses {
		for _, v := range allVariants {
			name := string(st)
			if name == "" {
				name = "yok"
			}
			t.Run(name+"/"+v.String(), func(t *testing.T) {
				got, err := resolve(st, v.c, v.m)
				if err != nil {
					t.Fatalf("hiçbir çift hata dönmemeli: %v", err)
				}
				if w := want[st][v.String()]; got != w {
					t.Errorf("istenen %+v, gelen %+v", w, got)
				}
			})
		}
	}
}

func TestResolveRejectsUnknownInput(t *testing.T) {
	cases := []struct {
		name string
		st   Status
		c    Command
		m    StartMode
	}{
		{"bilinmeyen komut", StatusReading, Command("read"), ModeAuto},
		{"bilinmeyen durum", Status("DRAFT"), CmdStart, ModeAuto},
		{"bilinmeyen biçim", StatusAbandoned, CmdStart, StartMode("continue")},
		{"biçim yalnız başla'da", StatusReading, CmdFinish, ModeResume},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolve(tc.st, tc.c, tc.m); !errors.Is(err, ErrInvalidCommand) {
				t.Errorf("ErrInvalidCommand bekleniyordu, gelen %v", err)
			}
		})
	}
	if ValidCommand("read") || !ValidCommand("mark-read") {
		t.Error("ValidCommand komut adlarını yanlış süzüyor")
	}
}

// TestCommandForStatus, eski PUT {status} eşlemesi (spec 17A § 3, Doğrulama 9). Komut makineden
// geçtiği için her durumdan hedef durum aynı: eski uç ne yaptıysa (durumu ayarla) o olur.
func TestCommandForStatus(t *testing.T) {
	want := map[Status]Command{
		StatusWantToRead: CmdWant, StatusReading: CmdStart, StatusRead: CmdMarkRead, StatusAbandoned: CmdAbandon,
	}
	for s, c := range want {
		got, ok := CommandForStatus(s)
		if !ok || got != c {
			t.Errorf("%s → %q (%v), istenen %q", s, got, ok, c)
		}
		for _, from := range allStatuses {
			st, err := resolve(from, got, ModeAuto)
			if err != nil {
				t.Fatalf("%s'den %s: %v", from, got, err)
			}
			if target := targetOf(from, st); target != s {
				t.Errorf("eski PUT %s, %s'den %s'e götürdü", s, from, target)
			}
		}
	}
	if _, ok := CommandForStatus("DONE"); ok {
		t.Error("bilinmeyen durum komuta çevrilmemeli")
	}
}

// targetOf, bir etkinin sonunda satırın durumu (StatusNone = satır yok).
func targetOf(from Status, st step) Status {
	switch st.act {
	case actNone:
		return from
	case actSetStatus, actClose:
		return st.status
	case actStart, actResume:
		return StatusReading
	case actPostpone:
		return StatusWantToRead
	case actMarkRead:
		return StatusRead
	}
	return StatusNone // actRemove
}

// TestValidProgress, 0017'deki library_progress_valid kısıtının Go karşılığı (spec Doğrulama 2).
func TestValidProgress(t *testing.T) {
	page, percent := new(UnitPage), new(UnitPercent)
	cases := []struct {
		name string
		r    reading
		ok   bool
	}{
		{"hiç girilmemiş (Faz 15'ten kalan)", reading{}, true},
		{"sayfa, toplam içinde", reading{Unit: page, Progress: new(int32(42)), TotalPages: new(int32(310))}, true},
		{"sayfa, son sayfa", reading{Unit: page, Progress: new(int32(310)), TotalPages: new(int32(310))}, true},
		{"sayfa, toplam bilinmiyor", reading{Unit: page, Progress: new(int32(900))}, true},
		{"sayfa, negatif", reading{Unit: page, Progress: new(int32(-1)), TotalPages: new(int32(310))}, false},
		{"sayfa, toplamı aşıyor", reading{Unit: page, Progress: new(int32(311)), TotalPages: new(int32(310))}, false},
		{"yüzde, sınırda", reading{Unit: percent, Progress: new(int32(100))}, true},
		{"yüzde, 100'ü aşıyor", reading{Unit: percent, Progress: new(int32(101))}, false},
		{"yüzde, toplamla", reading{Unit: percent, Progress: new(int32(5)), TotalPages: new(int32(310))}, false},
		{"birimsiz ilerleme", reading{Progress: new(int32(5))}, false},
		{"birimli ama ilerlemesiz", reading{Unit: page}, false},
		{"bilinmeyen birim", reading{Unit: new("chapter"), Progress: new(int32(3))}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validProgress(tc.r)
			if tc.ok != (err == nil) || (err != nil && !errors.Is(err, ErrInvalidProgress)) {
				t.Errorf("ok = %v bekleniyordu, gelen %v", tc.ok, err)
			}
		})
	}
}

func TestValidDates(t *testing.T) {
	d := func(s string) *time.Time {
		v, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatal(err)
		}
		return &v
	}
	if err := validDates(d("2026-11-12"), d("2026-11-01")); !errors.Is(err, ErrInvalidDates) {
		t.Errorf("bitiş başlangıçtan önce: ErrInvalidDates bekleniyordu, gelen %v", err)
	}
	for _, c := range [][2]*time.Time{{d("2026-11-01"), d("2026-11-01")}, {nil, d("2026-11-01")}, {d("2026-11-01"), nil}, {nil, nil}} {
		if err := validDates(c[0], c[1]); err != nil {
			t.Errorf("%v–%v geçerli olmalı: %v", c[0], c[1], err)
		}
	}
}
