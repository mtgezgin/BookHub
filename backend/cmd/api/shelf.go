package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/library"
)

// shelfService, raf uçlarının ihtiyacı kadar (tüketici tarafı interface); *library.Service sağlar.
// Bütün yazımlar buradan geçer: komutlar, okurken düzeltme, geçmiş. Eski PUT/DELETE de (library.go).
type shelfService interface {
	Get(ctx context.Context, userID, bookID int64) (library.Shelf, error)
	Do(ctx context.Context, userID, bookID int64, in library.CommandInput) (library.Shelf, error)
	UpdateReading(ctx context.Context, userID, bookID int64, p library.ReadingPatch) (library.Shelf, error)
	UpdateLogDates(ctx context.Context, userID, logID int64, startedOn, endedOn *time.Time) (library.Shelf, error)
	DeleteLog(ctx context.Context, userID, logID int64) (library.Shelf, error)
}

// shelfResponse, bir kitabın kullanıcının rafındaki hâli. Her raf ucu bunu döner: istemci komuttan
// sonra ayrıca GET yapmaz, cevabı önbelleğe doğrudan yazar (spec § Kararlar "Veri katmanı").
type shelfResponse struct {
	BookID int64                `json:"book_id"`
	Entry  *shelfEntryResponse  `json:"entry"` // null = kitaplıkta değil
	Log    []readingLogResponse `json:"log"`   // kapanmış okumalar, en yeni önce; kitaplıkta değilken de dolu olabilir
}

