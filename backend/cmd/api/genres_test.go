package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/genres"
)

// fakeGenres, genreStore sahtesi; Create/Update'e gelen argümanları kaydeder.
type fakeGenres struct {
	list      []db.Genre
	genre     db.Genre
	err       error
	gotName   string
	gotSlug   string
	gotUpdate [2]*string
}

func (f *fakeGenres) List(context.Context) ([]db.Genre, error) { return f.list, f.err }
func (f *fakeGenres) Create(_ context.Context, name, slug string) (db.Genre, error) {
	f.gotName, f.gotSlug = name, slug
	return f.genre, f.err
}
func (f *fakeGenres) Update(_ context.Context, _ int64, name, slug *string) (db.Genre, error) {
	f.gotUpdate = [2]*string{name, slug}
	return f.genre, f.err
}
func (f *fakeGenres) Delete(context.Context, int64) error { return f.err }

func TestHandleListGenres(t *testing.T) {
	fake := &fakeGenres{list: []db.Genre{{ID: 1, Name: "Bilim Kurgu", Slug: "bilim-kurgu"}}}
	rec := do(t, newRouter(deps{genres: fake}), http.MethodGet, "/api/genres", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("durum kodu: istenen 200, gelen %d", rec.Code)
	}
	var got []genreResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("geçersiz JSON: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "bilim-kurgu" || got[0].Name != "Bilim Kurgu" {
		t.Errorf("liste yanlış: %+v", got)
	}

	empty := do(t, newRouter(deps{genres: &fakeGenres{}}), http.MethodGet, "/api/genres", "")
	if strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Errorf("boş liste [] olmalı, gelen %q", empty.Body.String())
	}
}

func TestHandleCreateGenre(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		err        error
		wantStatus int
		wantCode   string
		wantSlug   string // Create'e gitmesi beklenen slug
	}{
		{name: "slug verilmezse addan türetilir", body: `{"name":"Bilim Kurgu"}`, wantStatus: http.StatusCreated, wantSlug: "bilim-kurgu"},
		{name: "verilen slug korunur", body: `{"name":"Bilim Kurgu","slug":"sci-fi"}`, wantStatus: http.StatusCreated, wantSlug: "sci-fi"},
		{name: "ad eksik → 400", body: `{"slug":"x"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "geçersiz slug → 400", body: `{"name":"X","slug":"Kötü Slug"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "aynı ad → 409 name_conflict", body: `{"name":"Tarih"}`, err: genres.ErrDuplicateName, wantStatus: http.StatusConflict, wantCode: "name_conflict"},
		{name: "aynı slug → 409 slug_conflict", body: `{"name":"Tarih"}`, err: genres.ErrDuplicateSlug, wantStatus: http.StatusConflict, wantCode: "slug_conflict"},
		{name: "repo hatası → 500", body: `{"name":"Tarih"}`, err: errors.New("x"), wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeGenres{genre: db.Genre{ID: 1}, err: tt.err}
			rec := do(t, adminRouter(deps{genres: fake}), http.MethodPost, "/api/admin/genres", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (gövde: %s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Errorf("error.code: istenen %q, gelen %q", tt.wantCode, got)
				}
			}
			if tt.wantSlug != "" && fake.gotSlug != tt.wantSlug {
				t.Errorf("Create'e giden slug: istenen %q, gelen %q", tt.wantSlug, fake.gotSlug)
			}
		})
	}
}

func TestHandleUpdateGenre(t *testing.T) {
	t.Run("yalnız ad gönderilir → slug nil (dokunulmaz)", func(t *testing.T) {
		fake := &fakeGenres{genre: db.Genre{ID: 3}}
		rec := do(t, adminRouter(deps{genres: fake}), http.MethodPatch, "/api/admin/genres/3", `{"name":" Yeni Ad "}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("durum kodu: istenen 200, gelen %d (%s)", rec.Code, rec.Body)
		}
		if fake.gotUpdate[0] == nil || *fake.gotUpdate[0] != "Yeni Ad" || fake.gotUpdate[1] != nil {
			t.Errorf("kısmi güncelleme yanlış: name=%v slug=%v", fake.gotUpdate[0], fake.gotUpdate[1])
		}
	})
	for _, tc := range []struct {
		name, url, body string
		err             error
		wantStatus      int
	}{
		{"boş gövde nesnesi → 400", "/api/admin/genres/3", `{}`, nil, http.StatusBadRequest},
		{"boş ad → 400", "/api/admin/genres/3", `{"name":"  "}`, nil, http.StatusBadRequest},
		{"geçersiz id → 400", "/api/admin/genres/x", `{"name":"a"}`, nil, http.StatusBadRequest},
		{"olmayan tür → 404", "/api/admin/genres/9", `{"name":"a"}`, genres.ErrNotFound, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, adminRouter(deps{genres: &fakeGenres{err: tc.err}}), http.MethodPatch, tc.url, tc.body)
			if rec.Code != tc.wantStatus {
				t.Errorf("durum kodu: istenen %d, gelen %d (%s)", tc.wantStatus, rec.Code, rec.Body)
			}
		})
	}
}

func TestHandleDeleteGenre(t *testing.T) {
	ok := do(t, adminRouter(deps{genres: &fakeGenres{}}), http.MethodDelete, "/api/admin/genres/3", "")
	if ok.Code != http.StatusNoContent || ok.Body.Len() != 0 {
		t.Errorf("istenen 204 + boş gövde, gelen %d %q", ok.Code, ok.Body.String())
	}
	missing := do(t, adminRouter(deps{genres: &fakeGenres{err: genres.ErrNotFound}}), http.MethodDelete, "/api/admin/genres/3", "")
	if missing.Code != http.StatusNotFound {
		t.Errorf("olmayan tür: istenen 404, gelen %d", missing.Code)
	}
}
