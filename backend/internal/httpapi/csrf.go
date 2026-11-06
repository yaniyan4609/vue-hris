package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

// The session token is random and HttpOnly; a domain-separated digest lets the
// same-origin client obtain a CSRF token without storing another secret in DB.
func csrfToken(sessionToken string) string {
	if sessionToken == "" {
		return ""
	}
	hash := sha256.Sum256([]byte("digaji-csrf-v1\x00" + sessionToken))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := csrfToken(cookieToken(r))
		provided := r.Header.Get("X-CSRF-Token")
		if expected == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			writeAPIError(w, r, http.StatusForbidden, "CSRF_INVALID", "Token keamanan permintaan tidak sah.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
