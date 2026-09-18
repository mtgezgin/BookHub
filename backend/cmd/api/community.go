package main

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/comments"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/ratings"
)

// commentService / ratingService: yorum ve puan uçlarının servislerden beklediği davranışlar.
type commentService interface {
	Create(ctx context.Context, userID, bookID int64, content string) (db.CreateCommentRow, error)
	List(ctx context.Context, bookID int64, limit, offset int32) ([]db.ListCommentsRow, int64, error)
	Delete(ctx context.Context, adminID, commentID int64) error
}

type ratingService interface {
	Rate(ctx context.Context, userID, bookID int64, score int) (ratings.Result, error)
	Remove(ctx context.Context, userID, bookID int64) (ratings.Stats, error)
	Mine(ctx context.Context, userID, bookID int64) (db.GetUserRatingRow, error)
	Summary(ctx context.Context, bookID int64) (ratings.Summary, error)
	TopRated(ctx context.Context, limit int32) (ratings.TopRated, error)
}

// commentAuthor: yazarın public bilgisi. Hesabı silinmiş yazarın yorumunda author = null.
// E-posta BİLEREK yok — yorum listesi herkese açık, e-posta kişisel veri.
type commentAuthor struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
}

type commentResponse struct {
	ID        int64          `json:"id"`
	BookID    int64          `json:"book_id"`
	Author    *commentAuthor `json:"author"`
	Content   string         `json:"content"`
	CreatedAt time.Time      `json:"created_at"`
}

func toCommentResponse(id, bookID int64, userID *int64, authorName *string, content string, createdAt time.Time) commentResponse {
	c := commentResponse{ID: id, BookID: bookID, Content: content, CreatedAt: createdAt}
	if userID != nil && authorName != nil {
		c.Author = &commentAuthor{ID: *userID, DisplayName: *authorName}
	}
	return c
}

type commentPage struct {
	Items  []commentResponse `json:"items"`
	Total  int64             `json:"total"`
	Limit  int32             `json:"limit"`
	Offset int32             `json:"offset"`
}

// handleListComments, GET /api/books/{id}/comments — herkese açık; yalnız ONAYLI kitabın
// görünür yorumları. Onaysız kitap 404 (görünürlük kuralı public kitap ucuyla aynı).
func handleListComments(store bookStore, svc commentService) http.HandlerFunc {
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
		if _, err := store.Get(r.Context(), id); err != nil {
			writeBookWriteError(w, "GetBook", err)
			return
		}
		rows, total, err := svc.List(r.Context(), id, limit, offset)
		if err != nil {
			writeInternalError(w, "ListComments", err)
			return
		}
		items := make([]commentResponse, 0, len(rows))
		for _, c := range rows {
			items = append(items, toCommentResponse(c.ID, c.BookID, c.UserID, c.AuthorName, c.Content, c.CreatedAt))
		}
		writeJSON(w, http.StatusOK, commentPage{Items: items, Total: total, Limit: limit, Offset: offset})
	}
}

type createCommentRequest struct {
	Content string `json:"content"`
}

// handleCreateComment, POST /api/books/{id}/comments (giriş yapmış kullanıcı) → 201.
func handleCreateComment(svc commentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req createCommentRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		req.Content = strings.TrimSpace(req.Content)
		var v validator
		v.required("content", req.Content)
		v.maxLen("content", req.Content, comments.MaxContentLen)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		c, err := svc.Create(r.Context(), user, id, req.Content)
		if err != nil {
			writeBookWriteError(w, "CreateComment", err)
			return
		}
		writeJSON(w, http.StatusCreated, toCommentResponse(c.ID, c.BookID, c.UserID, c.AuthorName, c.Content, c.CreatedAt))
	}
}

// handleDeleteComment, DELETE /api/admin/comments/{id} → 204 (yumuşak silme, yalnız admin).
func handleDeleteComment(svc commentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		switch err := svc.Delete(r.Context(), admin, id); {
		case errors.Is(err, comments.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "yorum bulunamadı")
		case err != nil:
			writeInternalError(w, "DeleteComment", err)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// statsResponse: kitabın türetilmiş puan istatistikleri. average_rating null = henüz puan yok.
type statsResponse struct {
	AverageRating *float64 `json:"average_rating"`
	RatingCount   int32    `json:"rating_count"`
}

type rateResponse struct {
	Score int16 `json:"score"`
	statsResponse
}

// rateRequest: score pointer — gönderilmedi (nil) ile geçersiz (0) ayrı hatalar.
type rateRequest struct {
	Score *int `json:"score"`
}

// writeRatingError, puan uçlarının hata haritası.
func writeRatingError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, books.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "kitap bulunamadı")
	case errors.Is(err, ratings.ErrNotRated):
		writeError(w, http.StatusNotFound, "not_rated", "bu kitaba henüz puan vermedin")
	case errors.Is(err, ratings.ErrInvalidScore):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	default:
		writeInternalError(w, op, err)
	}
}

