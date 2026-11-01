package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/czarist/api_brasileirao/internal/catalog"
	"github.com/czarist/api_brasileirao/internal/config"
	"github.com/czarist/api_brasileirao/internal/domain"
	"github.com/czarist/api_brasileirao/internal/repository/postgres"
	"github.com/czarist/api_brasileirao/internal/scraper/cbf"
	"github.com/czarist/api_brasileirao/internal/service"
)

func main() {
	once := flag.Bool("once", false, "collect the selected seasons and exit")
	competitionFlag := flag.String("competition", "", "competition code, a comma-separated list, or all (default: SYNC_COMPETITION)")
	yearFlag := flag.Int("year", 0, "one season year; overrides SYNC_YEARS=all")
	flag.Parse()

	config.LoadDotEnv(".env")
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration", "error", err.Error())
		os.Exit(1)
	}
	log := config.NewLogger(cfg.LogLevel)
	spec := cfg.SyncCompetition
	if *competitionFlag != "" {
		spec = *competitionFlag
	}
	codes, err := catalog.Parse(spec)
	if err != nil {
		log.Error("competition", "error", err.Error())
		os.Exit(1)
	}
	allYears := cfg.SyncAllYears && *yearFlag == 0
	year := *yearFlag
	if !allYears {
		if year == 0 {
			year = cfg.SyncYear
		}
		if year == 0 {
			year, err = cbf.CurrentYear(time.Now())
			if err != nil {
				log.Error("season year", "error", err.Error())
				os.Exit(1)
			}
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
	fmfClient, err := newFMFClient(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	portalSource, err := newPortalSource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	uikitSource, err := newUikitSource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	fafalSource, err := newFafalSource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	fafamSource, err := newFAFAMSource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	fbfSource, err := newFBFSource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	fcfSource, err := newFCFSource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	fpfprSource, err := newFPFPRSource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	fgfrsSource, err := newFGFRSSource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	fsfseSource, err := newFSFSESource(cfg.SourceTimeout, cfg.SourceMinInterval, cfg.SourceMaxBody, cfg.SourceRetries)
	if err != nil {
		log.Error("source client", "error", err.Error())
		os.Exit(1)
	}
	source := newRouter(client, fmfClient, portalSource, uikitSource, fafalSource, fafamSource, fbfSource, fcfSource, fpfprSource, fgfrsSource, fsfseSource)
	syncer := service.New(storeGate{Store: postgres.New(pool, cfg.StaleAfter)}, source, domain.Limits{
		MinMatches:   cfg.MinMatches,
		MinStandings: cfg.MinStandings,
		MaxStandings: cfg.MaxStandings,
	}, log)
	syncer.SetLimitsFor(limitsFor)

	run := func() error {
		source.Reset()
		planCtx, cancel := context.WithTimeout(ctx, cfg.SyncTimeout)
		plans, err := source.Plan(planCtx, codes)
		cancel()
		if err != nil {
			return err
		}
		if !allYears {
			filtered := make([]seasonPlan, 0, len(codes))
			for _, plan := range plans {
				if plan.year == year {
					filtered = append(filtered, plan)
				}
			}
			plans = filtered
		}
		if len(plans) == 0 {
			return fmt.Errorf("catalog has no seasons for the requested competitions")
		}
		log.Info("sync plan", "seasons", len(plans))
		var failed int
		for _, plan := range plans {
			if err := ctx.Err(); err != nil {
				return err
			}
			runCtx, runCancel := context.WithTimeout(ctx, cfg.SyncTimeout)
			err := syncer.Sync(runCtx, plan.code, plan.year)
			runCancel()
			if err != nil {
				if ctx.Err() != nil {
					return err
				}
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("%d season syncs failed", failed)
		}
		return nil
	}
	if *once {
		if err := run(); err != nil {
			log.Error("sync finished with failures", "error", err.Error())
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
