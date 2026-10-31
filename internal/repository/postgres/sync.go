package postgres

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Session holds the advisory lock for one sync.
type Session struct {
	runID       int64
	competition string
	year        int
	key         int64
	closed      bool
	release     func()
	exec        func(ctx context.Context, sql string, args ...any) error
	begin       func(ctx context.Context) (pgx.Tx, error)
}

// BeginSync takes the advisory lock and records a running sync.
// The lock is held until Close. A second caller receives ErrBusy.
func (s *Store) BeginSync(ctx context.Context, competition string, year int) (*Session, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, unavailable(err)
	}
	key := advisoryKey(competition, year)
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, key).Scan(&locked); err != nil {
		conn.Release()
		return nil, unavailable(err)
	}
	if !locked {
		conn.Release()
		return nil, domain.ErrBusy
	}
	session := &Session{
		runID:       0,
		competition: competition,
		year:        year,
		key:         key,
		release:     conn.Release,
		exec: func(ctx context.Context, sql string, args ...any) error {
			_, err := conn.Exec(ctx, sql, args...)
			return err
		},
		begin: conn.Begin,
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		session.Close()
		return nil, unavailable(err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO sync_runs (competition_code, season_year, source, status, started_at)
		VALUES ($1, $2, $3, 'running', now())
		RETURNING id`, competition, year, domain.SourceCBF).Scan(&session.runID); err != nil {
		_ = tx.Rollback(ctx)
		session.Close()
		return nil, unavailable(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sync_runs
		SET status = 'failed', finished_at = now(), error_message = 'interrupted before completion'
		WHERE competition_code = $1 AND season_year = $2 AND status = 'running' AND id <> $3`,
		competition, year, session.runID); err != nil {
		_ = tx.Rollback(ctx)
		session.Close()
		return nil, unavailable(err)
	}
	if err := tx.Commit(ctx); err != nil {
		session.Close()
		return nil, unavailable(err)
	}
	return session, nil
}

// Apply publishes the snapshot and marks the run successful in one transaction.
func (s *Session) Apply(ctx context.Context, snap domain.Snapshot) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return unavailable(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := publish(ctx, tx, snap, s.runID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return unavailable(err)
	}
	return nil
}

// Fail records the collection error and leaves domain rows unchanged.
func (s *Session) Fail(ctx context.Context, cause error) error {
	if cause == nil {
		cause = errors.New("sync failed")
	}
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	message := clip(cause.Error())
	if err := s.exec(writeCtx, `
		UPDATE sync_runs
		SET status = 'failed', finished_at = now(), error_message = $2
		WHERE id = $1 AND status = 'running'`, s.runID, message); err != nil {
		return unavailable(err)
	}
	if err := s.exec(writeCtx, `
		INSERT INTO season_sync_state (season_id, source, last_attempt_at, last_error)
		SELECT s.id, $3, now(), $4
		FROM seasons s
		JOIN competitions c ON c.id = s.competition_id
		WHERE c.code = $1 AND s.year = $2
		ON CONFLICT (season_id) DO UPDATE
		SET last_attempt_at = now(), last_error = EXCLUDED.last_error`,
		s.competition, s.year, domain.SourceCBF, message); err != nil {
		return unavailable(err)
	}
	return nil
}

// Close releases the advisory lock and the connection.
func (s *Session) Close() {
	if s == nil || s.closed {
		return
	}
	s.closed = true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.exec(ctx, `SELECT pg_advisory_unlock($1::bigint)`, s.key)
	s.release()
}

