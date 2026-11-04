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
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.Router(db),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("API berjalan", "address", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("API berhenti", "error", err)
		os.Exit(1)
	}
}
