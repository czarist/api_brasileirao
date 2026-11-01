package cbf

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// SeasonPlan is one catalog season the worker can collect.
type SeasonPlan struct {
	Competition Competition
	Year        int
	ExternalID  string
}

// Source collects the supported divisions from the configured origin.
type Source struct {
	client *Client

	mu      sync.Mutex
	catalog []byte
}

// NewSource binds the adapter to a client. Paths are fixed templates.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Reset drops the cached catalog so the next collection reads it again.
func (s *Source) Reset() {
	s.mu.Lock()
	s.catalog = nil
	s.mu.Unlock()
}

// Plan lists every catalog season for the requested division codes.
// A category the catalog omits contributes no seasons.
func (s *Source) Plan(ctx context.Context, codes []string) ([]SeasonPlan, error) {
	body, err := s.catalogBody(ctx)
	if err != nil {
		return nil, err
	}
	plans := make([]SeasonPlan, 0, len(codes))
	for _, code := range codes {
		comp, ok := Lookup(code)
		if !ok {
			return nil, fmt.Errorf("unsupported competition %s", code)
		}
		seasons, err := CatalogSeasons(body, comp.CatalogID, comp.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", code, err)
		}
		for _, season := range seasons {
			plans = append(plans, SeasonPlan{Competition: comp, Year: season.Year, ExternalID: season.ExternalID})
		}
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Competition.Code != plans[j].Competition.Code {
			return plans[i].Competition.Code < plans[j].Competition.Code
		}
		return plans[i].Year < plans[j].Year
	})
	return plans, nil
}

// Fetch downloads the catalog, the fixture list, schedule changes, and the
// standings page. It does not write a database.
func (s *Source) Fetch(ctx context.Context, competition string, year int, collected time.Time) (domain.Snapshot, error) {
	comp, ok := Lookup(competition)
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("unsupported competition %s", competition)
	}
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	body, err := s.catalogBody(ctx)
	if err != nil {
		return domain.Snapshot{}, err
	}
	externalID, err := SeasonExternalID(body, comp.CatalogID, comp.CategoryID, year)
	if err != nil {
		return domain.Snapshot{}, err
	}
	matches, err := s.client.GetJSON(ctx, "/api/cbf/jogos/tabela-detalhada/campeonato/"+externalID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	imt, err := s.client.GetJSON(ctx, "/api/cbf/campeonatos/"+externalID+"/imt")
	if err != nil {
		return domain.Snapshot{}, err
	}
	var standings []byte
	if comp.MaxRows > 0 {
		standings, err = s.standingsPage(ctx, comp, year)
		if err != nil {
			return domain.Snapshot{}, err
		}
	}
	return Assemble(comp, year, externalID, collected, matches, imt, standings)
}

func (s *Source) catalogBody(ctx context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.catalog != nil {
		return s.catalog, nil
	}
	body, err := s.client.GetJSON(ctx, "/api/cbf/campeonatos/anos")
	if err != nil {
		return nil, err
	}
	s.catalog = append([]byte(nil), body...)
	return s.catalog, nil
}

func (s *Source) standingsPage(ctx context.Context, comp Competition, year int) ([]byte, error) {
	if comp.Slug == "" {
		return nil, fmt.Errorf("competition %s has no standings page", comp.Code)
	}
	path := fmt.Sprintf("/futebol-brasileiro/tabelas/campeonato-brasileiro/%s/%d", comp.Slug, year)
	body, err := s.client.GetHTML(ctx, path)
	if err != nil {
		return nil, err
	}
	rows, _, parseErr := ParseStandings(body)
	if parseErr == nil && len(rows) > 0 {
		return body, nil
	}
	phaseID, ok := firstPointsPhase(body)
	if !ok {
		if parseErr != nil {
			return nil, parseErr
		}
		return nil, fmt.Errorf("standings page has no table")
	}
	return s.client.GetHTML(ctx, path+"/"+phaseID)
}
