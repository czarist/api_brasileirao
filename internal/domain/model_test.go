package domain

import (
	"errors"
	"testing"
	"time"
)

func TestSnapshotValidate(t *testing.T) {
	snap := sampleSnapshot()
	if err := snap.Validate(Limits{MinMatches: 1, MinStandings: 1, MaxStandings: 2}); err != nil {
		t.Fatal(err)
	}
	snap.Matches[0].HomeScore = nil
	if err := snap.Validate(Limits{MinMatches: 1, MinStandings: 1, MaxStandings: 2}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("finished without a full score: %v", err)
	}
	snap = sampleSnapshot()
	snap.Matches[0].Status = StatusScheduled
	if err := snap.Validate(Limits{MinMatches: 1, MinStandings: 1, MaxStandings: 2}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("score on a scheduled match: %v", err)
	}
	snap = sampleSnapshot()
	zero := 0
	snap.Matches[0].HomeScore = &zero
	snap.Matches[0].AwayScore = &zero
	if err := snap.Validate(Limits{MinMatches: 1, MinStandings: 1, MaxStandings: 2}); err != nil {
		t.Fatalf("real 0-0 was rejected: %v", err)
	}
	snap = sampleSnapshot()
	if err := snap.Validate(Limits{MinMatches: 5, MinStandings: 1, MaxStandings: 2}); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("short match list: %v", err)
	}
	snap = sampleSnapshot()
	snap.Standings = append(snap.Standings, StandingInput{ExternalTeamID: "2", GroupName: "GRUPO B", GroupOrder: 1, Position: 1})
	if err := snap.Validate(Limits{MinMatches: 1, MinStandings: 1, MaxStandings: 2}); err != nil {
		t.Fatalf("position repeated in another group: %v", err)
	}
	snap.Standings[1].GroupName = ""
	if err := snap.Validate(Limits{MinMatches: 1, MinStandings: 1, MaxStandings: 2}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate position in one table: %v", err)
	}
}

func sampleSnapshot() Snapshot {
	home := 2
	away := 1
	kick := time.Date(2026, 1, 28, 22, 0, 0, 0, time.UTC)
	return Snapshot{
		Source:           SourceCBF,
		CompetitionCode:  "brasileirao-serie-a",
		CompetitionName:  "Campeonato Brasileiro Série A",
		SeasonYear:       2026,
		SeasonName:       "Campeonato Brasileiro Série A 2026",
		SeasonExternalID: "1260611",
		CollectedAt:      time.Unix(10, 0).UTC(),
		Teams: []TeamInput{
			{ExternalID: "1", Name: "Home", ShortName: "HOM"},
			{ExternalID: "2", Name: "Away", ShortName: "AWA"},
		},
		Matches: []MatchInput{{
			ExternalID: "m1", RoundNumber: 1, Phase: "Fase Única",
			HomeExternalID: "1", AwayExternalID: "2", Kickoff: &kick,
			Status: StatusFinished, HomeScore: &home, AwayScore: &away,
		}},
		Standings: []StandingInput{{ExternalTeamID: "1", Position: 1, Points: 3}},
	}
}
