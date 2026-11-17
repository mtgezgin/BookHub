package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName, bu dosyanın ürettiği iz ve metriklerin "kim üretti" etiketi.
const instrumentationName = "github.com/mtgezgin/bookhub/backend/cmd/api"

// durationBuckets, istek süresi histogramının kova sınırları (saniye).
//
// Histogram tek tek süreleri saklamaz; "şu sınırın altında kaç istek bitti" sayaçlarını tutar. p95 gibi
// yüzdelikler bu kovalardan HESAPLANIR, yani çözünürlük kovaların nerede durduğuna bağlıdır. Sınırlar
// bu API'nin gerçek aralığına göre seçildi: çoğu istek 2-50 ms, parola özeti ~3 sn, bütçe tavanı 9 sn.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// observe, her istek için bir İZ (span) açar ve süresini bir METRİĞE (histogram) yazar.
//
// İZ ile METRİK aynı isteği iki ayrı soruya cevap verecek biçimde kaydeder:
//   - metrik: "son 5 dakikada GET /api/books isteklerinin p95'i kaç ms?" Bütün istekler sayılır,
//     ucuzdur, alarm bunun üzerine kurulur. Tek bir isteği gösteremez.
//   - iz: "ŞU istek neden 400 ms sürdü?" İsteğin içindeki adımlar (sorgular, Redis) alt span olarak
//     görünür. Pahalıdır; canlıda isteklerin yalnız bir kısmı örneklenir.
//
// OpenTelemetry'nin hazır net/http donatma paketi yerine elle yazıldı (o paket scripts/checks.sh'ta
// yasaklı): hazır paket isteğin yolunu ve SORGU DİZESİNİ de span'e yazar
// (arama terimi kişisel veri olabilir; CLAUDE.md log kuralı). Burada yalnız dört öznitelik var ve
// hepsi düşük kardinaliteli: method, ROTA KALIBI (/api/books/{id}; gerçek yol değil), durum kodu.
// Kardinalite: bir metriğin her farklı etiket bileşimi ayrı bir zaman serisidir. Etikete kitap kimliği
// girseydi kitap sayısı kadar seri oluşurdu (Grafana Cloud'un ücretsiz sınırı 10 bin seri).
//
// Telemetri kapalıyken tracer ve meter "yok" sürümleridir: span açmak ve ölçüm kaydetmek hiçbir şey
// yapmadan döner.
func observe() func(http.Handler) http.Handler {
	tracer := otel.Tracer(instrumentationName)
	duration, err := otel.Meter(instrumentationName).Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("HTTP isteklerinin süresi"),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	if err != nil {
		// Yalnız geçersiz ad ya da birimde olur; ikisi de yukarıda sabit. Yine de sessizce yutma.
		panic("istek süresi histogramı kurulamadı: " + err.Error())
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Sağlık kontrolü birkaç saniyede bir gelir: iz ve metrik kotasını gürültüyle doldurmasın.
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			// Gelen istek bir izin parçasıysa (traceparent başlığı) o izi sürdür; değilse yenisini başlat.
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, r.Method, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			if errors.Is(r.Context().Err(), context.Canceled) {
				status = statusClientClosedRequest // istemci gitti (bkz. requestLogger)
			}
			// Rota kalıbı ancak yönlendirme bittikten SONRA bilinir. Eşleşmeyen yollar (taramalar,
			// botlar) tek bir ada toplanır; her biri ayrı bir seri olmasın.
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "eşleşmedi"
			}
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", status),
			}
			span.SetName(r.Method + " " + route)
			span.SetAttributes(attrs...)
			if status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
			duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		})
	}
}
