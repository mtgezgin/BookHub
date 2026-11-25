package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/library"
)

// libraryStore, kişisel kitaplığın OKUMA uçlarının ihtiyacı kadar (tüketici tarafı interface):
// handler'lar internal/library'yi değil bu sözleşmeyi tanır, testler sahte depoyla koşar. Yazımlar
// (eski PUT/DELETE dahil) shelfService'ten geçer (Faz 17A-1, shelf.go).
type libraryStore interface {
	Statuses(ctx context.Context, userID int64, bookIDs []int64) (map[int64]string, error)
	List(ctx context.Context, userID int64, f library.ListFilter) ([]db.ListLibraryRow, int64, error)
}

// maxStatusIDs, tek istekte durum sorulabilecek kitap sayısı (sayfa tavanıyla aynı).
const maxStatusIDs = 100

// libraryEntryRequest, PUT gövdesinin KABUL ETTİĞİ tek alan; user_id gibi bir alan gelirse
// readJSON 400 döner (mass assignment yok — kullanıcı her zaman token'dan).
type libraryEntryRequest struct {
	Status string `json:"status"`
}

type libraryEntryResponse struct {
	BookID    int64     `json:"book_id"`
	Status    string    `json:"status"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type libraryItemResponse struct {
	Book      bookResponse `json:"book"`
	Status    string       `json:"status"`
	AddedAt   time.Time    `json:"added_at"`
	MyRating  *int         `json:"my_rating"` // null = puanım yok
	InCatalog bool         `json:"in_catalog"`
	// Kartlardaki ilerleme (Faz 17A-1): yalnız READING'de dolu, başka durumda null (0017'den beri
	// veritabanı kısıtı). Unit null = ilerleme henüz girilmedi.
	Unit       *string `json:"unit"`
	Progress   *int32  `json:"progress"`
	TotalPages *int32  `json:"total_pages"`
}

type libraryPage struct {
	Items  []libraryItemResponse `json:"items"`
	Total  int64                 `json:"total"`
	Limit  int32                 `json:"limit"`
	Offset int32                 `json:"offset"`
}

// libraryStatusesResponse: int64 anahtarlı map, JSON'da "54": "READ" biçiminde kodlanır.
type libraryStatusesResponse struct {
	Statuses map[int64]string `json:"statuses"`
}

// handlePutLibraryEntry, PUT /api/me/library/{id} {"status": …} — Faz 15'in "şu duruma ayarla" ucu.
// Faz 17A-1'den beri kendi yazımı yok: durum bir komuta çevrilir (library.CommandForStatus) ve raf
// servisinden geçer. Yayın penceresindeki eski arayüz böylece okumayı da doğru kapatır (READ, süren
// okumayı günlüğe yazar). Cevap Faz 15'teki şekliyle aynı.
func handlePutLibraryEntry(svc shelfService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req libraryEntryRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		c, valid := library.CommandForStatus(req.Status)
		if !valid {
			writeError(w, http.StatusBadRequest, "validation_error", "status yalnız WANT_TO_READ, READING, READ ya da ABANDONED olabilir")
			return
		}
		s, err := svc.Do(r.Context(), user, id, library.CommandInput{Command: c})
		if err != nil {
			writeShelfError(w, "PutLibraryEntry", errBookNotFound, err)
			return
		}
		if s.Entry == nil { // dört komut da bir satır bırakır; buraya düşmek servis hatasıdır
			writeInternalError(w, "PutLibraryEntry", errors.New("komuttan sonra kitaplık satırı yok"))
			return
		}
		e := s.Entry
		writeJSON(w, http.StatusOK, libraryEntryResponse{BookID: e.BookID, Status: e.Status, AddedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt})
	}
}

// handleRemoveLibraryEntry, DELETE /api/me/library/{id} — kayıt yoksa da 204 (idempotent). Raf
// servisinin "çıkar" komutu: okurken ilerleme varsa önce günlüğe "yarım bırakıldı" yazılır. Güncel rafı
// isteyen arayüz POST …/shelf/remove kullanır; bu uç Faz 15'in sözleşmesiyle (gövdesiz 204) kalır.
func handleRemoveLibraryEntry(svc shelfService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		if _, err := svc.Do(r.Context(), user, id, library.CommandInput{Command: library.CmdRemove}); err != nil {
			writeShelfError(w, "RemoveLibraryEntry", errBookNotFound, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// parseBookIDs, ?book_ids=1,2,3 değerini okur: pozitif tam sayılar, virgülle ayrılmış, en çok
// maxStatusIDs; tekrarlar sırayı bozmadan ayıklanır. Boş değer = boş liste.
func parseBookIDs(raw string) ([]int64, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxStatusIDs {
		return nil, badRequest("invalid_id", "book_ids en çok "+strconv.Itoa(maxStatusIDs)+" kimlik içerebilir")
	}
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]bool, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n < 1 {
			return nil, badRequest("invalid_id", "book_ids virgülle ayrılmış pozitif tam sayılar olmalı")
		}
		if !seen[n] {
			seen[n] = true
			ids = append(ids, n)
		}
	}
	return ids, nil
}

// handleLibraryStatuses, GET /api/me/library/statuses?book_ids=… — kartlardaki rozetler için
// sayfadaki kitapların durumu tek istekte. Katalog uçları kişisel veri taşımaz; o veri buradan gelir.
func handleLibraryStatuses(ls libraryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		ids, err := parseBookIDs(r.URL.Query().Get("book_ids"))
		if err != nil {
			writeRequestError(w, err)
			return
		}
		statuses := map[int64]string{}
		if len(ids) > 0 {
			m, err := ls.Statuses(r.Context(), user, ids)
			if err != nil {
				writeInternalError(w, "LibraryStatuses", err)
				return
			}
			for k, v := range m {
				statuses[k] = v
			}
		}
		writeJSON(w, http.StatusOK, libraryStatusesResponse{Statuses: statuses})
	}
}

// parseLibraryFilter, Kitaplığım'ın ?status=&q=&genre=&rated=&sort=&limit=&offset= parametrelerini
// sınırda doğrular: geçersiz değer sorguya ulaşıp sessizce boş liste döndürmez, 400 olur.
func parseLibraryFilter(r *http.Request) (library.ListFilter, error) {
	q := r.URL.Query()
	f := library.ListFilter{
		Status: q.Get("status"),
		Query:  strings.TrimSpace(q.Get("q")),
		Genre:  strings.TrimSpace(q.Get("genre")),
		Rated:  q.Get("rated"),
		Sort:   q.Get("sort"),
	}
	var v validator
	v.check(f.Status == "" || library.ValidStatus(f.Status), "status yalnız WANT_TO_READ, READING, READ ya da ABANDONED olabilir")
	v.match("genre", f.Genre, slugPattern, "tür slug'ı; ör. bilim-kurgu")
	v.maxLen("q", f.Query, maxQueryLen)
	v.text("q", f.Query)
	switch f.Rated {
	case "", library.RatedYes, library.RatedNo:
	default:
		v.check(false, "rated yalnız yes ya da no olabilir")
	}
	switch f.Sort {
	case "", library.SortAdded, library.SortTitle, library.SortMyRating:
	default:
		v.check(false, "sort yalnız added, title ya da my_rating olabilir")
	}
	f.Limit, f.Offset = parsePage(q, &v)
	return f, v.err()
}

// handleListLibrary, GET /api/me/library — kullanıcının kitaplığı, sayfalı. Türler katalogdaki
// gibi TOPLU yüklenir (tek sorgu, N+1 yok).
func handleListLibrary(ls libraryStore, bs bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := actorFrom(w, r)
		if !ok {
			return
		}
		filter, err := parseLibraryFilter(r)
		if err != nil {
			writeRequestError(w, err)
			return
		}
		rows, total, err := ls.List(r.Context(), user, filter)
		if err != nil {
			writeInternalError(w, "ListLibrary", err)
			return
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.BookView.ID)
		}
		genresByBook, err := bs.GenresForBooks(r.Context(), ids)
		if err != nil {
			writeInternalError(w, "GenresForBooks", err)
			return
		}
		items := make([]libraryItemResponse, 0, len(rows))
		for _, row := range rows {
			var mine *int
			if row.MyRating != nil {
				score := int(*row.MyRating)
				mine = &score
			}
			item := libraryItemResponse{
				Book:      toBookResponse(row.BookView, genresByBook[row.BookView.ID]),
				Status:    row.Status,
				AddedAt:   row.AddedAt,
				MyRating:  mine,
				InCatalog: row.InCatalog,
			}
			if row.Status == library.StatusReading {
				item.Unit, item.Progress, item.TotalPages = row.Unit, row.Progress, row.TotalPages
			}
			items = append(items, item)
		}
		writeJSON(w, http.StatusOK, libraryPage{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset})
	}
}