// handleRateBook, PUT /api/books/{id}/rating {"score":1-10} — "benim bu kitaba puanım".
//
// NEDEN PUT (brief'teki POST + PATCH yerine): istemci "puanım şu olsun" diyor; ilk kez mi
// veriyor yoksa güncelliyor mu umurunda değil. PUT tam bu: idempotent "şu değere ayarla".
// DB tarafındaki INSERT ... ON CONFLICT DO UPDATE'in HTTP karşılığı. Yeni puan 201, güncelleme 200.
func handleRateBook(svc ratingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req rateRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		var v validator
		v.check(req.Score != nil, "score zorunlu")
		if req.Score != nil {
			v.check(*req.Score >= ratings.MinScore && *req.Score <= ratings.MaxScore,
				"score %d-%d arasında olmalı", ratings.MinScore, ratings.MaxScore)
		}
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		res, err := svc.Rate(r.Context(), user, id, *req.Score)
		if err != nil {
			writeRatingError(w, "RateBook", err)
			return
		}
		status := http.StatusOK
		if res.Created {
			status = http.StatusCreated
		}
		writeJSON(w, status, rateResponse{Score: res.Score, statsResponse: statsResponse{res.Stats.Average, res.Stats.Count}})
	}
}

// handleRemoveRating, DELETE /api/books/{id}/rating → 200 + güncel istatistik.
func handleRemoveRating(svc ratingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		stats, err := svc.Remove(r.Context(), user, id)
		if err != nil {
			writeRatingError(w, "RemoveRating", err)
			return
		}
		writeJSON(w, http.StatusOK, statsResponse{stats.Average, stats.Count})
	}
}

type myRatingResponse struct {
	Score     int16     `json:"score"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// handleMyRating, GET /api/books/{id}/rating → çağıranın puanı (yoksa 404 not_rated).
func handleMyRating(svc ratingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		m, err := svc.Mine(r.Context(), user, id)
		if err != nil {
			writeRatingError(w, "MyRating", err)
			return
		}
		writeJSON(w, http.StatusOK, myRatingResponse{Score: m.Score, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt})
	}
}

type scoreVotes struct {
	Score int   `json:"score"`
	Votes int32 `json:"votes"`
}

type summaryResponse struct {
	statsResponse
	Distribution []scoreVotes `json:"distribution"`
}

// handleRatingSummary, GET /api/books/{id}/ratings/summary — herkese açık: ortalama, sayı
// ve 1..10 dağılımı (her puan için bir satır, sıfır oy dahil — frontend histogramı kolay çizsin).
func handleRatingSummary(svc ratingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		sum, err := svc.Summary(r.Context(), id)
		if err != nil {
			writeRatingError(w, "RatingSummary", err)
			return
		}
		dist := make([]scoreVotes, 0, len(sum.Distribution))
		for i, votes := range sum.Distribution {
			dist = append(dist, scoreVotes{Score: i + ratings.MinScore, Votes: votes})
		}
		writeJSON(w, http.StatusOK, summaryResponse{statsResponse: statsResponse{sum.Average, sum.Count}, Distribution: dist})
	}
}

const (
	defaultTopRatedLimit = 10
	maxTopRatedLimit     = 50
)

type topRatedItem struct {
	bookResponse
	WeightedScore float64 `json:"weighted_score"`
}

// topRatedResponse, sıralamayı üreten parametreleri de döner (m, c): "neden bu kitap
// birinci?" sorusu cevaplanabilsin — sıralama bir kara kutu olmasın.
type topRatedResponse struct {
	Items []topRatedItem `json:"items"`
	M     int64          `json:"m"`
	C     float64        `json:"c"`
}

func round4(x float64) float64 { return math.Round(x*10000) / 10000 }

// handleTopRated, GET /api/books/top-rated?limit= — ağırlıklı (Bayes) skora göre, basit
// ortalamaya göre DEĞİL: 1 oyla 10 alan kitap, 200 oyla 8.7 alanın önüne geçemez.
func handleTopRated(store bookStore, svc ratingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := int32(defaultTopRatedLimit)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > maxTopRatedLimit {
				writeError(w, http.StatusBadRequest, "validation_error", "limit 1-50 arasında bir sayı olmalı")
				return
			}
			limit = int32(n)
		}

		top, err := svc.TopRated(r.Context(), limit)
		if err != nil {
			writeInternalError(w, "TopRated", err)
			return
		}
		ids := make([]int64, 0, len(top.Books))
		for _, b := range top.Books {
			ids = append(ids, b.Book.ID)
		}
		genresByBook, err := store.GenresForBooks(r.Context(), ids)
		if err != nil {
			writeInternalError(w, "GenresForBooks", err)
			return
		}
		items := make([]topRatedItem, 0, len(top.Books))
		for _, b := range top.Books {
			items = append(items, topRatedItem{bookResponse: toBookResponse(b.Book, genresByBook[b.Book.ID]), WeightedScore: round4(b.WeightedScore)})
		}
		writeJSON(w, http.StatusOK, topRatedResponse{Items: items, M: top.M, C: round4(top.C)})
	}
}
