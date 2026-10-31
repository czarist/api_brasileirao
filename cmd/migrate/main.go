package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/czarist/api_brasileirao/internal/config"
	"github.com/czarist/api_brasileirao/internal/migrate"
	"github.com/czarist/api_brasileirao/internal/repository/postgres"
)

func main() {
	config.LoadDotEnv(".env")
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration", "error", err.Error())
		os.Exit(1)
	}
	log := config.NewLogger(cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, "migrate")
	if err != nil {
		log.Error("database", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()
	if err := migrate.Up(ctx, pool); err != nil {
		log.Error("migration failed", "error", err.Error())
		os.Exit(1)
	}
	log.Info("migrations complete")
}
