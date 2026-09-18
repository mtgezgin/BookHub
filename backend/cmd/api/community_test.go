package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/comments"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/ratings"
)

type fakeComments struct {
	created db.CreateCommentRow
	rows    []db.ListCommentsRow
	total   int64
	err     error

	gotUser, gotBook int64
	gotContent       string
	deleteCalled     bool
}

func (f *fakeComments) Create(_ context.Context, user, book int64, content string) (db.CreateCommentRow, error) {
	f.gotUser, f.gotBook, f.gotContent = user, book, content
	return f.created, f.err
}
func (f *fakeComments) List(_ context.Context, book int64, _, _ int32) ([]db.ListCommentsRow, int64, error) {
	f.gotBook = book
	return f.rows, f.total, f.err
}
func (f *fakeComments) Delete(_ context.Context, admin, id int64) error {
	f.deleteCalled, f.gotUser, f.gotBook = true, admin, id
	return f.err
}

type fakeRatings struct {
	result   ratings.Result
	stats    ratings.Stats
	mine     db.GetUserRatingRow
	summary  ratings.Summary
	top      ratings.TopRated
	err      error
	gotLimit int32

	gotUser, gotBook int64
	gotScore         int
	calls            int
}

func (f *fakeRatings) Rate(_ context.Context, user, book int64, score int) (ratings.Result, error) {
	f.calls++
	f.gotUser, f.gotBook, f.gotScore = user, book, score
	return f.result, f.err
}
func (f *fakeRatings) Remove(_ context.Context, user, book int64) (ratings.Stats, error) {
	f.calls++
	f.gotUser, f.gotBook = user, book
	return f.stats, f.err
}
func (f *fakeRatings) Mine(_ context.Context, user, book int64) (db.GetUserRatingRow, error) {
	f.calls++
	f.gotUser, f.gotBook = user, book
	return f.mine, f.err
}
func (f *fakeRatings) Summary(_ context.Context, book int64) (ratings.Summary, error) {
	f.calls++
	f.gotBook = book
	return f.summary, f.err
}
func (f *fakeRatings) TopRated(_ context.Context, limit int32) (ratings.TopRated, error) {
	f.calls++
	f.gotLimit = limit
	return f.top, f.err
}

func ptr[T any](v T) *T { return &v }

