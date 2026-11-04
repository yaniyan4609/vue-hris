package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"digaji.id/hris/internal/database"
	"github.com/pressly/goose/v3"
	"golang.org/x/term"
)

func main() {
	action := flag.String("action", "up", "up atau status")
	flag.Parse()
	if *action != "up" && *action != "status" {
		slog.Error("aksi migrasi tidak dikenal")
		os.Exit(2)
	}
	config := database.FromEnv()
	if config.Password == "" && term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Kata sandi database: ")
		secret, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			slog.Error("gagal membaca kata sandi", "error", err)
			os.Exit(1)
		}
		config.Password = string(secret)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, config)
	if err != nil {
		slog.Error("gagal terhubung ke database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := goose.SetDialect("mysql"); err != nil {
		slog.Error("gagal menyiapkan migrasi", "error", err)
		os.Exit(1)
	}
	if *action == "up" {
		err = goose.UpContext(ctx, db, "migrations")
	} else {
		err = goose.StatusContext(ctx, db, "migrations")
	}
	if err != nil {
		slog.Error("migrasi gagal", "error", err)
		os.Exit(1)
	}
}
