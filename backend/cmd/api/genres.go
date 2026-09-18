package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/genres"
	"github.com/mtgezgin/bookhub/backend/internal/slug"
)

// genreStore, tür handler'larının repository'den ihtiyaç duyduğu davranışlar.
type genreStore interface {
	List(ctx context.Context) ([]db.Genre, error)
	Create(ctx context.Context, name, slug string) (db.Genre, error)
	Update(ctx context.Context, id int64, name, slug *string) (db.Genre, error)
	Delete(ctx context.Context, id int64) error
}

// genreResponse, türün public temsili (created_at bilerek yok — istemcinin işine yaramaz).
type genreResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func toGenreResponse(g db.Genre) genreResponse {
	return genreResponse{ID: g.ID, Name: g.Name, Slug: g.Slug}
}

// genreRequest, hem oluşturma hem kısmi güncelleme (PATCH) gövdesi. Pointer alanlar
// "gönderilmedi" ile "boş gönderildi"yi ayırt etmek için: PATCH'te nil = dokunma.
type genreRequest struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}

func (req *genreRequest) normalize() {
	if req.Name != nil {
		*req.Name = strings.TrimSpace(*req.Name)
	}
	if req.Slug != nil {
		*req.Slug = strings.TrimSpace(*req.Slug)
	}
}

// validateFields, verilen (nil olmayan) alanları denetler.
func (req genreRequest) validateFields(v *validator) {
	if req.Name != nil {
		v.required("name", *req.Name)
		v.maxLen("name", *req.Name, maxNameLen)
	}
	if req.Slug != nil {
		v.required("slug", *req.Slug)
		v.maxLen("slug", *req.Slug, maxSlugLen)
		v.match("slug", *req.Slug, slugPattern, "küçük harf, rakam ve tek tire")
	}
}

// writeGenreError, tür domain hatalarını HTTP'ye çevirir (tek yerde).
func writeGenreError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, genres.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "tür bulunamadı")
	case errors.Is(err, genres.ErrDuplicateName):
		writeError(w, http.StatusConflict, "name_conflict", "bu tür adı zaten kullanılıyor")
	case errors.Is(err, genres.ErrDuplicateSlug):
		writeError(w, http.StatusConflict, "slug_conflict", "bu tür slug'ı zaten kullanılıyor")
	default:
		writeInternalError(w, op, err)
	}
}

// handleListGenres, GET /api/genres — herkese açık.
func handleListGenres(store genreStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := store.List(r.Context())
		if err != nil {
			writeInternalError(w, "ListGenres", err)
			return
		}
		writeJSON(w, http.StatusOK, toGenreResponses(list))
	}
}

// handleCreateGenre, POST /api/admin/genres — slug gönderilmezse addan türetilir.
func handleCreateGenre(store genreStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req genreRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		req.normalize()

		var v validator
		if req.Name == nil {
			v.add("name zorunlu")
		} else if req.Slug == nil || *req.Slug == "" {
			derived := slug.Make(*req.Name)
			req.Slug = &derived
		}
		req.validateFields(&v)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		g, err := store.Create(r.Context(), *req.Name, *req.Slug)
		if err != nil {
			writeGenreError(w, "CreateGenre", err)
			return
		}
		writeJSON(w, http.StatusCreated, toGenreResponse(g))
	}
}

// handleUpdateGenre, PATCH /api/admin/genres/{id} — yalnızca gönderilen alanlar değişir.
func handleUpdateGenre(store genreStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req genreRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		req.normalize()

		var v validator
		v.check(req.Name != nil || req.Slug != nil, "en az bir alan (name, slug) gönderilmeli")
		req.validateFields(&v)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		g, err := store.Update(r.Context(), id, req.Name, req.Slug)
		if err != nil {
			writeGenreError(w, "UpdateGenre", err)
			return
		}
		writeJSON(w, http.StatusOK, toGenreResponse(g))
	}
}

// handleDeleteGenre, DELETE /api/admin/genres/{id} — 204 No Content (gövdesiz başarı).
func handleDeleteGenre(store genreStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		if err := store.Delete(r.Context(), id); err != nil {
			writeGenreError(w, "DeleteGenre", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
