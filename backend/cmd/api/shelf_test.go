package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/library"
)

// fakeShelf, raf servisinin sahtesi: handler'ın servise NE gönderdiğini kaydeder, verilen rafı ya da
// hatayı döner. Servisin kendisi gerçek veritabanıyla internal/library'de test edilir.
type fakeShelf struct {
	shelf library.Shelf
	err   error

	calls                    int
	gotUser, gotBook, gotLog int64
	gotIn                    *library.CommandInput
	gotPatch                 *library.ReadingPatch
	gotStarted, gotEnded     *time.Time
}

func (f *fakeShelf) Get(_ context.Context, user, book int64) (library.Shelf, error) {
	f.calls++
	f.gotUser, f.gotBook = user, book
	return f.shelf, f.err
}
func (f *fakeShelf) Do(_ context.Context, user, book int64, in library.CommandInput) (library.Shelf, error) {
	f.calls++
	f.gotUser, f.gotBook, f.gotIn = user, book, &in
	return f.shelf, f.err
}
func (f *fakeShelf) UpdateReading(_ context.Context, user, book int64, p library.ReadingPatch) (library.Shelf, error) {
	f.calls++
	f.gotUser, f.gotBook, f.gotPatch = user, book, &p
	return f.shelf, f.err
}
func (f *fakeShelf) UpdateLogDates(_ context.Context, user, logID int64, started, ended *time.Time) (library.Shelf, error) {
	f.calls++
	f.gotUser, f.gotLog, f.gotStarted, f.gotEnded = user, logID, started, ended
	return f.shelf, f.err
}
func (f *fakeShelf) DeleteLog(_ context.Context, user, logID int64) (library.Shelf, error) {
	f.calls++
	f.gotUser, f.gotLog = user, logID
	return f.shelf, f.err
}

func shelfRouter(fake *fakeShelf) http.Handler {
	return userRouter(deps{shelf: fake})
}

func testDay(t *testing.T, s string) *time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatal(err)
	}
	return &d
}

var allShelfCommands = []library.Command{
	library.CmdWant, library.CmdStart, library.CmdFinish, library.CmdAbandon,
	library.CmdPostpone, library.CmdMarkRead, library.CmdRemove,
}

// TestShelfCommandRoutes: her komutun kendi rotası var ve gövdesiz istek "varsayılanlarla" o komuttur
// (tek tık). Kullanıcı token'dan, kitap adresten.
func TestShelfCommandRoutes(t *testing.T) {
	for _, c := range allShelfCommands {
		t.Run(string(c), func(t *testing.T) {
			fake := &fakeShelf{shelf: library.Shelf{BookID: 5}}
			rec := do(t, shelfRouter(fake), http.MethodPost, "/api/me/books/5/shelf/"+string(c), "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d (%s)", rec.Code, rec.Body)
			}
			if fake.gotUser != 7 || fake.gotBook != 5 || fake.gotIn == nil || *fake.gotIn != (library.CommandInput{Command: c}) {
				t.Errorf("servise giden: user %d book %d girdi %+v", fake.gotUser, fake.gotBook, fake.gotIn)
			}
		})
	}
	t.Run("bilinmeyen komut rotası yok", func(t *testing.T) {
		fake := &fakeShelf{}
		rec := do(t, shelfRouter(fake), http.MethodPost, "/api/me/books/5/shelf/read", "")
		if rec.Code != http.StatusNotFound || fake.calls != 0 {
			t.Errorf("status %d, çağrı %d", rec.Code, fake.calls)
		}
	})
}

