package fafal

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"github.com/czarist/api_brasileirao/internal/scraper/uikit"
)

// SeasonPlan is one catalog season the worker can collect.
type SeasonPlan struct {
	Code string
	Year int
}

// Source collects the adult Alagoano. The client stays on the FAF host.
type Source struct {
	client *Client

	mu    sync.Mutex
	cache []Link
	ready bool
}

// NewSource binds the adapter to its client.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Reset drops the cached catalog.
func (s *Source) Reset() {
	s.mu.Lock()
	s.cache = nil
	s.ready = false
	s.mu.Unlock()
}

// Plan lists every published adult season for the requested codes.
// A code the catalog does not publish is omitted.
func (s *Source) Plan(ctx context.Context, codes []string) ([]SeasonPlan, error) {
	wanted := map[string]struct{}{}
	for _, code := range codes {
		if _, ok := Lookup(code); !ok {
			return nil, fmt.Errorf("unsupported competition %s", code)
		}
		wanted[code] = struct{}{}
	}
	links, err := s.catalog(ctx)
	if err != nil {
		return nil, err
	}
	var plans []SeasonPlan
	for _, link := range links {
		if _, ok := wanted[link.Code]; !ok {
			continue
		}
		plans = append(plans, SeasonPlan{Code: link.Code, Year: link.Year})
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
	division, ok := Lookup(competition)
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("unsupported competition %s", competition)
	}
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	links, err := s.catalog(ctx)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var link Link
	found := false
	for _, item := range links {
		if item.Code == competition && item.Year == year {
			link = item
			found = true
			break
		}
	}
	if !found {
		return domain.Snapshot{}, fmt.Errorf("catalog has no %s season for %d", competition, year)
	}
	page, err := s.client.GetSeason(ctx, link.ID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	title, err := SeasonTitle(string(page), year, competition)
	if err != nil {
		return domain.Snapshot{}, err
	}
	phases, err := ParsePhases(string(page), link.ID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	loc, err := maceio()
	if err != nil {
		return domain.Snapshot{}, err
	}
	var fixtures []Fixture
	var tables [][]domain.StandingInput
	clubs := map[string]uikit.Club{}
	for _, phase := range phases {
		raw, err := s.client.GetPhase(ctx, link.ID, phase.ID)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("phase %s: %w", phase.Name, err)
		}
		body := string(raw)
		got, err := ParseMatches(body, phase.Name, year, loc)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("phase %s: %w", phase.Name, err)
		}
		fixtures = append(fixtures, got...)
		rows, names, err := ParseStandings(body)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("phase %s standings: %w", phase.Name, err)
		}
		for id, club := range names {
			if _, ok := clubs[id]; !ok {
				clubs[id] = club
			}
		}
		tables = append(tables, rows)
	}
	fixtures, err = uniqueFixtures(fixtures)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if len(fixtures) == 0 {
		return domain.Snapshot{}, fmt.Errorf("season %d has no matches", year)
	}
	standings, err := mergeStandings(tables)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if len(standings) == 0 {
		return domain.Snapshot{}, fmt.Errorf("season %d has no standings", year)
	}
	teams, err := teamsFrom(fixtures, standings, clubs)
	if err != nil {
		return domain.Snapshot{}, err
	}
	matches := make([]domain.MatchInput, len(fixtures))
	for i, fixture := range fixtures {
		matches[i] = fixture.Match
	}
	return domain.Snapshot{
		Source:           source,
		CompetitionCode:  competition,
		CompetitionName:  division,
		SeasonYear:       year,
		SeasonName:       title,
		SeasonExternalID: link.ID,
		Teams:            teams,
		Matches:          matches,
		Standings:        standings,
		CollectedAt:      collected.UTC(),
	}, nil
}

func (s *Source) catalog(ctx context.Context) ([]Link, error) {
	s.mu.Lock()
	if s.ready {
		rows := s.cache
		s.mu.Unlock()
		return rows, nil
	}
	s.mu.Unlock()
	rows, err := loadCatalog(ctx, s.client)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.ready {
		rows = s.cache
		s.mu.Unlock()
		return rows, nil
	}
	s.cache = rows
	s.ready = true
	s.mu.Unlock()
	return rows, nil
}

