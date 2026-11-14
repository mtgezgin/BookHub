package main

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/editions"
)

// editionService, baskı yazım uçlarının servisten beklediği davranışlar (Faz 16).
// *editions.Service bunu sağlar. Baskı OKUMA bookStore'da (Editions): kural yok, düz okuma.
type editionService interface {
	Create(ctx context.Context, actorID, bookID int64, p editions.CreateParams) (db.Edition, error)
	Update(ctx context.Context, actorID, editionID int64, p db.UpdateEditionParams) (db.Edition, error)
	Delete(ctx context.Context, actorID, editionID int64) error
	SetPrimary(ctx context.Context, actorID, editionID int64) (db.Edition, error)
}

// editionResponse, baskının herkese açık temsili. book_id ve zaman damgaları yok: baskı her
// zaman kitabının içinde döner; admin yazım uçları da aynı DTO'yu kullanır.
type editionResponse struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	Language        string `json:"language"`
	Translator      string `json:"translator"`
	Publisher       string `json:"publisher"`
	PublicationYear *int32 `json:"publication_year"`
	Isbn            string `json:"isbn"`
	PageCount       *int32 `json:"page_count"`
	CoverURL        string `json:"cover_url"`
	IsPrimary       bool   `json:"is_primary"`
}

func toEditionResponse(e db.Edition) editionResponse {
	return editionResponse{
		ID:              e.ID,
		Title:           e.Title,
		Language:        e.Language,
		Translator:      e.Translator,
		Publisher:       e.Publisher,
		PublicationYear: e.PublicationYear,
		Isbn:            e.Isbn,
		PageCount:       e.PageCount,
		CoverURL:        e.CoverUrl,
		IsPrimary:       e.IsPrimary,
	}
}

// toEditionResponses: baskısız kitapta bile "[]" (null değil).
func toEditionResponses(list []db.Edition) []editionResponse {
	out := make([]editionResponse, 0, len(list))
	for _, e := range list {
		out = append(out, toEditionResponse(e))
	}
	return out
}

// bookDetailResponse, GET /api/books/{id}: kitabın public hâli + baskıları. Liste uçları ve
// kitaplık öğesi bookResponse'ta kalır (baskılar yalnız detayda).
type bookDetailResponse struct {
	bookResponse
	Editions []editionResponse `json:"editions"`
}

// adminBookDetailResponse, tek kitap dönen admin uçları: admin DTO'su + baskılar.
type adminBookDetailResponse struct {
	adminBookResponse
	Editions []editionResponse `json:"editions"`
}

// isbnPattern: tire ve boşluk temizlendikten sonra ISBN-10 (son hane X olabilir) ya da ISBN-13.
// Sağlama (checksum) kontrolü bilerek yok (spec kapsam dışı): eski kayıtlarda hatalı ISBN'ler var.
var isbnPattern = regexp.MustCompile(`^(?:[0-9]{9}[0-9X]|[0-9]{13})$`)

// normalizeISBN: "978-975-07-0123-4" / "975 07 0123 x" → "9789750701234" / "975070123X".
func normalizeISBN(s string) string {
	s = strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(s))
	return strings.ToUpper(s)
}

func checkTranslator(v *validator, s string) { v.maxLen("translator", s, maxAuthorLen) }
func checkPublisher(v *validator, s string)  { v.maxLen("publisher", s, maxAuthorLen) }

func checkISBN(v *validator, s string) {
	if s != "" {
		v.match("isbn", s, isbnPattern, "10 ya da 13 hane; tire ve boşluk olabilir")
	}
}

// createEditionRequest, POST /api/admin/books/{id}/editions gövdesi. is_primary YOK: kitabın
// varsayılanı yoksa yeni baskı varsayılan olur; değiştirmek ayrı uç (…/primary).
type createEditionRequest struct {
	Title           string `json:"title"`
	Language        string `json:"language"`
	Translator      string `json:"translator"`
	Publisher       string `json:"publisher"`
	PublicationYear *int32 `json:"publication_year"`
	Isbn            string `json:"isbn"`
	PageCount       *int32 `json:"page_count"`
	CoverURL        string `json:"cover_url"`
}

func (req *createEditionRequest) normalize() {
	req.Title = strings.TrimSpace(req.Title)
	req.Language = strings.TrimSpace(req.Language)
	req.Translator = strings.TrimSpace(req.Translator)
	req.Publisher = strings.TrimSpace(req.Publisher)
	req.Isbn = normalizeISBN(req.Isbn)
	req.CoverURL = strings.TrimSpace(req.CoverURL)
}

// validate: kitap alanlarıyla AYNI kural fonksiyonları (checkTitle, checkLanguage …). Dil
// zorunlu ve varsayılanı yok: bir baskının dilini sunucu tahmin etmez.
func (req createEditionRequest) validate() error {
	var v validator
	checkTitle(&v, req.Title)
	checkLanguage(&v, req.Language)
	checkTranslator(&v, req.Translator)
	checkPublisher(&v, req.Publisher)
	checkPublicationYear(&v, req.PublicationYear)
	checkISBN(&v, req.Isbn)
	checkPageCount(&v, req.PageCount)
	checkCoverURL(&v, req.CoverURL)
	return v.err()
}