func TestShelfCommandBodies(t *testing.T) {
	t.Run("başla: biçim, baskı, birim ve gün servise gider", func(t *testing.T) {
		fake := &fakeShelf{shelf: library.Shelf{BookID: 5}}
		rec := do(t, shelfRouter(fake), http.MethodPost, "/api/me/books/5/shelf/start",
			`{"mode":"restart","edition_id":3,"unit":"page","started_on":"2026-11-24"}`)
		if rec.Code != http.StatusOK || fake.gotIn == nil {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body)
		}
		in := fake.gotIn
		if in.Command != library.CmdStart || in.Mode != library.ModeRestart || *in.EditionID != 3 || *in.Unit != "page" ||
			!in.StartedOn.Equal(*testDay(t, "2026-11-24")) || in.EndedOn != nil {
			t.Errorf("girdi %+v", in)
		}
	})
	for _, c := range []library.Command{library.CmdFinish, library.CmdAbandon} {
		t.Run(string(c)+": bitiş günü servise gider", func(t *testing.T) {
			fake := &fakeShelf{shelf: library.Shelf{BookID: 5}}
			rec := do(t, shelfRouter(fake), http.MethodPost, "/api/me/books/5/shelf/"+string(c), `{"ended_on":"2026-11-12"}`)
			if rec.Code != http.StatusOK || fake.gotIn == nil || fake.gotIn.EndedOn == nil || !fake.gotIn.EndedOn.Equal(*testDay(t, "2026-11-12")) {
				t.Errorf("status %d girdi %+v (%s)", rec.Code, fake.gotIn, rec.Body)
			}
		})
	}
	t.Run("gövdesi olmayan komuta boş nesne", func(t *testing.T) {
		fake := &fakeShelf{shelf: library.Shelf{BookID: 5}}
		if rec := do(t, shelfRouter(fake), http.MethodPost, "/api/me/books/5/shelf/want", `{}`); rec.Code != http.StatusOK {
			t.Errorf("status %d (%s)", rec.Code, rec.Body)
		}
	})

	// Her komut yalnız kendi alanlarını kabul eder; geçersiz istek servise hiç ulaşmaz.
	bad := []struct {
		name, cmd, body, code string
	}{
		{"bilinmeyen biçim", "start", `{"mode":"continue"}`, "validation_error"},
		{"başla'ya bitiş günü", "start", `{"ended_on":"2026-11-24"}`, "invalid_json"},
		{"bitir'e biçim", "finish", `{"mode":"resume"}`, "invalid_json"},
		{"istiyorum'a durum", "want", `{"status":"READ"}`, "invalid_json"},
		{"zaten okudum'a gün", "mark-read", `{"ended_on":"2026-11-24"}`, "invalid_json"},
		{"kullanıcı kimliği", "start", `{"user_id":9}`, "invalid_json"},
		{"takvimde olmayan gün", "finish", `{"ended_on":"2026-02-30"}`, "validation_error"},
		{"yerel biçimde gün", "abandon", `{"ended_on":"24.11.2026"}`, "validation_error"},
		{"saatli gün", "start", `{"started_on":"2026-11-24T10:00:00Z"}`, "validation_error"},
		{"bozuk JSON", "start", `{"mode":`, "invalid_json"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeShelf{}
			rec := do(t, shelfRouter(fake), http.MethodPost, "/api/me/books/5/shelf/"+tt.cmd, tt.body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != tt.code || fake.calls != 0 {
				t.Errorf("status %d kod %q çağrı %d; istenen 400 %q (%s)", rec.Code, errorCode(t, rec), fake.calls, tt.code, rec.Body)
			}
		})
	}
	t.Run("bozuk kitap kimliği", func(t *testing.T) {
		fake := &fakeShelf{}
		rec := do(t, shelfRouter(fake), http.MethodPost, "/api/me/books/abc/shelf/start", "")
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_id" || fake.calls != 0 {
			t.Errorf("status %d (%s)", rec.Code, rec.Body)
		}
	})
}

func TestPatchReading(t *testing.T) {
	t.Run("alanlar ve sürüm servise gider", func(t *testing.T) {
		fake := &fakeShelf{shelf: library.Shelf{BookID: 5}}
		rec := do(t, shelfRouter(fake), http.MethodPatch, "/api/me/books/5/shelf/reading",
			`{"version":4,"progress":57,"started_on":"2026-11-01","unit":"percent","edition_id":3}`)
		if rec.Code != http.StatusOK || fake.gotPatch == nil {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body)
		}
		p := fake.gotPatch
		if p.Version != 4 || *p.Progress != 57 || *p.Unit != "percent" || *p.EditionID != 3 || !p.StartedOn.Equal(*testDay(t, "2026-11-01")) {
			t.Errorf("yama %+v", p)
		}
	})
	t.Run("yalnız ilerleme: gelmeyen alan nil", func(t *testing.T) {
		fake := &fakeShelf{shelf: library.Shelf{BookID: 5}}
		do(t, shelfRouter(fake), http.MethodPatch, "/api/me/books/5/shelf/reading", `{"version":1,"progress":0}`)
		if p := fake.gotPatch; p == nil || p.Version != 1 || p.Progress == nil || *p.Progress != 0 || p.Unit != nil || p.EditionID != nil || p.StartedOn != nil {
			t.Errorf("yama %+v", p)
		}
	})
	for _, tt := range []struct{ name, body, code string }{
		{"sürüm yok", `{"progress":5}`, "validation_error"},
		{"bozuk gün", `{"version":1,"started_on":"dün"}`, "validation_error"},
		{"int32'yi aşan ilerleme", `{"version":1,"progress":3000000000}`, "invalid_json"},
		{"bilinmeyen alan", `{"version":1,"status":"READ"}`, "invalid_json"},
		{"gövde yok", ``, "invalid_json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeShelf{}
			rec := do(t, shelfRouter(fake), http.MethodPatch, "/api/me/books/5/shelf/reading", tt.body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != tt.code || fake.calls != 0 {
				t.Errorf("status %d kod %q çağrı %d; istenen 400 %q (%s)", rec.Code, errorCode(t, rec), fake.calls, tt.code, rec.Body)
			}
		})
	}
}

