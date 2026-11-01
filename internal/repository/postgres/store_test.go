package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"github.com/czarist/api_brasileirao/internal/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSyncIsIdempotentAndKeepsRowsAfterFailure(t *testing.T) {
	store, pool := newTestStore(t)
	ctx := context.Background()
	snap := testSnapshot()

	session, err := store.BeginSync(ctx, snap.CompetitionCode, snap.SeasonYear)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginSync(ctx, snap.CompetitionCode, snap.SeasonYear); !errors.Is(err, domain.ErrBusy) {
		session.Close()
		t.Fatalf("overlapping sync: %v", err)
	}
	if err := session.Apply(ctx, snap); err != nil {
		session.Close()
		t.Fatal(err)
	}
	session.Close()

	matches, err := store.ListMatches(ctx, snap.CompetitionCode, snap.SeasonYear, domain.MatchFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if matches.Total != 2 || matches.Items[0].Status != domain.StatusFinished || matches.Items[1].Status != domain.StatusPostponed {
		t.Fatalf("matches = %+v", matches.Items)
	}
	if matches.Items[0].Kickoff == nil || matches.Items[1].Kickoff != nil || matches.Items[1].HomeScore != nil {
		t.Fatalf("order or nulls = %+v / %+v", matches.Items[0], matches.Items[1])
	}
	finishedID := matches.Items[0].ID
	var teamID int64
	if err := pool.QueryRow(ctx, `SELECT entity_id FROM source_refs WHERE source = 'cbf' AND entity_type = 'team' AND external_id = '10'`).Scan(&teamID); err != nil {
		t.Fatal(err)
	}

	failed, err := store.BeginSync(ctx, snap.CompetitionCode, snap.SeasonYear)
	if err != nil {
		t.Fatal(err)
	}
	if err := failed.Fail(ctx, errors.New("standings page is empty")); err != nil {
		failed.Close()
		t.Fatal(err)
	}
	failed.Close()

	standings, meta, err := store.ListStandings(ctx, snap.CompetitionCode, snap.SeasonYear)
	if err != nil {
		t.Fatal(err)
	}
	if len(standings) != 2 || standings[0].Points != 3 || !meta.Stale || meta.CollectionStatus != domain.CollectionOK || meta.Name != domain.SourceCBF {
		t.Fatalf("after failure standings=%+v meta=%+v", standings, meta)
	}
	again, err := store.ListMatches(ctx, snap.CompetitionCode, snap.SeasonYear, domain.MatchFilter{Page: 1, PageSize: 20})
	if err != nil || again.Items[0].ID != finishedID {
		t.Fatalf("match id changed: %v %+v", err, again.Items)
	}

	snap.Teams[0].Name = "Alpha FC"
	snap.Standings[0].Points = 4
	snap.CollectedAt = snap.CollectedAt.Add(time.Hour)
	updated, err := store.BeginSync(ctx, snap.CompetitionCode, snap.SeasonYear)
	if err != nil {
		t.Fatal(err)
	}
	if err := updated.Apply(ctx, snap); err != nil {
		updated.Close()
		t.Fatal(err)
	}
	updated.Close()
	var name string
	var id int64
	if err := pool.QueryRow(ctx, `SELECT id, name FROM teams WHERE id = $1`, teamID).Scan(&id, &name); err != nil {
		t.Fatal(err)
	}
	if id != teamID || name != "Alpha FC" {
		t.Fatalf("team changed id=%d name=%s", id, name)
	}
	standings, meta, err = store.ListStandings(ctx, snap.CompetitionCode, snap.SeasonYear)
	if err != nil || standings[0].Points != 4 || meta.Stale {
		t.Fatalf("updated standings=%+v meta=%+v err=%v", standings, meta, err)
	}

	orphaned, err := store.BeginSync(ctx, snap.CompetitionCode, snap.SeasonYear)
	if err != nil {
		t.Fatal(err)
	}
	orphaned.Close()
	next, err := store.BeginSync(ctx, snap.CompetitionCode, snap.SeasonYear)
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
	var interrupted int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM sync_runs WHERE status = 'failed' AND error_message = 'interrupted before completion'`).Scan(&interrupted); err != nil {
		t.Fatal(err)
	}
	if interrupted == 0 {
		t.Fatal("an abandoned run was not marked failed")
	}
}

func TestReadMetaUsesTheStoredAdapter(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	cbfSnap := testSnapshot()
	fmfSnap := testSnapshot()
	fmfSnap.Source = domain.SourceFMF
	fmfSnap.CompetitionCode = "mineiro-modulo-i"
	fmfSnap.CompetitionName = "Campeonato Mineiro Módulo I"
	fmfSnap.SeasonYear = 2098
	fmfSnap.SeasonName = "MÓDULO I - 2098"
	fmfSnap.SeasonExternalID = "1899"
	fmfSnap.CollectedAt = cbfSnap.CollectedAt.Add(time.Hour)
	for i := range fmfSnap.Matches {
		fmfSnap.Matches[i].ExternalID = "fmf-" + fmfSnap.Matches[i].ExternalID
	}
	apply := func(snap domain.Snapshot) {
		t.Helper()
		session, err := store.BeginSync(ctx, snap.CompetitionCode, snap.SeasonYear)
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Apply(ctx, snap); err != nil {
			session.Close()
			t.Fatal(err)
		}
		session.Close()
	}
	apply(cbfSnap)
	apply(fmfSnap)

	_, cbfMeta, err := store.ListStandings(ctx, cbfSnap.CompetitionCode, cbfSnap.SeasonYear)
	if err != nil || cbfMeta.Name != domain.SourceCBF {
		t.Fatalf("serie a source: %v %+v", err, cbfMeta)
	}
	_, fmfMeta, err := store.ListStandings(ctx, fmfSnap.CompetitionCode, fmfSnap.SeasonYear)
	if err != nil || fmfMeta.Name != domain.SourceFMF {
		t.Fatalf("mineiro source: %v %+v", err, fmfMeta)
	}
	_, compMeta, err := store.GetCompetition(ctx, cbfSnap.CompetitionCode)
	if err != nil || compMeta.Name != domain.SourceCBF {
		t.Fatalf("serie a competition source: %v %+v", err, compMeta)
	}
	_, listMeta, err := store.ListCompetitions(ctx)
	if err != nil || listMeta.Name != domain.SourceFMF {
		t.Fatalf("list source: %v %+v", err, listMeta)
	}
}

func newTestStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	schema := "it_" + hex.EncodeToString(raw[:])
	if !regexp.MustCompile(`^it_[a-z0-9]+$`).MatchString(schema) {
		t.Fatal(schema)
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	admin.Close()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 4
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SELECT set_config('search_path', $1, false)", schema)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		drop, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			return
		}
		defer drop.Close()
		_, _ = drop.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	if err := migrate.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return New(pool, time.Hour), pool
}

func testSnapshot() domain.Snapshot {
	home, away := 1, 0
	kick := time.Date(2099, 5, 2, 18, 0, 0, 0, time.UTC)
	return domain.Snapshot{
		Source:           domain.SourceCBF,
		CompetitionCode:  "brasileirao-serie-a",
		CompetitionName:  "Campeonato Brasileiro Série A",
		SeasonYear:       2099,
		SeasonName:       "Campeonato Brasileiro Série A 2099",
		SeasonExternalID: "999",
		CollectedAt:      time.Date(2099, 5, 3, 0, 0, 0, 0, time.UTC),
		Teams: []domain.TeamInput{
			{ExternalID: "10", Name: "Alpha", ShortName: "ALP", State: "SP", Aliases: []string{"Alpha"}},
			{ExternalID: "20", Name: "Beta", ShortName: "BET", State: "RJ"},
		},
		Matches: []domain.MatchInput{
			{ExternalID: "900", MatchNumber: 1, RoundNumber: 1, Phase: "Fase Única", HomeExternalID: "10", AwayExternalID: "20", Kickoff: &kick, Status: domain.StatusFinished, HomeScore: &home, AwayScore: &away, Venue: "Arena"},
			{ExternalID: "901", MatchNumber: 2, RoundNumber: 2, Phase: "Fase Única", HomeExternalID: "20", AwayExternalID: "10", Status: domain.StatusPostponed},
		},
		Standings: []domain.StandingInput{
			{ExternalTeamID: "10", Position: 1, Points: 3, Played: 1, Wins: 1},
			{ExternalTeamID: "20", Position: 2, Points: 0, Played: 1, Losses: 1},
		},
	}
}
