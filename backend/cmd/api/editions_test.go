package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/editions"
)

// fakeEditions, editionService'in sahtesi: gelen parametreleri yakalar, söyleneni döner.
type fakeEditions struct {
	ed        db.Edition
	err       error
	calls     []string
	gotActor  int64
	gotID     int64
	gotCreate editions.CreateParams
	gotUpdate db.UpdateEditionParams
}

func (f *fakeEditions) rec(call string, actor, id int64) {
	f.calls = append(f.calls, call)
	f.gotActor, f.gotID = actor, id
}

func (f *fakeEditions) Create(_ context.Context, actor, bookID int64, p editions.CreateParams) (db.Edition, error) {
	f.rec("Create", actor, bookID)
	f.gotCreate = p
	return f.ed, f.err
}
func (f *fakeEditions) Update(_ context.Context, actor, id int64, p db.UpdateEditionParams) (db.Edition, error) {
	f.rec("Update", actor, id)
	f.gotUpdate = p
	return f.ed, f.err
}
func (f *fakeEditions) Delete(_ context.Context, actor, id int64) error {
	f.rec("Delete", actor, id)
	return f.err
}
func (f *fakeEditions) SetPrimary(_ context.Context, actor, id int64) (db.Edition, error) {
	f.rec("SetPrimary", actor, id)
	return f.ed, f.err
}

func TestCreateEdition(t *testing.T) {
	const valid = `{"title":"Преступление и наказание","language":"ru","translator":" ","publisher":"Русский вестник",
		"publication_year":1866,"isbn":"978-5-17-090630-7","page_count":672,"cover_url":"https://example.test/ru.jpg"}`

	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "geçerli → 201", body: valid, wantStatus: http.StatusCreated},
		{name: "bilinmeyen alan (is_primary) → 400", body: `{"title":"x","language":"ru","is_primary":true}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "başlık ve dil zorunlu → 400", body: `{"translator":"x"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "geçersiz dil → 400", body: `{"title":"x","language":"Rusça"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "ISBN 11 hane → 400", body: `{"title":"x","language":"ru","isbn":"12345678901"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "çevirmen 201 karakter → 400", body: `{"title":"x","language":"ru","translator":"` + strings.Repeat("a", 201) + `"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "yıl 1450'den önce → 400", body: `{"title":"x","language":"ru","publication_year":1000}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "kapak http(s) değil → 400", body: `{"title":"x","language":"ru","cover_url":"javascript:alert(1)"}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "kitap yok → 404", body: valid, svcErr: books.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "beklenmeyen hata → 500, ham hata gitmez", body: valid, svcErr: errors.New("pq: gizli ayrıntı"), wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEditions{ed: db.Edition{ID: 9, BookID: 3, Title: "Преступление и наказание", Language: "ru"}, err: tt.svcErr}
			rec := do(t, adminRouter(deps{editions: fake}), http.MethodPost, "/api/admin/books/3/editions", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" && errorCode(t, rec) != tt.wantCode {
				t.Errorf("hata kodu: istenen %q, gelen %q", tt.wantCode, errorCode(t, rec))
			}
			if strings.Contains(rec.Body.String(), "gizli ayrıntı") {
				t.Error("ham hata istemciye sızdı")
			}
			if tt.wantStatus == http.StatusBadRequest && len(fake.calls) != 0 {
				t.Errorf("geçersiz istekte servis çağrılmamalı: %v", fake.calls)
			}
		})
	}

	t.Run("gövde temizlenir; kitap ve aktör URL'den ve token'dan", func(t *testing.T) {
		fake := &fakeEditions{ed: db.Edition{ID: 9}}
		rec := do(t, adminRouter(deps{editions: fake}), http.MethodPost, "/api/admin/books/3/editions", valid)
		if rec.Code != http.StatusCreated {
			t.Fatalf("durum kodu %d (%s)", rec.Code, rec.Body)
		}
		p := fake.gotCreate
		if fake.gotID != 3 || fake.gotActor != 1 {
			t.Errorf("kitap 3 ve aktör 1 bekleniyordu: kitap %d, aktör %d", fake.gotID, fake.gotActor)
		}
		if p.Isbn != "9785170906307" || p.Translator != "" || p.Language != "ru" || p.PageCount == nil || *p.PageCount != 672 {
			t.Errorf("ISBN tiresiz, çevirmen kırpılmış olmalı: %+v", p)
		}
	})

	t.Run("ISBN-10'un küçük x'i büyür", func(t *testing.T) {
		fake := &fakeEditions{}
		rec := do(t, adminRouter(deps{editions: fake}), http.MethodPost, "/api/admin/books/3/editions", `{"title":"x","language":"ru","isbn":"975 07 0123 x"}`)
		if rec.Code != http.StatusCreated || fake.gotCreate.Isbn != "975070123X" {
			t.Errorf("durum %d, ISBN %q", rec.Code, fake.gotCreate.Isbn)
		}
	})
}

func TestUpdateEdition(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "tek alan → 200", body: `{"translator":"Mazlum Beyhan"}`, wantStatus: http.StatusOK},
		{name: "boş gövde → 400", body: `{}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "başlık boşaltılamaz → 400", body: `{"title":"  "}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "is_primary PATCH'le değişmez → 400", body: `{"is_primary":true}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_json"},
		{name: "baskı yok → 404", body: `{"translator":"x"}`, svcErr: editions.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEditions{ed: db.Edition{ID: 5}, err: tt.svcErr}
			rec := do(t, adminRouter(deps{editions: fake}), http.MethodPatch, "/api/admin/editions/5", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" && errorCode(t, rec) != tt.wantCode {
				t.Errorf("hata kodu: istenen %q, gelen %q", tt.wantCode, errorCode(t, rec))
			}
		})
	}

	t.Run("yalnız gönderilen alanlar servise gider", func(t *testing.T) {
		fake := &fakeEditions{ed: db.Edition{ID: 5}}
		do(t, adminRouter(deps{editions: fake}), http.MethodPatch, "/api/admin/editions/5", `{"isbn":"975-07-0123-x","publisher":""}`)
		u := fake.gotUpdate
		if fake.gotID != 5 || u.Isbn == nil || *u.Isbn != "975070123X" || u.Publisher == nil || *u.Publisher != "" || u.Title != nil || u.PageCount != nil {
			t.Errorf("parametreler yanlış: %+v", u)
		}
	})
}