func TestListComments(t *testing.T) {
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	fake := &fakeComments{total: 2, rows: []db.ListCommentsRow{
		{ID: 2, BookID: 5, UserID: ptr(int64(7)), AuthorName: ptr("Ayşe"), Content: "<b>güzel</b>", CreatedAt: now},
		{ID: 1, BookID: 5, Content: "yazarı silinmiş", CreatedAt: now}, // user_id NULL
	}}
	rec := do(t, newRouter(deps{books: fakeBooks{book: db.Book{ID: 5}}, comments: fake}), http.MethodGet, "/api/books/5/comments", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	var page commentPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if page.Total != 2 || page.Items[0].Author == nil || page.Items[0].Author.DisplayName != "Ayşe" || page.Items[1].Author != nil {
		t.Errorf("sayfa/yazar yanlış: %+v", page)
	}
	if strings.Contains(rec.Body.String(), "email") {
		t.Error("yorum listesinde e-posta olmamalı")
	}

	missing := do(t, newRouter(deps{books: fakeBooks{err: books.ErrNotFound}, comments: &fakeComments{}}), http.MethodGet, "/api/books/5/comments", "")
	if missing.Code != http.StatusNotFound {
		t.Errorf("onaysız/olmayan kitap 404 olmalı: %d", missing.Code)
	}
}

func TestCreateComment(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
	}{
		{"geçerli → 201", `{"content":"  Çok iyi  "}`, nil, http.StatusCreated},
		{"boş → 400", `{"content":"   "}`, nil, http.StatusBadRequest},
		{"çok uzun → 400", `{"content":"` + strings.Repeat("ğ", comments.MaxContentLen+1) + `"}`, nil, http.StatusBadRequest},
		{"onaysız kitap → 404", `{"content":"x"}`, books.ErrNotFound, http.StatusNotFound},
		{"beklenmeyen → 500", `{"content":"x"}`, errors.New("db"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeComments{err: tt.svcErr, created: db.CreateCommentRow{ID: 1, BookID: 5, UserID: ptr(int64(7)), AuthorName: ptr("Ali"), Content: "Çok iyi"}}
			rec := do(t, userRouter(deps{comments: fake}), http.MethodPost, "/api/books/5/comments", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status: istenen %d, gelen %d (%s)", tt.wantStatus, rec.Code, rec.Body)
			}
			if tt.wantStatus == http.StatusCreated && (fake.gotUser != 7 || fake.gotBook != 5 || fake.gotContent != "Çok iyi") {
				t.Errorf("servise kırpılmış içerik + token kullanıcısı gitmeli: %+v", fake)
			}
		})
	}

	rec := do(t, newRouter(deps{comments: &fakeComments{}, tokens: fakeVerifier{}}), http.MethodPost, "/api/books/5/comments", `{"content":"x"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token'sız yorum 401 olmalı: %d", rec.Code)
	}
}

func TestDeleteComment(t *testing.T) {
	fake := &fakeComments{}
	rec := do(t, adminRouter(deps{comments: fake}), http.MethodDelete, "/api/admin/comments/9", "")
	if rec.Code != http.StatusNoContent || !fake.deleteCalled || fake.gotUser != 1 || fake.gotBook != 9 {
		t.Errorf("204 + admin(1) ile silme bekleniyordu: %d %+v", rec.Code, fake)
	}
	missing := do(t, adminRouter(deps{comments: &fakeComments{err: comments.ErrNotFound}}), http.MethodDelete, "/api/admin/comments/9", "")
	if missing.Code != http.StatusNotFound {
		t.Errorf("olmayan yorum 404: %d", missing.Code)
	}
}

func TestRateBook(t *testing.T) {
	avg := 7.5
	tests := []struct {
		name       string
		body       string
		result     ratings.Result
		svcErr     error
		wantStatus int
		wantCalls  int
	}{
		{"ilk puan → 201", `{"score":8}`, ratings.Result{Score: 8, Created: true, Stats: ratings.Stats{Average: &avg, Count: 2}}, nil, http.StatusCreated, 1},
		{"güncelleme → 200", `{"score":8}`, ratings.Result{Score: 8}, nil, http.StatusOK, 1},
		{"score yok → 400", `{}`, ratings.Result{}, nil, http.StatusBadRequest, 0},
		{"score 0 → 400", `{"score":0}`, ratings.Result{}, nil, http.StatusBadRequest, 0},
		{"score 11 → 400", `{"score":11}`, ratings.Result{}, nil, http.StatusBadRequest, 0},
		{"ondalık → 400", `{"score":7.5}`, ratings.Result{}, nil, http.StatusBadRequest, 0},
		{"başkası adına user_id → 400", `{"score":5,"user_id":99}`, ratings.Result{}, nil, http.StatusBadRequest, 0},
		{"onaysız kitap → 404", `{"score":5}`, ratings.Result{}, books.ErrNotFound, http.StatusNotFound, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRatings{result: tt.result, err: tt.svcErr}
			rec := do(t, userRouter(deps{ratings: fake}), http.MethodPut, "/api/books/5/rating", tt.body)
			if rec.Code != tt.wantStatus || fake.calls != tt.wantCalls {
				t.Fatalf("status %d (istenen %d), çağrı %d (istenen %d): %s", rec.Code, tt.wantStatus, fake.calls, tt.wantCalls, rec.Body)
			}
			if tt.wantCalls == 1 && (fake.gotUser != 7 || fake.gotBook != 5) {
				t.Errorf("puan token kullanıcısı (7) adına verilmeli: user=%d book=%d", fake.gotUser, fake.gotBook)
			}
		})
	}
}

func TestRatingReadAndRemove(t *testing.T) {
	t.Run("puanım yok → 404 not_rated", func(t *testing.T) {
		rec := do(t, userRouter(deps{ratings: &fakeRatings{err: ratings.ErrNotRated}}), http.MethodGet, "/api/books/5/rating", "")
		if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_rated" {
			t.Errorf("404 not_rated: %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("puanı sil → 200 + istatistik", func(t *testing.T) {
		rec := do(t, userRouter(deps{ratings: &fakeRatings{stats: ratings.Stats{Count: 0}}}), http.MethodDelete, "/api/books/5/rating", "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"average_rating":null`) {
			t.Errorf("200 + null ortalama: %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("özet herkese açık, 10 satırlık dağılım", func(t *testing.T) {
		sum := ratings.Summary{Stats: ratings.Stats{Count: 3}}
		sum.Distribution[9] = 3
		rec := do(t, newRouter(deps{ratings: &fakeRatings{summary: sum}}), http.MethodGet, "/api/books/5/ratings/summary", "")
		var got summaryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("%d %v", rec.Code, err)
		}
		if len(got.Distribution) != 10 || got.Distribution[0].Score != 1 || got.Distribution[9].Score != 10 || got.Distribution[9].Votes != 3 {
			t.Errorf("dağılım: %+v", got.Distribution)
		}
	})
	t.Run("yazma uçları token ister", func(t *testing.T) {
		h := newRouter(deps{ratings: &fakeRatings{}, tokens: fakeVerifier{}})
		for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			if rec := do(t, h, m, "/api/books/5/rating", `{"score":5}`); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s token'sız 401 olmalı: %d", m, rec.Code)
			}
		}
	})
}

