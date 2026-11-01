package fcf

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// SeasonPlan is one catalog season the worker can collect.
type SeasonPlan struct {
	Code string
	Year int
}

// Source collects the adult Catarinense. The client stays on fcf.com.br.
type Source struct {
	client *Client

	mu     sync.Mutex
	loaded bool
	refs   []SeasonRef
}

// NewSource binds the adapter to its client.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Reset drops the cached catalog.
func (s *Source) Reset() {
	s.mu.Lock()
	s.loaded = false
	s.refs = nil
	s.mu.Unlock()
}

// Plan lists every published adult season for the requested codes.
// A code the catalog does not list is omitted.
func (s *Source) Plan(ctx context.Context, codes []string) ([]SeasonPlan, error) {
	refs, err := s.catalog(ctx)
	if err != nil {
		return nil, err
	}
	wanted := map[string]struct{}{}
	for _, code := range codes {
		if _, ok := Lookup(code); !ok {
			return nil, fmt.Errorf("unsupported competition %s", code)
		}
		wanted[code] = struct{}{}
	}
	var plans []SeasonPlan
	for _, ref := range refs {
		if _, ok := wanted[ref.Code]; !ok {
			continue
		}
		plans = append(plans, SeasonPlan{Code: ref.Code, Year: ref.Year})
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Code != plans[j].Code {
			return plans[i].Code < plans[j].Code
		}
		return plans[i].Year < plans[j].Year
	})
	return plans, nil
}

// Fetch collects one season. A failed page is returned as an error and does
// not replace rows by itself; the sync service keeps the previous snapshot.
func (s *Source) Fetch(ctx context.Context, competition string, year int, collected time.Time) (domain.Snapshot, error) {
	name, ok := Lookup(competition)
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("unsupported competition %s", competition)
	}
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	refs, err := s.catalog(ctx)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var id string
	for _, ref := range refs {
		if ref.Code == competition && ref.Year == year {
			id = ref.ID
			break
		}
	}
	if id == "" {
		return domain.Snapshot{}, fmt.Errorf("catalog has no %s season for %d", competition, year)
	}
	body, err := s.client.GetSeason(ctx, strconv.Itoa(year), id)
	if err != nil {
		return domain.Snapshot{}, err
	}
	loc, err := saopaulo()
	if err != nil {
		return domain.Snapshot{}, err
	}
	parsed, err := ParsePage(string(body), year, loc)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if len(parsed.matches) == 0 {
		return domain.Snapshot{}, fmt.Errorf("season %d has no matches", year)
	}
	if err := uniqueMatches(parsed.matches); err != nil {
		return domain.Snapshot{}, err
	}
	standings := mergeStandings(parsed.tables)
	if len(standings) == 0 {
		return domain.Snapshot{}, fmt.Errorf("season %d has no standings", year)
	}
	teams, err := teamsFrom(parsed.matches, standings, parsed.clubs)
	if err != nil {
		return domain.Snapshot{}, err
	}
	return domain.Snapshot{
		Source:           source,
		CompetitionCode:  competition,
		CompetitionName:  name,
		SeasonYear:       year,
		SeasonName:       fmt.Sprintf("%s %d", name, year),
		SeasonExternalID: id + "-" + strconv.Itoa(year),
		Teams:            teams,
		Matches:          parsed.matches,
		Standings:        standings,
		CollectedAt:      collected.UTC(),
	}, nil
}

func (s *Source) catalog(ctx context.Context) ([]SeasonRef, error) {
	s.mu.Lock()
	if s.loaded {
		refs := append([]SeasonRef(nil), s.refs...)
		s.mu.Unlock()
		return refs, nil
	}
	s.mu.Unlock()

	body, err := s.client.GetCatalog(ctx)
	if err != nil {
		return nil, err
	}
	years, err := ParseYears(string(body))
	if err != nil {
		return nil, err
	}
	var refs []SeasonRef
	for _, year := range years {
		page, err := s.client.GetYear(ctx, strconv.Itoa(year))
		if err != nil {
			return nil, fmt.Errorf("year %d: %w", year, err)
		}
		got, err := ParseSeasons(string(page), year)
		if err != nil {
			return nil, fmt.Errorf("year %d: %w", year, err)
		}
		refs = append(refs, got...)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return append([]SeasonRef(nil), s.refs...), nil
	}
	s.refs = refs
	s.loaded = true
	return append([]SeasonRef(nil), refs...), nil
}

func uniqueMatches(matches []domain.MatchInput) error {
	seen := map[string]struct{}{}
	for _, match := range matches {
		if _, ok := seen[match.ExternalID]; ok {
			return fmt.Errorf("match %s is published twice", match.ExternalID)
		}
		seen[match.ExternalID] = struct{}{}
	}
	return nil
}

func mergeStandings(tables [][]domain.StandingInput) []domain.StandingInput {
	best := map[string]domain.StandingInput{}
	var order []string
	for _, table := range tables {
		for _, row := range table {
			prev, ok := best[row.ExternalTeamID]
			if !ok {
				best[row.ExternalTeamID] = row
				order = append(order, row.ExternalTeamID)
				continue
			}
			if row.Played > prev.Played {
				best[row.ExternalTeamID] = row
			}
		}
	}
	out := make([]domain.StandingInput, 0, len(order))
	for _, id := range order {
		out = append(out, best[id])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GroupOrder != out[j].GroupOrder {
			return out[i].GroupOrder < out[j].GroupOrder
		}
		return out[i].Position < out[j].Position
	})
	return out
}

func teamsFrom(matches []domain.MatchInput, standings []domain.StandingInput, clubs map[string]club) ([]domain.TeamInput, error) {
	ids := map[string]struct{}{}
	for _, match := range matches {
		ids[match.HomeExternalID] = struct{}{}
		ids[match.AwayExternalID] = struct{}{}
	}
	for _, row := range standings {
		ids[row.ExternalTeamID] = struct{}{}
	}
	teams := make([]domain.TeamInput, 0, len(ids))
	for id := range ids {
		item, ok := clubs[id]
		if !ok || item.Name == "" {
			return nil, fmt.Errorf("club %s has no published name", id)
		}
		short := item.Short
		if short == "" {
			short = item.Name
		}
		teams = append(teams, domain.TeamInput{
			ExternalID: id,
			Name:       item.Name,
			ShortName:  short,
			State:      "SC",
		})
	}
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].Name < teams[j].Name
	})
	return teams, nil
}
