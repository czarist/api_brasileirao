package cbf

import (
	"fmt"
	"strings"

	"github.com/czarist/api_brasileirao/internal/domain"
)

// Competition is one championship the public CBF catalog publishes.
// Slug is the standings path segment. It is empty when the source has no table.
// CatalogID and CategoryID select one row of the catalog. Category ids are not unique across championships.
type Competition struct {
	Code       string
	Name       string
	Slug       string
	CatalogID  string
	CategoryID string
	MinMatches int
	MinRows    int
	MaxRows    int
}

// SerieA is the first division. It stays exported for the existing tests.
var SerieA = mustCompetition("brasileirao-serie-a")

var catalog = []Competition{
	{Code: "brasileirao-serie-a", Name: "Campeonato Brasileiro Série A", Slug: "serie-a", CatalogID: "42", CategoryID: "1", MinMatches: 100, MinRows: 18, MaxRows: 22},
	{Code: "brasileirao-serie-b", Name: "Campeonato Brasileiro Série B", Slug: "serie-b", CatalogID: "42", CategoryID: "2", MinMatches: 100, MinRows: 18, MaxRows: 22},
	{Code: "brasileirao-serie-c", Name: "Campeonato Brasileiro Série C", Slug: "serie-c", CatalogID: "42", CategoryID: "3", MinMatches: 40, MinRows: 8, MaxRows: 40},
	{Code: "brasileirao-serie-d", Name: "Campeonato Brasileiro Série D", Slug: "serie-d", CatalogID: "42", CategoryID: "5", MinMatches: 20, MinRows: 16, MaxRows: 128},
	// Professional Copa do Brasil is knockout. MaxRows 0 means there is no standings table to collect.
	// MinMatches is below every catalog season counted on 2026-10-31 (120 through 154) and rejects a short document.
	{Code: "copa-do-brasil", Name: "Copa do Brasil", Slug: "", CatalogID: "24", CategoryID: "4", MinMatches: 100, MinRows: 0, MaxRows: 0},
}

// Lookup returns a supported competition.
func Lookup(code string) (Competition, bool) {
	for _, item := range catalog {
		if item.Code == code {
			return item, true
		}
	}
	return Competition{}, false
}

// ParseCodes accepts "all" or a comma-separated list of competition codes.
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

// Limits is the completeness check for this competition.
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