func TestTopRated(t *testing.T) {
	avg := 8.5
	fake := &fakeRatings{top: ratings.TopRated{M: 10, C: 7.123456, Books: []ratings.TopRatedBook{
		{Book: db.Book{ID: 2, Title: "Çok oylu", AverageRating: &avg, RatingCount: 2000}, WeightedScore: 8.49253731},
	}}}
	rec := do(t, newRouter(deps{books: fakeBooks{}, ratings: fake}), http.MethodGet, "/api/books/top-rated", "")
	if rec.Code != http.StatusOK || fake.gotLimit != defaultTopRatedLimit {
		t.Fatalf("status %d, limit %d (%s)", rec.Code, fake.gotLimit, rec.Body)
	}
	var got topRatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if got.M != 10 || got.C != 7.1235 || len(got.Items) != 1 || got.Items[0].WeightedScore != 8.4925 || got.Items[0].RatingCount != 2000 {
		t.Errorf("cevap (4 haneye yuvarlanmış): %+v", got)
	}

	for _, bad := range []string{"0", "51", "abc"} {
		f := &fakeRatings{}
		if rec := do(t, newRouter(deps{books: fakeBooks{}, ratings: f}), http.MethodGet, "/api/books/top-rated?limit="+bad, ""); rec.Code != http.StatusBadRequest || f.calls != 0 {
			t.Errorf("limit=%s → 400 bekleniyordu: %d", bad, rec.Code)
		}
	}
}

// TestMyRatingSuccess: puanı olan kullanıcı 200 + servis satırındaki score/created_at/updated_at alır.
func TestMyRatingSuccess(t *testing.T) {
	created := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	updated := created.Add(90 * time.Minute)
	fake := &fakeRatings{mine: db.GetUserRatingRow{Score: 8, CreatedAt: created, UpdatedAt: updated}}
	rec := do(t, userRouter(deps{ratings: fake}), http.MethodGet, "/api/books/5/rating", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	var got myRatingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if got.Score != 8 || !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(updated) {
		t.Errorf("servis satırı kopyalanmalı: %+v", got)
	}
	if fake.gotUser != 7 || fake.gotBook != 5 {
		t.Errorf("Mine token kullanıcısı (7) ve path id (5) ile çağrılmalı: user=%d book=%d", fake.gotUser, fake.gotBook)
	}
}

func TestTopRatedValidLimitReachesService(t *testing.T) {
	fake := &fakeRatings{}
	rec := do(t, newRouter(deps{books: fakeBooks{}, ratings: fake}), http.MethodGet, "/api/books/top-rated?limit=50", "")
	if rec.Code != http.StatusOK || fake.gotLimit != int32(ratings.MaxTopRated) {
		t.Errorf("limit=50 servise %d olarak gitmeli: status %d, gelen limit %d", ratings.MaxTopRated, rec.Code, fake.gotLimit)
	}
}

func TestTopRatedServiceErrorIs500WithoutDetail(t *testing.T) {
	fake := &fakeRatings{err: errors.New("pq: connection refused to 10.0.0.9")}
	rec := do(t, newRouter(deps{books: fakeBooks{}, ratings: fake}), http.MethodGet, "/api/books/top-rated", "")
	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "internal" {
		t.Fatalf("500 internal bekleniyordu: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("iç hata detayı cevaba sızmamalı: %s", rec.Body)
	}
}

func TestTopRatedGenresErrorIs500(t *testing.T) {
	fake := &fakeRatings{top: ratings.TopRated{Books: []ratings.TopRatedBook{{Book: db.Book{ID: 2}}}}}
	rec := do(t, newRouter(deps{books: fakeBooks{genreErr: errors.New("db")}, ratings: fake}), http.MethodGet, "/api/books/top-rated", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("GenresForBooks hatası 500 olmalı: %d %s", rec.Code, rec.Body)
	}
}

func TestListCommentsServiceErrorIs500(t *testing.T) {
	fake := &fakeComments{err: errors.New("db")}
	rec := do(t, newRouter(deps{books: fakeBooks{book: db.Book{ID: 5}}, comments: fake}), http.MethodGet, "/api/books/5/comments", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("svc.List hatası 500 olmalı: %d %s", rec.Code, rec.Body)
	}
}

func TestDeleteCommentUnexpectedErrorIs500(t *testing.T) {
	rec := do(t, adminRouter(deps{comments: &fakeComments{err: errors.New("db")}}), http.MethodDelete, "/api/admin/comments/9", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ErrNotFound dışı hata 500 olmalı (404 değil): %d %s", rec.Code, rec.Body)
	}
}

// TestWriteRatingError: handler skoru önceden doğruladığı için ErrInvalidScore ve bilinmeyen
// hata dalları haritalayıcıya doğrudan verilerek test edilir.
func TestWriteRatingError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"geçersiz skor → 400", ratings.ErrInvalidScore, http.StatusBadRequest, "validation_error"},
		{"bilinmeyen (DB kapalı) → 500", errors.New("db down"), http.StatusInternalServerError, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { writeRatingError(w, "Test", tt.err) })
			rec := do(t, h, http.MethodGet, "/", "")
			if rec.Code != tt.wantStatus || errorCode(t, rec) != tt.wantCode {
				t.Errorf("istenen %d %s, gelen %d %s", tt.wantStatus, tt.wantCode, rec.Code, rec.Body)
			}
		})
	}
}
