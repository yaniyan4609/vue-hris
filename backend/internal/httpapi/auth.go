package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

const (
	sessionCookieName       = "dgj_session"
	legacySessionCookieName = "digaji_session"
)

func mountAuth(r chi.Router, db *sql.DB, config Options) {
	service := auth.Service{DB: db}
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Get("/csrf", func(w http.ResponseWriter, req *http.Request) {
			if _, err := service.Pending(req.Context(), cookieToken(req)); err != nil {
				writeAPIError(w, req, http.StatusUnauthorized, "AUTH_SESSION_INVALID", "Sesi tidak sah atau berakhir.")
				return
			}
			writeAPIData(w, http.StatusOK, map[string]string{"csrf_token": csrfToken(cookieToken(req))}, nil)
		})
		r.With(checkOrigin(config)).Post("/reauth", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				Password string `json:"password"`
			}
			if !readJSON(w, req, &body) {
				return
			}
			until, err := service.Reauthenticate(req.Context(), cookieToken(req), body.Password)
			if err != nil {
				switch {
				case errors.Is(err, auth.ErrInvalidCredentials):
					writeAPIError(w, req, 403, "REAUTH_FAILED", "Kata sandi tidak benar.")
				case errors.Is(err, auth.ErrUnauthenticated):
					writeAPIError(w, req, 401, "AUTH_SESSION_INVALID", "Sesi tidak sah atau berakhir.")
				default:
					writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal memverifikasi ulang akun.")
				}
				return
			}
			writeAPIData(w, 200, map[string]any{"valid_until": until}, nil)
		})
		r.Get("/me", func(w http.ResponseWriter, req *http.Request) {
			state, err := service.Pending(req.Context(), cookieToken(req))
			if err != nil {
				writeAuthError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, state)
		})
		r.With(checkOrigin(config)).Post("/login", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				Email    string `json:"email"`
				Password string `json:"password"`
				Remember bool   `json:"remember"`
			}
			if !readJSON(w, req, &body) {
				return
			}
			if len(body.Email) > 320 || len(body.Password) > 1024 || strings.TrimSpace(body.Email) == "" || body.Password == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email dan kata sandi wajib diisi"})
				return
			}
			token, state, err := service.Login(req.Context(), body.Email, body.Password, body.Remember, sessionDevice(req))
			if err != nil {
				writeAuthError(w, err)
				return
			}
			setSessionCookie(w, token, state.Remember, config.CookieSecure)
			writeJSON(w, http.StatusOK, state)
		})
		r.With(checkOrigin(config)).Post("/select-tenant", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				TenantID string `json:"tenantId"`
			}
			if !readJSON(w, req, &body) {
				return
			}
			token, state, err := service.SelectTenant(req.Context(), cookieToken(req), body.TenantID, sessionDevice(req))
			if err != nil {
				writeAuthError(w, err)
				return
			}
			setSessionCookie(w, token, state.Remember, config.CookieSecure)
			writeJSON(w, http.StatusOK, state)
		})
		r.With(checkOrigin(config)).Post("/logout", func(w http.ResponseWriter, req *http.Request) {
			if err := service.Logout(req.Context(), cookieToken(req)); err != nil {
				writeAuthError(w, err)
				return
			}
			clearSessionCookie(w, config.CookieSecure)
			w.WriteHeader(http.StatusNoContent)
		})
		r.Get("/sessions", func(w http.ResponseWriter, req *http.Request) {
			items, err := service.ActiveSessions(req.Context(), cookieToken(req))
			if err != nil {
				writeAuthError(w, err)
				return
			}
			writeJSON(w, 200, items)
		})
		r.With(checkOrigin(config)).Post("/logout-all", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				ExceptCurrent bool `json:"except_current"`
			}
			if !readJSON(w, req, &body) {
				return
			}
			if err := service.LogoutAllExceptCurrent(req.Context(), cookieToken(req), body.ExceptCurrent); err != nil {
				writeAuthError(w, err)
				return
			}
			if !body.ExceptCurrent {
				clearSessionCookie(w, config.CookieSecure)
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r.With(checkOrigin(config)).Post("/change-password", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				CurrentPassword string `json:"currentPassword"`
				NewPassword     string `json:"newPassword"`
			}
			if !readJSON(w, req, &body) {
				return
			}
			if err := service.ChangePassword(req.Context(), cookieToken(req), body.CurrentPassword, body.NewPassword); err != nil {
				switch {
				case errors.Is(err, auth.ErrPasswordPolicy):
					writeJSON(w, 400, map[string]string{"error": "Kata sandi baru minimal 8 karakter"})
				case errors.Is(err, auth.ErrInvalidCredentials):
					writeJSON(w, 400, map[string]string{"error": "Kata sandi saat ini salah atau sama dengan yang baru"})
				default:
					writeAuthError(w, err)
				}
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		if config.DemoLogin {
			r.Get("/demo-accounts", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, http.StatusOK, []map[string]string{
					{"id": "super", "email": "super@digaji.id", "label": "Superadmin"},
					{"id": "admin", "email": "admin@digaji.id", "label": "Admin Tenant"},
					{"id": "ess", "email": "ess@digaji.id", "label": "Karyawan ESS"},
				})
			})
			r.With(checkOrigin(config)).Post("/demo-login", func(w http.ResponseWriter, req *http.Request) {
				var body struct {
					Account string `json:"account"`
				}
				if !readJSON(w, req, &body) {
					return
				}
				token, state, err := service.DemoLogin(req.Context(), body.Account, sessionDevice(req))
				if err != nil {
					writeAuthError(w, err)
					return
				}
				setSessionCookie(w, token, false, config.CookieSecure)
				writeJSON(w, http.StatusOK, state)
			})
		}
		mountMFA(r, service, config)
		mountPasswordReset(r, service, config)
	})
	mountAuthContract(r, db, config)
}

