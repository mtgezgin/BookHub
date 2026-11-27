package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/reviews"
	"github.com/mtgezgin/bookhub/backend/internal/richtext"
)

// reviewService, inceleme uçlarının ihtiyacı kadar (tüketici tarafı interface); *reviews.Service sağlar.
type reviewService interface {
	List(ctx context.Context, bookID int64, limit, offset int32) ([]db.ListBookReviewsRow, int64, error)
	Mine(ctx context.Context, userID, bookID int64) (db.Review, error)
	Save(ctx context.Context, userID, bookID int64, d reviews.Draft) (db.Review, error)
	Delete(ctx context.Context, userID, bookID int64) error
	Hide(ctx context.Context, adminID, reviewID int64, reason string) (db.Review, error)
	Unhide(ctx context.Context, adminID, reviewID int64) (db.Review, error)
}

// publicReviewResponse, herkese açık listedeki inceleme. E-posta yok (kişisel veri); sürüm ve gizleme
// alanları yok (yalnız sahibini ve yöneticiyi ilgilendirir). Spoiler metni de gelir: gizlemek arayüzün
// işi (spec § Kararlar "Spoiler").
//
// Doc (Faz 17C), incelemenin belgesi; her zaman dolu (belgesi olmayan eski inceleme düz metninden
// çevrilir). Body ve HasSpoiler kalır: belgeden türetilmiş hâlleridir, yayın anında açık duran eski
// arayüz onları okur.
type publicReviewResponse struct {
	ID          int64           `json:"id"`
	Author      commentAuthor   `json:"author"`       // hesap silinince inceleme de silinir: yazar hep var
	AuthorScore *int16          `json:"author_score"` // yazarın bu kitaba puanı; null = puan vermemiş
	Body        string          `json:"body"`
	HasSpoiler  bool            `json:"has_spoiler"`
	Doc         json.RawMessage `json:"doc"`
	CreatedAt   time.Time       `json:"created_at"`
	EditedAt    *time.Time      `json:"edited_at"` // null = hiç düzenlenmedi
}

type reviewPage struct {
	Items  []publicReviewResponse `json:"items"`
	Total  int64                  `json:"total"`
	Limit  int32                  `json:"limit"`
	Offset int32                  `json:"offset"`
}

// reviewResponse, sahibinin (ve gizleyip açan yöneticinin) gördüğü inceleme. Version düzenlemede geri
// gönderilir. Hidden null = görünür; doluysa gerekçesiyle. Gizleyen yöneticinin kimliği BİLEREK yok.
type reviewResponse struct {
	ID         int64           `json:"id"`
	BookID     int64           `json:"book_id"`
	Body       string          `json:"body"`
	HasSpoiler bool            `json:"has_spoiler"`
	Doc        json.RawMessage `json:"doc"`
	Version    int32           `json:"version"`
	CreatedAt  time.Time       `json:"created_at"`
	EditedAt   *time.Time      `json:"edited_at"`
	Hidden     *reviewHiddenAt `json:"hidden"`
}

type reviewHiddenAt struct {
	At     time.Time `json:"at"`
	Reason string    `json:"reason"`
}

func toReviewResponse(r db.Review) reviewResponse {
	out := reviewResponse{
		ID: r.ID, BookID: r.BookID, Body: r.Body, HasSpoiler: r.HasSpoiler,
		Doc:     reviews.DocJSON(r.Body, r.HasSpoiler, r.BodyDoc),
		Version: r.Version, CreatedAt: r.CreatedAt, EditedAt: r.EditedAt,
	}
	if r.HiddenAt != nil {
		out.Hidden = &reviewHiddenAt{At: *r.HiddenAt, Reason: r.HiddenReason}
	}
	return out
}

// saveReviewRequest: kullanıcı ve kitap kimliği yok (token'dan ve adresten). Version: ilk yazımda
// gönderilmez, düzenlemede GET'te ya da son yazımda dönen sürüm.
//
// İki gövde kabul edilir (spec karar 7): yeni arayüz Doc gönderir; yayın anında açık duran eski arayüz
// Body ve HasSpoiler gönderir. İkisi birlikte gelirse 400: hangisinin geçerli olduğu belirsiz olurdu.
// Eski gövde desteği daraltma PR'ında kalkar.
type saveReviewRequest struct {
	Doc        json.RawMessage `json:"doc"`
	Body       string          `json:"body"`
	HasSpoiler bool            `json:"has_spoiler"`
	Version    *int32          `json:"version"`
}

// document, isteğin belgesini doğrulayıp döndürür. Doc verilmişse yalnız o okunur; verilmemişse eski
// gövde bugünkü kurallarıyla (kırpma, zorunlu, 5000 karakter) denetlenir ve belgeye çevrilir. İki
// yolda da son söz richtext.Parse'ındır: servise doğrulanmamış belge gitmez.
func (req saveReviewRequest) document() (richtext.Doc, error) {
	var v validator
	v.check(req.Version == nil || *req.Version >= 1, "version 1 ya da daha büyük olmalı")
	raw := req.Doc
	if len(raw) == 0 || string(raw) == "null" {
		body := strings.TrimSpace(req.Body)
		v.required("body", body)
		v.maxLen("body", body, reviews.MaxBodyLen)
		if err := v.err(); err != nil {
			return richtext.Doc{}, err
		}
		raw = richtext.FromPlain(body, req.HasSpoiler).JSON()
	} else {
		v.check(req.Body == "" && !req.HasSpoiler, "doc ile birlikte body ya da has_spoiler gönderilemez")
		if err := v.err(); err != nil {
			return richtext.Doc{}, err
		}
	}
	doc, err := richtext.Parse(raw)
	switch {
	case errors.Is(err, richtext.ErrEmpty):
		return richtext.Doc{}, badRequest("validation_error", "inceleme metni boş olamaz")
	case errors.Is(err, richtext.ErrInvalid):
		// Mesaj istemcinin gönderdiği değerleri taşımaz; yalnız sorunun yeri ve türü (richtext).
		return richtext.Doc{}, badRequest("validation_error", err.Error())
	case err != nil:
		return richtext.Doc{}, err
	}
	return doc, nil
}

