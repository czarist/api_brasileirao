package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/czarist/api_brasileirao/internal/config"
	"github.com/czarist/api_brasileirao/internal/httpapi"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, "api")
	if err != nil {
		log.Error("database", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()
	handler := httpapi.New(postgres.New(pool, cfg.StaleAfter), httpapi.Options{
		CORSOrigins: cfg.CORSOrigins,
		RateRPS:     cfg.RateLimitRPS,
		RateBurst:   cfg.RateLimitBurst,
		TrustProxy:  cfg.TrustProxy,
		Log:         log,
	})
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr)
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("api stopped", "error", err.Error())
			os.Exit(1)
		}
		return
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "error", err.Error())
		os.Exit(1)
	}
}
