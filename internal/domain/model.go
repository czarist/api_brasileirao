// Package domain holds the competition model shared by the scraper, the
// database, and the API. It does not perform I/O.
package domain

import (
	"fmt"
	"time"
)

// MatchStatus is the normalized state of a match.
type MatchStatus string

const (
	StatusScheduled  MatchStatus = "scheduled"
	StatusInProgress MatchStatus = "in_progress"
	StatusFinished   MatchStatus = "finished"
	StatusPostponed  MatchStatus = "postponed"
	StatusSuspended  MatchStatus = "suspended"
	StatusCancelled  MatchStatus = "cancelled"
)

// Valid reports whether the status is one of the stored values.
func (s MatchStatus) Valid() bool {
	switch s {
	case StatusScheduled, StatusInProgress, StatusFinished, StatusPostponed, StatusSuspended, StatusCancelled:
		return true
	default:
		return false
	}
}

// Source names published by the adapters.
const (
	SourceCBF   = "cbf"
	SourceFMF   = "fmf"
	SourceFFDF  = "ffdf"
	SourceFGFGO = "fgf-go"
	SourceFPFPE = "fpf-pe"
	SourceFMFMA = "fmf-ma"
	SourceFMFMT = "fmf-mt"
	SourceFAFAL = "faf-al"
	SourceFBF   = "fbf"
	SourceFCF   = "fcf"
	SourceFPFPR = "fpf-pr"
)

// Collection states returned with API payloads.
const (
	CollectionOK             = "ok"
	CollectionNeverCollected = "never_collected"
)

// SourceMeta describes where a payload came from and whether it is stale.
type SourceMeta struct {
	Name             string
	CollectedAt      *time.Time
	LastSuccessAt    *time.Time
	Stale            bool
	CollectionStatus string
}

// Limits is the minimum snapshot the sync is allowed to publish.
type Limits struct {
	MinMatches   int
	MinStandings int
	MaxStandings int
}

// TeamInput is a club seen in one collection, before it has an internal id.
type TeamInput struct {
	ExternalID string
	Name       string
	ShortName  string
	State      string
	Aliases    []string
}

// MatchInput is one fixture from the source.
type MatchInput struct {
	ExternalID     string
	MatchNumber    int
	RoundNumber    int
	Phase          string
	HomeExternalID string
	AwayExternalID string
	Kickoff        *time.Time
	Status         MatchStatus
	HomeScore      *int
	AwayScore      *int
	HomePenalties  *int
	AwayPenalties  *int
	Venue          string
	City           string
	State          string
}

// StandingInput is one official table row. GoalDifference is the published
// figure, which can include a punishment. GroupName is empty for a single table.
type StandingInput struct {
	ExternalTeamID string
	GroupName      string
	GroupOrder     int
	Position       int
	Played         int
	Wins           int
	Draws          int
	Losses         int
	GoalsFor       int
	GoalsAgainst   int
	GoalDifference int
	Points         int
	YellowCards    int
	RedCards       int
	WinPercentage  int
}

// Snapshot is a validated collection ready to publish in one transaction.
type Snapshot struct {
	Source           string
	CompetitionCode  string
	CompetitionName  string
	SeasonYear       int
	SeasonName       string
	SeasonExternalID string
	Teams            []TeamInput
	Matches          []MatchInput
	Standings        []StandingInput
	CollectedAt      time.Time
}

// Validate rejects an empty, partial, or contradictory snapshot.
func (s Snapshot) Validate(limits Limits) error {
	if s.Source == "" || s.CompetitionCode == "" || s.CompetitionName == "" || s.SeasonName == "" || s.SeasonExternalID == "" {
		return fmt.Errorf("snapshot is missing its identity: %w", ErrInvalid)
	}
	if s.SeasonYear < 1990 || s.SeasonYear > 2100 {
		return fmt.Errorf("season year %d is out of range: %w", s.SeasonYear, ErrInvalid)
	}
	if s.CollectedAt.IsZero() {
		return fmt.Errorf("snapshot has no collection time: %w", ErrInvalid)
	}
	if len(s.Matches) < limits.MinMatches {
		return fmt.Errorf("match count %d is below %d: %w", len(s.Matches), limits.MinMatches, ErrIncomplete)
	}
	if len(s.Standings) < limits.MinStandings || len(s.Standings) > limits.MaxStandings {
		return fmt.Errorf("standings count %d is outside %d-%d: %w", len(s.Standings), limits.MinStandings, limits.MaxStandings, ErrIncomplete)
	}

	teams := make(map[string]struct{}, len(s.Teams))
	for _, team := range s.Teams {
		if team.ExternalID == "" || team.Name == "" || team.ShortName == "" {
			return fmt.Errorf("team is missing an id or name: %w", ErrInvalid)
		}
		if _, ok := teams[team.ExternalID]; ok {
			return fmt.Errorf("duplicate team %s: %w", team.ExternalID, ErrInvalid)
		}
		teams[team.ExternalID] = struct{}{}
	}

	seenMatch := make(map[string]struct{}, len(s.Matches))
	for _, match := range s.Matches {
		if err := validateMatch(match, teams); err != nil {
			return err
		}
		if _, ok := seenMatch[match.ExternalID]; ok {
			return fmt.Errorf("duplicate match %s: %w", match.ExternalID, ErrInvalid)
		}
		seenMatch[match.ExternalID] = struct{}{}
	}

	positions := make(map[string]struct{}, len(s.Standings))
	standingTeams := make(map[string]struct{}, len(s.Standings))
	for _, row := range s.Standings {
		if row.Position <= 0 || row.ExternalTeamID == "" {
			return fmt.Errorf("standing is missing a position or team: %w", ErrInvalid)
		}
		if row.GroupOrder < 0 {
			return fmt.Errorf("standing group order is negative: %w", ErrInvalid)
		}
		key := row.GroupName + "\x00" + fmt.Sprint(row.Position)
		if _, ok := positions[key]; ok {
			return fmt.Errorf("duplicate standing position %d in group %q: %w", row.Position, row.GroupName, ErrInvalid)
		}
		if _, ok := standingTeams[row.ExternalTeamID]; ok {
			return fmt.Errorf("duplicate standing team %s: %w", row.ExternalTeamID, ErrInvalid)
		}
		if _, ok := teams[row.ExternalTeamID]; !ok {
			return fmt.Errorf("standing team %s is not in the team list: %w", row.ExternalTeamID, ErrInvalid)
		}
		positions[key] = struct{}{}
		standingTeams[row.ExternalTeamID] = struct{}{}
	}
	return nil
}

