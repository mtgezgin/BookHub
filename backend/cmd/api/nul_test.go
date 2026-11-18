package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestReadJSONRejectsNUL: JSON'da "\u0000" olarak yazılabilen NUL, Postgres'in metinde kabul
// etmediği tek karakterdir. Sorguya ulaşsaydı istemcinin hatası 500 ve ERROR satırı olurdu
// (REVIEW_FEEDBACK F30). Denetim alan alan değil gövdenin tamamında: iç içe alanlar da kapsanır.
func TestReadJSONRejectsNUL(t *testing.T) {
	type inner struct {
		Tags []string          `json:"tags"`
		Meta map[string]string `json:"meta"`
		Note *string           `json:"note"`
	}
	type body struct {
		Title string `json:"title"`
		Inner inner  `json:"inner"`
	}
	tests := []struct {
		name, payload string
		wantErr       bool
	}{
		{"temiz gövde", `{"title":"Dune","inner":{"tags":["a"],"meta":{"k":"v"},"note":"n"}}`, false},
		{"üst düzey metinde NUL", `{"title":"Du\u0000ne"}`, true},
		{"dilimde NUL", `{"title":"Dune","inner":{"tags":["a","b\u0000"]}}`, true},
		{"map değerinde NUL", `{"title":"Dune","inner":{"meta":{"k":"\u0000"}}}`, true},
		{"işaretçi metinde NUL", `{"title":"Dune","inner":{"note":"\u0000"}}`, true},
		{"boş işaretçi sorun değil", `{"title":"Dune","inner":{"note":null}}`, false},
		// Geçersiz UTF-8'i encoding/json U+FFFD'ye çevirir: veritabanına geçerli metin gider.
		{"geçersiz UTF-8 bayt → yerine koyma karakteri", "{\"title\":\"Du\xffne\"}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got body
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.payload))
			err := readJSON(httptest.NewRecorder(), req, &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("hata: %v, istenen hata var mı: %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var re *requestError
				if !errors.As(err, &re) || re.status != http.StatusBadRequest || re.code != "invalid_json" {
					t.Errorf("400 invalid_json bekleniyordu, gelen %+v", err)
				}
			}
		})
	}
}

// TestWriteInternalErrorMapsInvalidInput: sınırdaki doğrulamayı atlayan geçersiz bir değer
// veritabanına ulaşırsa (SQLSTATE 22xxx) cevap 500 değil 400'dür. Diğer veritabanı hataları 500 kalır.
func TestWriteInternalErrorMapsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"geçersiz bayt dizisi (22021)", fmt.Errorf("sorgu: %w", &pgconn.PgError{Code: "22021"}), http.StatusBadRequest, "validation_error"},
		{"metinde NUL (22P05)", &pgconn.PgError{Code: "22P05"}, http.StatusBadRequest, "validation_error"},
		{"benzersizlik ihlali 500 kalır", &pgconn.PgError{Code: "23505"}, http.StatusInternalServerError, "internal"},
		{"bağlantı hatası 500 kalır", errors.New("bağlantı koptu"), http.StatusInternalServerError, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeInternalError(rec, "Test", tt.err)
			if rec.Code != tt.wantStatus || errorCode(t, rec) != tt.wantCode {
				t.Errorf("istenen %d %s, gelen %d %s", tt.wantStatus, tt.wantCode, rec.Code, rec.Body)
			}
		})
	}
}