func sessionDevice(req *http.Request) auth.SessionDevice {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	return auth.SessionDevice{UserAgent: req.UserAgent(), IP: net.ParseIP(host)}
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	for _, name := range []string{sessionCookieName, legacySessionCookieName} {
		http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	}
}

func cookieToken(req *http.Request) string {
	cookie, err := req.Cookie(sessionCookieName)
	if err != nil {
		cookie, err = req.Cookie(legacySessionCookieName)
		if err != nil {
			return ""
		}
	}
	return cookie.Value
}

func setSessionCookie(w http.ResponseWriter, token string, remember, secure bool) {
	cookie := &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode}
	if remember {
		cookie.MaxAge = 400 * 24 * 60 * 60
	}
	http.SetCookie(w, cookie)
	http.SetCookie(w, &http.Cookie{Name: legacySessionCookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func readJSON(w http.ResponseWriter, req *http.Request, dst any) bool {
	if !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type harus application/json"})
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, req.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON tidak valid"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON tidak valid"})
		return false
	}
	return true
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Email atau kata sandi salah"})
	case errors.Is(err, auth.ErrUnauthenticated):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Sesi tidak sah atau berakhir"})
	case errors.Is(err, auth.ErrTenantUnavailable):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Perusahaan tidak tersedia"})
	case errors.Is(err, auth.ErrMFARequired):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Verifikasi dua langkah diperlukan", "code": "MFA_REQUIRED"})
	case errors.Is(err, auth.ErrMFAUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Kunci MFA platform belum dikonfigurasi", "code": "MFA_UNAVAILABLE"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Terjadi kesalahan server"})
	}
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func checkOrigin(config Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "Asal permintaan ditolak"})
				return
			}
			origin := req.Header.Get("Origin")
			if origin != "" && origin != config.AllowedOrigin && origin != "http://"+req.Host && origin != "https://"+req.Host {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "Asal permintaan ditolak"})
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