func TestDeleteAndSetPrimaryEdition(t *testing.T) {
	t.Run("silme → 204, gövdesiz", func(t *testing.T) {
		fake := &fakeEditions{}
		rec := do(t, adminRouter(deps{editions: fake}), http.MethodDelete, "/api/admin/editions/5", "")
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || fake.gotID != 5 {
			t.Errorf("durum %d, gövde %q, id %d", rec.Code, rec.Body, fake.gotID)
		}
	})
	t.Run("varsayılan baskıyı silmek → 409", func(t *testing.T) {
		fake := &fakeEditions{err: editions.ErrPrimaryEditionDelete}
		rec := do(t, adminRouter(deps{editions: fake}), http.MethodDelete, "/api/admin/editions/5", "")
		if rec.Code != http.StatusConflict || errorCode(t, rec) != "primary_edition_delete" {
			t.Errorf("durum %d, kod %q", rec.Code, errorCode(t, rec))
		}
	})
	t.Run("varsayılan yap → 200 ve baskı", func(t *testing.T) {
		fake := &fakeEditions{ed: db.Edition{ID: 5, IsPrimary: true}}
		rec := do(t, adminRouter(deps{editions: fake}), http.MethodPost, "/api/admin/editions/5/primary", "")
		var got editionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil || !got.IsPrimary || got.ID != 5 {
			t.Errorf("durum %d, gövde %s", rec.Code, rec.Body)
		}
	})
	t.Run("geçersiz id → 400, servis çağrılmaz", func(t *testing.T) {
		fake := &fakeEditions{}
		rec := do(t, adminRouter(deps{editions: fake}), http.MethodPost, "/api/admin/editions/abc/primary", "")
		if rec.Code != http.StatusBadRequest || len(fake.calls) != 0 {
			t.Errorf("durum %d, çağrılar %v", rec.Code, fake.calls)
		}
	})
}

// Detay uçları: public ve admin kitap detayı baskıları taşır; baskı okuma hatası 500.
func TestBookDetailIncludesEditions(t *testing.T) {
	store := fakeBooks{book: db.BookView{ID: 1, Title: "Suç ve Ceza", Status: "APPROVED"}, editions: []db.Edition{
		{ID: 1, Title: "Suç ve Ceza", Language: "tr", IsPrimary: true},
		{ID: 2, Title: "Преступление и наказание", Language: "ru"},
	}}
	rec := do(t, newRouter(deps{books: store}), http.MethodGet, "/api/books/1", "")
	var got struct {
		Editions []editionResponse `json:"editions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil || len(got.Editions) != 2 || !got.Editions[0].IsPrimary {
		t.Fatalf("durum %d, gövde %s", rec.Code, rec.Body)
	}

	t.Run("baskısız kitapta editions boş dizi (null değil)", func(t *testing.T) {
		rec := do(t, newRouter(deps{books: fakeBooks{book: db.BookView{ID: 1}}}), http.MethodGet, "/api/books/1", "")
		if !strings.Contains(rec.Body.String(), `"editions":[]`) {
			t.Errorf("gövde: %s", rec.Body)
		}
	})
	t.Run("baskı okunamazsa 500", func(t *testing.T) {
		rec := do(t, newRouter(deps{books: fakeBooks{book: db.BookView{ID: 1}, editionsErr: errors.New("x")}}), http.MethodGet, "/api/books/1", "")
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("durum %d", rec.Code)
		}
	})
	t.Run("admin detayı da baskıları taşır", func(t *testing.T) {
		fake := &fakeModeration{book: db.BookView{ID: 1, Status: "PENDING"}}
		rec := do(t, adminRouter(deps{moderation: fake, books: store}), http.MethodGet, "/api/admin/books/1", "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"editions":[{"id":1`) {
			t.Errorf("durum %d, gövde %s", rec.Code, rec.Body)
		}
	})
}
