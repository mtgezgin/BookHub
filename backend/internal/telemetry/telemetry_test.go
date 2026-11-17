package telemetry

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mtgezgin/bookhub/backend/internal/database"
)

func TestQueryName(t *testing.T) {
	tests := []struct{ sql, want string }{
		{"-- name: ListBooks :many\nSELECT b.* FROM book_view b", "ListBooks"},
		{"\n-- name: GetUserByEmail :one\nSELECT * FROM users WHERE email = $1", "GetUserByEmail"},
		{"begin", "BEGIN"},
		{"commit", "COMMIT"},
		{"SET statement_timeout = '10s';\nSET lock_timeout = '3s';", "SET"},
		{"SELECT 1", "SELECT"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := QueryName(tt.sql); got != tt.want {
			t.Errorf("QueryName(%q) = %q, istenen %q", tt.sql, got, tt.want)
		}
	}
}

// TestSetupDisabled: adres boşken hiçbir şey kurulmaz; küresel sağlayıcılar değişmez ve kapanış
// fonksiyonu zararsızdır.
func TestSetupDisabled(t *testing.T) {
	before := otel.GetTracerProvider()
	shutdown, err := Setup(context.Background(), Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if otel.GetTracerProvider() != before {
		t.Error("kapalıyken küresel iz sağlayıcısı değişmemeli")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("kapanış: %v", err)
	}
}

// record, izleri bellekte toplayan bir sağlayıcıyı küresel olarak kurar.
func record(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	spans := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))
	// Önceki sağlayıcıyı geri koymak yetmez: OpenTelemetry'nin varsayılan küresel sağlayıcısı, ilk
	// kurulan gerçek sağlayıcıya KALICI olarak bağlanır. Açıkça "yok" sağlayıcısı konur.
	t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })
	return spans
}

// dump, bir span'in adını ve bütün özniteliklerini tek metne çevirir ("içinde şu var mı?" denetimi için).
func dump(s sdktrace.ReadOnlySpan) string {
	var b strings.Builder
	b.WriteString(s.Name())
	for _, kv := range s.Attributes() {
		b.WriteString(" " + string(kv.Key) + "=" + kv.Value.String())
	}
	for _, e := range s.Events() {
		for _, kv := range e.Attributes {
			b.WriteString(" " + kv.Value.String())
		}
	}
	return b.String()
}

// TestPgxSpansCarryNoArgumentsRealDB: sorgu span'i sorgunun adını ve SQL metnini taşır; ARGÜMANLARI
// (e-posta, arama terimi) ve bağlantı bilgisini (veritabanı kullanıcısı, sunucu) taşımaz.
func TestPgxSpansCarryNoArgumentsRealDB(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil → entegrasyon testi atlanıyor")
	}
	spans := record(t)
	ctx := context.Background()
	pool, err := database.New(ctx, dsn, 2, PgxTracer())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(pool.Close)

	var n int
	const q = "-- name: CountUsersByEmail :one\nSELECT count(*) FROM users WHERE email = $1"
	// Sorgu span'i yalnız kaydedilen bir üst span'in (HTTP isteği) İÇİNDE açılır: açılıştaki ve
	// istek dışındaki sorgular iz üretmez (otelpgx'in davranışı; kapalıyken maliyet de bu yüzden yok).
	if err := pool.QueryRow(ctx, q, "gizli@example.com").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if got := len(spans.Ended()); got != 0 {
		t.Fatalf("üst span yokken sorgu span'i açılmamalı, gelen %d", got)
	}
	reqCtx, parent := otel.Tracer("test").Start(ctx, "GET /test")
	if err := pool.QueryRow(reqCtx, q, "gizli@example.com").Scan(&n); err != nil {
		t.Fatal(err)
	}
	parent.End()

	var found bool
	for _, s := range spans.Ended() {
		text := dump(s)
		if strings.Contains(text, "gizli@example.com") {
			t.Errorf("span sorgu argümanını taşıyor: %s", text)
		}
		if strings.Contains(text, "bookhub_dev") || strings.Contains(text, "db.user") || strings.Contains(text, "server.address") {
			t.Errorf("span bağlantı bilgisi taşıyor: %s", text)
		}
		if s.Name() == "CountUsersByEmail" {
			found = true
			if !strings.Contains(text, "WHERE email = $1") {
				t.Errorf("SQL metni (yer tutucuyla) bekleniyordu: %s", text)
			}
		}
	}
	if !found {
		t.Errorf("sqlc adıyla bir span bekleniyordu; görülenler: %d span", len(spans.Ended()))
	}
}

// TestRedisSpansCarryNoKeysRealRedis: Redis span'i komutun adını taşır, anahtarını ve değerini taşımaz
// (rate limit anahtarı ziyaretçinin IP'sini içerir).
func TestRedisSpansCarryNoKeysRealRedis(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL tanımlı değil → Redis testi atlanıyor")
	}
	spans := record(t)
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	if err := InstrumentRedis(client); err != nil {
		t.Fatalf("InstrumentRedis: %v", err)
	}

	ctx := context.Background()
	const key = "rate-limit:login:203.0.113.9"
	if err := client.Set(ctx, key, "gizli-deger", 0).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Del(context.Background(), key) })
	pipe := client.TxPipeline()
	pipe.Incr(ctx, key+":n")
	pipe.Del(ctx, key+":n")
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if len(spans.Ended()) == 0 {
		t.Fatal("en az bir Redis span'i bekleniyordu")
	}
	for _, s := range spans.Ended() {
		if text := dump(s); strings.Contains(text, "203.0.113.9") || strings.Contains(text, "gizli-deger") {
			t.Errorf("span anahtar ya da değer taşıyor: %s", text)
		}
	}
}
