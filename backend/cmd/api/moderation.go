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
	"github.com/mtgezgin/bookhub/backend/internal/moderation"
)

// moderationService, kitap yaşam döngüsü uçlarının servisten beklediği davranışlar.
// *moderation.Service bunu sağlar.
type moderationService interface {
	Suggest(ctx context.Context, userID int64, p books.CreateParams) (db.BookView, error)
	CreateApproved(ctx context.Context, actorID int64, p books.CreateParams) (db.BookView, error)
	Update(ctx context.Context, actorID, bookID int64, p books.UpdateParams) (db.BookView, error)
	Approve(ctx context.Context, actorID, bookID int64) (db.BookView, error)
	Reject(ctx context.Context, actorID, bookID int64, reason string) (db.BookView, error)
	Archive(ctx context.Context, actorID, bookID int64) (db.BookView, error)
	Restore(ctx context.Context, actorID, bookID int64) (db.BookView, error)
	Get(ctx context.Context, bookID int64) (db.BookView, error)
	List(ctx context.Context, status string, limit, offset int32) ([]db.BookView, int64, error)
	History(ctx context.Context, bookID int64) ([]db.AuditLog, error)
}

// adminBookResponse, kitabın ADMIN temsili: public DTO + moderasyon alanları.
//
// İki ayrı DTO'nun karşılığı burada görülüyor: Faz 4 books tablosuna 4 kolon ekledi ve
// bunların HİÇBİRİ public cevaba sızmadı — çünkü public bookResponse alanları elle
// seçiyor. Faz 2'de db.Book'u (bugün db.BookView) doğrudan serileştirseydik, rejection_reason ("spam
// hesabı") gibi iç notlar herkese açık API'de yayınlanırdı.
//
// Gömülü (embedded) struct: bookResponse'un alanları JSON'da AYNI seviyeye açılır.
type adminBookResponse struct {
	bookResponse
	Status          string     `json:"status"`
	SubmittedBy     *int64     `json:"submitted_by"`
	RejectionReason *string    `json:"rejection_reason"`
	ModeratedBy     *int64     `json:"moderated_by"`
	ModeratedAt     *time.Time `json:"moderated_at"`
}

func toAdminBookResponse(b db.BookView, genreList []db.Genre) adminBookResponse {
	return adminBookResponse{
		bookResponse:    toBookResponse(b, genreList),
		Status:          b.Status,
		SubmittedBy:     b.SubmittedBy,
		RejectionReason: b.RejectionReason,
		ModeratedBy:     b.ModeratedBy,
		ModeratedAt:     b.ModeratedAt,
	}
}

// suggestionResponse, öneren kullanıcıya dönen cevap: kitabın public hâli + statüsü
// (kullanıcı önerisinin beklemede olduğunu görsün). Moderasyon iç alanları yok.
type suggestionResponse struct {
	bookResponse
	Status string `json:"status"`
}

type adminBookPage struct {
	Items  []adminBookResponse `json:"items"`
	Total  int64               `json:"total"`
	Limit  int32               `json:"limit"`
	Offset int32               `json:"offset"`
}

type auditEntryResponse struct {
	ID        int64           `json:"id"`
	ActorID   *int64          `json:"actor_id"`
	Action    string          `json:"action"`
	Details   json.RawMessage `json:"details"`
	CreatedAt time.Time       `json:"created_at"`
}

// actorFrom, işlemi yapanın kimliğini okur. Rota requireAuth arkasında değilse (yanlış
// kurulum) 401 yazar — kimliksiz bir yazım asla "aktörsüz" denetim kaydına dönüşmesin.
func actorFrom(w http.ResponseWriter, r *http.Request) (int64, bool) {
	p, ok := principalFrom(r.Context())
	if !ok {
		unauthorized(w, "missing_token", "kimlik doğrulaması gerekli")
		return 0, false
	}
	return p.UserID, true
}

// writeBookWriteError, kitap yazma/moderasyon hatalarını HTTP'ye çevirir (tek yer).
func writeBookWriteError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, books.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "kitap bulunamadı")
	case errors.Is(err, books.ErrDuplicateSlug):
		writeError(w, http.StatusConflict, "slug_conflict", "bu slug zaten kullanılıyor")
	case errors.Is(err, moderation.ErrInvalidTransition):
		// 409: istek biçimce doğru ama kaynağın ŞU ANKİ durumuyla çelişiyor. Mesaj iç
		// detay taşımıyor (yalnızca statü ve işlem adı) → istemciye gösterilebilir.
		writeError(w, http.StatusConflict, "invalid_transition", err.Error())
	default:
		writeInternalError(w, op, err)
	}
}