type shelfEntryResponse struct {
	Status string `json:"status"`
	// Version, okurken düzeltmede (PATCH …/shelf/reading) geri gönderilir; eskiyse 409.
	Version   int32            `json:"version"`
	AddedAt   time.Time        `json:"added_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Reading   *readingResponse `json:"reading"` // null = okumuyor (READING dışı her durum)
}

// readingResponse, süren okuma. Unit ve Progress null ise ilerleme henüz girilmemiş (Faz 15'ten
// kalan "Şu an okuyorum"); TotalPages null ise toplam bilinmiyor ya da birim yüzde.
type readingResponse struct {
	EditionID  *int64  `json:"edition_id"`
	Unit       *string `json:"unit"`
	Progress   *int32  `json:"progress"`
	TotalPages *int32  `json:"total_pages"`
	StartedOn  *string `json:"started_on"` // "2026-11-24"; null = bilinmiyor
}

type readingLogResponse struct {
	ID         int64     `json:"id"`
	EditionID  *int64    `json:"edition_id"`
	Outcome    string    `json:"outcome"` // FINISHED | ABANDONED
	Unit       *string   `json:"unit"`
	Progress   *int32    `json:"progress"`
	TotalPages *int32    `json:"total_pages"`
	StartedOn  *string   `json:"started_on"`
	EndedOn    *string   `json:"ended_on"` // null = "Zaten okudum", tarih bilinmiyor
	CreatedAt  time.Time `json:"created_at"`
}

// day, takvim günü (date kolonu) ↔ "YYYY-AA-GG". Saat ve saat dilimi taşımaz: "bugün"ü istemci kendi
// yerel gününden söyler (spec kavram 6), sunucu onu olduğu gibi saklar.
func day(t *time.Time) *string {
	if t == nil {
		return nil
	}
	return new(t.Format(time.DateOnly))
}

func toShelfResponse(s library.Shelf) shelfResponse {
	out := shelfResponse{BookID: s.BookID, Log: make([]readingLogResponse, 0, len(s.Log))}
	if e := s.Entry; e != nil {
		out.Entry = &shelfEntryResponse{Status: e.Status, Version: e.Version, AddedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
		if e.Status == library.StatusReading {
			out.Entry.Reading = &readingResponse{
				EditionID: e.EditionID, Unit: e.Unit, Progress: e.Progress, TotalPages: e.TotalPages, StartedOn: day(e.StartedOn),
			}
		}
	}
	for _, l := range s.Log {
		out.Log = append(out.Log, toReadingLogResponse(l))
	}
	return out
}

func toReadingLogResponse(l db.ReadingLog) readingLogResponse {
	return readingLogResponse{
		ID: l.ID, EditionID: l.EditionID, Outcome: l.Outcome, Unit: l.Unit, Progress: l.Progress,
		TotalPages: l.TotalPages, StartedOn: day(l.StartedOn), EndedOn: day(l.EndedOn), CreatedAt: l.CreatedAt,
	}
}

// Gövdeler: her biri ayrı DTO; readJSON bilinmeyen alanı 400 yapar (mass assignment yok). Hiçbirinde
// kullanıcı ya da kitap kimliği yok: kullanıcı token'dan, kitap adresten.
type startShelfRequest struct {
	Mode      string  `json:"mode"` // "" | resume | restart
	EditionID *int64  `json:"edition_id"`
	Unit      *string `json:"unit"`
	StartedOn *string `json:"started_on"`
}

type endShelfRequest struct {
	EndedOn *string `json:"ended_on"`
}

type readingPatchRequest struct {
	Version   *int32  `json:"version"` // zorunlu: GET'te ya da son komutta dönen sürüm
	Progress  *int32  `json:"progress"`
	StartedOn *string `json:"started_on"`
	Unit      *string `json:"unit"`
	EditionID *int64  `json:"edition_id"`
}

type logDatesRequest struct {
	StartedOn *string `json:"started_on"`
	EndedOn   *string `json:"ended_on"`
}

// parseDay, isteğe bağlı bir gün alanını okur; biçim hatası validator'a birikir. nil = alan gelmedi.
func parseDay(v *validator, field string, s *string) *time.Time {
	if s == nil {
		return nil
	}
	d, err := time.Parse(time.DateOnly, *s)
	if err != nil {
		v.add("%s YYYY-AA-GG biçiminde gerçek bir gün olmalı", field)
		return nil
	}
	return &d
}

// writeShelfError, raf uçlarının hata haritası. notFound, o ucun "bulunamadı" cümlesi: kitap, okuma
// ya da geçmiş kaydı. Başkasının kaydı da aynı 404'tür (IDOR: var olduğu bile söylenmez).
func writeShelfError(w http.ResponseWriter, op, notFound string, err error) {
	switch {
	case errors.Is(err, books.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", notFound)
	case errors.Is(err, library.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict", "okuman başka bir yerde güncellenmiş; güncel hâlini görüp yeniden dene")
	case errors.Is(err, library.ErrInvalidProgress):
		writeError(w, http.StatusBadRequest, "invalid_progress",
			"ilerleme geçersiz: sayfa 0'dan küçük ya da toplamdan büyük olamaz, yüzde 0–100 arası; birim page ya da percent")
	case errors.Is(err, library.ErrInvalidDates):
		writeError(w, http.StatusBadRequest, "invalid_dates", "bitiş günü başlangıç gününden önce olamaz")
	case errors.Is(err, library.ErrInvalidEdition):
		writeError(w, http.StatusBadRequest, "invalid_edition", "seçilen baskı bu kitaba ait değil")
	case errors.Is(err, library.ErrInvalidCommand):
		writeError(w, http.StatusBadRequest, "validation_error", "bilinmeyen komut ya da başlama biçimi")
	default:
		writeInternalError(w, op, err)
	}
}

const errBookNotFound = "kitap bulunamadı"

// handleGetShelf, GET /api/me/books/{id}/shelf — durum, süren okuma, sürüm ve geçmiş. Kitaplıkta
// olmayan kitap 200 ve entry null (geçmişi olabilir); yayında olmayan kitap 404.
func handleGetShelf(svc shelfService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		s, err := svc.Get(r.Context(), user, id)
		if err != nil {
			writeShelfError(w, "GetShelf", errBookNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, toShelfResponse(s))
	}
}

// handleShelfCommand, POST /api/me/books/{id}/shelf/{komut} — bütün komut uçları bu tek üreticiden
// (Faz 4'teki handleTransition gibi; Sonar tekrar kapısı). Komut adresten değil, rota kurulurken
// sabit gelir: "shelf/{cmd}" gibi bir parametre yazım hatasını derleme yerine çalışma zamanına
// bırakırdı. Komut her durumda tanımlı (spec kavram 2): niyet için 409 yok, çift tık zararsız.
func handleShelfCommand(svc shelfService, c library.Command) http.HandlerFunc {
	op := "ShelfCommand:" + string(c)
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		in, err := readCommandInput(w, r, c)
		if err != nil {
			writeRequestError(w, err)
			return
		}
		s, err := svc.Do(r.Context(), user, id, in)
		if err != nil {
			writeShelfError(w, op, errBookNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, toShelfResponse(s))
	}
}

// readCommandInput, komutun gövdesini okur. Gövde her komutta isteğe bağlı (tek tık: varsayılanlar);
// varsa yalnız o komutun alanlarını kabul eder: "bitir"e mode, "istiyorum"a herhangi bir alan 400.
func readCommandInput(w http.ResponseWriter, r *http.Request, c library.Command) (library.CommandInput, error) {
	in := library.CommandInput{Command: c}
	if r.ContentLength == 0 {
		return in, nil
	}
	var v validator
	switch c {
	case library.CmdStart:
		var req startShelfRequest
		if err := readJSON(w, r, &req); err != nil {
			return in, err
		}
		mode := library.StartMode(req.Mode)
		v.check(mode == library.ModeAuto || mode == library.ModeResume || mode == library.ModeRestart,
			"mode yalnız resume ya da restart olabilir")
		in.Mode, in.EditionID, in.Unit = mode, req.EditionID, req.Unit
		in.StartedOn = parseDay(&v, "started_on", req.StartedOn)
	case library.CmdFinish, library.CmdAbandon:
		var req endShelfRequest
		if err := readJSON(w, r, &req); err != nil {
			return in, err
		}
		in.EndedOn = parseDay(&v, "ended_on", req.EndedOn)
	default:
		if err := readJSON(w, r, &struct{}{}); err != nil {
			return in, err
		}
	}
	return in, v.err()
}

// handlePatchReading, PATCH /api/me/books/{id}/shelf/reading — okurken ilerleme, başlangıç günü,
// birim ya da baskı. version zorunlu: eski sürüm 409 (iki sekme), okunmayan kitap 404.
func handlePatchReading(svc shelfService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req readingPatchRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		var v validator
		v.check(req.Version != nil, "version zorunlu")
		started := parseDay(&v, "started_on", req.StartedOn)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}
		s, err := svc.UpdateReading(r.Context(), user, id, library.ReadingPatch{
			Version: *req.Version, Progress: req.Progress, Unit: req.Unit, EditionID: req.EditionID, StartedOn: started,
		})
		if err != nil {
			writeShelfError(w, "UpdateReading", "bu kitabı şu an okumuyorsun", err)
			return
		}
		writeJSON(w, http.StatusOK, toShelfResponse(s))
	}
}

const errLogNotFound = "okuma kaydı bulunamadı"

// handlePatchReadingLog, PATCH /api/me/reading-log/{id} — kapanmış bir okumanın günlerini düzeltir
// ("Okudun · tarih ekle" dahil). Gelmeyen alan değişmez. Kaydın kitabın rafını döner.
func handlePatchReadingLog(svc shelfService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req logDatesRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		var v validator
		started, ended := parseDay(&v, "started_on", req.StartedOn), parseDay(&v, "ended_on", req.EndedOn)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}
		s, err := svc.UpdateLogDates(r.Context(), user, id, started, ended)
		if err != nil {
			writeShelfError(w, "UpdateLogDates", errLogNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, toShelfResponse(s))
	}
}

// handleDeleteReadingLog, DELETE /api/me/reading-log/{id} — kaydı geçmişten siler ve kaydın kitabının
// rafını döner (200, 204 değil: istemci rafı önbelleğe yazar). İkinci silme 404.
func handleDeleteReadingLog(svc shelfService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		s, err := svc.DeleteLog(r.Context(), user, id)
		if err != nil {
			writeShelfError(w, "DeleteLog", errLogNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, toShelfResponse(s))
	}
}
