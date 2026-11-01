package uikit

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
	Code string
	Year int
}

// Source collects the UIKit federations. Each client stays on its own host.
type Source struct {
	clients []*Client

	mu    sync.Mutex
	cache map[string][]Link
}

// NewSource binds the adapter to one client per federation.
func NewSource(clients ...*Client) *Source {
	return &Source{clients: clients}
}

// Reset drops cached catalog pages.
func (s *Source) Reset() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// Plan lists every published professional season for the requested codes.
func (s *Source) Plan(ctx context.Context, codes []string) ([]SeasonPlan, error) {
	wanted := map[string]struct{}{}
	hosts := map[string]struct{}{}
	for _, code := range codes {
		site, _, ok := Lookup(code)
		if !ok {
			return nil, fmt.Errorf("unsupported competition %s", code)
		}
		wanted[code] = struct{}{}
		hosts[site.Host] = struct{}{}
	}
	var plans []SeasonPlan
	for host := range hosts {
		links, err := s.catalog(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			if _, ok := wanted[link.Code]; !ok {
				continue
			}
			plans = append(plans, SeasonPlan{Code: link.Code, Year: link.Year})
		}
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
	site, division, ok := Lookup(competition)
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("unsupported competition %s", competition)
	}
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	links, err := s.catalog(ctx, site.Host)
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
	client, err := s.client(site.Host)
	if err != nil {
		return domain.Snapshot{}, err
	}
	page, err := client.GetSeason(ctx, link.ID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	phases, err := ParsePhases(string(page), link.ID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	query, err := ParseFeedQuery(string(page), link.ID, year)
	if err != nil {
		return domain.Snapshot{}, err
	}
	loc, err := loadLocation(site.Location)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var fixtures []Fixture
	var tables [][]domain.StandingInput
	clubs := map[string]Club{}
	for _, phase := range phases {
		raw, err := client.PostMatches(ctx, query, phase.ID)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("phase %s: %w", phase.Name, err)
		}
		got, err := ParseMatches(string(raw), phase.Name, year, loc)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("phase %s: %w", phase.Name, err)
		}
		fixtures = append(fixtures, got...)
		tableRaw, err := client.PostStandings(ctx, query, phase.ID)
		if err != nil {
			return domain.Snapshot{}, fmt.Errorf("phase %s standings: %w", phase.Name, err)
		}
		rows, names, err := ParseStandings(string(tableRaw))
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
		Source:           site.Source,
		CompetitionCode:  division.Code,
		CompetitionName:  division.Name,
		SeasonYear:       year,
		SeasonName:       link.Title,
		SeasonExternalID: link.ID,
		Teams:            teams,
		Matches:          matches,
		Standings:        standings,
		CollectedAt:      collected.UTC(),
	}, nil
}

func (s *Source) catalog(ctx context.Context, host string) ([]Link, error) {
	s.mu.Lock()
	if s.cache != nil {
		if rows, ok := s.cache[host]; ok {
			s.mu.Unlock()
			return rows, nil
		}
	}
	s.mu.Unlock()
	client, err := s.client(host)
	if err != nil {
		return nil, err
	}
	rows, err := loadCatalog(ctx, client)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.cache == nil {
		s.cache = map[string][]Link{}
	}
	if existing, ok := s.cache[host]; ok {
		s.mu.Unlock()
		return existing, nil
	}
	s.cache[host] = rows
	s.mu.Unlock()
	return rows, nil
}

func loadCatalog(ctx context.Context, client *Client) ([]Link, error) {
	body, err := client.GetIndex(ctx)
	if err != nil {
		return nil, err
	}
	selected, links, years, err := ParseCatalog(client.site, string(body))
	if err != nil {
		return nil, err
	}
	all := append([]Link{}, links...)
	for _, year := range years {
		if year == selected {
			continue
		}
		page, err := client.PostIndex(ctx, fmt.Sprintf("%d", year))
		if err != nil {
			return nil, err
		}
		gotSelected, gotLinks, _, err := ParseCatalog(client.site, string(page))
		if err != nil {
			return nil, err
		}
		if gotSelected != year {
			return nil, fmt.Errorf("catalog page for %d came back as %d", year, gotSelected)
		}
		all = append(all, gotLinks...)
	}
	return all, nil
}

func (s *Source) client(host string) (*Client, error) {
	for _, client := range s.clients {
		if client.site.Host == host {
			return client, nil
		}
	}
	return nil, fmt.Errorf("no client for %s", host)
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

func teamsFrom(fixtures []Fixture, standings []domain.StandingInput, clubs map[string]Club) ([]domain.TeamInput, error) {
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
