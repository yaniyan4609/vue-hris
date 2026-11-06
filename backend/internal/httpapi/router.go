package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Options struct {
	DemoLogin     bool
	CookieSecure  bool
	AllowedOrigin string
}

func Router(db *sql.DB, options ...Options) http.Handler {
	config := Options{CookieSecure: true}
	if len(options) > 0 {
		config = options[0]
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Request-Id", middleware.GetReqID(req.Context()))
			next.ServeHTTP(w, req)
		})
	})
	r.Use(middleware.Recoverer)
	r.Use(requireMutationCSRF)
	mountAPIDocs(r)
	r.Get("/api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(struct {
			Status string `json:"status"`
		}{Status: "ok"})
	})
	r.Get("/api/v1/ready", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if db == nil || db.PingContext(ctx) != nil {
			writeStatus(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		writeStatus(w, http.StatusOK, "ready")
	})
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { writeStatus(w, http.StatusOK, "ok") })
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if db == nil || db.PingContext(ctx) != nil {
			writeStatus(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		writeStatus(w, http.StatusOK, "ready")
	})
	if db != nil {
		mountAuth(r, db, config)
		mountAccess(r, db)
		mountAdmin(r, db, config)
		mountESS(r, db)
		mountPlatform(r, db, config)
		mountNotifications(r, db, config)
		mountRealtime(r, db, config)
	}
	return r
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
	}{Status: status})
}
