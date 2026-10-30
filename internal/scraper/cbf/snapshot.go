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
	// CompetitionCode is the only competition this adapter publishes.
	CompetitionCode   = "brasileirao-serie-a"
	competitionName   = "Campeonato Brasileiro Série A"
	catalogCampeonato = "42"
	catalogCategoria  = "1"
)

var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

// Assemble turns the three collected documents into one snapshot.
// Standings names win over fixture names. A differing fixture name is kept as an alias.
func Assemble(year int, externalID string, collected time.Time, matchesJSON, imtJSON, standingsHTML []byte) (domain.Snapshot, error) {
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
	standings, names, err := ParseStandings(standingsHTML)
	if err != nil {
		return domain.Snapshot{}, err
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
		CompetitionCode:  CompetitionCode,
		CompetitionName:  competitionName,
		SeasonYear:       year,
		SeasonName:       fmt.Sprintf("%s %d", competitionName, year),
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

// SeasonExternalID finds the Série A championship id for a year.
func SeasonExternalID(body []byte, year int) (string, error) {
	var doc catalogDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("parse catalog: %w", err)
	}
	want := strconv.Itoa(year)
	for _, championship := range doc.Campeonatos {
		if championship.ID != catalogCampeonato {
			continue
		}
		for _, category := range championship.Categorias {
			if category.ID != catalogCategoria || category.Years == nil || category.Championships == nil {
				continue
			}
			years := splitList(*category.Years)
			ids := splitList(*category.Championships)
			if len(years) != len(ids) {
				return "", fmt.Errorf("catalog year list and id list differ in length")
			}
			for i, item := range years {
				if item == want {
					if !digitsOnly.MatchString(ids[i]) {
						return "", fmt.Errorf("catalog id %q is not numeric", ids[i])
					}
					return ids[i], nil
				}
			}
		}
	}
	return "", fmt.Errorf("catalog has no Série A season for %d", year)
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
