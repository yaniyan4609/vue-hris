package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

var themeColorCode = regexp.MustCompile(`^[a-z0-9_-]{1,20}$`)

func mountAuthContract(r chi.Router, db *sql.DB, config Options) {
	service := auth.Service{DB: db}
	r.With(checkOrigin(config)).Post("/api/v1/auth/switch-tenant", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			TenantID string `json:"tenant_id"`
		}
		if !readJSON(w, req, &body) {
			return
		}
		token, state, err := service.SelectTenant(req.Context(), cookieToken(req), body.TenantID, sessionDevice(req))
		if err != nil {
			writeContractAuthError(w, req, err)
			return
		}
		setSessionCookie(w, token, state.Remember, config.CookieSecure)
		writeAPIData(w, 200, state, nil)
	})
	r.With(checkOrigin(config)).Post("/api/v1/auth/password/change", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if !readJSON(w, req, &body) {
			return
		}
		if err := service.ChangePassword(req.Context(), cookieToken(req), body.CurrentPassword, body.NewPassword); err != nil {
			switch {
			case errors.Is(err, auth.ErrPasswordPolicy):
				writeAPIError(w, req, 400, "VALIDATION_FAILED", "Kata sandi baru minimal 8 karakter.")
			case errors.Is(err, auth.ErrInvalidCredentials):
				writeAPIError(w, req, 403, "REAUTH_FAILED", "Kata sandi saat ini tidak benar.")
			default:
				writeContractAuthError(w, req, err)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Get("/api/v1/me/sessions", func(w http.ResponseWriter, req *http.Request) {
		items, err := service.ActiveSessions(req.Context(), cookieToken(req))
		if err != nil {
			writeContractAuthError(w, req, err)
			return
		}
		data := make([]map[string]any, 0, len(items))
		for _, item := range items {
			data = append(data, map[string]any{"id": item.ID, "device_label": item.DeviceLabel, "ip": item.IP, "remember": item.Remember, "current": item.Current, "created_at": item.CreatedAt, "last_seen_at": item.LastSeenAt})
		}
		writeAPIData(w, 200, data, nil)
	})
	r.With(checkOrigin(config)).Delete("/api/v1/me/sessions/{id}", func(w http.ResponseWriter, req *http.Request) {
		current, err := service.RevokeSession(req.Context(), cookieToken(req), chi.URLParam(req, "id"))
		if err != nil {
			if errors.Is(err, auth.ErrSessionNotFound) {
				writeAPIError(w, req, 404, "NOT_FOUND", "Sesi perangkat tidak ditemukan.")
			} else {
				writeContractAuthError(w, req, err)
			}
			return
		}
		if current {
			clearSessionCookie(w, config.CookieSecure)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Get("/api/v1/me/preferences", func(w http.ResponseWriter, req *http.Request) {
		state, err := service.Current(req.Context(), cookieToken(req))
		if err != nil {
			writeContractAuthError(w, req, err)
			return
		}
		var mode, color string
		err = db.QueryRowContext(req.Context(), `SELECT theme_mode,theme_color FROM user_preferences WHERE user_id=?`, state.User.ID).Scan(&mode, &color)
		if errors.Is(err, sql.ErrNoRows) {
			mode, color = "system", "default"
		} else if err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca preferensi.")
			return
		}
		writeAPIData(w, 200, map[string]string{"theme_mode": mode, "theme_color": color}, nil)
	})
	r.With(checkOrigin(config)).Patch("/api/v1/me/preferences", func(w http.ResponseWriter, req *http.Request) {
		state, err := service.Current(req.Context(), cookieToken(req))
		if err != nil {
			writeContractAuthError(w, req, err)
			return
		}
		var body struct {
			ThemeMode  *string `json:"theme_mode"`
			ThemeColor *string `json:"theme_color"`
		}
		if !readJSON(w, req, &body) {
			return
		}
		if body.ThemeMode == nil && body.ThemeColor == nil {
			writeAPIError(w, req, 400, "VALIDATION_FAILED", "Pilih preferensi yang akan diubah.")
			return
		}
		mode, color := "system", "default"
		err = db.QueryRowContext(req.Context(), `SELECT theme_mode,theme_color FROM user_preferences WHERE user_id=?`, state.User.ID).Scan(&mode, &color)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca preferensi.")
			return
		}
		if body.ThemeMode != nil {
			mode = strings.TrimSpace(*body.ThemeMode)
		}
		if body.ThemeColor != nil {
			color = strings.TrimSpace(*body.ThemeColor)
		}
		if mode != "system" && mode != "light" && mode != "dark" || !themeColorCode.MatchString(color) {
			writeAPIError(w, req, 400, "VALIDATION_FAILED", "Tema atau warna tidak valid.")
			return
		}
		if _, err := db.ExecContext(req.Context(), `INSERT INTO user_preferences (user_id,theme_mode,theme_color) VALUES (?,?,?) ON DUPLICATE KEY UPDATE theme_mode=VALUES(theme_mode),theme_color=VALUES(theme_color)`, state.User.ID, mode, color); err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menyimpan preferensi.")
			return
		}
		writeAPIData(w, 200, map[string]string{"theme_mode": mode, "theme_color": color}, nil)
	})
}

func writeContractAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		writeAPIError(w, r, 401, "AUTH_SESSION_INVALID", "Sesi tidak sah atau berakhir.")
	case errors.Is(err, auth.ErrTenantUnavailable):
		writeAPIError(w, r, 403, "TENANT_UNAVAILABLE", "Perusahaan tidak tersedia untuk akun ini.")
	case errors.Is(err, auth.ErrMFARequired):
		writeAPIError(w, r, 403, "MFA_REQUIRED", "Verifikasi dua langkah diperlukan.")
	default:
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Permintaan tidak dapat diproses.")
	}
}