func loadCatalog(ctx context.Context, client *Client) ([]Link, error) {
	body, err := client.GetIndex(ctx)
	if err != nil {
		return nil, err
	}
	page := string(body)
	years, err := ParseYears(page)
	if err != nil {
		return nil, err
	}
	selected, err := PageYear(page)
	if err != nil {
		return nil, err
	}
	var all []Link
	seen := map[string]struct{}{}
	add := func(year int, raw string) error {
		links, err := ParseCatalog(raw, year)
		if err != nil {
			return err
		}
		for _, link := range links {
			key := link.Code + "/" + link.ID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			all = append(all, link)
		}
		return nil
	}
	if err := add(selected, page); err != nil {
		return nil, err
	}
	for _, year := range years {
		if year == selected {
			continue
		}
		raw, err := client.GetYear(ctx, fmt.Sprintf("%d", year))
		if err != nil {
			return nil, err
		}
		if err := add(year, string(raw)); err != nil {
			return nil, err
		}
	}
	return all, nil
}

func uniqueFixtures(fixtures []Fixture) ([]Fixture, error) {
	seen := map[string]Fixture{}
	var out []Fixture
	for _, fixture := range fixtures {
		if previous, ok := seen[fixture.Match.ExternalID]; ok {
			if sameScore(previous.Match, fixture.Match) {
				continue
			}
			return nil, fmt.Errorf("match %s is published twice with different scores", fixture.Match.ExternalID)
		}
		seen[fixture.Match.ExternalID] = fixture
		out = append(out, fixture)
	}
	return out, nil
}

func sameScore(left, right domain.MatchInput) bool {
	return sameInt(left.HomeScore, right.HomeScore) && sameInt(left.AwayScore, right.AwayScore)
}

func sameInt(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func mergeStandings(tables [][]domain.StandingInput) ([]domain.StandingInput, error) {
	seen := map[string]struct{}{}
	var out []domain.StandingInput
	for _, table := range tables {
		if len(table) == 0 {
			continue
		}
		fresh := 0
		for _, row := range table {
			if _, ok := seen[row.ExternalTeamID]; !ok {
				fresh++
			}
		}
		if fresh == 0 || fresh != len(table) {
			continue
		}
		for _, row := range table {
			seen[row.ExternalTeamID] = struct{}{}
			out = append(out, row)
		}
	}
	order := map[string]int{}
	next := 0
	for i := range out {
		key := out[i].GroupName
		if _, ok := order[key]; !ok {
			order[key] = next
			next++
		}
		out[i].GroupOrder = order[key]
	}
	return out, nil
}

func teamsFrom(fixtures []Fixture, standings []domain.StandingInput, clubs map[string]uikit.Club) ([]domain.TeamInput, error) {
	byID := map[string]*domain.TeamInput{}
	var order []string
	add := func(id, name, short string) error {
		if id == "" {
			return nil
		}
		if name == "" {
			return fmt.Errorf("club %s has no published name", id)
		}
		if short == "" {
			short = name
		}
		if existing, ok := byID[id]; ok {
			if existing.Name != name {
				existing.Aliases = append(existing.Aliases, name)
			}
			return nil
		}
		byID[id] = &domain.TeamInput{ExternalID: id, Name: name, ShortName: short}
		order = append(order, id)
		return nil
	}
	for _, fixture := range fixtures {
		homeName, homeShort := fixture.HomeName, fixture.HomeShort
		awayName, awayShort := fixture.AwayName, fixture.AwayShort
		if club, ok := clubs[fixture.Match.HomeExternalID]; ok {
			homeName, homeShort = club.Name, club.Short
		}
		if club, ok := clubs[fixture.Match.AwayExternalID]; ok {
			awayName, awayShort = club.Name, club.Short
		}
		if err := add(fixture.Match.HomeExternalID, homeName, homeShort); err != nil {
			return nil, fmt.Errorf("match %s: %w", fixture.Match.ExternalID, err)
		}
		if err := add(fixture.Match.AwayExternalID, awayName, awayShort); err != nil {
			return nil, fmt.Errorf("match %s: %w", fixture.Match.ExternalID, err)
		}
	}
	for _, row := range standings {
		club := clubs[row.ExternalTeamID]
		if err := add(row.ExternalTeamID, club.Name, club.Short); err != nil {
			return nil, err
		}
	}
	out := make([]domain.TeamInput, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}