func TestReadingLogEndpoints(t *testing.T) {
	t.Run("gün düzeltme: kayıt kimliği ve gelen günler", func(t *testing.T) {
		fake := &fakeShelf{shelf: library.Shelf{BookID: 5}}
		rec := do(t, shelfRouter(fake), http.MethodPatch, "/api/me/reading-log/12", `{"ended_on":"2026-11-12"}`)
		if rec.Code != http.StatusOK || fake.gotUser != 7 || fake.gotLog != 12 || fake.gotStarted != nil ||
			fake.gotEnded == nil || !fake.gotEnded.Equal(*testDay(t, "2026-11-12")) {
			t.Errorf("status %d user %d log %d günler %v %v (%s)", rec.Code, fake.gotUser, fake.gotLog, fake.gotStarted, fake.gotEnded, rec.Body)
		}
	})
	t.Run("silme 200 ve raf", func(t *testing.T) {
		fake := &fakeShelf{shelf: library.Shelf{BookID: 5}}
		rec := do(t, shelfRouter(fake), http.MethodDelete, "/api/me/reading-log/12", "")
		if rec.Code != http.StatusOK || fake.gotLog != 12 || !strings.Contains(rec.Body.String(), `"book_id":5`) {
			t.Errorf("status %d log %d (%s)", rec.Code, fake.gotLog, rec.Body)
		}
	})
	for _, tt := range []struct{ name, body, code string }{
		{"bilinmeyen alan", `{"outcome":"FINISHED"}`, "invalid_json"},
		{"bozuk gün", `{"started_on":"2026-13-01"}`, "validation_error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeShelf{}
			rec := do(t, shelfRouter(fake), http.MethodPatch, "/api/me/reading-log/12", tt.body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != tt.code || fake.calls != 0 {
				t.Errorf("status %d kod %q çağrı %d (%s)", rec.Code, errorCode(t, rec), fake.calls, rec.Body)
			}
		})
	}
}

// TestShelfErrorMapping: servisin her hatası doğru HTTP koduna ve hata zarfına çevrilir; ham hata
// metni istemciye gitmez.
func TestShelfErrorMapping(t *testing.T) {
	leak := "gizli-ayrinti"
	cases := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{books.ErrNotFound, http.StatusNotFound, "not_found"},
		{library.ErrVersionConflict, http.StatusConflict, "version_conflict"},
		{library.ErrInvalidProgress, http.StatusBadRequest, "invalid_progress"},
		{library.ErrInvalidDates, http.StatusBadRequest, "invalid_dates"},
		{library.ErrInvalidEdition, http.StatusBadRequest, "invalid_edition"},
		{library.ErrInvalidCommand, http.StatusBadRequest, "validation_error"},
		{errors.New(leak), http.StatusInternalServerError, "internal"},
	}
	requests := []struct{ method, url, body string }{
		{http.MethodGet, "/api/me/books/5/shelf", ""},
		{http.MethodPost, "/api/me/books/5/shelf/start", ""},
		{http.MethodPatch, "/api/me/books/5/shelf/reading", `{"version":1}`},
		{http.MethodPatch, "/api/me/reading-log/12", `{}`},
		{http.MethodDelete, "/api/me/reading-log/12", ""},
	}
	for _, c := range cases {
		for _, r := range requests {
			t.Run(r.method+" "+r.url+" "+c.wantCode, func(t *testing.T) {
				// Sarılmış hata da aynı koda düşer (servis bağlam ekleyerek döner).
				err := errors.Join(errors.New(leak), c.err)
				rec := do(t, shelfRouter(&fakeShelf{err: err}), r.method, r.url, r.body)
				if rec.Code != c.wantStatus || errorCode(t, rec) != c.wantCode {
					t.Errorf("status %d kod %q, istenen %d %q (%s)", rec.Code, errorCode(t, rec), c.wantStatus, c.wantCode, rec.Body)
				}
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("ham hata cevaba sızdı: %s", rec.Body)
				}
			})
		}
	}
}

