package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"digaji.id/hris/internal/httpapi"
)

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("API berjalan", "address", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("API berhenti", "error", err)
		os.Exit(1)
	}
}
