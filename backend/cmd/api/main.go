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
			processed, err := httpapi.RunDueTerminations(workerContext, db)
			if err != nil {
				slog.Error("penerapan terminasi gagal", "error", err)
			}
			if processed > 0 {
				slog.Info("terminasi diterapkan", "count", processed)
			}
			applied, err := httpapi.RunDueEmployeeChanges(workerContext, db)
			if err != nil {
				slog.Error("penerapan perubahan karyawan gagal", "error", err)
			}
			if applied > 0 {
				slog.Info("perubahan karyawan diterapkan", "count", applied)
			}
			generated, err := httpapi.RunUpcomingShiftInstances(workerContext, db)
			if err != nil {
				slog.Error("materialisasi jadwal mendatang gagal", "error", err)
			}
			if generated > 0 {
				slog.Info("shift instance mendatang dibuat", "count", generated)
			}
			credited, err := httpapi.RunDueLeaveAccruals(workerContext, db)
			if err != nil {
				slog.Error("akrual cuti gagal", "error", err)
			}
			if credited > 0 {
				slog.Info("saldo cuti tahunan diberikan", "count", credited)
			}
			notified, err := httpapi.RunNotificationOutbox(workerContext, db)
			if err != nil {
				slog.Error("dispatcher notifikasi gagal", "error", err)
			}
			if notified > 0 {
				slog.Info("outbox notifikasi diproses", "count", notified)
			}
			broadcasts, err := httpapi.RunDueBroadcasts(workerContext, db)
			if err != nil {
				slog.Error("pengiriman broadcast gagal", "error", err)
			}
			if broadcasts > 0 {
				slog.Info("broadcast diproses", "count", broadcasts)
			}
			pushes, err := httpapi.RunPushDeliveries(workerContext, db)
			if err != nil {
				slog.Error("pengiriman push gagal", "error", err)
			}
			if pushes > 0 {
				slog.Info("push diproses", "count", pushes)
			}
			reminders, err := httpapi.RunDueReminders(workerContext, db)
			if err != nil {
				slog.Error("pengingat presensi gagal", "error", err)
			}
			if reminders > 0 {
				slog.Info("pengingat presensi diproses", "count", reminders)
			}
			emails, err := httpapi.RunEmailDeliveries(workerContext, db)
			if err != nil {
				slog.Error("pengiriman email notifikasi gagal", "error", err)
			}
			if emails > 0 {
				slog.Info("email notifikasi diproses", "count", emails)
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