func publish(ctx context.Context, tx pgx.Tx, snap domain.Snapshot, runID int64) error {
	var competitionID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO competitions (code, name)
		VALUES ($1, $2)
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
		RETURNING id`, snap.CompetitionCode, snap.CompetitionName).Scan(&competitionID); err != nil {
		return unavailable(err)
	}
	var seasonID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO seasons (competition_id, year, name, external_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (competition_id, year) DO UPDATE
		SET name = EXCLUDED.name, external_id = EXCLUDED.external_id, updated_at = now()
		RETURNING id`, competitionID, snap.SeasonYear, snap.SeasonName, snap.SeasonExternalID).Scan(&seasonID); err != nil {
		return unavailable(err)
	}
	teamIDs := make(map[string]int64, len(snap.Teams))
	for _, team := range snap.Teams {
		id, err := upsertTeam(ctx, tx, snap.Source, team)
		if err != nil {
			return err
		}
		teamIDs[team.ExternalID] = id
		if _, err := tx.Exec(ctx, `
			INSERT INTO season_teams (season_id, team_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, seasonID, id); err != nil {
			return unavailable(err)
		}
	}
	roundIDs := map[string]int64{}
	for _, match := range snap.Matches {
		key := match.Phase + "\x00" + fmt.Sprint(match.RoundNumber)
		if _, ok := roundIDs[key]; ok {
			continue
		}
		var roundID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO rounds (season_id, phase, number, name)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (season_id, phase, number) DO UPDATE SET name = EXCLUDED.name
			RETURNING id`, seasonID, match.Phase, match.RoundNumber, fmt.Sprintf("Round %d", match.RoundNumber)).Scan(&roundID); err != nil {
			return unavailable(err)
		}
		roundIDs[key] = roundID
	}
	for _, match := range snap.Matches {
		home, ok := teamIDs[match.HomeExternalID]
		if !ok {
			return fmt.Errorf("match %s home team is missing: %w", match.ExternalID, domain.ErrInvalid)
		}
		away, ok := teamIDs[match.AwayExternalID]
		if !ok {
			return fmt.Errorf("match %s away team is missing: %w", match.ExternalID, domain.ErrInvalid)
		}
		roundID := roundIDs[match.Phase+"\x00"+fmt.Sprint(match.RoundNumber)]
		if _, err := tx.Exec(ctx, `
			INSERT INTO matches (
				season_id, round_id, home_team_id, away_team_id, source, external_id, match_number,
				kickoff_at, status, home_score, away_score, home_penalty_score, away_penalty_score,
				venue, city, state, collected_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
			)
			ON CONFLICT (source, external_id) DO UPDATE SET
				season_id = EXCLUDED.season_id,
				round_id = EXCLUDED.round_id,
				home_team_id = EXCLUDED.home_team_id,
				away_team_id = EXCLUDED.away_team_id,
				match_number = EXCLUDED.match_number,
				kickoff_at = EXCLUDED.kickoff_at,
				status = EXCLUDED.status,
				home_score = EXCLUDED.home_score,
				away_score = EXCLUDED.away_score,
				home_penalty_score = EXCLUDED.home_penalty_score,
				away_penalty_score = EXCLUDED.away_penalty_score,
				venue = EXCLUDED.venue,
				city = EXCLUDED.city,
				state = EXCLUDED.state,
				collected_at = EXCLUDED.collected_at,
				updated_at = now()`,
			seasonID, roundID, home, away, snap.Source, match.ExternalID, nullInt(match.MatchNumber),
			match.Kickoff, string(match.Status), match.HomeScore, match.AwayScore, match.HomePenalties, match.AwayPenalties,
			nullIfEmpty(match.Venue), nullIfEmpty(match.City), nullIfEmpty(match.State), snap.CollectedAt,
		); err != nil {
			return unavailable(err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM standings WHERE season_id = $1`, seasonID); err != nil {
		return unavailable(err)
	}
	for _, row := range snap.Standings {
		teamID, ok := teamIDs[row.ExternalTeamID]
		if !ok {
			return fmt.Errorf("standing team %s is missing: %w", row.ExternalTeamID, domain.ErrInvalid)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO standings (
				season_id, team_id, position, group_name, group_order, played, wins, draws, losses,
				goals_for, goals_against, goal_difference, points, yellow_cards, red_cards,
				win_percentage, source, collected_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18
			)`,
			seasonID, teamID, row.Position, row.GroupName, row.GroupOrder, row.Played, row.Wins, row.Draws, row.Losses,
			row.GoalsFor, row.GoalsAgainst, row.GoalDifference, row.Points, row.YellowCards, row.RedCards,
			row.WinPercentage, snap.Source, snap.CollectedAt,
		); err != nil {
			return unavailable(err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO season_sync_state (season_id, source, last_attempt_at, last_success_at, last_error)
		VALUES ($1, $2, $3, $3, NULL)
		ON CONFLICT (season_id) DO UPDATE SET
			source = EXCLUDED.source,
			last_attempt_at = EXCLUDED.last_attempt_at,
			last_success_at = EXCLUDED.last_success_at,
			last_error = NULL`, seasonID, snap.Source, snap.CollectedAt); err != nil {
		return unavailable(err)
	}
	processed := len(snap.Teams) + len(snap.Matches) + len(snap.Standings)
	if _, err := tx.Exec(ctx, `
		UPDATE sync_runs
		SET status = 'success', finished_at = $2, records_processed = $3, error_message = NULL
		WHERE id = $1`, runID, snap.CollectedAt, processed); err != nil {
		return unavailable(err)
	}
	return nil
}

func upsertTeam(ctx context.Context, tx pgx.Tx, source string, team domain.TeamInput) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		SELECT entity_id FROM source_refs
		WHERE source = $1 AND entity_type = 'team' AND external_id = $2`, source, team.ExternalID).Scan(&id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, unavailable(err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `
			INSERT INTO teams (name, short_name, state) VALUES ($1, $2, $3) RETURNING id`,
			team.Name, team.ShortName, nullIfEmpty(team.State)).Scan(&id); err != nil {
			return 0, unavailable(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO source_refs (source, entity_type, external_id, entity_id)
			VALUES ($1, 'team', $2, $3)`, source, team.ExternalID, id); err != nil {
			return 0, unavailable(err)
		}
	} else if _, err := tx.Exec(ctx, `
		UPDATE teams
		SET name = $2, short_name = $3, state = COALESCE(NULLIF($4, ''), state), updated_at = now()
		WHERE id = $1`, id, team.Name, team.ShortName, team.State); err != nil {
		return 0, unavailable(err)
	}
	for _, alias := range team.Aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || strings.EqualFold(alias, team.Name) {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO team_aliases (team_id, alias) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, id, alias); err != nil {
			return 0, unavailable(err)
		}
	}
	return id, nil
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func clip(message string) string {
	if len(message) > 500 {
		return message[:500]
	}
	return message
}

func advisoryKey(competition string, year int) int64 {
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "brasileirao:%s:%d", competition, year)
	return int64(hash.Sum64())
}
