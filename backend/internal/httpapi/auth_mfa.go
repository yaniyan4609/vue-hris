package httpapi

import (
	"errors"
	"net/http"

	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

func mountMFA(r chi.Router, service auth.Service, config Options) {
	r.Get("/mfa/status", func(w http.ResponseWriter, req *http.Request) {
		required, enrolled, err := service.MFAStatus(req.Context(), cookieToken(req))
		if err != nil {
			writeMFAError(w, req, err)
			return
		}
		writeAPIData(w, 200, map[string]bool{"required": required, "enrolled": enrolled}, nil)
	})
	r.With(checkOrigin(config)).Post("/mfa/setup", func(w http.ResponseWriter, req *http.Request) {
		secret, uri, err := service.MFASetup(req.Context(), cookieToken(req))
		if err != nil {
			writeMFAError(w, req, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeAPIData(w, 200, map[string]string{"secret": secret, "otpauth_uri": uri}, nil)
	})
	r.With(checkOrigin(config)).Post("/mfa/confirm", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Code string `json:"code"`
		}
		if !readJSON(w, req, &body) {
			return
		}
		codes, state, err := service.MFAConfirm(req.Context(), cookieToken(req), body.Code)
		if err != nil {
			writeMFAError(w, req, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeAPIData(w, 200, map[string]any{"recovery_codes": codes, "state": state}, nil)
	})
	r.With(checkOrigin(config)).Post("/mfa/verify", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Code string `json:"code"`
		}
		if !readJSON(w, req, &body) {
			return
		}
		state, err := service.MFAVerify(req.Context(), cookieToken(req), body.Code)
		if err != nil {
			writeMFAError(w, req, err)
			return
		}
		writeAPIData(w, 200, state, nil)
	})
}

func writeMFAError(w http.ResponseWriter, req *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		writeAPIError(w, req, http.StatusUnauthorized, "AUTH_SESSION_INVALID", "Sesi tidak sah atau berakhir.")
	case errors.Is(err, auth.ErrMFAInvalid):
		writeAPIError(w, req, http.StatusForbidden, "MFA_INVALID", "Kode authenticator atau pemulihan tidak sah.")
	case errors.Is(err, auth.ErrMFASetupRequired):
		writeAPIError(w, req, http.StatusConflict, "MFA_SETUP_REQUIRED", "Aktifkan authenticator terlebih dahulu.")
	case errors.Is(err, auth.ErrMFAEnrolled):
		writeAPIError(w, req, http.StatusConflict, "MFA_ENROLLED", "Authenticator sudah aktif.")
	case errors.Is(err, auth.ErrMFAUnavailable):
		writeAPIError(w, req, http.StatusServiceUnavailable, "MFA_UNAVAILABLE", "Kunci MFA platform belum dikonfigurasi.")
	case errors.Is(err, auth.ErrTenantUnavailable), errors.Is(err, auth.ErrMFARequired):
		writeAPIError(w, req, http.StatusForbidden, "FORBIDDEN", "Akses ditolak.")
	default:
		writeAPIError(w, req, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memproses authenticator.")
	}
}
