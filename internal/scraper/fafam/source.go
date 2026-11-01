package fafam

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

// Source collects the adult Amazonas divisions. The client stays on
// fafamazonas.com.br. Each season page is one round, so Fetch follows the
// previous and next links for that slug only.
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

// Fetch collects one season. Round pages repeat the knockout blocks. A failed
// page is returned as an error and does not replace rows by itself.
func (s *Source) Fetch(ctx context.Context, competition string, year int, collected time.Time) (domain.Snapshot, error) {
	item, ok := divisionByCode(competition)
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("unsupported competition %s", competition)
	}
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	ref, ok, err := s.season(ctx, competition, year)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("catalog has no %s season for %d", competition, year)
	}
	loc, err := manaus()
	if err != nil {
		return domain.Snapshot{}, err
	}
	bodies, err := s.rounds(ctx, ref)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var combined page
	combined.clubs = map[string]club{}
	for _, body := range bodies {
		parsed, parseErr := ParseRound(body, item.key, year, loc)
		if parseErr != nil {
			return domain.Snapshot{}, parseErr
		}
		if addErr := appendPage(&combined, parsed); addErr != nil {
			return domain.Snapshot{}, addErr
		}
	}
	if len(combined.matches) == 0 {
		return domain.Snapshot{}, fmt.Errorf("season %d has no matches", year)
	}
	standings := mergeStandings(combined.tables)
	if len(standings) == 0 {
		return domain.Snapshot{}, fmt.Errorf("season %d has no standings", year)
	}
	teams, err := teamsFrom(combined.matches, standings, combined.clubs)
	if err != nil {
		return domain.Snapshot{}, err
	}
	return domain.Snapshot{
		Source:           source,
		CompetitionCode:  competition,
		CompetitionName:  item.name,
		SeasonYear:       year,
		SeasonName:       fmt.Sprintf("%s %d", item.name, year),
		SeasonExternalID: ref.Slug,
		Teams:            teams,
		Matches:          combined.matches,
		Standings:        standings,
		CollectedAt:      collected.UTC(),
	}, nil
}

func (s *Source) catalog(ctx context.Context) ([]SeasonRef, error) {
	s.mu.Lock()
	if s.loaded {
		out := append([]SeasonRef(nil), s.refs...)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()
	var refs []SeasonRef
	seen := map[int]struct{}{}
	queue := []int{1}
	for len(queue) > 0 {
		page := queue[0]
		queue = queue[1:]
		if _, ok := seen[page]; ok {
			continue
		}
		seen[page] = struct{}{}
		if len(seen) > 10 {
			return nil, fmt.Errorf("amazonense catalog has too many pages")
		}
		body, err := s.client.GetCatalog(ctx, page)
		if err != nil {
			return nil, err
		}
		got, err := ParseCatalog(string(body))
		if err != nil {
			return nil, err
		}
		for _, ref := range got {
			refs, err = mergeRefs(refs, ref)
			if err != nil {
				return nil, err
			}
		}
		pages, err := CatalogPages(string(body))
		if err != nil {
			return nil, err
		}
		for _, next := range pages {
			if _, ok := seen[next]; !ok {
				queue = append(queue, next)
			}
		}
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("amazonense catalog has no adult seasons")
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

func (s *Source) season(ctx context.Context, code string, year int) (SeasonRef, bool, error) {
	refs, err := s.catalog(ctx)
	if err != nil {
		return SeasonRef{}, false, err
	}
	for _, ref := range refs {
		if ref.Code == code && ref.Year == year {
			return ref, true, nil
		}
	}
	return SeasonRef{}, false, nil
}

func (s *Source) rounds(ctx context.Context, ref SeasonRef) ([]string, error) {
	bodies := map[string]string{}
	seen := map[string]struct{}{ref.Round: {}}
	queue := []string{ref.Round}
	for len(queue) > 0 {
		round := queue[0]
		queue = queue[1:]
		if len(seen) > 40 {
			return nil, fmt.Errorf("season %s has too many rounds", ref.Slug)
		}
		body, err := s.client.GetRound(ctx, ref.Slug, round)
		if err != nil {
			return nil, err
		}
		bodies[round] = string(body)
		links, err := RoundLinks(string(body), ref.Slug)
		if err != nil {
			return nil, err
		}
		for _, next := range links {
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	order := make([]string, 0, len(bodies))
	for round := range bodies {
		order = append(order, round)
	}
	sort.Slice(order, func(i, j int) bool {
		left, _ := strconv.Atoi(order[i])
		right, _ := strconv.Atoi(order[j])
		return left < right
	})
	out := make([]string, len(order))
	for i, round := range order {
		out[i] = bodies[round]
	}
	return out, nil
}

func appendPage(dst *page, src page) error {
	if dst.clubs == nil {
		dst.clubs = map[string]club{}
	}
	for _, match := range src.matches {
		if err := addMatch(dst, match); err != nil {
			return err
		}
	}
	dst.tables = append(dst.tables, src.tables...)
	for id, item := range src.clubs {
		remember(dst.clubs, id, item.Name)
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
	seenGroup := map[string]int{}
	next := 0
	for _, id := range order {
		row := best[id]
		if _, ok := seenGroup[row.GroupName]; !ok {
			seenGroup[row.GroupName] = next
			next++
		}
		row.GroupOrder = seenGroup[row.GroupName]
		out = append(out, row)
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
		teams = append(teams, domain.TeamInput{
			ExternalID: id,
			Name:       item.Name,
			ShortName:  item.Short,
			State:      "AM",
		})
	}
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].Name < teams[j].Name
	})
	return teams, nil
}
