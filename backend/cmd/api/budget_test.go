package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mtgezgin/bookhub/backend/internal/reqstats"
)

// chain, üretimdeki ara katman sırasını (logger → recoverer → bütçe) tek handler'ın çevresine kurar.
func chain(h http.HandlerFunc) http.Handler {
	return requestLogger(recoverer(requestBudget(h)))
}

// TestRequestBudget: okumalar istemci iptalinden ayrılır ve 5 sn'lik bütçe alır; yazmalar iptale
// bağlı kalır ve 9 sn'lik bütçe alır (REVIEW_FEEDBACK F21, F08).
func TestRequestBudget(t *testing.T) {
	tests := []struct {
		method       string
		wantDetached bool
		wantBudget   time.Duration
	}{
		{http.MethodGet, true, readBudget},
		{http.MethodHead, true, readBudget},
		{http.MethodPost, false, writeBudget},
		{http.MethodDelete, false, writeBudget},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			var (
				ctxErr   error
				until    time.Duration
				gone     bool
				hasStats bool
			)
			h := chain(func(w http.ResponseWriter, r *http.Request) {
				ctxErr = r.Context().Err()
				if d, ok := r.Context().Deadline(); ok {
					until = time.Until(d)
				}
				gone = clientGone(r)
				hasStats = reqstats.From(r.Context()) != nil
				w.WriteHeader(http.StatusNoContent)
			})
			req := httptest.NewRequest(tt.method, "/x", nil)
			ctx, cancel := context.WithCancel(req.Context())
			cancel() // istemci gitti
			h.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

			if detached := ctxErr == nil; detached != tt.wantDetached {
				t.Errorf("iptalden ayrılmış mı: %v, istenen %v (ctx hatası %v)", detached, tt.wantDetached, ctxErr)
			}
			if until <= 0 || until > tt.wantBudget {
				t.Errorf("süre bütçesi (0, %v] aralığında olmalı, gelen %v", tt.wantBudget, until)
			}
			if !gone {
				t.Error("clientGone, istemcinin gittiğini görmeli")
			}
			if !hasStats {
				t.Error("istek sayacı ayrılmış context'e de taşınmalı")
			}
		})
	}
}

// TestAccessLogClientGoneAndTimings: giden istemcinin isteği logda 499 ve INFO'dur (handler ne
// yazarsa yazsın); erişim satırı veritabanı ve Redis sürelerini taşır (REVIEW_FEEDBACK F21, F32).
func TestAccessLogClientGoneAndTimings(t *testing.T) {
	buf := captureLogs(t)
	h := chain(func(w http.ResponseWriter, r *http.Request) {
		st := reqstats.From(r.Context())
		st.AddDB(3 * time.Millisecond)
		st.AddDB(2 * time.Millisecond)
		st.AddRedis(time.Millisecond)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
	})

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/books?q=gizli", nil))
	req := httptest.NewRequest(http.MethodGet, "/api/books", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	h.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

	lines := logLines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("iki erişim satırı bekleniyordu, gelen %d", len(lines))
	}
	first, second := lines[0], lines[1]
	if first["status"] != float64(200) || first["db_queries"] != float64(2) || first["db_ms"] != float64(5) || first["redis_ms"] != float64(1) {
		t.Errorf("ilk satır: %v", first)
	}
	if first["path"] != "/api/books" {
		t.Errorf("yol sorgusuz yazılmalı: %v", first["path"])
	}
	if second["status"] != float64(statusClientClosedRequest) || second["level"] != "INFO" {
		t.Errorf("giden istemci 499 ve INFO olmalı: %v", second)
	}
}

// TestRecovererWritesEnvelope: panik eden handler hata zarfı ve X-Request-Id ile 500 döner; panik
// değeri istemciye gitmez, yığın izi loga yazılır (REVIEW_FEEDBACK F32).
func TestRecovererWritesEnvelope(t *testing.T) {
	buf := captureLogs(t)
	h := chain(func(http.ResponseWriter, *http.Request) { panic("gizli ayrıntı") })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "internal" {
		t.Fatalf("500 internal zarfı bekleniyordu: %d %s", rec.Code, rec.Body)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("gizli")) {
		t.Errorf("panik değeri istemciye gitmemeli: %s", rec.Body)
	}
	var sawPanic, sawAccess bool
	for _, l := range logLines(t, buf) {
		if l["msg"] == "handler panik etti" && l["stack"] != "" && l["level"] == "ERROR" {
			sawPanic = true
		}
		if l["msg"] == "http" && l["status"] == float64(500) {
			sawAccess = true
		}
	}
	if !sawPanic || !sawAccess {
		t.Errorf("panik satırı (%v) ve 500'lük erişim satırı (%v) bekleniyordu", sawPanic, sawAccess)
	}

	t.Run("http.ErrAbortHandler yutulmaz", func(t *testing.T) {
		defer func() {
			if rec := recover(); rec != http.ErrAbortHandler { //nolint:errorlint // panik değeri birebir karşılaştırılır
				t.Errorf("ErrAbortHandler yeniden fırlatılmalıydı, gelen %v", rec)
			}
		}()
		recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).
			ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	})
}

// TestWriteInternalErrorCancelAndTimeout: iptal 499 ve INFO'dur (ERROR değil); süre aşımı 503 timeout
// ve Retry-After'dır (REVIEW_FEEDBACK F21, F08).
func TestWriteInternalErrorCancelAndTimeout(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantLevel  string
	}{
		{"istemci iptali", fmt.Errorf("sorgu: %w", context.Canceled), statusClientClosedRequest, "", "INFO"},
		{"context süresi doldu", fmt.Errorf("sorgu: %w", context.DeadlineExceeded), http.StatusServiceUnavailable, "timeout", "ERROR"},
		{"statement_timeout (57014)", &pgconn.PgError{Code: "57014"}, http.StatusServiceUnavailable, "timeout", "ERROR"},
		{"lock_timeout (55P03)", &pgconn.PgError{Code: "55P03"}, http.StatusServiceUnavailable, "timeout", "ERROR"},
		{"başka hata", errors.New("bozuk"), http.StatusInternalServerError, "internal", "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLogs(t)
			rec := httptest.NewRecorder()
			writeInternalError(rec, "Test", tt.err)
			if rec.Code != tt.wantStatus {
				t.Fatalf("durum %d, istenen %d", rec.Code, tt.wantStatus)
			}
			if tt.wantCode == "" {
				if rec.Body.Len() != 0 {
					t.Errorf("giden istemciye gövde yazılmamalı: %s", rec.Body)
				}
			} else if errorCode(t, rec) != tt.wantCode {
				t.Errorf("kod %s, istenen %s", rec.Body, tt.wantCode)
			}
			if tt.wantCode == "timeout" && rec.Header().Get("Retry-After") == "" {
				t.Error("Retry-After bekleniyordu")
			}
			if l := logLines(t, buf); len(l) != 1 || l[0]["level"] != tt.wantLevel {
				t.Errorf("tek %s satırı bekleniyordu: %v", tt.wantLevel, l)
			}
		})
	}
}
