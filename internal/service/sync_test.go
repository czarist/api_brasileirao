package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
)

type fakeSession struct {
	applied int
	failed  string
}

func (f *fakeSession) Apply(ctx context.Context, snap domain.Snapshot) error {
	f.applied++
	return nil
}

func (f *fakeSession) Fail(ctx context.Context, cause error) error {
	f.failed = cause.Error()
	return nil
}

func (f *fakeSession) Close() {}

type fakeGate struct {
	session *fakeSession
	busy    bool
	calls   int
}

func (g *fakeGate) BeginSync(ctx context.Context, competition string, year int) (SyncSession, error) {
	g.calls++
	if g.busy {
		return nil, domain.ErrBusy
	}
	return g.session, nil
}

type fakeSource struct {
	snap domain.Snapshot
	err  error
	hits int
}

func (f *fakeSource) Fetch(ctx context.Context, year int, collected time.Time) (domain.Snapshot, error) {
	f.hits++
	if f.err != nil {
		return domain.Snapshot{}, f.err
	}
	return f.snap, nil
}

func TestSyncKeepsPreviousRowsWhenCollectionFails(t *testing.T) {
	session := &fakeSession{}
	source := &fakeSource{err: errors.New("empty standings page")}
	syncer := New(&fakeGate{session: session}, source, domain.Limits{MinMatches: 1, MinStandings: 1, MaxStandings: 2}, nil)
	err := syncer.Sync(context.Background(), "brasileirao-serie-a", 2026)
	if err == nil || session.applied != 0 || session.failed == "" {
		t.Fatalf("err=%v applied=%d failed=%q", err, session.applied, session.failed)
	}
}

func TestSyncDoesNotFetchWhenBusy(t *testing.T) {
	source := &fakeSource{}
	syncer := New(&fakeGate{busy: true}, source, domain.Limits{}, nil)
	err := syncer.Sync(context.Background(), "brasileirao-serie-a", 2026)
	if !errors.Is(err, domain.ErrBusy) || source.hits != 0 {
		t.Fatalf("err=%v hits=%d", err, source.hits)
	}
}

func TestSyncPublishesAValidSnapshot(t *testing.T) {
	home, away := 1, 0
	kick := time.Date(2026, 3, 1, 19, 0, 0, 0, time.UTC)
	snap := domain.Snapshot{
		Source: "cbf", CompetitionCode: "brasileirao-serie-a", CompetitionName: "Campeonato Brasileiro Série A",
		SeasonYear: 2026, SeasonName: "Campeonato Brasileiro Série A 2026", SeasonExternalID: "1260611",
		CollectedAt: time.Unix(20, 0).UTC(),
		Teams:       []domain.TeamInput{{ExternalID: "1", Name: "A", ShortName: "AAA"}, {ExternalID: "2", Name: "B", ShortName: "BBB"}},
		Matches: []domain.MatchInput{{
			ExternalID: "m", RoundNumber: 1, Phase: "Fase Única", HomeExternalID: "1", AwayExternalID: "2",
			Kickoff: &kick, Status: domain.StatusFinished, HomeScore: &home, AwayScore: &away,
		}},
		Standings: []domain.StandingInput{{ExternalTeamID: "1", Position: 1, Points: 3}},
	}
	session := &fakeSession{}
	syncer := New(&fakeGate{session: session}, &fakeSource{snap: snap}, domain.Limits{MinMatches: 1, MinStandings: 1, MaxStandings: 2}, nil)
	if err := syncer.Sync(context.Background(), "brasileirao-serie-a", 2026); err != nil {
		t.Fatal(err)
	}
	if session.applied != 1 || session.failed != "" {
		t.Fatalf("applied=%d failed=%q", session.applied, session.failed)
	}
}
