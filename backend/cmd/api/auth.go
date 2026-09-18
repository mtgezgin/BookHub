package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
)

// authService, auth handler'larının servisten beklediği davranışlar (tüketen tarafta
// tanımlı). *auth.Service bunu sağlar.
type authService interface {
	Register(ctx context.Context, email, displayName, password string) (auth.User, error)
	Login(ctx context.Context, email, password string) (auth.Session, error)
	Refresh(ctx context.Context, refreshToken string) (auth.Session, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID int64) (auth.User, error)
}

// userResponse, kullanıcının dışa açık temsili. password_hash'in burada olmaması tesadüf
// değil: auth.User tipinde o alan zaten yok (bkz. auth.User).
type userResponse struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func toUserResponse(u auth.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Role: u.Role, CreatedAt: u.CreatedAt}
}

// sessionResponse, giriş/yenileme cevabı. İstemci access token'ı her istekte
// "Authorization: Bearer ..." olarak yollar; süresi dolunca refresh_token ile yenisini alır.
type sessionResponse struct {
	AccessToken      string       `json:"access_token"`
	TokenType        string       `json:"token_type"`
	AccessExpiresAt  time.Time    `json:"access_expires_at"`
	RefreshToken     string       `json:"refresh_token"`
	RefreshExpiresAt time.Time    `json:"refresh_expires_at"`
	User             userResponse `json:"user"`
}

func toSessionResponse(s auth.Session) sessionResponse {
	return sessionResponse{
		AccessToken:      s.AccessToken,
		TokenType:        "Bearer",
		AccessExpiresAt:  s.AccessExpiresAt,
		RefreshToken:     s.RefreshToken,
		RefreshExpiresAt: s.RefreshExpiresAt,
		User:             toUserResponse(s.User),
	}
}

// registerRequest: role alanı YOK (mass-assignment) — `{"role":"ADMIN"}` yollayan istek
// DisallowUnknownFields sayesinde 400 alır; alan olsaydı bile servis imzası rol almıyor.
type registerRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// handleRegister, POST /api/auth/register → 201 + kullanıcı.
func handleRegister(svc authService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}

		u, err := svc.Register(r.Context(), req.Email, req.DisplayName, req.Password)
		var ve *auth.ValidationError
		switch {
		case errors.As(err, &ve):
			writeError(w, http.StatusBadRequest, "validation_error", ve.Error())
		// 409 e-postanın kayıtlı olduğunu söyler — login'in aksine burada bilinçli bir
		// ödünleşim: aksi hâlde kullanıcı neden kayıt olamadığını anlayamaz. Toplu
		// sorgulamaya karşı savunma rate limit (Faz 7). E-posta doğrulamalı akışta
		// ("bağlantı gönderdik") bu sızıntı da kapanır.
		case errors.Is(err, auth.ErrEmailTaken):
			writeError(w, http.StatusConflict, "email_taken", "bu e-posta zaten kayıtlı")
		case err != nil:
			writeInternalError(w, "Register", err)
		default:
			writeJSON(w, http.StatusCreated, toUserResponse(u))
		}
	}
}

// handleLogin, POST /api/auth/login → 200 + oturum. Hatalı kimlikte TEK mesaj.
func handleLogin(svc authService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		var v validator
		v.required("email", req.Email)
		v.required("password", req.Password)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		sess, err := svc.Login(r.Context(), req.Email, req.Password)
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "e-posta veya parola hatalı")
		case err != nil:
			writeInternalError(w, "Login", err)
		default:
			writeJSON(w, http.StatusOK, toSessionResponse(sess))
		}
	}
}

// readRefreshToken, refresh/logout gövdesini okur ve token'ın varlığını denetler.
func readRefreshToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req refreshRequest
	if err := readJSON(w, r, &req); err != nil {
		writeRequestError(w, err)
		return "", false
	}
	var v validator
	v.required("refresh_token", req.RefreshToken)
	v.maxLen("refresh_token", req.RefreshToken, 256) // bizim ürettiğimiz 26 karakter
	if err := v.err(); err != nil {
		writeRequestError(w, err)
		return "", false
	}
	return req.RefreshToken, true
}

// handleRefresh, POST /api/auth/refresh → 200 + YENİ oturum (eski refresh token ölür).
func handleRefresh(svc authService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := readRefreshToken(w, r)
		if !ok {
			return
		}
		sess, err := svc.Refresh(r.Context(), token)
		switch {
		case errors.Is(err, auth.ErrInvalidToken):
			writeError(w, http.StatusUnauthorized, "invalid_token", "geçersiz ya da süresi dolmuş refresh token")
		case err != nil:
			writeInternalError(w, "Refresh", err)
		default:
			writeJSON(w, http.StatusOK, toSessionResponse(sess))
		}
	}
}

// handleLogout, POST /api/auth/logout → 204. Token'ın sahibi olmak yeterli yetki:
// refresh token'ı bilen zaten o oturumu yenileyebilir, öldürebilmesi de doğal.
func handleLogout(svc authService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := readRefreshToken(w, r)
		if !ok {
			return
		}
		if err := svc.Logout(r.Context(), token); err != nil {
			writeInternalError(w, "Logout", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleMe, GET /api/auth/me (requireAuth arkasında) → token sahibinin güncel bilgisi.
func handleMe(svc authService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			// requireAuth'suz bağlanmış bir rota = programlama hatası. Kapalı başarısız ol.
			unauthorized(w, "missing_token", "kimlik doğrulaması gerekli")
			return
		}
		u, err := svc.Me(r.Context(), p.UserID)
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			// Token geçerli ama hesap silinmiş: kimlik artık yok.
			unauthorized(w, "invalid_token", "bu token'ın kullanıcısı artık yok")
		case err != nil:
			writeInternalError(w, "Me", err)
		default:
			writeJSON(w, http.StatusOK, toUserResponse(u))
		}
	}
}