// handleSuggestBook, POST /api/books/suggestions (giriş yapmış kullanıcı) → 201.
// Kitap PENDING doğar ve admin onaylayana kadar public listede görünmez.
func handleSuggestBook(svc moderationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		req, ok := readBookRequest(w, r)
		if !ok {
			return
		}
		book, err := svc.Suggest(r.Context(), user, req.toParams())
		if err != nil {
			writeBookWriteError(w, "SuggestBook", err)
			return
		}
		writeJSON(w, http.StatusCreated, suggestionResponse{bookResponse: toBookResponse(book, nil), Status: book.Status})
	}
}

// handleAdminListBooks, GET /api/admin/books?status=PENDING&limit=&offset= .
// status verilmezse PENDING: admin panelinin ana ekranı "bekleyen öneriler" kuyruğu.
func handleAdminListBooks(svc moderationService, store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var v validator
		status := books.StatusPending
		if raw := q.Get("status"); raw != "" {
			parsed, err := books.ParseStatus(strings.ToUpper(raw))
			v.check(err == nil, "status PENDING, APPROVED, REJECTED ya da ARCHIVED olmalı")
			status = parsed
		}
		limit, offset := parsePage(q, &v)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		list, total, err := svc.List(r.Context(), status, limit, offset)
		if err != nil {
			writeInternalError(w, "AdminListBooks", err)
			return
		}
		ids := make([]int64, 0, len(list))
		for _, b := range list {
			ids = append(ids, b.ID)
		}
		genresByBook, err := store.GenresForBooks(r.Context(), ids)
		if err != nil {
			writeInternalError(w, "GenresForBooks", err)
			return
		}
		items := make([]adminBookResponse, 0, len(list))
		for _, b := range list {
			items = append(items, toAdminBookResponse(b, genresByBook[b.ID]))
		}
		writeJSON(w, http.StatusOK, adminBookPage{Items: items, Total: total, Limit: limit, Offset: offset})
	}
}

// writeAdminBook, tek kitabı türleri ve baskılarıyla admin DTO'su olarak yazar.
func writeAdminBook(w http.ResponseWriter, r *http.Request, store bookStore, b db.BookView) {
	genreList, err := store.GenresForBook(r.Context(), b.ID)
	if err != nil {
		writeInternalError(w, "GenresForBook", err)
		return
	}
	eds, err := store.Editions(r.Context(), b.ID)
	if err != nil {
		writeInternalError(w, "Editions", err)
		return
	}
	writeJSON(w, http.StatusOK, adminBookDetailResponse{adminBookResponse: toAdminBookResponse(b, genreList), Editions: toEditionResponses(eds)})
}

// handleAdminGetBook, GET /api/admin/books/{id} — statüden bağımsız.
func handleAdminGetBook(svc moderationService, store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		b, err := svc.Get(r.Context(), id)
		if err != nil {
			writeBookWriteError(w, "AdminGetBook", err)
			return
		}
		writeAdminBook(w, r, store, b)
	}
}

// updateBookRequest, PATCH gövdesi: her alan pointer — nil = "gönderilmedi, dokunma".
// Status yok (statü yalnızca approve/reject/archive/restore ile, durum makinesinden geçer).
type updateBookRequest struct {
	Title           *string `json:"title"`
	Slug            *string `json:"slug"`
	Author          *string `json:"author"`
	Description     *string `json:"description"`
	CoverURL        *string `json:"cover_url"`
	PublicationYear *int32  `json:"publication_year"`
	PageCount       *int32  `json:"page_count"`
	Language        *string `json:"language"`
}

func trimPtr(p *string) {
	if p != nil {
		*p = strings.TrimSpace(*p)
	}
}

func (req *updateBookRequest) normalize() {
	for _, p := range []*string{req.Title, req.Slug, req.Author, req.Description, req.CoverURL, req.Language} {
		trimPtr(p)
	}
}

