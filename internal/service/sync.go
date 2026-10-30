// Package service runs a sync without exposing the scraper to HTTP handlers.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// SyncSession is one locked sync attempt.
type SyncSession interface {
	Apply(ctx context.Context, snap domain.Snapshot) error
	Fail(ctx context.Context, cause error) error
	Close()
}

// Gate starts a sync or reports that one is already running.
type Gate interface {
	BeginSync(ctx context.Context, competition string, year int) (SyncSession, error)
}

// Collector downloads and parses one season.
type Collector interface {
	Fetch(ctx context.Context, year int, collected time.Time) (domain.Snapshot, error)
}

// Syncer publishes a validated snapshot or keeps the previous rows.
type Syncer struct {
	gate      Gate
	collector Collector
	limits    domain.Limits
	log       *slog.Logger
	now       func() time.Time
}

// New builds a syncer.
func New(gate Gate, collector Collector, limits domain.Limits, log *slog.Logger) *Syncer {
	if log == nil {
		log = slog.Default()
	}
	return &Syncer{gate: gate, collector: collector, limits: limits, log: log, now: time.Now}
}

// Sync collects one competition season. A failed collection does not publish.
func (s *Syncer) Sync(ctx context.Context, competition string, year int) error {
	session, err := s.gate.BeginSync(ctx, competition, year)
	if err != nil {
		if errors.Is(err, domain.ErrBusy) {
			s.log.Info("sync skipped because another run holds the lock", "competition", competition, "year", year)
		}
		return err
	}
	defer session.Close()
	s.log.Info("sync started", "competition", competition, "year", year)
	snap, err := s.collector.Fetch(ctx, year, s.now().UTC())
	if err != nil {
		return s.fail(ctx, session, competition, year, err)
	}
	if err := snap.Validate(s.limits); err != nil {
		return s.fail(ctx, session, competition, year, err)
	}
	if err := session.Apply(ctx, snap); err != nil {
		return s.fail(ctx, session, competition, year, err)
	}
	s.log.Info("sync finished", "competition", competition, "year", year, "matches", len(snap.Matches), "standings", len(snap.Standings))
	return nil
}

func (s *Syncer) fail(ctx context.Context, session SyncSession, competition string, year int, cause error) error {
	s.log.Error("sync failed", "competition", competition, "year", year, "error", cause.Error())
	if ferr := session.Fail(ctx, cause); ferr != nil {
		return fmt.Errorf("%w; recording the failure also failed: %s", cause, ferr.Error())
	}
	return cause
}
