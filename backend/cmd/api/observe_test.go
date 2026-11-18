package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// recordTelemetry, test süresince izleri ve metrikleri BELLEKTE toplayan sağlayıcıları küresel olarak
// kurar (ağ yok) ve okuyucularını döner. Test bitince önceki sağlayıcılar geri konur.
func recordTelemetry(t *testing.T) (*tracetest.SpanRecorder, *sdkmetric.ManualReader) {
	t.Helper()
	spans := tracetest.NewSpanRecorder()
	reader := sdkmetric.NewManualReader()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	// Önceki sağlayıcıyı geri koymak yetmez: varsayılan küresel sağlayıcı, ilk kurulan gerçek
	// sağlayıcıya KALICI olarak bağlanır. Açıkça "yok" sağlayıcıları konur.
	t.Cleanup(func() {
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		otel.SetMeterProvider(metricnoop.NewMeterProvider())
	})
	return spans, reader
}

func attrMap(kvs []attribute.KeyValue) map[string]string {
	out := map[string]string{}
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value.String()
	}
	return out
}

// TestObserveSpanAndMetric: her istek bir span ve bir süre ölçümü üretir. Rota etiketi KALIPTIR
// (/api/books/{id}), gerçek yol değil: yoksa her kitap ayrı bir zaman serisi olurdu.
func TestObserveSpanAndMetric(t *testing.T) {
	spans, reader := recordTelemetry(t)
	router := newRouter(deps{books: fakeBooks{book: db.BookView{ID: 42, Title: "Dune"}}})

	rec := do(t, router, http.MethodGet, "/api/books/42", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("200 bekleniyordu: %d %s", rec.Code, rec.Body)
	}
	do(t, router, http.MethodGet, "/wp-login.php", "")
	do(t, router, http.MethodGet, "/health", "")

	ended := spans.Ended()
	if len(ended) != 2 {
		t.Fatalf("iki span bekleniyordu (/health izlenmez), gelen %d", len(ended))
	}
	if got := ended[0].Name(); got != "GET /api/books/{id}" {
		t.Errorf("span adı rota kalıbı olmalı, gelen %q", got)
	}
	a := attrMap(ended[0].Attributes())
	if a["http.route"] != "/api/books/{id}" || a["http.request.method"] != "GET" || a["http.response.status_code"] != "200" {
		t.Errorf("span öznitelikleri: %v", a)
	}
	if got := ended[1].Name(); got != "GET eşleşmedi" {
		t.Errorf("eşleşmeyen yollar tek ada toplanmalı, gelen %q", got)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var routes []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "http.server.request.duration" {
				continue
			}
			h, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("histogram bekleniyordu: %T", m.Data)
			}
			for _, dp := range h.DataPoints {
				route, _ := dp.Attributes.Value("http.route")
				routes = append(routes, fmt.Sprintf("%s×%d", route.AsString(), dp.Count))
			}
		}
	}
	if got := strings.Join(routes, ","); !strings.Contains(got, "/api/books/{id}×1") || !strings.Contains(got, "eşleşmedi×1") || strings.Contains(got, "42") {
		t.Errorf("metrik serileri rota kalıbıyla olmalı: %s", got)
	}
}

// TestObserveSendsNoRequestData: izler üçüncü bir tarafa gider; CLAUDE.md'deki log kuralı izler için de
// geçerlidir. Sorgu dizesi (arama terimi), başlıklar (token, çerez) ve gövde (parola, e-posta) hiçbir
// span adına ya da özniteliğine girmez.
func TestObserveSendsNoRequestData(t *testing.T) {
	spans, _ := recordTelemetry(t)
	router := newRouter(deps{books: fakeBooks{}, auth: &fakeAuth{}})

	req := httptest.NewRequest(http.MethodGet, "/api/books?q=gizli-arama-terimi", nil)
	req.Header.Set("Authorization", "Bearer gizli-token")
	req.Header.Set("Cookie", "bookhub_refresh=gizli-cerez")
	router.ServeHTTP(httptest.NewRecorder(), req)
	do(t, router, http.MethodPost, "/api/auth/login", `{"email":"gizli@example.com","password":"gizli-parola-123"}`)

	if len(spans.Ended()) != 2 {
		t.Fatalf("iki span bekleniyordu, gelen %d", len(spans.Ended()))
	}
	for _, s := range spans.Ended() {
		text := s.Name()
		for _, kv := range s.Attributes() {
			text += " " + string(kv.Key) + "=" + kv.Value.String()
		}
		for _, e := range s.Events() {
			text += " " + e.Name
			for _, kv := range e.Attributes {
				text += " " + kv.Value.String()
			}
		}
		if strings.Contains(text, "gizli") {
			t.Errorf("span istek verisi taşıyor: %s", text)
		}
		if n := len(s.Attributes()); n != 3 {
			t.Errorf("tam üç öznitelik bekleniyordu (method, rota, durum), gelen %d: %v", n, attrMap(s.Attributes()))
		}
	}
}

// TestAccessLogCarriesTraceID: erişim satırındaki trace_id o isteğin iziyle aynıdır (logdan ize geçiş).
// Telemetri kapalıyken alan hiç yazılmaz.
func TestAccessLogCarriesTraceID(t *testing.T) {
	buf := captureLogs(t)
	do(t, newRouter(deps{books: fakeBooks{}}), http.MethodGet, "/api/books", "")
	if lines := logLines(t, buf); len(lines) != 1 || lines[0]["trace_id"] != nil {
		t.Fatalf("telemetri kapalıyken trace_id olmamalı: %v", lines)
	}

	buf.Reset()
	spans, _ := recordTelemetry(t)
	do(t, newRouter(deps{books: fakeBooks{}}), http.MethodGet, "/api/books", "")
	lines := logLines(t, buf)
	if len(lines) != 1 || len(spans.Ended()) != 1 {
		t.Fatalf("bir satır ve bir span bekleniyordu: %d satır, %d span", len(lines), len(spans.Ended()))
	}
	if want := spans.Ended()[0].SpanContext().TraceID().String(); lines[0]["trace_id"] != want {
		t.Errorf("trace_id: log %v, iz %s", lines[0]["trace_id"], want)
	}
}
