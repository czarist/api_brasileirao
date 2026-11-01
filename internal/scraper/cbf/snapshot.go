package cbf

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

const (
	// CompetitionCode is Série A. The adapter also publishes Série B, C, D, and Copa do Brasil.
	CompetitionCode = "brasileirao-serie-a"
)

var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

// Assemble turns the collected documents into one snapshot.
// Standings names win over fixture names. A differing fixture name is kept as an alias.
// A nil or empty standings document is valid only for a competition with no table (MaxRows 0).
func Assemble(comp Competition, year int, externalID string, collected time.Time, matchesJSON, imtJSON, standingsHTML []byte) (domain.Snapshot, error) {
	if !digitsOnly.MatchString(externalID) {
		return domain.Snapshot{}, fmt.Errorf("season external id %q is not numeric", externalID)
	}
	loc, err := saoPaulo()
	if err != nil {
		return domain.Snapshot{}, err
	}
	matches, teams, err := ParseMatches(matchesJSON, loc)
	if err != nil {
		return domain.Snapshot{}, err
	}
	changes, err := ParseIMT(imtJSON, loc)
	if err != nil {
		return domain.Snapshot{}, err
	}
	applySchedule(matches, changes)
	var standings []domain.StandingInput
	names := map[string]string{}
	if strings.TrimSpace(string(standingsHTML)) == "" {
		if comp.MaxRows > 0 {
			return domain.Snapshot{}, fmt.Errorf("standings page is empty")
		}
	} else {
		standings, names, err = ParseStandings(standingsHTML)
		if err != nil {
			return domain.Snapshot{}, err
		}
	}
	byID := make(map[string]*domain.TeamInput, len(teams))
	for i := range teams {
		byID[teams[i].ExternalID] = &teams[i]
	}
	for id, name := range names {
		club, ok := byID[id]
		if !ok {
			byID[id] = &domain.TeamInput{ExternalID: id, Name: name, ShortName: ShortName(name)}
			continue
		}
		if fold(club.Name) != fold(name) {
			rememberAlias(club, club.Name)
			club.Name = name
			club.ShortName = ShortName(name)
		}
	}
	merged := make([]domain.TeamInput, 0, len(byID))
	for _, club := range byID {
		merged = append(merged, *club)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ExternalID < merged[j].ExternalID })
	return domain.Snapshot{
		Source:           domain.SourceCBF,
		CompetitionCode:  comp.Code,
		CompetitionName:  comp.Name,
		SeasonYear:       year,
		SeasonName:       fmt.Sprintf("%s %d", comp.Name, year),
		SeasonExternalID: externalID,
		Teams:            merged,
		Matches:          matches,
		Standings:        standings,
		CollectedAt:      collected.UTC(),
	}, nil
}

func applySchedule(matches []domain.MatchInput, changes map[string]scheduleChange) {
	for i := range matches {
		change, ok := changes[matches[i].ExternalID]
		if !ok {
			continue
		}
		if change.start != nil {
			matches[i].Kickoff = change.start
		}
		if change.venue != "" {
			matches[i].Venue = change.venue
		}
		matches[i].Status = statusFor(matches[i].HomeScore, matches[i].AwayScore, matches[i].Kickoff, change.note)
	}
}

type catalogDoc struct {
	Campeonatos []struct {
		ID         string `json:"id"`
		Categorias []struct {
			ID            string  `json:"id"`
			Years         *string `json:"anos"`
			Championships *string `json:"id_campeonato"`
		} `json:"categorias"`
	} `json:"campeonatos"`
}

// SeasonRef is one year published for a catalog category.
type SeasonRef struct {
	Year       int
	ExternalID string
}

// CatalogSeasons lists the years and championship ids of one catalog row.
// Championship id and category id are both required. Category ids repeat across championships.
// A missing row returns no seasons. The two lists must be the same length.
func CatalogSeasons(body []byte, championshipID, categoryID string) ([]SeasonRef, error) {
	var doc catalogDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	for _, championship := range doc.Campeonatos {
		if championship.ID != championshipID {
			continue
		}
		for _, category := range championship.Categorias {
			if category.ID != categoryID {
				continue
			}
			if category.Years == nil || category.Championships == nil {
				return nil, nil
			}
			years := splitList(*category.Years)
			ids := splitList(*category.Championships)
			if len(years) != len(ids) {
				return nil, fmt.Errorf("catalog year list and id list differ in length")
			}
			out := make([]SeasonRef, 0, len(years))
			seen := make(map[int]struct{}, len(years))
			for i, item := range years {
				year, err := strconv.Atoi(item)
				if err != nil {
					return nil, fmt.Errorf("catalog year %q is not numeric", item)
				}
				if !digitsOnly.MatchString(ids[i]) {
					return nil, fmt.Errorf("catalog id %q is not numeric", ids[i])
				}
				if _, ok := seen[year]; ok {
					return nil, fmt.Errorf("catalog repeats year %d", year)
				}
				seen[year] = struct{}{}
				out = append(out, SeasonRef{Year: year, ExternalID: ids[i]})
			}
			return out, nil
		}
	}
	return nil, nil
}

// SeasonExternalID finds the championship id for one catalog row and year.
func SeasonExternalID(body []byte, championshipID, categoryID string, year int) (string, error) {
	seasons, err := CatalogSeasons(body, championshipID, categoryID)
	if err != nil {
		return "", err
	}
	for _, season := range seasons {
		if season.Year == year {
			return season.ExternalID, nil
		}
	}
	return "", fmt.Errorf("catalog has no championship %s category %s season for %d", championshipID, categoryID, year)
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
