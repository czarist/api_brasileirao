package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/czarist/api_brasileirao/internal/config"
	"github.com/czarist/api_brasileirao/internal/domain"
	"github.com/czarist/api_brasileirao/internal/repository/postgres"
	"github.com/czarist/api_brasileirao/internal/scraper/cbf"
	"github.com/czarist/api_brasileirao/internal/service"
)

func main() {
	once := flag.Bool("once", false, "collect one season and exit")
	competitionFlag := flag.String("competition", "", "competition code (default: SYNC_COMPETITION)")
	yearFlag := flag.Int("year", 0, "season year (default: SYNC_YEAR, or the current year in America/Sao_Paulo)")
	flag.Parse()

	config.LoadDotEnv(".env")
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration", "error", err.Error())
		os.Exit(1)
	}
	log := config.NewLogger(cfg.LogLevel)
	competition := cfg.SyncCompetition
	if *competitionFlag != "" {
		competition = *competitionFlag
	}
	if competition != cbf.CompetitionCode {
		log.Error("unsupported competition", "competition", competition)
		os.Exit(1)
	}
	year := cfg.SyncYear
	if *yearFlag != 0 {
		year = *yearFlag
	}
	if year == 0 {
		year, err = cbf.CurrentYear(time.Now())
		if err != nil {
			log.Error("season year", "error", err.Error())
			os.Exit(1)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, "worker")
	if err != nil {
		log.Error("database", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()
	client, err := cbf.NewClient(cbf.ClientOptions{
		BaseURL:     cfg.SourceBaseURL,
		Timeout:     cfg.SourceTimeout,
		MinInterval: cfg.SourceMinInterval,
		MaxBody:     cfg.SourceMaxBody,
		Retries:     cfg.SourceRetries,
	})
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	syncer := service.New(storeGate{Store: postgres.New(pool, cfg.StaleAfter)}, cbf.NewSource(client), domain.Limits{
		MinMatches:   cfg.MinMatches,
		MinStandings: cfg.MinStandings,
		MaxStandings: cfg.MaxStandings,
	}, log)

	run := func() error {
		runCtx, cancel := context.WithTimeout(ctx, cfg.SyncTimeout)
		defer cancel()
		return syncer.Sync(runCtx, competition, year)
	}
	if *once {
		if err := run(); err != nil {
			os.Exit(1)
		}
		return
	}

	var busy sync.Mutex
	collect := func() {
		if !busy.TryLock() {
			log.Info("sync skipped because this process is still collecting")
			return
		}
		defer busy.Unlock()
		if err := run(); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("sync will retry on the next interval", "error", err.Error())
		}
	}
	collect()
	ticker := time.NewTicker(cfg.SyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped")
			return
		case <-ticker.C:
			collect()
		}
	}
}

// storeGate adapts the repository session to the service interface.
type storeGate struct {
	*postgres.Store
}

func (g storeGate) BeginSync(ctx context.Context, competition string, year int) (service.SyncSession, error) {
	return g.Store.BeginSync(ctx, competition, year)
}
