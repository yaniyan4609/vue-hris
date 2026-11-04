package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
)

type Config struct {
	Address  string
	User     string
	Password string
	Name     string
	TLS      string
}

func FromEnv() Config {
	return Config{
		Address:  os.Getenv("DB_ADDR"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
		TLS:      os.Getenv("DB_TLS"),
	}
}

func Open(ctx context.Context, config Config) (*sql.DB, error) {
	if config.Address == "" || config.User == "" || config.Password == "" || config.Name == "" {
		return nil, errors.New("DB_ADDR, DB_USER, DB_PASSWORD, dan DB_NAME wajib diisi")
	}
	cfg := mysql.NewConfig()
	cfg.User = config.User
	cfg.Passwd = config.Password
	cfg.Net = "tcp"
	cfg.Addr = config.Address
	cfg.DBName = config.Name
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.Params = map[string]string{"time_zone": "'+00:00'"}
	if config.TLS != "" {
		cfg.TLSConfig = config.TLS
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
