package main

import (
	"net/http"
	"testing"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
)

// TestAuthBusy: parola özeti kuyruğu doluyken üç uç da 503 + Retry-After döner (REVIEW_FEEDBACK F04).
// 500 değil (sunucu bozuk değil), 429 değil (istemci bir sınırı aşmadı).
func TestAuthBusy(t *testing.T) {
	tests := []struct{ name, path, body string }{
		{"kayıt", "/api/auth/register", `{"email":"a@b.co","display_name":"Ali","password":"yeterince-uzun-parola"}`},
		{"giriş", "/api/auth/login", `{"email":"a@b.co","password":"yeterince-uzun-parola"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newRouter(deps{auth: &fakeAuth{err: auth.ErrBusy}}), http.MethodPost, tt.path, tt.body)
			if rec.Code != http.StatusServiceUnavailable || errorCode(t, rec) != "busy" || rec.Header().Get("Retry-After") == "" {
				t.Errorf("503 busy + Retry-After bekleniyordu: %d %s %q", rec.Code, rec.Body, rec.Header().Get("Retry-After"))
			}
		})
	}
}
