package fgfrs

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

// Source collects the adult Gaúcho divisions. The client stays on www.fgf.com.br.
type Source struct {
	client *Client

	mu    sync.Mutex
	years map[string][]int
	pages map[string]yearPage
}

type yearPage struct {
	body   string
	phases []PhaseRef
}

// NewSource binds the adapter to its client.
func NewSource(client *Client) *Source {
	return &Source{client: client}
}

// Reset drops the cached year lists.
func (s *Source) Reset() {
	s.mu.Lock()
	s.years = nil
	s.pages = nil
	s.mu.Unlock()
}

// Plan lists every published adult season for the requested codes.
// A code the catalog does not list is omitted. A division page that fails
// fails the plan.
func (s *Source) Plan(ctx context.Context, codes []string) ([]SeasonPlan, error) {
	wanted := map[string]struct{}{}
	for _, code := range codes {
		if _, ok := Lookup(code); !ok {
			return nil, fmt.Errorf("unsupported competition %s", code)
		}
		wanted[code] = struct{}{}
	}
	var plans []SeasonPlan
	for _, item := range divisions {
		if _, ok := wanted[item.code]; !ok {
			continue
		}
		years, err := s.yearsFor(ctx, item.code)
		if err != nil {
			return nil, err
		}
		for _, year := range years {
			ok, err := s.seasonPublished(ctx, item, year)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			plans = append(plans, SeasonPlan{Code: item.code, Year: year})
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

// Fetch collects one season. Every phase linked from the year page is read.
// The year page itself is read only when it publishes no phase link. A page
// whose phase buttons all belong to another year is not this season. A failed
// page is returned as an error and does not replace rows by itself.
func (s *Source) Fetch(ctx context.Context, competition string, year int, collected time.Time) (domain.Snapshot, error) {
	item, ok := divisionByCode(competition)
	if !ok {
		return domain.Snapshot{}, fmt.Errorf("unsupported competition %s", competition)
	}
	if year < 1990 || year > 2100 {
		return domain.Snapshot{}, fmt.Errorf("season year %d is out of range", year)
	}
	years, err := s.yearsFor(ctx, competition)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if !containsYear(years, year) {
		return domain.Snapshot{}, fmt.Errorf("catalog has no %s season for %d", competition, year)
	}
	loc, err := saopaulo()
	if err != nil {
		return domain.Snapshot{}, err
	}
	yearText := strconv.Itoa(year)
	loaded, err := s.loadYear(ctx, item, year)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var combined page
	if len(loaded.phases) == 0 {
		foreign, foreignErr := foreignYearOnly(loaded.body, item.id, yearText)
		if foreignErr != nil {
			return domain.Snapshot{}, foreignErr
		}
		if foreign {
			return domain.Snapshot{}, fmt.Errorf("season %d publishes phases for another year", year)
		}
		combined, err = ParsePhase(loaded.body, item.id, year, item.name, loc)
		if err != nil {
			return domain.Snapshot{}, err
		}
	} else {
		combined.clubs = map[string]club{}
		for _, phase := range loaded.phases {
			phaseBody, phaseErr := s.client.GetPhase(ctx, item.id, yearText, phase.ID)
			if phaseErr != nil {
				return domain.Snapshot{}, fmt.Errorf("phase %s: %w", phase.ID, phaseErr)
			}
			parsed, parseErr := ParsePhase(string(phaseBody), item.id, year, phase.Name, loc)
			if parseErr != nil {
				return domain.Snapshot{}, fmt.Errorf("phase %s: %w", phase.Name, parseErr)
			}
			if len(parsed.matches) == 0 && len(parsed.tables) == 0 {
				continue
			}
			if addErr := appendPage(&combined, parsed); addErr != nil {
				return domain.Snapshot{}, addErr
			}
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
		SeasonExternalID: item.id + "-" + yearText,
		Teams:            teams,
		Matches:          combined.matches,
		Standings:        standings,
		CollectedAt:      collected.UTC(),
	}, nil
}

func (s *Source) yearsFor(ctx context.Context, code string) ([]int, error) {
	s.mu.Lock()
	if s.years != nil {
		if got, ok := s.years[code]; ok {
			out := append([]int(nil), got...)
			s.mu.Unlock()
			return out, nil
		}
	}
	s.mu.Unlock()

	item, ok := divisionByCode(code)
	if !ok {
		return nil, fmt.Errorf("unsupported competition %s", code)
	}
	body, err := s.client.GetDivision(ctx, item.id)
	if err != nil {
		return nil, err
	}
	years, err := ParseYears(string(body), item.id)
	if err != nil {
		return nil, err
	}
	if len(years) == 0 {
		return nil, fmt.Errorf("%s catalog has no years", code)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.years == nil {
		s.years = map[string][]int{}
	}
	if got, ok := s.years[code]; ok {
		return append([]int(nil), got...), nil
	}
	s.years[code] = years
	return append([]int(nil), years...), nil
}

func (s *Source) seasonPublished(ctx context.Context, item division, year int) (bool, error) {
	loaded, err := s.loadYear(ctx, item, year)
	if err != nil {
		return false, err
	}
	if len(loaded.phases) > 0 {
		return true, nil
	}
	foreign, err := foreignYearOnly(loaded.body, item.id, strconv.Itoa(year))
	if err != nil {
		return false, err
	}
	if foreign {
		return false, nil
	}
	loc, err := saopaulo()
	if err != nil {
		return false, err
	}
	parsed, err := ParsePhase(loaded.body, item.id, year, item.name, loc)
	if err != nil {
		return true, nil
	}
	return len(parsed.matches) > 0 || len(parsed.tables) > 0, nil
}

func (s *Source) loadYear(ctx context.Context, item division, year int) (yearPage, error) {
	key := item.code + "/" + strconv.Itoa(year)
	s.mu.Lock()
	if s.pages != nil {
		if got, ok := s.pages[key]; ok {
			s.mu.Unlock()
			return got, nil
		}
	}
	s.mu.Unlock()
	body, err := s.client.GetYear(ctx, item.id, strconv.Itoa(year))
	if err != nil {
		return yearPage{}, err
	}
	phases, err := ParsePhases(string(body), item.id, strconv.Itoa(year))
	if err != nil {
		return yearPage{}, err
	}
	loaded := yearPage{body: string(body), phases: phases}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pages == nil {
		s.pages = map[string]yearPage{}
	}
	if got, ok := s.pages[key]; ok {
		return got, nil
	}
	s.pages[key] = loaded
	return loaded, nil
}

func containsYear(years []int, year int) bool {
	for _, got := range years {
		if got == year {
			return true
		}
	}
	return false
}

func appendPage(dst *page, src page) error {
	if dst.clubs == nil {
		dst.clubs = map[string]club{}
	}
	seen := map[string]domain.MatchInput{}
	for _, match := range dst.matches {
		seen[match.ExternalID] = match
	}
	for _, match := range src.matches {
		prev, ok := seen[match.ExternalID]
		if ok {
			if sameMatch(prev, match) {
				continue
			}
			return fmt.Errorf("match %s is published twice", match.ExternalID)
		}
		seen[match.ExternalID] = match
		dst.matches = append(dst.matches, match)
	}
	dst.tables = append(dst.tables, src.tables...)
	for id, item := range src.clubs {
		remember(dst.clubs, id, item.Name, item.Short)
	}
	return nil
}

func sameMatch(a, b domain.MatchInput) bool {
	if a.ExternalID != b.ExternalID || a.RoundNumber != b.RoundNumber || a.Phase != b.Phase || a.HomeExternalID != b.HomeExternalID || a.AwayExternalID != b.AwayExternalID || a.Status != b.Status || a.Venue != b.Venue || a.State != b.State {
		return false
	}
	if !sameScore(a.HomeScore, b.HomeScore) || !sameScore(a.AwayScore, b.AwayScore) {
		return false
	}
	if (a.Kickoff == nil) != (b.Kickoff == nil) {
		return false
	}
	if a.Kickoff != nil && !a.Kickoff.Equal(*b.Kickoff) {
		return false
	}
	return true
}

func sameScore(a, b *int) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
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
		short := item.Short
		if short == "" {
			short = item.Name
		}
		teams = append(teams, domain.TeamInput{
			ExternalID: id,
			Name:       item.Name,
			ShortName:  short,
			State:      "RS",
		})
	}
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].Name < teams[j].Name
	})
	return teams, nil
}