func validateMatch(match MatchInput, teams map[string]struct{}) error {
	if match.ExternalID == "" || match.RoundNumber <= 0 || match.Phase == "" {
		return fmt.Errorf("match %s is missing an id, round, or phase: %w", match.ExternalID, ErrInvalid)
	}
	if _, ok := teams[match.HomeExternalID]; !ok {
		return fmt.Errorf("match %s home team is missing: %w", match.ExternalID, ErrInvalid)
	}
	if _, ok := teams[match.AwayExternalID]; !ok {
		return fmt.Errorf("match %s away team is missing: %w", match.ExternalID, ErrInvalid)
	}
	if match.HomeExternalID == match.AwayExternalID {
		return fmt.Errorf("match %s lists the same club twice: %w", match.ExternalID, ErrInvalid)
	}
	if !match.Status.Valid() {
		return fmt.Errorf("match %s has status %q: %w", match.ExternalID, match.Status, ErrInvalid)
	}
	if (match.HomeScore == nil) != (match.AwayScore == nil) {
		return fmt.Errorf("match %s has only one score: %w", match.ExternalID, ErrInvalid)
	}
	if match.HomeScore != nil && (*match.HomeScore < 0 || *match.AwayScore < 0) {
		return fmt.Errorf("match %s has a negative score: %w", match.ExternalID, ErrInvalid)
	}
	if (match.HomePenalties == nil) != (match.AwayPenalties == nil) {
		return fmt.Errorf("match %s has only one penalty score: %w", match.ExternalID, ErrInvalid)
	}
	if match.Status == StatusFinished && match.HomeScore == nil {
		return fmt.Errorf("match %s is finished without a score: %w", match.ExternalID, ErrInvalid)
	}
	if match.Status != StatusFinished && match.HomeScore != nil {
		return fmt.Errorf("match %s has a score but status %s: %w", match.ExternalID, match.Status, ErrInvalid)
	}
	return nil
}

// Competition is a stored competition.
type Competition struct {
	ID   int64
	Code string
	Name string
}

// Season is one year of a competition.
type Season struct {
	Year       int
	Name       string
	ExternalID string
}

// Team is a club with its internal id.
type Team struct {
	ID        int64
	Name      string
	ShortName string
	State     string
}

// Round is a numbered round inside a phase.
type Round struct {
	Number     int
	Phase      string
	Name       string
	MatchCount int
}

// Standing is an official table row joined to a team.
type Standing struct {
	GroupName      string
	GroupOrder     int
	Position       int
	Team           Team
	Played         int
	Wins           int
	Draws          int
	Losses         int
	GoalsFor       int
	GoalsAgainst   int
	GoalDifference int
	Points         int
	YellowCards    int
	RedCards       int
	WinPercentage  int
}

// Match is a stored fixture.
type Match struct {
	ID            int64
	MatchNumber   int
	Kickoff       *time.Time
	Status        MatchStatus
	HomeScore     *int
	AwayScore     *int
	HomePenalties *int
	AwayPenalties *int
	Venue         string
	City          string
	State         string
	RoundNumber   int
	Phase         string
	RoundName     string
	Home          Team
	Away          Team
	CollectedAt   time.Time
	UpdatedAt     time.Time
}

// MatchFilter selects a page of matches. Zero option pointers are ignored.
type MatchFilter struct {
	Round    *int
	TeamID   *int64
	Status   *MatchStatus
	From     *time.Time
	To       *time.Time
	Page     int
	PageSize int
}

// Page is a stable slice of a collection.
type Page[T any] struct {
	Items    []T
	Page     int
	PageSize int
	Total    int
	Source   SourceMeta
}

// ApplyStats counts rows written by one successful sync.
type ApplyStats struct {
	Teams     int
	Matches   int
	Standings int
}
