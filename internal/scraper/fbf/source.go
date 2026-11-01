package fbf

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

// Source collects the adult Baianão. The client stays on www.fbf.org.br.
type Source struct {
	client *Client

	mu     sync.Mutex
	cached map[string][]int
}

// NewSource binds the adapter to its client.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Reset drops the cached year lists.
func (s *Source) Reset() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

// Plan lists every published adult season for the requested codes.
// A code whose select lists no year is omitted.
func (s *Source) Plan(ctx context.Context, codes []string) ([]SeasonPlan, error) {
	var plans []SeasonPlan
	for _, code := range codes {
		if _, ok := Lookup(code); !ok {
			return nil, fmt.Errorf("unsupported competition %s", code)
		}
		division, _ := divisionOf(code)
		years, err := s.years(ctx, division)
		if err != nil {
			return nil, err
		}
		for _, year := range years {
			plans = append(plans, SeasonPlan{Code: code, Year: year})
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
	divisionName, ok := Lookup(competition)
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("unsupported competition %s", competition)
	}
	division, _ := divisionOf(competition)
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	years, err := s.years(ctx, division)
	if err != nil {
		return domain.Snapshot{}, err
	}
	found := false
	for _, item := range years {
		if item == year {
			found = true
			break
		}
	}
	if !found {
		return domain.Snapshot{}, fmt.Errorf("catalog has no %s season for %d", competition, year)
	}
	raw, err := s.client.GetSeason(ctx, division, strconv.Itoa(year))
	if err != nil {
		return domain.Snapshot{}, err
	}
	shell := string(raw)
	if path, ok := PublishedPath(shell); ok && !pathMatchesSeason(path, division, year) {
		return domain.Snapshot{}, fmt.Errorf("season page is %s, not %s %d", path, competition, year)
	}
	phases, err := ParsePhases(shell, division, year)
	if err != nil {
		return domain.Snapshot{}, err
	}
	loc, err := bahia()
	if err != nil {
		return domain.Snapshot{}, err
	}
	var pages []Page
	if len(phases) == 0 {
		page, err := ParsePage(shell, "Classificação", year, loc)
		if err != nil {
			return domain.Snapshot{}, err
		}
		pages = append(pages, page)
	} else {
		for _, phase := range phases {
			body, err := s.client.GetPhase(ctx, division, strconv.Itoa(year), phase.ID)
			if err != nil {
				return domain.Snapshot{}, fmt.Errorf("phase %s: %w", phase.Name, err)
			}
			page, err := ParsePage(string(body), phase.Name, year, loc)
			if err != nil {
				return domain.Snapshot{}, fmt.Errorf("phase %s: %w", phase.Name, err)
			}
			pages = append(pages, page)
		}
	}
	matches, err := uniqueMatches(pages)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if len(matches) == 0 {
		return domain.Snapshot{}, fmt.Errorf("season %d has no matches", year)
	}
	var tables [][]domain.StandingInput
	clubs := map[string]Club{}
	for _, page := range pages {
		tables = append(tables, page.Tables...)
		for id, club := range page.Clubs {
			remember(clubs, id, club.Name, club.Short)
		}
	}
	matches, err = linkPlaceholderClubs(matches, clubs)
	if err != nil {
		return domain.Snapshot{}, err
	}
	standings := mergeStandings(tables)
	if len(standings) == 0 {
		return domain.Snapshot{}, fmt.Errorf("season %d has no standings", year)
	}
	teams, err := teamsFrom(matches, standings, clubs)
	if err != nil {
		return domain.Snapshot{}, err
	}
	return domain.Snapshot{
		Source:           source,
		CompetitionCode:  competition,
		CompetitionName:  divisionName,
		SeasonYear:       year,
		SeasonName:       fmt.Sprintf("%s %d", divisionName, year),
		SeasonExternalID: division + "-" + strconv.Itoa(year),
		Teams:            teams,
		Matches:          matches,
		Standings:        standings,
		CollectedAt:      collected.UTC(),
	}, nil
}

func (s *Source) years(ctx context.Context, division string) ([]int, error) {
	s.mu.Lock()
	if rows, ok := s.cached[division]; ok {
		s.mu.Unlock()
		return rows, nil
	}
	s.mu.Unlock()
	body, err := s.client.GetDivision(ctx, division)
	if err != nil {
		return nil, err
	}
	parsed, err := ParseYears(string(body), division)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if rows, ok := s.cached[division]; ok {
		return rows, nil
	}
	if s.cached == nil {
		s.cached = map[string][]int{}
	}
	s.cached[division] = parsed
	return parsed, nil
}

func uniqueMatches(pages []Page) ([]domain.MatchInput, error) {
	seen := map[string]domain.MatchInput{}
	var out []domain.MatchInput
	for _, page := range pages {
		for _, match := range page.Matches {
			if previous, ok := seen[match.ExternalID]; ok {
				if sameScore(previous, match) {
					continue
				}
				return nil, fmt.Errorf("match %s is published twice with different scores", match.ExternalID)
			}
			seen[match.ExternalID] = match
			out = append(out, match)
		}
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

func mergeStandings(tables [][]domain.StandingInput) []domain.StandingInput {
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
		if fresh != len(table) {
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
	return out
}

func teamsFrom(matches []domain.MatchInput, standings []domain.StandingInput, clubs map[string]Club) ([]domain.TeamInput, error) {
	byID := map[string]*domain.TeamInput{}
	var order []string
	add := func(id string) error {
		if id == "" {
			return nil
		}
		if _, ok := byID[id]; ok {
			return nil
		}
		club := clubs[id]
		name := club.Name
		short := club.Short
		if name == "" {
			name = short
		}
		if name == "" {
			return fmt.Errorf("club %s has no published name", id)
		}
		if short == "" {
			short = name
		}
		byID[id] = &domain.TeamInput{ExternalID: id, Name: name, ShortName: short}
		order = append(order, id)
		return nil
	}
	for _, row := range standings {
		if err := add(row.ExternalTeamID); err != nil {
			return nil, err
		}
	}
	for _, match := range matches {
		if err := add(match.HomeExternalID); err != nil {
			return nil, fmt.Errorf("match %s: %w", match.ExternalID, err)
		}
		if err := add(match.AwayExternalID); err != nil {
			return nil, fmt.Errorf("match %s: %w", match.ExternalID, err)
		}
	}
	out := make([]domain.TeamInput, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}