func (req createEditionRequest) toParams() editions.CreateParams {
	return editions.CreateParams{
		Title:           req.Title,
		Language:        req.Language,
		Translator:      req.Translator,
		Publisher:       req.Publisher,
		PublicationYear: req.PublicationYear,
		Isbn:            req.Isbn,
		PageCount:       req.PageCount,
		CoverURL:        req.CoverURL,
	}
}

// updateEditionRequest, PATCH /api/admin/editions/{id}: nil = dokunma. Bir metin alanını
// boşaltmak için "" gönderilir (çevirmen bilinmiyor); sayılar NULL'a geri çekilemez (kitaptaki
// PATCH'le aynı sınır).
type updateEditionRequest struct {
	Title           *string `json:"title"`
	Language        *string `json:"language"`
	Translator      *string `json:"translator"`
	Publisher       *string `json:"publisher"`
	PublicationYear *int32  `json:"publication_year"`
	Isbn            *string `json:"isbn"`
	PageCount       *int32  `json:"page_count"`
	CoverURL        *string `json:"cover_url"`
}

func (req *updateEditionRequest) normalize() {
	for _, p := range []*string{req.Title, req.Language, req.Translator, req.Publisher, req.CoverURL} {
		trimPtr(p)
	}
	if req.Isbn != nil {
		*req.Isbn = normalizeISBN(*req.Isbn)
	}
}

func (req updateEditionRequest) validate() error {
	var v validator
	if req == (updateEditionRequest{}) {
		v.add("en az bir alan gönderilmeli")
		return v.err()
	}
	if req.Title != nil {
		checkTitle(&v, *req.Title)
	}
	if req.Language != nil {
		checkLanguage(&v, *req.Language)
	}
	if req.Translator != nil {
		checkTranslator(&v, *req.Translator)
	}
	if req.Publisher != nil {
		checkPublisher(&v, *req.Publisher)
	}
	if req.Isbn != nil {
		checkISBN(&v, *req.Isbn)
	}
	if req.CoverURL != nil {
		checkCoverURL(&v, *req.CoverURL)
	}
	checkPublicationYear(&v, req.PublicationYear)
	checkPageCount(&v, req.PageCount)
	return v.err()
}

func (req updateEditionRequest) toParams() db.UpdateEditionParams {
	return db.UpdateEditionParams{
		Title:           req.Title,
		Language:        req.Language,
		Translator:      req.Translator,
		Publisher:       req.Publisher,
		PublicationYear: req.PublicationYear,
		Isbn:            req.Isbn,
		PageCount:       req.PageCount,
		CoverUrl:        req.CoverURL,
	}
}

// writeEditionError, baskı yazım hatalarını HTTP'ye çevirir (tek yer). Ham hata istemciye gitmez.
func writeEditionError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, books.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "kitap bulunamadı")
	case errors.Is(err, editions.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "baskı bulunamadı")
	case errors.Is(err, editions.ErrPrimaryEditionDelete):
		writeError(w, http.StatusConflict, "primary_edition_delete", err.Error())
	default:
		writeInternalError(w, op, err)
	}
}

// handleCreateEdition, POST /api/admin/books/{id}/editions → 201.
func handleCreateEdition(svc editionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, bookID, ok := actorAndID(w, r)
		if !ok {
			return
		}
		req, ok := readBody[createEditionRequest](w, r)
		if !ok {
			return
		}
		ed, err := svc.Create(r.Context(), actor, bookID, req.toParams())
		if err != nil {
			writeEditionError(w, "CreateEdition", err)
			return
		}
		writeJSON(w, http.StatusCreated, toEditionResponse(ed))
	}
}

// handleUpdateEdition, PATCH /api/admin/editions/{id} → 200.
func handleUpdateEdition(svc editionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := actorAndID(w, r)
		if !ok {
			return
		}
		req, ok := readBody[updateEditionRequest](w, r)
		if !ok {
			return
		}
		ed, err := svc.Update(r.Context(), actor, id, req.toParams())
		if err != nil {
			writeEditionError(w, "UpdateEdition", err)
			return
		}
		writeJSON(w, http.StatusOK, toEditionResponse(ed))
	}
}

// handleDeleteEdition, DELETE /api/admin/editions/{id} → 204. Varsayılan baskı → 409.
func handleDeleteEdition(svc editionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := actorAndID(w, r)
		if !ok {
			return
		}
		if err := svc.Delete(r.Context(), actor, id); err != nil {
			writeEditionError(w, "DeleteEdition", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleSetPrimaryEdition, POST /api/admin/editions/{id}/primary → 200. Gövdesiz; idempotent
// (zaten varsayılan olan baskı için de 200).
func handleSetPrimaryEdition(svc editionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := actorAndID(w, r)
		if !ok {
			return
		}
		ed, err := svc.SetPrimary(r.Context(), actor, id)
		if err != nil {
			writeEditionError(w, "SetPrimaryEdition", err)
			return
		}
		writeJSON(w, http.StatusOK, toEditionResponse(ed))
	}
}