type hideReviewRequest struct {
	Reason string `json:"reason"`
}

// writeReviewError, okurun inceleme uçlarının hata haritası. "İncelemen yok" ayrı bir kod: istemci onu
// "kitap yok"tan ayırıp boş düzenleyici gösterir (puandaki not_rated gibi).
func writeReviewError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, books.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "kitap bulunamadı")
	case errors.Is(err, reviews.ErrNotFound):
		writeError(w, http.StatusNotFound, "no_review", "bu kitaba henüz inceleme yazmadın")
	case errors.Is(err, reviews.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict", "incelemen başka bir yerde değişmiş; güncel hâlini görüp yeniden dene")
	default:
		writeInternalError(w, op, err)
	}
}

// handleListReviews, GET /api/books/{id}/reviews — herkese açık, sayfalı, en yeni önce; gizlenenler yok.
func handleListReviews(svc reviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var v validator
		limit, offset := parsePage(r.URL.Query(), &v)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}
		rows, total, err := svc.List(r.Context(), id, limit, offset)
		if err != nil {
			writeReviewError(w, "ListReviews", err)
			return
		}
		items := make([]publicReviewResponse, 0, len(rows))
		for _, row := range rows {
			items = append(items, publicReviewResponse{
				ID: row.ID, Author: commentAuthor{ID: row.UserID, DisplayName: row.AuthorName}, AuthorScore: row.AuthorScore,
				Body: row.Body, HasSpoiler: row.HasSpoiler, Doc: reviews.DocJSON(row.Body, row.HasSpoiler, row.BodyDoc),
				CreatedAt: row.CreatedAt, EditedAt: row.EditedAt,
			})
		}
		writeJSON(w, http.StatusOK, reviewPage{Items: items, Total: total, Limit: limit, Offset: offset})
	}
}

// handleMyReview, GET /api/books/{id}/review — çağıranın incelemesi (gizlenmişse gerekçesiyle);
// yoksa 404 no_review.
func handleMyReview(svc reviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, id, ok := actorAndID(w, r)
		if !ok {
			return
		}
		rev, err := svc.Mine(r.Context(), user, id)
		if err != nil {
			writeReviewError(w, "MyReview", err)
			return
		}
		writeJSON(w, http.StatusOK, toReviewResponse(rev))
	}
}

// handleSaveReview, PUT /api/books/{id}/review — yaz ya da düzenle. Yeni inceleme 201, düzenleme 200.
// Sürümsüz bir istek ancak ilk yazımsa başarılı olur (servis, inceleme varken sürümsüz yazımı 409
// yapar); bu yüzden durum kodu istekten bilinir.
func handleSaveReview(svc reviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, id, ok := actorAndID(w, r)
		if !ok {
			return
		}
		var req saveReviewRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		doc, err := req.document()
		if err != nil {
			writeRequestError(w, err)
			return
		}

		rev, err := svc.Save(r.Context(), user, id, reviews.Draft{Doc: doc, Version: req.Version})
		if err != nil {
			writeReviewError(w, "SaveReview", err)
			return
		}
		status := http.StatusOK
		if req.Version == nil {
			status = http.StatusCreated
		}
		writeJSON(w, status, toReviewResponse(rev))
	}
}

// handleDeleteReview, DELETE /api/books/{id}/review → 204. Puan silinmez.
func handleDeleteReview(svc reviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, id, ok := actorAndID(w, r)
		if !ok {
			return
		}
		if err := svc.Delete(r.Context(), user, id); err != nil {
			writeReviewError(w, "DeleteReview", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleHideReview, POST /api/admin/reviews/{id}/hide {"reason"} — gerekçe zorunlu (sahibine gösterilir).
func handleHideReview(svc reviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, id, ok := actorAndID(w, r)
		if !ok {
			return
		}
		var req hideReviewRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		req.Reason = strings.TrimSpace(req.Reason)
		var v validator
		v.required("reason", req.Reason)
		v.maxLen("reason", req.Reason, reviews.MaxReasonLen)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}
		rev, err := svc.Hide(r.Context(), admin, id, req.Reason)
		writeModeratedReview(w, "HideReview", rev, err)
	}
}

// handleUnhideReview, POST /api/admin/reviews/{id}/unhide — gövdesiz.
func handleUnhideReview(svc reviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, id, ok := actorAndID(w, r)
		if !ok {
			return
		}
		rev, err := svc.Unhide(r.Context(), admin, id)
		writeModeratedReview(w, "UnhideReview", rev, err)
	}
}

// writeModeratedReview, yönetici uçlarının cevabı. Burada {id} incelemenin kendi numarası; olmayan
// inceleme düz not_found.
func writeModeratedReview(w http.ResponseWriter, op string, rev db.Review, err error) {
	switch {
	case errors.Is(err, reviews.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "inceleme bulunamadı")
	case err != nil:
		writeInternalError(w, op, err)
	default:
		writeJSON(w, http.StatusOK, toReviewResponse(rev))
	}
}
