package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/httpapi"
)

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		slog.Error("database tidak siap", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	go func() {
		run := func() {
			workerContext, stop := context.WithTimeout(context.Background(), 45*time.Second)
			defer stop()
			applied, err := httpapi.RunDueEmployeeChanges(workerContext, db)
			if err != nil {
				slog.Error("penerapan perubahan karyawan gagal", "error", err)
			}
			if applied > 0 {
				slog.Info("perubahan karyawan diterapkan", "count", applied)
			}
		}
		run()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	}()
	server := &http.Server{
		Addr: addr,
		Handler: httpapi.Router(db, httpapi.Options{
			DemoLogin:     os.Getenv("APP_ENV") == "staging" && os.Getenv("ENABLE_DEMO_LOGIN") == "true" && os.Getenv("DB_NAME") == "staging-saas-hris",
			CookieSecure:  os.Getenv("COOKIE_SECURE") != "false",
			AllowedOrigin: os.Getenv("APP_ORIGIN"),
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("API berjalan", "address", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("API berhenti", "error", err)
		os.Exit(1)
	}
}