// validate: yalnızca GÖNDERİLEN alanlar, create ile aynı kurallarla (check* fonksiyonları).
func (req updateBookRequest) validate() error {
	var v validator
	if req == (updateBookRequest{}) {
		v.add("en az bir alan gönderilmeli")
		return v.err()
	}
	if req.Title != nil {
		checkTitle(&v, *req.Title)
	}
	if req.Slug != nil {
		checkSlug(&v, *req.Slug)
	}
	if req.Author != nil {
		checkAuthor(&v, *req.Author)
	}
	if req.Description != nil {
		checkDescription(&v, *req.Description)
	}
	if req.CoverURL != nil {
		checkCoverURL(&v, *req.CoverURL)
	}
	if req.Language != nil {
		checkLanguage(&v, *req.Language)
	}
	checkPublicationYear(&v, req.PublicationYear)
	checkPageCount(&v, req.PageCount)
	return v.err()
}

func (req updateBookRequest) toParams() books.UpdateParams {
	return books.UpdateParams{
		UpdateBookDetailsParams: db.UpdateBookDetailsParams{
			Title:           req.Title,
			Slug:            req.Slug,
			Author:          req.Author,
			Description:     req.Description,
			PublicationYear: req.PublicationYear,
			Language:        req.Language,
		},
		CoverURL:  req.CoverURL,
		PageCount: req.PageCount,
	}
}

// handleAdminUpdateBook, PATCH /api/admin/books/{id} — onay öncesi düzeltme gibi.
func handleAdminUpdateBook(svc moderationService, store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req updateBookRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		req.normalize()
		if err := req.validate(); err != nil {
			writeRequestError(w, err)
			return
		}
		b, err := svc.Update(r.Context(), actor, id, req.toParams())
		if err != nil {
			writeBookWriteError(w, "AdminUpdateBook", err)
			return
		}
		writeAdminBook(w, r, store, b)
	}
}

// transitionFunc, gövdesiz geçişlerin (approve/archive/restore) ortak imzası.
//
// METHOD EXPRESSION: `moderationService.Approve` yazınca Go, alıcıyı (receiver) İLK
// parametre yapan sıradan bir fonksiyon verir: func(svc, ctx, actorID, bookID). Böylece
// üç handler'ı tek fonksiyonla, string anahtarsız ("approve" yazım hatası derlenirdi)
// ve servis nil olsa bile (yalnız bir alanı dolduran testler) güvenle kurarız.
type transitionFunc func(svc moderationService, ctx context.Context, actorID, bookID int64) (db.BookView, error)

// handleTransition, gövdesiz bir durum geçişini HTTP'ye bağlar.
// DELETE /api/admin/books/{id} de buraya (Archive) bağlı: "silme" = yumuşak silme —
// kitap yayından kalkar ama yorumları, puanları ve denetim geçmişi yaşar.
func handleTransition(op string, svc moderationService, fn transitionFunc, store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		b, err := fn(svc, r.Context(), actor, id)
		if err != nil {
			writeBookWriteError(w, op, err)
			return
		}
		writeAdminBook(w, r, store, b)
	}
}

const maxRejectionReasonLen = 500

type rejectRequest struct {
	Reason string `json:"reason"`
}

// handleRejectBook, POST /api/admin/books/{id}/reject — gövde opsiyonel: {"reason":"..."}.
func handleRejectBook(svc moderationService, store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req rejectRequest
		// Gövdesiz istek = gerekçesiz red. Gövde VARSA tam doğrulamadan geçer.
		if r.ContentLength != 0 {
			if err := readJSON(w, r, &req); err != nil {
				writeRequestError(w, err)
				return
			}
		}
		req.Reason = strings.TrimSpace(req.Reason)
		var v validator
		v.maxLen("reason", req.Reason, maxRejectionReasonLen)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		b, err := svc.Reject(r.Context(), actor, id, req.Reason)
		if err != nil {
			writeBookWriteError(w, "RejectBook", err)
			return
		}
		writeAdminBook(w, r, store, b)
	}
}

// handleBookHistory, GET /api/admin/books/{id}/audit — kitabın denetim kaydı.
func handleBookHistory(svc moderationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		logs, err := svc.History(r.Context(), id)
		if err != nil {
			writeBookWriteError(w, "BookHistory", err)
			return
		}
		out := make([]auditEntryResponse, 0, len(logs))
		for _, l := range logs {
			out = append(out, auditEntryResponse{ID: l.ID, ActorID: l.ActorID, Action: l.Action, Details: l.Details, CreatedAt: l.CreatedAt})
		}
		writeJSON(w, http.StatusOK, out)
	}
}
