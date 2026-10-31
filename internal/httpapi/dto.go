package httpapi

import (
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

type sourceDTO struct {
	Name             string     `json:"name"`
	CollectedAt      *time.Time `json:"collected_at"`
	LastSuccessAt    *time.Time `json:"last_success_at"`
	Stale            bool       `json:"stale"`
	CollectionStatus string     `json:"collection_status"`
}

type teamDTO struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	ShortName string  `json:"short_name"`
	State     *string `json:"state"`
}

type competitionDTO struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type seasonDTO struct {
	Year       int    `json:"year"`
	Name       string `json:"name"`
	ExternalID string `json:"external_id"`
}

type standingDTO struct {
	Position       int     `json:"position"`
	GroupName      string  `json:"group_name"`
	Team           teamDTO `json:"team"`
	Played         int     `json:"played"`
	Wins           int     `json:"wins"`
	Draws          int     `json:"draws"`
	Losses         int     `json:"losses"`
	GoalsFor       int     `json:"goals_for"`
	GoalsAgainst   int     `json:"goals_against"`
	GoalDifference int     `json:"goal_difference"`
	Points         int     `json:"points"`
	YellowCards    int     `json:"yellow_cards"`
	RedCards       int     `json:"red_cards"`
	WinPercentage  int     `json:"win_percentage"`
}

type roundDTO struct {
	Number     int    `json:"number"`
	Phase      string `json:"phase"`
	Name       string `json:"name"`
	MatchCount int    `json:"match_count"`
}

type matchDTO struct {
	ID            int64      `json:"id"`
	MatchNumber   *int       `json:"match_number"`
	KickoffAt     *time.Time `json:"kickoff_at"`
	Status        string     `json:"status"`
	HomeScore     *int       `json:"home_score"`
	AwayScore     *int       `json:"away_score"`
	HomePenalties *int       `json:"home_penalties"`
	AwayPenalties *int       `json:"away_penalties"`
	Venue         *string    `json:"venue"`
	City          *string    `json:"city"`
	State         *string    `json:"state"`
	RoundNumber   int        `json:"round_number"`
	Phase         string     `json:"phase"`
	RoundName     string     `json:"round_name"`
	Home          teamDTO    `json:"home"`
	Away          teamDTO    `json:"away"`
	CollectedAt   time.Time  `json:"collected_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type listDTO[T any] struct {
	Data   []T       `json:"data"`
	Source sourceDTO `json:"source"`
}

type matchListDTO struct {
	Data     []matchDTO `json:"data"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int        `json:"total"`
	Source   sourceDTO  `json:"source"`
}

type errorDTO struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func newSourceDTO(meta domain.SourceMeta) sourceDTO {
	return sourceDTO{
		Name:             meta.Name,
		CollectedAt:      meta.CollectedAt,
		LastSuccessAt:    meta.LastSuccessAt,
		Stale:            meta.Stale,
		CollectionStatus: meta.CollectionStatus,
	}
}

func newTeamDTO(team domain.Team) teamDTO {
	return teamDTO{ID: team.ID, Name: team.Name, ShortName: team.ShortName, State: emptyNil(team.State)}
}

func newMatchDTO(match domain.Match) matchDTO {
	var number *int
	if match.MatchNumber != 0 {
		n := match.MatchNumber
		number = &n
	}
	return matchDTO{
		ID:            match.ID,
		MatchNumber:   number,
		KickoffAt:     match.Kickoff,
		Status:        string(match.Status),
		HomeScore:     match.HomeScore,
		AwayScore:     match.AwayScore,
		HomePenalties: match.HomePenalties,
		AwayPenalties: match.AwayPenalties,
		Venue:         emptyNil(match.Venue),
		City:          emptyNil(match.City),
		State:         emptyNil(match.State),
		RoundNumber:   match.RoundNumber,
		Phase:         match.Phase,
		RoundName:     match.RoundName,
		Home:          newTeamDTO(match.Home),
		Away:          newTeamDTO(match.Away),
		CollectedAt:   match.CollectedAt.UTC(),
		UpdatedAt:     match.UpdatedAt.UTC(),
	}
}

func emptyNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