func TestShelfRequiresAuth(t *testing.T) {
	routes := []struct{ method, url string }{
		{http.MethodGet, "/api/me/books/5/shelf"},
		{http.MethodPatch, "/api/me/books/5/shelf/reading"},
		{http.MethodPatch, "/api/me/reading-log/12"},
		{http.MethodDelete, "/api/me/reading-log/12"},
	}
	for _, c := range allShelfCommands {
		routes = append(routes, struct{ method, url string }{http.MethodPost, "/api/me/books/5/shelf/" + string(c)})
	}
	for _, r := range routes {
		fake := &fakeShelf{}
		rec := do(t, newRouter(deps{shelf: fake, tokens: fakeVerifier{}}), r.method, r.url, "")
		if rec.Code != http.StatusUnauthorized || fake.calls != 0 {
			t.Errorf("%s %s: status %d, çağrı %d; istenen 401 ve 0", r.method, r.url, rec.Code, fake.calls)
		}
	}
}

// TestShelfResponseContract, raf cevabının JSON şeklini sabitler: kullanıcı kimliği yok, günler
// "YYYY-AA-GG", okuma yalnız READING'de, kitaplıkta değilken entry null ve log boş dizi.
func TestShelfResponseContract(t *testing.T) {
	at := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	reading := &db.LibraryEntry{
		UserID: 7, BookID: 5, Status: library.StatusReading, CreatedAt: at, UpdatedAt: at, Version: 3,
		EditionID: new(int64(2)), Unit: new("page"), Progress: new(int32(42)), TotalPages: new(int32(310)),
		StartedOn: testDay(t, "2026-11-01"),
	}
	logRow := db.ReadingLog{
		ID: 12, UserID: 7, BookID: 5, Outcome: library.OutcomeFinished, Unit: new("page"),
		Progress: new(int32(310)), TotalPages: new(int32(310)), StartedOn: testDay(t, "2026-10-01"), CreatedAt: at,
	}

	get := func(t *testing.T, s library.Shelf) map[string]json.RawMessage {
		t.Helper()
		rec := do(t, shelfRouter(&fakeShelf{shelf: s}), http.MethodGet, "/api/me/books/5/shelf", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body)
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		checkContract(t, body, []string{"book_id", "entry", "log"})
		return body
	}

	t.Run("okurken", func(t *testing.T) {
		body := get(t, library.Shelf{BookID: 5, Entry: reading, Log: []db.ReadingLog{logRow}})
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(body["entry"], &entry); err != nil {
			t.Fatal(err)
		}
		checkContract(t, entry, []string{"status", "version", "added_at", "updated_at", "reading"})
		var r map[string]json.RawMessage
		if err := json.Unmarshal(entry["reading"], &r); err != nil {
			t.Fatal(err)
		}
		checkContract(t, r, []string{"edition_id", "unit", "progress", "total_pages", "started_on"})
		if string(r["started_on"]) != `"2026-11-01"` || string(entry["version"]) != "3" {
			t.Errorf("gün ya da sürüm: %s %s", r["started_on"], entry["version"])
		}
		var log []map[string]json.RawMessage
		if err := json.Unmarshal(body["log"], &log); err != nil || len(log) != 1 {
			t.Fatalf("log: %v %s", err, body["log"])
		}
		checkContract(t, log[0], []string{"id", "edition_id", "outcome", "unit", "progress", "total_pages", "started_on", "ended_on", "created_at"})
		if string(log[0]["ended_on"]) != "null" || string(log[0]["started_on"]) != `"2026-10-01"` {
			t.Errorf("günlük günleri: %s %s", log[0]["started_on"], log[0]["ended_on"])
		}
	})

	t.Run("okumuyorken reading null", func(t *testing.T) {
		read := *reading
		read.Status = library.StatusRead
		body := get(t, library.Shelf{BookID: 5, Entry: &read})
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(body["entry"], &entry); err != nil {
			t.Fatal(err)
		}
		if string(entry["reading"]) != "null" {
			t.Errorf("READ'de reading null olmalı: %s", entry["reading"])
		}
	})

	t.Run("kitaplıkta değil", func(t *testing.T) {
		body := get(t, library.Shelf{BookID: 5})
		if string(body["entry"]) != "null" || string(body["log"]) != "[]" || string(body["book_id"]) != "5" {
			t.Errorf("gövde %v", body)
		}
	})
}
