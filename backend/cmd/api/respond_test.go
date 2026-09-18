package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// errReader, gövde okunurken taşıma katmanı arızasını taklit eder (ör. ReadTimeout).
// Gerçek dünyada bu hatanın metni sunucunun iç IP:port'unu içerir.
type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read tcp 10.0.0.5:8080->203.0.113.9:51234: i/o timeout")
}

// TestReadJSON, readJSON'un kendi garantilerini DOĞRUDAN test eder (handler üzerinden
// dolaylı değil): boyut sınırı, tek-nesne kuralı ve — en önemlisi — tanınmayan
// hataların ham metninin istemciye SIZMAMASI.
func TestReadJSON(t *testing.T) {
	type payload struct {
		Title string `json:"title"`
	}

	tests := []struct {
		name        string
		body        io.Reader
		wantStatus  int
		wantCode    string
		wantMessage string // mesajda geçmesi gereken parça
		mustNotLeak string // mesajda ASLA geçmemesi gereken parça
	}{
		{name: "1 MiB üstü gövde → 413", body: strings.NewReader(`{"title":"` + strings.Repeat("A", maxRequestBody) + `"}`),
			wantStatus: http.StatusRequestEntityTooLarge, wantCode: "body_too_large", wantMessage: "çok büyük"},
		{name: "iki nesne → 400", body: strings.NewReader(`{"title":"a"}{"title":"b"}`),
			wantStatus: http.StatusBadRequest, wantCode: "invalid_json", wantMessage: "tek bir JSON"},
		{name: "söz dizimi hatası → konumlu mesaj", body: strings.NewReader(`{"title":,}`),
			wantStatus: http.StatusBadRequest, wantCode: "invalid_json", wantMessage: "baytta bozuk"},
		{name: "yanlış tip → alan adıyla mesaj, Go tip adı yok", body: strings.NewReader(`{"title":123}`),
			wantStatus: http.StatusBadRequest, wantCode: "invalid_json", wantMessage: `"title"`, mustNotLeak: "string"},
		// Üst düzey tip uyuşmazlığı: typeErr.Field boş → alan adı olmayan genel mesaj.
		{name: "üst düzey dizi → tip mesajı", body: strings.NewReader(`[]`),
			wantStatus: http.StatusBadRequest, wantCode: "invalid_json", wantMessage: "gövde beklenen JSON tipinde değil"},
		{name: "üst düzey string → tip mesajı", body: strings.NewReader(`"str"`),
			wantStatus: http.StatusBadRequest, wantCode: "invalid_json", wantMessage: "gövde beklenen JSON tipinde değil"},
		// Geçerli nesne + 1 MiB'ı aşan artık veri: ikinci Decode sınıra takılır → 400 değil 413.
		{name: "geçerli nesne + sınırı aşan artık veri → 413", body: strings.NewReader(`{"title":"a"}` + strings.Repeat(" ", maxRequestBody)),
			wantStatus: http.StatusRequestEntityTooLarge, wantCode: "body_too_large", wantMessage: "çok büyük"},
		{name: "bilinmeyen alan", body: strings.NewReader(`{"status":"APPROVED"}`),
			wantStatus: http.StatusBadRequest, wantCode: "invalid_json", wantMessage: "bilinmeyen alan"},
		{name: "boş gövde", body: strings.NewReader(``),
			wantStatus: http.StatusBadRequest, wantCode: "invalid_json", wantMessage: "boş"},
		{name: "okuma arızası → generic mesaj, iç adres SIZMAZ", body: errReader{},
			wantStatus: http.StatusBadRequest, wantCode: "invalid_request", wantMessage: "okunamadı", mustNotLeak: "10.0.0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", tt.body)
			rec := httptest.NewRecorder()

			var p payload
			err := readJSON(rec, req, &p)

			var re *requestError
			if !errors.As(err, &re) {
				t.Fatalf("*requestError bekleniyordu, gelen %T: %v", err, err)
			}
			if re.status != tt.wantStatus || re.code != tt.wantCode {
				t.Errorf("istenen %d/%s, gelen %d/%s", tt.wantStatus, tt.wantCode, re.status, re.code)
			}
			if !strings.Contains(re.message, tt.wantMessage) {
				t.Errorf("mesaj %q içermeli, gelen %q", tt.wantMessage, re.message)
			}
			if tt.mustNotLeak != "" && strings.Contains(re.message, tt.mustNotLeak) {
				t.Errorf("mesaj %q SIZDIRIYOR: %q", tt.mustNotLeak, re.message)
			}

			// Uçtan uca: writeRequestError da ham metni gövdeye yazmamalı.
			out := httptest.NewRecorder()
			writeRequestError(out, err)
			if tt.mustNotLeak != "" && strings.Contains(out.Body.String(), tt.mustNotLeak) {
				t.Errorf("cevap gövdesi %q SIZDIRIYOR: %s", tt.mustNotLeak, out.Body)
			}
		})
	}

	t.Run("geçerli gövde → hata yok, değer dolu", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"title":"ok"}`))
		var p payload
		if err := readJSON(httptest.NewRecorder(), req, &p); err != nil || p.Title != "ok" {
			t.Fatalf("istenen hata yok + title=ok, gelen %v / %+v", err, p)
		}
	})
}

// TestWriteRequestErrorNonRequestError: requestError olmayan bir hata bir programlama
// hatasıdır → 500 ve metni istemciye gitmez.
func TestWriteRequestErrorNonRequestError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeRequestError(rec, errors.New("gizli iç detay"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "gizli") {
		t.Fatalf("istenen 500 + generic gövde, gelen %d %s", rec.Code, rec.Body)
	}
}
