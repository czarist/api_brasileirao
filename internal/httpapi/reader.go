package httpapi

import (
	"context"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// Reader loads published data. Implementations must not call the source site.
type Reader interface {
	Ping(ctx context.Context) error
	ListCompetitions(ctx context.Context) ([]domain.Competition, domain.SourceMeta, error)
	GetCompetition(ctx context.Context, code string) (domain.Competition, domain.SourceMeta, error)
	ListSeasons(ctx context.Context, code string) ([]domain.Season, domain.SourceMeta, error)
	GetSeason(ctx context.Context, code string, year int) (domain.Season, domain.SourceMeta, error)
	ListTeams(ctx context.Context, code string, year int) ([]domain.Team, domain.SourceMeta, error)
	ListStandings(ctx context.Context, code string, year int) ([]domain.Standing, domain.SourceMeta, error)
	ListRounds(ctx context.Context, code string, year int) ([]domain.Round, domain.SourceMeta, error)
	ListMatches(ctx context.Context, code string, year int, filter domain.MatchFilter) (domain.Page[domain.Match], error)
	GetMatch(ctx context.Context, id int64) (domain.Match, domain.SourceMeta, error)
}
