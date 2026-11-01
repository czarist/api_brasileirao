package fmf

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

// Source collects the Mineiro professional divisions from www.fmf.com.br.
type Source struct {
	client *Client

	mu    sync.Mutex
	pages map[string][]byte
}

// NewSource binds the adapter to a client. Paths are fixed templates.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Reset drops the cached division pages so the next collection reads them again.
func (s *Source) Reset() {
	s.mu.Lock()
	s.pages = nil
	s.mu.Unlock()
}

// Plan lists every catalog season for the requested competition codes.
func (s *Source) Plan(ctx context.Context, codes []string) ([]SeasonPlan, error) {
	plans := make([]SeasonPlan, 0, len(codes))
	for _, code := range codes {
		comp, ok := Lookup(code)
		if !ok {
			return nil, fmt.Errorf("unsupported competition %s", code)
		}
		seasons, err := s.seasons(ctx, comp)
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

// Fetch downloads one division season. It does not write a database.
func (s *Source) Fetch(ctx context.Context, competition string, year int, collected time.Time) (domain.Snapshot, error) {
	comp, ok := Lookup(competition)
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("unsupported competition %s", competition)
	}
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	seasons, err := s.seasons(ctx, comp)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var season Season
	for _, item := range seasons {
		if item.Year == year {
			season = item
			break
		}
	}
	if season.ExternalID == "" {
		return domain.Snapshot{}, fmt.Errorf("catalog has no %s season for %d", competition, year)
	}
	body, err := s.client.PostSeason(ctx, comp.Division, season.ExternalID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	page, err := ParsePage(body, comp.Division)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if page.Season.Year != year || page.Season.ExternalID != season.ExternalID {
		return domain.Snapshot{}, fmt.Errorf("season page returned %s %d instead of %d", page.Season.ExternalID, page.Season.Year, year)
	}
	return domain.Snapshot{
		Source:           domain.SourceFMF,
		CompetitionCode:  comp.Code,
		CompetitionName:  comp.Name,
		SeasonYear:       year,
		SeasonName:       page.Season.Name,
		SeasonExternalID: page.Season.ExternalID,
		Teams:            page.Teams,
		Matches:          page.Matches,
		Standings:        page.Standings,
		CollectedAt:      collected.UTC(),
	}, nil
}

func (s *Source) seasons(ctx context.Context, comp Competition) ([]Season, error) {
	body, err := s.page(ctx, comp.Division)
	if err != nil {
		return nil, err
	}
	return ParseCatalog(body, comp.Division)
}

func (s *Source) page(ctx context.Context, division string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pages == nil {
		s.pages = map[string][]byte{}
	}
	if body, ok := s.pages[division]; ok {
		return body, nil
	}
	body, err := s.client.GetDivision(ctx, division)
	if err != nil {
		return nil, err
	}
	s.pages[division] = append([]byte(nil), body...)
	return s.pages[division], nil
}
