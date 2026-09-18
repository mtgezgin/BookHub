package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
	"github.com/mtgezgin/bookhub/backend/internal/slug"
)

// bookStore, kitap handler'larının repository'den ihtiyaç duyduğu davranışlar.
// Interface TÜKETEN tarafta tanımlı (GO_CONVENTIONS §3): *books.Repository bunu
// "implements" demeden sağlar (yapısal tipleme); testlerde sahte takılabilir.
// Adı "Getter" değil "Store": tür bağlama (SetGenres) gibi yazma da içeriyor — Go'da
// "-er" soneki tek metotlu interface konvansiyonudur. Kitap OLUŞTURMA/moderasyon ise
// Faz 4'ten beri moderationService'te (her yazım bir denetim kaydıyla birlikte).
type bookStore interface {
	Get(ctx context.Context, id int64) (db.Book, error)
	List(ctx context.Context, f books.ListFilter) ([]db.Book, int64, error)
	GenresForBook(ctx context.Context, bookID int64) ([]db.Genre, error)
	GenresForBooks(ctx context.Context, bookIDs []int64) (map[int64][]db.Genre, error)
	SetGenres(ctx context.Context, bookID int64, genreIDs []int64) error
}

// bookResponse, kitabın HERKESE AÇIK temsili (response DTO).
//
// db.Book'u doğrudan serileştirmiyoruz: o struct sqlc'nin TABLODAN ürettiği şeydir ve
// şemaya eklenen her kolon (ör. Faz 4'ün moderasyon notu) kimse karar vermeden public
// API'ye sızardı. createBookRequest'in giriş tarafında yaptığını bu, çıkış tarafında yapar:
// istemciye ne gideceğine BİZ karar veririz. `status` bilerek yok — public uçlar zaten
// yalnızca APPROVED döndürür, alan bilgi taşımaz.
type bookResponse struct {
	ID              int64           `json:"id"`
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	Author          string          `json:"author"`
	Description     string          `json:"description"`
	CoverURL        string          `json:"cover_url"`
	PublicationYear *int32          `json:"publication_year"`
	PageCount       *int32          `json:"page_count"`
	Language        string          `json:"language"`
	Genres          []genreResponse `json:"genres"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// toBookResponse, kitabı türleriyle birlikte DTO'ya çevirir. Türsüz kitapta bile
// genres "[]" olur (null değil) — istemci her zaman diziyle çalışsın.
func toBookResponse(b db.Book, genreList []db.Genre) bookResponse {
	return bookResponse{
		ID:              b.ID,
		Title:           b.Title,
		Slug:            b.Slug,
		Author:          b.Author,
		Description:     b.Description,
		CoverURL:        b.CoverUrl,
		PublicationYear: b.PublicationYear,
		PageCount:       b.PageCount,
		Language:        b.Language,
		Genres:          toGenreResponses(genreList),
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
}

func toGenreResponses(list []db.Genre) []genreResponse {
	out := make([]genreResponse, 0, len(list))
	for _, g := range list {
		out = append(out, toGenreResponse(g))
	}
	return out
}

// handleGetBook, GET /api/books/{id} handler'ını ÜRETİR (closure ile DI):
// repository dışarıdan gelir, handler onu kapanışında (closure) taşır.
func handleGetBook(store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) URL'deki {id}'yi al ve doğrula. Kullanıcı girdisi = güvenilmez veri;
		//    sayı değilse/pozitif değilse sorguya hiç gitmeden 400 dön.
		id, ok := parseID(w, r)
		if !ok {
			return
		}

		// 2) Repository'den çek — r.Context() İLETİLİR: istemci bağlantıyı
		//    keserse sorgu da iptal olur.
		book, err := store.Get(r.Context(), id)

		// 3) Hata HARİTALAMA: domain hatası → HTTP dili. Sıra önemli:
		//    önce bilinen (beklenen) durumlar, en sonda bilinmeyenler.
		switch {
		case errors.Is(err, books.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "kitap bulunamadı")
			return
		case err != nil:
			writeInternalError(w, "GetBook", err)
			return
		}

		// 4) Türleri ikinci bir sorguyla ekle (çoka-çok: ara tablo üzerinden JOIN).
		genreList, err := store.GenresForBook(r.Context(), book.ID)
		if err != nil {
			writeInternalError(w, "GenresForBook", err)
			return
		}
		writeJSON(w, http.StatusOK, toBookResponse(book, genreList))
	}
}

// Sayfalama sınırları: limit tavanı tek istekte çekilebilecek veriyi sınırlar; offset
// tavanı ise pahalı derin taramayı (OFFSET 1.000.000 = milyon satır okuyup atmak) keser.
const (
	defaultPageLimit = 20
	maxPageLimit     = 100
	maxPageOffset    = 10_000
	maxQueryLen      = 100
)

// bookPage, sayfalanmış liste cevabı. Düz dizi yerine zarf: istemci toplam sonuç sayısını
// ve hangi dilimi gördüğünü bilir ("57 sonuçtan 1-20").
type bookPage struct {
	Items  []bookResponse `json:"items"`
	Total  int64          `json:"total"`
	Limit  int32          `json:"limit"`
	Offset int32          `json:"offset"`
}

// parseListFilter, ?genre=&q=&limit=&offset= parametrelerini doğrulayıp filtreye çevirir.
func parseListFilter(r *http.Request) (books.ListFilter, error) {
	q := r.URL.Query()
	f := books.ListFilter{
		Genre: strings.TrimSpace(q.Get("genre")),
		Query: strings.TrimSpace(q.Get("q")),
	}

	var v validator
	v.match("genre", f.Genre, slugPattern, "tür slug'ı; ör. bilim-kurgu")
	v.maxLen("q", f.Query, maxQueryLen)
	f.Limit, f.Offset = parsePage(q, &v)
	return f, v.err()
}

// parsePage, ?limit=&offset= okur (varsayılan: 20, 0). Public liste ve admin listesi
// aynı kuralları kullansın diye tek yerde.
func parsePage(q url.Values, v *validator) (limit, offset int32) {
	limit = defaultPageLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		v.check(err == nil && n >= 1 && n <= maxPageLimit, "limit 1-%d arasında bir sayı olmalı", maxPageLimit)
		limit = int32(n)
	}
	if raw := q.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		v.check(err == nil && n >= 0 && n <= maxPageOffset, "offset 0-%d arasında bir sayı olmalı", maxPageOffset)
		offset = int32(n)
	}
	return limit, offset
}

// handleListBooks, GET /api/books — onaylı kitapları en yeniden eskiye, sayfalı listeler.
// Opsiyonel: ?genre=<slug> (türe göre), ?q=<metin> (başlık/yazar), ?limit=&offset=.
// Görünürlük sınırı (yalnızca APPROVED) burada değil SQL'de yaşıyor (ListBooks sorgusu).
func handleListBooks(store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parseListFilter(r)
		if err != nil {
			writeRequestError(w, err)
			return
		}

		list, total, err := store.List(r.Context(), filter)
		if err != nil {
			writeInternalError(w, "ListBooks", err)
			return
		}

		// Türleri TOPLU yükle: tek sorgu, kitap sayısından bağımsız (N+1'den kaçınma).
		ids := make([]int64, 0, len(list))
		for _, b := range list {
			ids = append(ids, b.ID)
		}
		genresByBook, err := store.GenresForBooks(r.Context(), ids)
		if err != nil {
			writeInternalError(w, "GenresForBooks", err)
			return
		}

		// make(..., 0, n): sayfa BOŞ olsa bile JSON'da "null" değil "[]" çıkar.
		items := make([]bookResponse, 0, len(list))
		for _, b := range list {
			items = append(items, toBookResponse(b, genresByBook[b.ID]))
		}
		writeJSON(w, http.StatusOK, bookPage{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset})
	}
}

// createBookRequest, POST /api/admin/books'un İSTEMCİDEN KABUL ETTİĞİ alanlar.
//
// db.CreateBookParams'a doğrudan decode ETMİYORUZ: o struct'ta `Status` alanı var ve
// istemci `{"status":"APPROVED"}` yollayarak kendi kitabını yayına alabilirdi
// (MASS-ASSIGNMENT — canlı olarak gösterip kapattık). İki savunma hattı üst üste:
//  1. Bu struct'ta `status` alanının OLMAMASI — kabul edilen yüzeyi biz tanımlıyoruz.
//  2. readJSON'daki DisallowUnknownFields — tanımadığımız alan gelirse istek 400 olur.
type createBookRequest struct {
	Title           string `json:"title"`
	Slug            string `json:"slug"`
	Author          string `json:"author"`
	Description     string `json:"description"`
	CoverURL        string `json:"cover_url"`
	PublicationYear *int32 `json:"publication_year"`
	PageCount       *int32 `json:"page_count"`
	Language        string `json:"language"`
	// status YOK — bilerek. Statüyü sunucu belirler.
}

// defaultLanguage, language gönderilmediğinde kullanılır.
//
// NEDEN UYGULAMADA: migration'da `language ... DEFAULT 'tr'` var ama o DEFAULT devreye
// GİRMEZ — sqlc parametreyi her zaman açıkça yazdığı için boş string gider ve Postgres
// "değer verildi" sayar. Parametreli INSERT'te varsayılanı uygulama düşünmek zorunda.
const defaultLanguage = "tr"

// minPublicationYear, matbaanın icadı civarı — bundan eskisi veri hatasıdır.
const minPublicationYear = 1450

// normalize, gövdeyi temizler: boşluk kırpar, varsayılan dili uygular.
// Pointer receiver: alanları DEĞİŞTİRİYOR (config.validate() ile aynı desen).
func (req *createBookRequest) normalize() {
	req.Title = strings.TrimSpace(req.Title)
	req.Slug = strings.TrimSpace(req.Slug)
	req.Author = strings.TrimSpace(req.Author)
	req.Description = strings.TrimSpace(req.Description)
	req.CoverURL = strings.TrimSpace(req.CoverURL)
	req.Language = strings.TrimSpace(req.Language)
	if req.Language == "" {
		req.Language = defaultLanguage
	}
	// Slug gönderilmediyse başlıktan türet: kullanıcıdan (öneri formunda) URL kimliği
	// uydurmasını beklemek kötü deneyim. Türetilen slug da aynı doğrulamadan geçer.
	if req.Slug == "" {
		// Türetilen slug kesilir: 300 karakterlik geçerli bir başlık, istemcinin hiç
		// göndermediği bir alan yüzünden "slug çok uzun" hatası almamalı.
		req.Slug = slug.Truncate(slug.Make(req.Title), maxSlugLen)
	}
}

// validate, normalize edilmiş gövdeyi denetler; TÜM problemleri tek hatada toplar.
func (req createBookRequest) validate() error {
	var v validator
	checkTitle(&v, req.Title)
	checkSlug(&v, req.Slug) // title'dan türetilemediyse (ör. yalnız emoji) boş kalır
	checkAuthor(&v, req.Author)
	checkDescription(&v, req.Description)
	checkCoverURL(&v, req.CoverURL)
	checkLanguage(&v, req.Language)
	checkPublicationYear(&v, req.PublicationYear)
	checkPageCount(&v, req.PageCount)
	return v.err()
}

// toParams, doğrulanmış gövdeyi sorgu parametrelerine ELLE eşler. Angarya gibi görünür
// ama asıl korumadır: yeni bir DB kolonu otomatik olarak istemciye AÇILMAZ, buraya
// bilinçli bir satır eklemek gerekir. Status/SubmittedBy BİLEREK boş: onları servis
// belirler (admin → APPROVED, öneri → PENDING).
func (req createBookRequest) toParams() db.CreateBookParams {
	return db.CreateBookParams{
		Title:           req.Title,
		Slug:            req.Slug,
		Author:          req.Author,
		Description:     req.Description,
		CoverUrl:        req.CoverURL,
		PublicationYear: req.PublicationYear,
		PageCount:       req.PageCount,
		Language:        req.Language,
	}
}

// Kitap alanlarının kuralları — create (tüm alanlar) ve PATCH (yalnız gönderilenler)
// AYNI fonksiyonları çağırır: iki uçta kuralların ayrışması (biri 300, öteki 500
// karakter) böyle imkânsız olur.
func checkTitle(v *validator, s string) {
	v.required("title", s)
	v.maxLen("title", s, maxTitleLen)
}

func checkSlug(v *validator, s string) {
	v.required("slug", s)
	v.maxLen("slug", s, maxSlugLen)
	v.match("slug", s, slugPattern, "küçük harf, rakam ve tek tire; ör. clean-code")
}

func checkAuthor(v *validator, s string) {
	v.required("author", s)
	v.maxLen("author", s, maxAuthorLen)
}

func checkDescription(v *validator, s string) { v.maxLen("description", s, maxDescriptionLen) }

func checkCoverURL(v *validator, s string) {
	v.maxLen("cover_url", s, maxURLLen)
	v.httpURL("cover_url", s)
}

func checkLanguage(v *validator, s string) {
	v.required("language", s)
	v.match("language", s, languagePattern, "ör. tr, en, en-US")
}

// Opsiyonel sayılar: nil = "bilinmiyor" (geçerli), ama VERİLDİYSE anlamlı olmalı.
// Pointer kullanmamızın sebebi tam bu ayrım — 0 ile "gönderilmedi" ayrı şeyler.
func checkPublicationYear(v *validator, y *int32) {
	if y == nil {
		return
	}
	maxYear := int32(time.Now().Year() + 1) // gelecek yıl = ön sipariş, makul
	v.check(*y >= minPublicationYear && *y <= maxYear,
		"publication_year %d-%d aralığında olmalı", minPublicationYear, maxYear)
}

func checkPageCount(v *validator, n *int32) {
	if n != nil {
		v.check(*n >= 1, "page_count pozitif olmalı")
	}
}

// readBookRequest, create ve öneri uçlarının ortak gövde okuma + doğrulama adımı.
func readBookRequest(w http.ResponseWriter, r *http.Request) (createBookRequest, bool) {
	var req createBookRequest
	if err := readJSON(w, r, &req); err != nil {
		writeRequestError(w, err)
		return req, false
	}
	req.normalize()
	if err := req.validate(); err != nil {
		writeRequestError(w, err)
		return req, false
	}
	return req, true
}

// handleCreateBook, POST /api/admin/books — admin doğrudan (onaylı) kitap ekler.
// Rota /api/admin altında: yetkilendirme middleware olarak routing tablosunda durur,
// handler gövdesinde "if user.IsAdmin" olarak DEĞİL.
func handleCreateBook(svc moderationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFrom(w, r)
		if !ok {
			return
		}
		req, ok := readBookRequest(w, r)
		if !ok {
			return
		}

		// Statüyü (APPROVED) ve ekleyeni SERVİS belirler; kayıt ile denetim satırı aynı tx.
		book, err := svc.CreateApproved(r.Context(), actor, req.toParams())
		if err != nil {
			writeBookWriteError(w, "CreateBook", err)
			return
		}
		// 201 Created: "istek işlendi VE yeni bir kaynak doğdu".
		writeJSON(w, http.StatusCreated, toBookResponse(book, nil))
	}
}

// setGenresRequest, PUT /api/admin/books/{id}/genres gövdesi. Replace semantiği:
// verilen liste kitabın YENİ tür listesidir; boş dizi = tüm türleri kaldır.
// genre_ids ZORUNLU (nil ≠ boş): gövdesiz bir PUT'un yanlışlıkla her şeyi silmesin.
type setGenresRequest struct {
	GenreIDs *[]int64 `json:"genre_ids"`
}

// handleSetBookGenres, bir kitabın türlerini atomik olarak değiştirir ve yeni listeyi döner.
func handleSetBookGenres(store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		var req setGenresRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}

		var v validator
		if req.GenreIDs == nil {
			v.add("genre_ids zorunlu (tüm türleri kaldırmak için boş dizi gönder)")
		} else {
			v.check(len(*req.GenreIDs) <= books.MaxGenresPerBook, "en fazla %d tür atanabilir", books.MaxGenresPerBook)
			for _, gid := range *req.GenreIDs {
				if gid < 1 {
					v.add("genre_ids yalnızca pozitif id içerebilir")
					break
				}
			}
		}
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		switch err := store.SetGenres(r.Context(), id, *req.GenreIDs); {
		case errors.Is(err, books.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "kitap bulunamadı")
			return
		case errors.Is(err, books.ErrUnknownGenre):
			writeError(w, http.StatusBadRequest, "unknown_genre", "genre_ids içinde var olmayan bir tür var")
			return
		case err != nil:
			writeInternalError(w, "SetGenres", err)
			return
		}

		genreList, err := store.GenresForBook(r.Context(), id)
		if err != nil {
			writeInternalError(w, "GenresForBook", err)
			return
		}
		writeJSON(w, http.StatusOK, toGenreResponses(genreList))
	}
}

// parseID, URL'deki {id} parametresini pozitif int64 olarak okur; geçersizse 400 yazar
// ve ok=false döner. Her "{id}" rotası aynı kuralı kullansın diye tek yerde.
func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return parseIDParam(w, r, "id")
}

func parseIDParam(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_id", name+" pozitif bir tam sayı olmalı")
		return 0, false
	}
	return id, true
}
