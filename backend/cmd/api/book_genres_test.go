package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// countingBooks, tür yükleme çağrılarını SAYAR — N+1 korumasının testi için.
type countingBooks struct {
	fakeBooks
	single, batch int
}

func (c *countingBooks) GenresForBook(ctx context.Context, id int64) ([]db.Genre, error) {
	c.single++
	return c.fakeBooks.GenresForBook(ctx, id)
}
func (c *countingBooks) GenresForBooks(ctx context.Context, ids []int64) (map[int64][]db.Genre, error) {
	c.batch++
	return c.fakeBooks.GenresForBooks(ctx, ids)
}

// TestListBooksLoadsGenresInOneQuery, liste ucunun N+1 yapmadığını kanıtlar: kitap
// başına GenresForBook DEĞİL, bütün liste için TEK bir GenresForBooks çağrısı.
// Birisi döngü içinde tekil sorguya dönerse bu test kırılır.
func TestListBooksLoadsGenresInOneQuery(t *testing.T) {
	store := &countingBooks{fakeBooks: fakeBooks{
		list: []db.Book{{ID: 1}, {ID: 2}, {ID: 3}},
		genres: map[int64][]db.Genre{
			1: {{ID: 10, Name: "Roman", Slug: "roman"}},
			3: {{ID: 11, Name: "Tarih", Slug: "tarih"}, {ID: 10, Name: "Roman", Slug: "roman"}},
		},
	}}
	rec := do(t, newRouter(deps{books: store}), http.MethodGet, "/api/books", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("durum kodu: istenen 200, gelen %d", rec.Code)
	}
	if store.single != 0 || store.batch != 1 {
		t.Errorf("N+1: istenen 0 tekil + 1 toplu sorgu, gelen %d tekil + %d toplu", store.single, store.batch)
	}

	var got []bookResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("geçersiz JSON: %v", err)
	}
	if len(got[0].Genres) != 1 || got[0].Genres[0].Slug != "roman" {
		t.Errorf("1. kitabın türleri yanlış: %+v", got[0].Genres)
	}
	if got[1].Genres == nil || len(got[1].Genres) != 0 {
		t.Errorf("türsüz kitapta genres [] olmalı (null değil), gelen %#v", got[1].Genres)
	}
	if len(got[2].Genres) != 2 {
		t.Errorf("3. kitabın 2 türü olmalı, gelen %+v", got[2].Genres)
	}
}

func TestGetBookIncludesGenres(t *testing.T) {
	store := fakeBooks{
		book:   db.Book{ID: 5, Title: "Dune"},
		genres: map[int64][]db.Genre{5: {{ID: 1, Name: "Bilim Kurgu", Slug: "bilim-kurgu"}}},
	}
	rec := do(t, newRouter(deps{books: store}), http.MethodGet, "/api/books/5", "")
	var got bookResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("istenen 200 + JSON, gelen %d %v", rec.Code, err)
	}
	if len(got.Genres) != 1 || got.Genres[0].Name != "Bilim Kurgu" {
		t.Errorf("türler yanlış: %+v", got.Genres)
	}

	failing := fakeBooks{book: db.Book{ID: 5}, genreErr: errors.New("x")}
	if rec := do(t, newRouter(deps{books: failing}), http.MethodGet, "/api/books/5", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("tür sorgusu hatası: istenen 500, gelen %d", rec.Code)
	}
}

func TestHandleSetBookGenres(t *testing.T) {
	tests := []struct {
		name       string
		url, body  string
		store      fakeBooks
		wantStatus int
		wantCode   string
	}{
		{name: "geçerli → 200 + yeni liste", url: "/api/admin/books/5/genres", body: `{"genre_ids":[1,2]}`,
			store: fakeBooks{genres: map[int64][]db.Genre{5: {{ID: 1}, {ID: 2}}}}, wantStatus: http.StatusOK},
		{name: "boş dizi → tüm türleri kaldırır (200)", url: "/api/admin/books/5/genres", body: `{"genre_ids":[]}`, wantStatus: http.StatusOK},
		{name: "genre_ids yok → 400 (gövdesiz PUT her şeyi silmesin)", url: "/api/admin/books/5/genres", body: `{}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "negatif id → 400", url: "/api/admin/books/5/genres", body: `{"genre_ids":[1,-2]}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "çok fazla tür → 400", url: "/api/admin/books/5/genres", body: `{"genre_ids":[1,2,3,4,5,6,7,8,9,10,11]}`, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "olmayan kitap → 404", url: "/api/admin/books/5/genres", body: `{"genre_ids":[1]}`, store: fakeBooks{setErr: books.ErrNotFound}, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "olmayan tür → 400 unknown_genre", url: "/api/admin/books/5/genres", body: `{"genre_ids":[999]}`, store: fakeBooks{setErr: books.ErrUnknownGenre}, wantStatus: http.StatusBadRequest, wantCode: "unknown_genre"},
		{name: "repo hatası → 500", url: "/api/admin/books/5/genres", body: `{"genre_ids":[1]}`, store: fakeBooks{setErr: errors.New("x")}, wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newRouter(deps{books: tt.store}), http.MethodPut, tt.url, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("durum kodu: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Errorf("error.code: istenen %q, gelen %q", tt.wantCode, got)
				}
			}
		})
	}
}
