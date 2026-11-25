package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/library"
	"github.com/mtgezgin/bookhub/backend/internal/testdb"
)

// idVerifier, "user-<id>" token'ını o kimlikteki USER'a çevirir: uçtan uca testte iki GERÇEK kullanıcı
// (fakeVerifier'ın sabit 7'si veritabanında yok).
type idVerifier struct{}

func (idVerifier) Verify(raw string) (auth.Principal, error) {
	id, err := strconv.ParseInt(strings.TrimPrefix(raw, "user-"), 10, 64)
	if err != nil || !strings.HasPrefix(raw, "user-") {
		return auth.Principal{}, auth.ErrInvalidToken
	}
	return auth.Principal{UserID: id, Role: auth.RoleUser}, nil
}

// TestShelfEndpointsRealDB, raf uçlarını UÇTAN UCA doğrular: router → handler → gerçek servis →
// Postgres. Birim testleri sahte servisle koşar, servis testleri HTTP'yi görmez; servisin hatalarının
// (eski sürüm, başkasının kaydı) doğru koda ve zarfa çevrildiğini yalnız bu dikey kesit kanıtlar
// (spec Doğrulama 3'ün API yarısı, 9).
func TestShelfEndpointsRealDB(t *testing.T) {
	pool := testdb.Pool(t)
	router := newRouter(deps{
		books: books.NewRepository(pool), library: library.NewRepository(pool),
		shelf: library.NewService(pool), tokens: idVerifier{},
	})
	alice, bob := testdb.CreateUser(t, pool, "Okur"), testdb.CreateUser(t, pool, "Okur")
	book := testdb.CreateBook(t, pool, "", books.StatusApproved)
	bookURL := "/api/me/books/" + strconv.FormatInt(book, 10)

	call := func(t *testing.T, user int64, method, url, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer user-"+strconv.FormatInt(user, 10))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	shelf := func(t *testing.T, rec *httptest.ResponseRecorder) shelfResponse {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d (%s)", rec.Code, rec.Body)
		}
		var s shelfResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	started := shelf(t, call(t, alice, http.MethodPost, bookURL+"/shelf/start", `{"started_on":"2026-11-01"}`))
	if started.Entry == nil || started.Entry.Status != library.StatusReading || started.Entry.Reading == nil ||
		*started.Entry.Reading.StartedOn != "2026-11-01" {
		t.Fatalf("başla: %+v", started.Entry)
	}
	v := started.Entry.Version

	t.Run("iki sekme: eski sürüm 409, satır değişmez", func(t *testing.T) {
		patch := `{"version":` + strconv.Itoa(int(v)) + `,"progress":10}`
		first := shelf(t, call(t, alice, http.MethodPatch, bookURL+"/shelf/reading", patch))
		if first.Entry.Version != v+1 || *first.Entry.Reading.Progress != 10 {
			t.Fatalf("ilk sekme: %+v", first.Entry)
		}
		rec := call(t, alice, http.MethodPatch, bookURL+"/shelf/reading", strings.Replace(patch, "10", "57", 1))
		if rec.Code != http.StatusConflict || errorCode(t, rec) != "version_conflict" {
			t.Fatalf("ikinci sekme: %d %s", rec.Code, rec.Body)
		}
		now := shelf(t, call(t, alice, http.MethodGet, bookURL+"/shelf", ""))
		if *now.Entry.Reading.Progress != 10 || now.Entry.Version != v+1 {
			t.Errorf("409 satırı değiştirmemeli: %+v", now.Entry.Reading)
		}
	})

	t.Run("eski PUT READ okumayı tarihsiz kapatır", func(t *testing.T) {
		rec := call(t, alice, http.MethodPut, "/api/me/library/"+strconv.FormatInt(book, 10), `{"status":"READ"}`)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"READ"`) {
			t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
		}
		now := shelf(t, call(t, alice, http.MethodGet, bookURL+"/shelf", ""))
		if now.Entry.Status != library.StatusRead || now.Entry.Reading != nil || len(now.Log) != 1 {
			t.Fatalf("raf: %+v, günlük %d", now.Entry, len(now.Log))
		}
		if l := now.Log[0]; l.Outcome != library.OutcomeFinished || l.EndedOn != nil || *l.Progress != 10 || *l.StartedOn != "2026-11-01" {
			t.Errorf("günlük satırı: %+v", l)
		}
	})

	logURL := func(t *testing.T) string {
		t.Helper()
		now := shelf(t, call(t, alice, http.MethodGet, bookURL+"/shelf", ""))
		if len(now.Log) == 0 {
			t.Fatal("günlük boş")
		}
		return "/api/me/reading-log/" + strconv.FormatInt(now.Log[0].ID, 10)
	}

	t.Run("başkasının günlük kaydı her uçta 404", func(t *testing.T) {
		url := logURL(t)
		for _, m := range []struct{ method, body string }{{http.MethodPatch, `{"ended_on":"2026-11-12"}`}, {http.MethodDelete, ""}} {
			rec := call(t, bob, m.method, url, m.body)
			if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
				t.Errorf("bob %s: %d %s", m.method, rec.Code, rec.Body)
			}
		}
		if bobShelf := shelf(t, call(t, bob, http.MethodGet, bookURL+"/shelf", "")); bobShelf.Entry != nil || len(bobShelf.Log) != 0 {
			t.Errorf("bob alice'in rafını görmemeli: %+v", bobShelf)
		}
		if now := shelf(t, call(t, alice, http.MethodGet, bookURL+"/shelf", "")); len(now.Log) != 1 || now.Log[0].EndedOn != nil {
			t.Errorf("alice'in kaydı değişmemeli: %+v", now.Log)
		}
	})

	t.Run("tarih ekle; bitiş başlangıçtan önce 400", func(t *testing.T) {
		url := logURL(t)
		got := shelf(t, call(t, alice, http.MethodPatch, url, `{"ended_on":"2026-11-12"}`))
		if *got.Log[0].EndedOn != "2026-11-12" {
			t.Errorf("bitiş günü: %+v", got.Log[0])
		}
		rec := call(t, alice, http.MethodPatch, url, `{"started_on":"2026-11-20"}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_dates" {
			t.Errorf("ters günler: %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("DELETE kitaplıktan çıkarır, geçmiş kalır", func(t *testing.T) {
		rec := call(t, alice, http.MethodDelete, "/api/me/library/"+strconv.FormatInt(book, 10), "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("DELETE: %d %s", rec.Code, rec.Body)
		}
		now := shelf(t, call(t, alice, http.MethodGet, bookURL+"/shelf", ""))
		if now.Entry != nil || len(now.Log) != 1 {
			t.Errorf("çıkarınca entry null, geçmiş 1 olmalı: %+v", now)
		}
		again := shelf(t, call(t, alice, http.MethodPost, bookURL+"/shelf/remove", ""))
		if again.Entry != nil || len(again.Log) != 1 {
			t.Errorf("ikinci çıkarma zararsız olmalı: %+v", again)
		}
	})

	t.Run("geçmişten silmek 200, ikinci silme 404", func(t *testing.T) {
		url := logURL(t)
		if got := shelf(t, call(t, alice, http.MethodDelete, url, "")); len(got.Log) != 0 || got.BookID != book {
			t.Errorf("silme sonrası raf: %+v", got)
		}
		if rec := call(t, alice, http.MethodDelete, url, ""); rec.Code != http.StatusNotFound {
			t.Errorf("ikinci silme: %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("yayında olmayan kitap 404, geçersiz ilerleme 400", func(t *testing.T) {
		pending := testdb.CreateBook(t, pool, "", books.StatusPending)
		rec := call(t, alice, http.MethodPost, "/api/me/books/"+strconv.FormatInt(pending, 10)+"/shelf/want", "")
		if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
			t.Errorf("bekleyen kitap: %d %s", rec.Code, rec.Body)
		}
		s := shelf(t, call(t, alice, http.MethodPost, bookURL+"/shelf/start", ""))
		rec = call(t, alice, http.MethodPatch, bookURL+"/shelf/reading", `{"version":`+strconv.Itoa(int(s.Entry.Version))+`,"progress":-1}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_progress" {
			t.Errorf("negatif ilerleme: %d %s", rec.Code, rec.Body)
		}
	})
}
