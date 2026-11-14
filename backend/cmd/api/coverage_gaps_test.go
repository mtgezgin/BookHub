package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/ratings"
)

// Kapsam kapısının adını koyduğu, henüz hiç çalışmamış hata dalları.

func TestRatingErrorBranches(t *testing.T) {
	t.Run("puanı olmayan kullanıcı silmeye çalışır → 404 not_rated", func(t *testing.T) {
		rec := do(t, userRouter(deps{ratings: &fakeRatings{err: ratings.ErrNotRated}}), http.MethodDelete, "/api/books/5/rating", "")
		if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_rated" {
			t.Errorf("404 not_rated bekleniyordu: %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("onaysız/olmayan kitabın özeti → 404 not_found", func(t *testing.T) {
		rec := do(t, newRouter(deps{ratings: &fakeRatings{err: books.ErrNotFound}}), http.MethodGet, "/api/books/5/ratings/summary", "")
		if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
			t.Errorf("404 not_found bekleniyordu: %d %s", rec.Code, rec.Body)
		}
	})
}

// TestAdminPatchRejectsInvalidFields: PATCH'te geçersiz alan 400 olur ve servise GİTMEZ.
func TestAdminPatchRejectsInvalidFields(t *testing.T) {
	for _, body := range []string{
		`{"author":"   "}`,
		`{"author":"` + strings.Repeat("y", maxAuthorLen+1) + `"}`,
		`{"description":"` + strings.Repeat("d", maxDescriptionLen+1) + `"}`,
		`{"language":"türkçe"}`,
		`{"language":""}`,
	} {
		fake := &fakeModeration{book: db.BookView{ID: 5}}
		rec := do(t, adminRouter(deps{moderation: fake, books: fakeBooks{}}), http.MethodPatch, "/api/admin/books/5", body)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_error" || len(fake.calls) != 0 {
			t.Errorf("%.40s…: 400 validation_error ve servis çağrısı YOK bekleniyordu: %d %v", body, rec.Code, fake.calls)
		}
	}
}

func TestRejectMalformedBody(t *testing.T) {
	fake := &fakeModeration{book: db.BookView{ID: 5}}
	rec := do(t, adminRouter(deps{moderation: fake, books: fakeBooks{}}), http.MethodPost, "/api/admin/books/5/reject", `{"reason":`)
	if rec.Code != http.StatusBadRequest || len(fake.calls) != 0 {
		t.Errorf("bozuk gövde 400 olmalı ve Reject çağrılmamalı: %d %v", rec.Code, fake.calls)
	}
}

// TestWriteJSONEncodeFailureLogsRequestID: kodlanamayan bir değer (Inf) loga, cevabın
// X-Request-Id'siyle düşmeli — hangi isteğin cevabının yarım kaldığı bulunabilsin.
func TestWriteJSONEncodeFailureLogsRequestID(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	rec := httptest.NewRecorder()
	rec.Header().Set(requestIDHeader, "istek-42")
	writeJSON(rec, http.StatusOK, map[string]float64{"x": math.Inf(1)})

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("uyarı satırı bekleniyordu: %q (%v)", buf.String(), err)
	}
	if line["msg"] != "cevap JSON yazılamadı" || line["request_id"] != "istek-42" || line["level"] != "WARN" {
		t.Errorf("uyarı request_id'yle yazılmalı: %v", line)
	}
}
