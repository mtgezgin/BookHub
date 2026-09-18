package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// bookStore, kitap handler'larının repository'den ihtiyaç duyduğu davranışlar.
// Interface TÜKETEN tarafta tanımlı (GO_CONVENTIONS §3): *books.Repository bunu
// "implements" demeden sağlar (yapısal tipleme); testlerde sahte takılabilir.
// Adı "Getter" değil "Store": artık yazma (Create) da içeriyor — Go'da "-er" soneki
// tek metotlu interface konvansiyonudur.
type bookStore interface {
	Get(ctx context.Context, id int64) (db.Book, error)
	List(ctx context.Context) ([]db.Book, error)
	Create(ctx context.Context, params db.CreateBookParams) (db.Book, error)
}

// bookResponse, kitabın HERKESE AÇIK temsili (response DTO).
//
// db.Book'u doğrudan serileştirmiyoruz: o struct sqlc'nin TABLODAN ürettiği şeydir ve
// şemaya eklenen her kolon (ör. Faz 4'ün moderasyon notu) kimse karar vermeden public
// API'ye sızardı. createBookRequest'in giriş tarafında yaptığını bu, çıkış tarafında yapar:
// istemciye ne gideceğine BİZ karar veririz. `status` bilerek yok — public uçlar zaten
// yalnızca APPROVED döndürür, alan bilgi taşımaz.
type bookResponse struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Author          string    `json:"author"`
	Description     string    `json:"description"`
	CoverURL        string    `json:"cover_url"`
	PublicationYear *int32    `json:"publication_year"`
	PageCount       *int32    `json:"page_count"`
	Language        string    `json:"language"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func toBookResponse(b db.Book) bookResponse {
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
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
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
		case err != nil:
			writeInternalError(w, "GetBook", err)
		default:
			writeJSON(w, http.StatusOK, toBookResponse(book))
		}
	}
}

// handleListBooks, GET /api/books — onaylı kitapları en yeniden eskiye listeler.
// Görünürlük sınırı (yalnızca APPROVED) burada değil SQL'de yaşıyor (ListBooks sorgusu).
func handleListBooks(store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := store.List(r.Context())
		if err != nil {
			writeInternalError(w, "ListBooks", err)
			return
		}

		// make(..., 0, n): liste BOŞ olsa bile JSON'da "null" değil "[]" çıkar.
		// (nil slice "null" olarak serileşir ve istemcinin books.map(...)'ini patlatır.)
		out := make([]bookResponse, 0, len(list))
		for _, b := range list {
			out = append(out, toBookResponse(b))
		}
		writeJSON(w, http.StatusOK, out)
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
}

// validate, normalize edilmiş gövdeyi denetler; TÜM problemleri tek hatada toplar.
func (req createBookRequest) validate() error {
	var v validator
	v.required("title", req.Title)
	v.maxLen("title", req.Title, maxTitleLen)
	v.required("slug", req.Slug)
	v.maxLen("slug", req.Slug, maxSlugLen)
	v.match("slug", req.Slug, slugPattern, "küçük harf, rakam ve tek tire; ör. clean-code")
	v.required("author", req.Author)
	v.maxLen("author", req.Author, maxAuthorLen)
	v.maxLen("description", req.Description, maxDescriptionLen)
	v.maxLen("cover_url", req.CoverURL, maxURLLen)
	v.httpURL("cover_url", req.CoverURL)
	v.match("language", req.Language, languagePattern, "ör. tr, en, en-US")

	// Opsiyonel sayılar: nil = "bilinmiyor" (geçerli), ama VERİLDİYSE anlamlı olmalı.
	// Pointer kullanmamızın sebebi tam bu ayrım — 0 ile "gönderilmedi" ayrı şeyler.
	if req.PublicationYear != nil {
		maxYear := int32(time.Now().Year() + 1) // gelecek yıl = ön sipariş, makul
		y := *req.PublicationYear
		v.check(y >= minPublicationYear && y <= maxYear,
			"publication_year %d-%d aralığında olmalı", minPublicationYear, maxYear)
	}
	if req.PageCount != nil {
		v.check(*req.PageCount >= 1, "page_count pozitif olmalı")
	}
	return v.err()
}

// handleCreateBook, POST /api/admin/books — admin doğrudan (onaylı) kitap ekler.
// Rota /api/admin altında: yetkilendirme middleware olarak routing tablosunda durur,
// handler gövdesinde "if user.IsAdmin" olarak DEĞİL.
func handleCreateBook(store bookStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createBookRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		req.normalize()
		if err := req.validate(); err != nil {
			writeRequestError(w, err)
			return
		}

		// ELLE eşleme: angarya gibi görünür ama asıl korumadır — yeni bir DB kolonu
		// otomatik olarak istemciye AÇILMAZ, buraya bilinçli bir satır eklemek gerekir.
		book, err := store.Create(r.Context(), db.CreateBookParams{
			Title:           req.Title,
			Slug:            req.Slug,
			Author:          req.Author,
			Description:     req.Description,
			CoverUrl:        req.CoverURL,
			PublicationYear: req.PublicationYear,
			PageCount:       req.PageCount,
			Language:        req.Language,
			// Statüyü SUNUCU belirler. Admin doğrudan eklediği için onaylı başlar;
			// kullanıcı önerileri (Faz 4) PENDING başlar.
			Status: books.StatusApproved,
		})

		// Hata haritalama: önce istemcinin düzeltebileceği durumlar, en sonda bilinmeyenler.
		switch {
		case errors.Is(err, books.ErrDuplicateSlug):
			writeError(w, http.StatusConflict, "slug_conflict", "bu slug zaten kullanılıyor")
		case err != nil:
			writeInternalError(w, "CreateBook", err)
		default:
			// 201 Created: "istek işlendi VE yeni bir kaynak doğdu".
			writeJSON(w, http.StatusCreated, toBookResponse(book))
		}
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
