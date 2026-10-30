package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the PostgreSQL reader and sync lock owner.
type Store struct {
	pool       *pgxpool.Pool
	staleAfter time.Duration
	now        func() time.Time
}

// New builds a store. staleAfter marks a successful collection as stale.
func New(pool *pgxpool.Pool, staleAfter time.Duration) *Store {
	return &Store{pool: pool, staleAfter: staleAfter, now: time.Now}
}

// Ping checks that the database accepts a query.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return unavailable(err)
	}
	return nil
}

func unavailable(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("%w: %s", domain.ErrUnavailable, err.Error())
}
