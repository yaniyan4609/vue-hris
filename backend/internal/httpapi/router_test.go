package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	Router(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected response: %d %q", rec.Code, rec.Body.String())
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Status != "ok" {
		t.Fatalf("invalid health body: %+v, %v", body, err)
	}
}

func TestNotificationRoutesRequireSession(t *testing.T) {
	r := chi.NewRouter()
	mountNotifications(r, &sql.DB{}, Options{})
	for _, path := range []string{"/api/v1/notifications", "/api/v1/notifications/sync", "/api/v1/notifications/unread-count"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", path, rec.Code)
		}
	}
}

func TestRealtimeOriginValidation(t *testing.T) {
	cases := []struct {
		origin, configured string
		allowed            bool
	}{
		{"http://127.0.0.1:5174", "http://127.0.0.1:5174", true},
		{"http://evil.test", "http://127.0.0.1:5174", false},
		{"", "", false},
		{"http://evil.test", "", false},
		{"http://127.0.0.1:5174/other", "", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:5174/api/v1/realtime/ws", nil)
		req.Header.Set("Origin", tc.origin)
		if got := allowedRealtimeOrigin(req, tc.configured); got != tc.allowed {
			t.Errorf("origin %q allowed=%v, want %v", tc.origin, got, tc.allowed)
		}
	}
}

func TestReadyWithoutDatabase(t *testing.T) {
	rec := httptest.NewRecorder()
	Router(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
