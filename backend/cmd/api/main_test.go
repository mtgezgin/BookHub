package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest" // gerçek port/sunucu açmadan handler test etmeyi sağlar
	"testing"           // Go'nun standart test paketi (stdlib)
)

// Test fonksiyonları:
//   - dosya adı _test.go ile biter,
//   - fonksiyon adı Test ile başlar,
//   - tek parametre *testing.T alır.
//
// `go test` bunları otomatik bulur ve çalıştırır.
func TestHandleHealth(t *testing.T) {
	// 1) Sahte bir GET /health isteği oluştur (ağ yok, sadece bellekte).
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// 2) Cevabı kaydeden bir "recorder" — ResponseWriter'ın test sürümü.
	rec := httptest.NewRecorder()

	// 3) Handler'ı DOĞRUDAN çağır. Sunucu başlatmaya, porta gerek yok.
	handleHealth(rec, req)

	// 4) Durum kodu 200 (OK) mü?
	if rec.Code != http.StatusOK {
		// Errorf: hatayı bildir ama teste devam et.
		t.Errorf("durum kodu yanlış: istenen %d, gelen %d", http.StatusOK, rec.Code)
	}

	// 5) Content-Type başlığı application/json mı?
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type yanlış: istenen application/json, gelen %q", ct)
	}

	// 6) Gövde gerçekten {"status":"ok"} mü? JSON'u çözüp alanı kontrol et.
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		// Fatalf: bu noktadan sonrası anlamsız (gövde JSON değilse), testi durdur.
		t.Fatalf("cevap geçerli JSON değil: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status alanı yanlış: istenen \"ok\", gelen %q", body["status"])
	}
}
