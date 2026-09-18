package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// captureLogs, testin süresince varsayılan slog'u bir tampona (JSON) yönlendirir.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(buf)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("log satırı JSON değil: %q", sc.Text())
		}
		out = append(out, m)
	}
	return out
}

// TestInternalErrorCorrelatesWithRequest: 500 veren bir isteğin (1) cevabında X-Request-Id
// var, (2) hata satırı ve erişim satırı AYNI request_id'yi taşıyor — "hangi istek patladı?"
// sorusu tek bir id ile cevaplanır (review bulgusu F8).
func TestInternalErrorCorrelatesWithRequest(t *testing.T) {
	buf := captureLogs(t)
	rec := do(t, newRouter(deps{books: fakeBooks{err: errors.New("db çöktü")}}), http.MethodGet, "/api/books/1", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("500 bekleniyordu: %d", rec.Code)
	}
	id := rec.Header().Get(requestIDHeader)
	if id == "" {
		t.Fatal("cevap X-Request-Id taşımalı")
	}

	var errLine, accessLine map[string]any
	for _, l := range logLines(t, buf) {
		switch l["msg"] {
		case "istek başarısız":
			errLine = l
		case "http":
			accessLine = l
		}
	}
	if errLine == nil || accessLine == nil {
		t.Fatalf("hata ve erişim satırı bekleniyordu: %s", buf)
	}
	if errLine["request_id"] != id || accessLine["request_id"] != id {
		t.Errorf("request_id eşleşmeli: başlık=%q hata=%v erişim=%v", id, errLine["request_id"], accessLine["request_id"])
	}
	if errLine["err"] != "db çöktü" || errLine["op"] != "GetBook" {
		t.Errorf("hata satırı op + err taşımalı: %v", errLine)
	}
	if accessLine["status"] != float64(500) || accessLine["level"] != "ERROR" {
		t.Errorf("erişim satırı 5xx'i ERROR seviyesinde yazmalı: %v", accessLine)
	}
}

// TestAccessLogOmitsQuery: arama terimleri (kişisel veri olabilir) log'a düşmez.
func TestAccessLogOmitsQuery(t *testing.T) {
	buf := captureLogs(t)
	do(t, newRouter(deps{books: fakeBooks{}}), http.MethodGet, "/api/books?q=gizli-arama", "")
	if bytes.Contains(buf.Bytes(), []byte("gizli-arama")) {
		t.Errorf("sorgu metni log'a düşmemeli: %s", buf)
	}
}

func TestHealthcheck(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(handleHealth))
	defer ok.Close()
	if code := healthcheck(ok.URL); code != 0 {
		t.Errorf("sağlıklı sunucu 0 dönmeli: %d", code)
	}
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer broken.Close()
	if code := healthcheck(broken.URL); code != 1 {
		t.Errorf("503 veren sunucu 1 dönmeli: %d", code)
	}
	if code := healthcheck("http://127.0.0.1:1/health"); code != 1 {
		t.Errorf("ulaşılamayan sunucu 1 dönmeli: %d", code)
	}
}
