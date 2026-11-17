package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/ratelimit"
	"github.com/mtgezgin/bookhub/backend/internal/reqstats"
)

// tokenVerifier, middleware'in token doğrulayıcıdan beklediği tek davranış.
// *auth.TokenManager bunu sağlar; testlerde sahte takılır.
type tokenVerifier interface {
	Verify(raw string) (auth.Principal, error)
}

// principalKey, context'e koyduğumuz değerin anahtarı. Tipi DIŞARI AÇIK DEĞİL: başka bir
// paket aynı anahtarı üretip context'teki kimliği ezemez/okuyamaz. (Düz string anahtar —
// ör. "user" — her paketin çarpışabileceği ortak bir ad alanı olurdu.)
type principalKey struct{}

func withPrincipal(ctx context.Context, p auth.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// principalFrom, requireAuth'un koyduğu kimliği okur. ok=false → rota korunmuyor demektir.
func principalFrom(ctx context.Context) (auth.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(auth.Principal)
	return p, ok
}

// bearerToken, "Authorization: Bearer <token>" başlığından token'ı çıkarır.
// Şema adı büyük/küçük harf duyarsız (RFC 7235), token boş olamaz.
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	token = strings.TrimSpace(token)
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

// unauthorized, 401 yazar. WWW-Authenticate başlığı standardın gereği (RFC 6750):
// istemciye "hangi şemayla kimlik göstermelisin" der.
func unauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="bookhub"`)
	writeError(w, http.StatusUnauthorized, code, message)
}

// requireAuth, geçerli bir access token ister; kimliği context'e koyar.
//
// MIDDLEWARE NEDİR: handler'ı saran bir handler — isteği önce o görür, uygun değilse
// zinciri KESER (next'i çağırmaz). Express'teki app.use((req,res,next) => ...) ile aynı
// fikir; Go'da imza func(http.Handler) http.Handler.
//
// 401 vs 403: 401 = "seni tanımıyorum" (token yok/geçersiz), 403 = "seni tanıyorum ama
// buna yetkin yok" (requireAdmin).
func requireAuth(v tokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				unauthorized(w, "missing_token", "kimlik doğrulaması gerekli (Authorization: Bearer başlığı)")
				return
			}
			p, err := v.Verify(raw)
			if err != nil {
				// Nedeni (süre mi, imza mı) istemciye söylemiyoruz; her iki durumda da
				// istemcinin yapacağı aynı: yeniden giriş ya da /refresh.
				unauthorized(w, "invalid_token", "geçersiz ya da süresi dolmuş token")
				return
			}
			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), p)))
		})
	}
}

// requireAdmin, requireAuth'tan SONRA zincirlenir: kimlik belli, şimdi rol kontrolü.
// Kaba (rol bazlı) yetki burada, routing tablosunda görünür yerde yaşar; kaynak
// SAHİPLİĞİ kontrolleri (bu puan senin mi?) servis katmanında (GO_CONVENTIONS §12).
func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			// requireAuth'suz zincirlenmiş = yanlış kurulum. AÇIK değil KAPALI başarısız ol.
			unauthorized(w, "missing_token", "kimlik doğrulaması gerekli")
			return
		}
		if !p.IsAdmin() {
			writeError(w, http.StatusForbidden, "forbidden", "bu işlem için admin yetkisi gerekli")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIPKey, resolveClientIP'in çözdüğü istemci IP'sinin context anahtarı.
type clientIPKey struct{}

// resolveClientIP, isteğin GERÇEK istemci IP'sini bir kez çözer ve context'e koyar.
//
// Varsayılan: RemoteAddr (TCP bağlantısının karşı ucu). Bir reverse proxy (NGINX, Faz 11)
// arkasında bu HER ZAMAN proxy'nin adresidir — rate limit tüm kullanıcıları tek kovaya
// atardı. Bu yüzden: bağlantı GÜVENİLEN bir proxy'den geliyorsa, onun koyduğu X-Real-IP
// başlığı kullanılır.
//
// Neden yalnız güvenilen proxy'den: X-Real-IP / X-Forwarded-For'u İSTEMCİ de yazabilir.
// Her istekte sahte bir IP yollayıp rate limit'i atlatmasın diye, doğrudan gelen isteklerde
// bu başlıklar YOK SAYILIR. NGINX yapılandırması başlığı kendi $remote_addr'ıyla EZER
// (istemcinin gönderdiği değer proxy'den geçemez).
// X-Forwarded-For yerine X-Real-IP: XFF bir listedir ve istemci başına ekleme yapabilir;
// doğru okumak "sağdan ilk güvenilmeyen" mantığı ister. Tek hop'lu yapımızda gerek yok.
func resolveClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteHost(r)
			if addr, err := netip.ParseAddr(ip); err == nil && isTrusted(addr, trusted) {
				if realIP, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
					ip = realIP.String()
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip)))
		})
	}
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// clientIP, resolveClientIP'in çözdüğü adres; middleware zincirde yoksa RemoteAddr.
func clientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPKey{}).(string); ok {
		return ip
	}
	return remoteHost(r)
}

// rateLimit, kaba kuvvet (brute force) ve toplu hesap açmaya karşı IP başına sınır koyar.
// Aşılınca 429 + Retry-After (istemci ne kadar beklemesi gerektiğini bilsin).
//
// Sınırlayıcı arızası (Redis düştü) → İSTEĞE İZİN VER (fail-open) ve logla. Ödünleşim bilinçli:
// fail-closed, Redis'in her kesintisini TÜM kullanıcılar için "giriş yapılamıyor"a çevirirdi.
// Parola kaba kuvvetine karşı asıl duvar zaten bcrypt'in yavaşlığı; rate limit ikinci hat.
func rateLimit(l ratelimit.Limiter, name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			d, err := l.Allow(r.Context(), name+":"+clientIP(r))
			if err != nil {
				slog.Warn("rate limit kontrol edilemedi (izin veriliyor)", "err", err, "request_id", w.Header().Get(requestIDHeader))
				next.ServeHTTP(w, r)
				return
			}
			if !d.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(d.RetryAfter.Seconds()))))
				writeError(w, http.StatusTooManyRequests, "rate_limited", "çok fazla deneme; biraz bekleyip tekrar dene")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requestIDHeader, isteğin kimliğini istemciye de döndürdüğümüz başlık. Kullanıcı bir hata
// bildirdiğinde bu değeri verirse, log'da o isteğin bütün satırları bulunur.
const requestIDHeader = "X-Request-Id"

// requestLogger, chi'nin düz metin Logger'ının yerine: her istek için TEK yapılandırılmış
// satır (slog) — method, yol, status, süre, boyut, request_id. JSON modunda log toplayıcılar
// "status>=500 olanlar" gibi sorguları metin ayrıştırmadan yapabilir.
// middleware.RequestID'den SONRA zincirlenmeli (id'yi o üretir).
//
// Satır ayrıca sürenin NEREYE gittiğini söyler (REVIEW_FEEDBACK F32): db_queries ve db_ms
// veritabanına yapılan gidiş-dönüşlerin sayısı ve toplam süresi, redis_ms Redis'inki. Toplamdan
// geriye kalan, işlemcide (JSON, bcrypt) ve havuzdan bağlantı beklerken geçen süredir.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := middleware.GetReqID(r.Context())
		w.Header().Set(requestIDHeader, id)

		ctx, stats := reqstats.New(r.Context())
		r = r.WithContext(ctx)

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor) // status/boyutu yakalamak için
		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK // handler hiç WriteHeader çağırmadıysa net/http 200 yazar
		}
		// İstemci cevabı beklemeden gittiyse handler'ın yazdığı durum kimseye ulaşmadı. Logda 200 ya da
		// 500 gibi görünmesin: 499 (NGINX'in "istemci bağlantıyı kapattı" kodu). r burada ara
		// katmanların DEĞİŞTİRMEDİĞİ istektir; context'i yalnız istemci giderse iptal olur.
		if errors.Is(r.Context().Err(), context.Canceled) {
			status = statusClientClosedRequest
		}
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		}
		dbQueries, dbTime := stats.DB()
		_, redisTime := stats.Redis()
		attrs := []any{}
		// trace_id: bu satırdan Grafana'daki ize geçmenin anahtarı (telemetri kapalıyken yok).
		if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
			attrs = append(attrs, "trace_id", sc.TraceID().String())
		}
		slog.Log(r.Context(), level, "http", append(attrs,
			"method", r.Method,
			"path", r.URL.Path, // query BİLEREK yok: arama terimleri kişisel veri olabilir
			"status", status,
			"bytes", ww.BytesWritten(),
			"duration_ms", ms(time.Since(start)),
			"db_queries", dbQueries,
			"db_ms", ms(dbTime),
			"redis_ms", ms(redisTime),
			"remote", clientIP(r),
			"request_id", id,
		)...)
	})
}

// ms, süreyi log için milisaniyeye (mikrosaniye çözünürlüğünde) çevirir.
func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// statusClientClosedRequest, istemci cevabı beklemeden gittiğinde erişim loguna yazılan durum
// (NGINX'in 499'u). Standart bir HTTP kodu değildir ve kimseye ulaşmaz: bağlantı zaten kapalıdır.
const statusClientClosedRequest = 499

// Süre bütçeleri. İkisi de sunucunun cevap yazma süresinden (main.go WriteTimeout, 10 sn) kısadır:
// bütçesi dolan istek, bağlantı kesilmeden ÖNCE düzgün bir hata cevabı yazabilsin.
const (
	// readBudget: bir okuma isteğinin (GET/HEAD) bütün sorgularının toplam süresi.
	readBudget = 5 * time.Second
	// writeBudget: yazma isteklerinin süresi. Parola özeti kuyruğunda 5 sn bekleyip ~3,3 sn özet
	// hesaplayan bir giriş bu sürede biter (auth.defaultHashWait).
	writeBudget = 9 * time.Second
)

type clientContextKey struct{}

// requestBudget, her isteğe bir süre bütçesi koyar ve OKUMA isteklerini istemcinin iptalinden ayırır.
//
// NEDEN ayırma: istemci giderse (yeni harf, sekme kapatma, sayfadan ayrılma) iptal isteğin
// context'ine, oradan uçuştaki sorguya ulaşır ve pgx o sorgunun BAĞLANTISINI kapatır (ölçüm: yarıda
// kesilen 20 istek 19 yeni veritabanı bağlantısı açtırdı; REVIEW_FEEDBACK F21). Milisaniyeler süren
// bir okumayı bitirmek, bağlantıyı TLS ve kimlik doğrulamayla yeniden kurmaktan ucuzdur. Bütçe,
// kimsenin beklemediği işin tavanıdır. WithoutCancel değerleri (request id, sayaç) korur.
//
// NEDEN yazmalar ayrılmıyor: istemcinin vazgeçtiği bir yazmayı tamamlamak, kullanıcının görmediği
// bir değişiklik bırakır; işlem geri alınır ve bağlantının yeniden kurulması kabul edilir.
//
// NEDEN bütçe (REVIEW_FEEDBACK F08): http.Server'ın WriteTimeout'u yalnız bağlantıyı keser, çalışan
// handler'ı durdurmaz. Sorguları durduran, context'in süre sınırıdır.
func requestBudget(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			client := r.Context()
			ctx = context.WithValue(context.WithoutCancel(client), clientContextKey{}, client)
			ctx, cancel = context.WithTimeout(ctx, readBudget)
		} else {
			ctx, cancel = context.WithTimeout(r.Context(), writeBudget)
		}
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// clientGone, istemci cevabı beklemeden gitti mi diye bakar. Okuma isteklerinde r.Context() iptalden
// ayrılmıştır (requestBudget); istemcinin asıl context'i onun içinde saklanır.
func clientGone(r *http.Request) bool {
	if client, ok := r.Context().Value(clientContextKey{}).(context.Context); ok {
		return client.Err() != nil
	}
	return errors.Is(r.Context().Err(), context.Canceled)
}

// recoverer, panik eden bir handler'ın süreci çökertmesini önler ve hata zarfıyla 500 döner.
// chi'nin Recoverer'ı boş gövdeli 500 dönüyordu: arayüz mesaj gösteremiyor, cevap zarf sözleşmesini
// bozuyordu (REVIEW_FEEDBACK F32). Yığın izi loga yazılır; panik değeri istemciye gitmez.
// requestLogger'dan SONRA zincirlenir: panik eden istek de erişim loguna 500 olarak düşer.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// net/http'nin "cevabı sessizce kes" işareti: yutulmaz, sunucuya bırakılır.
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			slog.Error("handler panik etti",
				"panic", fmt.Sprint(rec),
				"stack", string(debug.Stack()),
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", w.Header().Get(requestIDHeader),
			)
			writeError(w, http.StatusInternalServerError, "internal", internalErrorMessage)
		}()
		next.ServeHTTP(w, r)
	})
}
