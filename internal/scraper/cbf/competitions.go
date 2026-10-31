package cbf

import (
	"fmt"
	"strings"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// Competition is one division the public CBF catalog publishes.
type Competition struct {
	Code       string
	Name       string
	Slug       string
	CategoryID string
	MinMatches int
	MinRows    int
	MaxRows    int
}

// SerieA is the first division. It stays exported for the existing tests.
var SerieA = mustCompetition("brasileirao-serie-a")

var catalog = []Competition{
	{Code: "brasileirao-serie-a", Name: "Campeonato Brasileiro Série A", Slug: "serie-a", CategoryID: "1", MinMatches: 100, MinRows: 18, MaxRows: 22},
	{Code: "brasileirao-serie-b", Name: "Campeonato Brasileiro Série B", Slug: "serie-b", CategoryID: "2", MinMatches: 100, MinRows: 18, MaxRows: 22},
	{Code: "brasileirao-serie-c", Name: "Campeonato Brasileiro Série C", Slug: "serie-c", CategoryID: "3", MinMatches: 40, MinRows: 8, MaxRows: 40},
	{Code: "brasileirao-serie-d", Name: "Campeonato Brasileiro Série D", Slug: "serie-d", CategoryID: "5", MinMatches: 20, MinRows: 16, MaxRows: 128},
}

// Lookup returns a supported division.
func Lookup(code string) (Competition, bool) {
	for _, item := range catalog {
		if item.Code == code {
			return item, true
		}
	}
	return Competition{}, false
}

// ParseCodes accepts "all" or a comma-separated list of division codes.
func ParseCodes(spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "all" {
		codes := make([]string, len(catalog))
		for i, item := range catalog {
			codes[i] = item.Code
		}
		return codes, nil
	}
	parts := strings.Split(spec, ",")
	codes := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := Lookup(part); !ok {
			return nil, fmt.Errorf("unsupported competition %s", part)
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		codes = append(codes, part)
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("SYNC_COMPETITION is empty")
	}
	return codes, nil
}

// Limits is the completeness check for this division.
func (c Competition) Limits() domain.Limits {
	return domain.Limits{MinMatches: c.MinMatches, MinStandings: c.MinRows, MaxStandings: c.MaxRows}
}

func mustCompetition(code string) Competition {
	item, ok := Lookup(code)
	if !ok {
		panic(code)
	}
	return item
}
