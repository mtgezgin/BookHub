package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/mtgezgin/bookhub/backend/internal/books"
	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// bookGetter, bu handler'ın repository'den İHTİYAÇ DUYDUĞU tek davranış.
// Interface'i TÜKETEN tarafta tanımlıyoruz (GO_CONVENTIONS §5): *books.Repository
// bunu zaten sağlıyor (ekstra "implements" bildirimi gerekmez — yapısal tipleme),
// testte ise sahte (fake) bir uygulama takabiliyoruz → DB'siz handler testi.
type bookGetter interface {
	Get(ctx context.Context, id int64) (db.Book, error)
}

// handleGetBook, GET /api/books/{id} handler'ını ÜRETİR (closure ile DI):
// repository dışarıdan gelir, handler onu kapanışında (closure) taşır.
func handleGetBook(repo bookGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) URL'deki {id}'yi al ve doğrula. Kullanıcı girdisi = güvenilmez veri;
		//    sayı değilse/pozitif değilse sorguya hiç gitmeden 400 dön.
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id < 1 {
			writeError(w, http.StatusBadRequest, "invalid_id", "id pozitif bir tam sayı olmalı")
			return
		}

		// 2) Repository'den çek — r.Context() İLETİLİR: istemci bağlantıyı
		//    keserse sorgu da iptal olur (Faz 1'den beri kurduğumuz zincir).
		book, err := repo.Get(r.Context(), id)

		// 3) Hata HARİTALAMA: domain hatası → HTTP dili. Sıra önemli:
		//    önce bilinen (beklenen) durumlar, en sonda bilinmeyenler.
		switch {
		case errors.Is(err, books.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "kitap bulunamadı")
		case err != nil:
			// Detayı LOG'a yaz (operatör için), istemciye SIZDIRMA (güvenlik):
			// iç hata mesajları şema/sürücü bilgisi taşıyabilir.
			log.Printf("GetBook başarısız: %v", err)
			writeError(w, http.StatusInternalServerError, "internal", "beklenmeyen bir hata oluştu")
		default:
			writeJSON(w, http.StatusOK, book)
		}
	}
}
