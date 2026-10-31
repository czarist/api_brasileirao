package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ListCompetitions returns every stored competition.
func (s *Store) ListCompetitions(ctx context.Context) ([]domain.Competition, domain.SourceMeta, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, code, name FROM competitions ORDER BY name, id`)
	if err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	defer rows.Close()
	var out []domain.Competition
	for rows.Next() {
		var item domain.Competition
		if err := rows.Scan(&item.ID, &item.Code, &item.Name); err != nil {
			return nil, domain.SourceMeta{}, unavailable(err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	meta, err := s.latestMeta(ctx)
	if err != nil {
		return nil, domain.SourceMeta{}, err
	}
	return out, meta, nil
}

// GetCompetition returns one competition by its stable code.
func (s *Store) GetCompetition(ctx context.Context, code string) (domain.Competition, domain.SourceMeta, error) {
	var item domain.Competition
	err := s.pool.QueryRow(ctx, `SELECT id, code, name FROM competitions WHERE code = $1`, code).Scan(&item.ID, &item.Code, &item.Name)
	if err != nil {
		return domain.Competition{}, domain.SourceMeta{}, unavailable(err)
	}
	meta, err := s.competitionMeta(ctx, item.ID)
	if err != nil {
		return domain.Competition{}, domain.SourceMeta{}, err
	}
	return item, meta, nil
}

// ListSeasons returns the years stored for a competition.
func (s *Store) ListSeasons(ctx context.Context, code string) ([]domain.Season, domain.SourceMeta, error) {
	competition, meta, err := s.GetCompetition(ctx, code)
	if err != nil {
		return nil, domain.SourceMeta{}, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT year, name, external_id
		FROM seasons
		WHERE competition_id = $1
		ORDER BY year`, competition.ID)
	if err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	defer rows.Close()
	var out []domain.Season
	for rows.Next() {
		var item domain.Season
		if err := rows.Scan(&item.Year, &item.Name, &item.ExternalID); err != nil {
			return nil, domain.SourceMeta{}, unavailable(err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	return out, meta, nil
}

// GetSeason returns one season.
func (s *Store) GetSeason(ctx context.Context, code string, year int) (domain.Season, domain.SourceMeta, error) {
	id, err := s.seasonID(ctx, code, year)
	if err != nil {
		return domain.Season{}, domain.SourceMeta{}, err
	}
	var item domain.Season
	err = s.pool.QueryRow(ctx, `SELECT year, name, external_id FROM seasons WHERE id = $1`, id).Scan(&item.Year, &item.Name, &item.ExternalID)
	if err != nil {
		return domain.Season{}, domain.SourceMeta{}, unavailable(err)
	}
	meta, err := s.seasonMeta(ctx, id)
	if err != nil {
		return domain.Season{}, domain.SourceMeta{}, err
	}
	return item, meta, nil
}

// ListTeams returns the clubs of a season.
func (s *Store) ListTeams(ctx context.Context, code string, year int) ([]domain.Team, domain.SourceMeta, error) {
	id, meta, err := s.requireSeason(ctx, code, year)
	if err != nil {
		return nil, domain.SourceMeta{}, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.name, t.short_name, COALESCE(t.state, '')
		FROM teams t
		JOIN season_teams st ON st.team_id = t.id
		WHERE st.season_id = $1
		ORDER BY t.name, t.id`, id)
	if err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	defer rows.Close()
	out, err := scanTeams(rows)
	if err != nil {
		return nil, domain.SourceMeta{}, err
	}
	return out, meta, nil
}

// ListStandings returns the official table in group order, then position.
func (s *Store) ListStandings(ctx context.Context, code string, year int) ([]domain.Standing, domain.SourceMeta, error) {
	id, meta, err := s.requireSeason(ctx, code, year)
	if err != nil {
		return nil, domain.SourceMeta{}, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT st.group_name, st.group_order, st.position, t.id, t.name, t.short_name, COALESCE(t.state, ''),
		       st.played, st.wins, st.draws, st.losses, st.goals_for, st.goals_against,
		       st.goal_difference, st.points, st.yellow_cards, st.red_cards, st.win_percentage
		FROM standings st
		JOIN teams t ON t.id = st.team_id
		WHERE st.season_id = $1
		ORDER BY st.group_order, st.position, st.id`, id)
	if err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	defer rows.Close()
	var out []domain.Standing
	for rows.Next() {
		var row domain.Standing
		if err := rows.Scan(
			&row.GroupName, &row.GroupOrder, &row.Position, &row.Team.ID, &row.Team.Name, &row.Team.ShortName, &row.Team.State,
			&row.Played, &row.Wins, &row.Draws, &row.Losses, &row.GoalsFor, &row.GoalsAgainst,
			&row.GoalDifference, &row.Points, &row.YellowCards, &row.RedCards, &row.WinPercentage,
		); err != nil {
			return nil, domain.SourceMeta{}, unavailable(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	return out, meta, nil
}

// ListRounds returns rounds and their match counts.
func (s *Store) ListRounds(ctx context.Context, code string, year int) ([]domain.Round, domain.SourceMeta, error) {
	id, meta, err := s.requireSeason(ctx, code, year)
	if err != nil {
		return nil, domain.SourceMeta{}, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.number, r.phase, r.name, COUNT(m.id)
		FROM rounds r
		LEFT JOIN matches m ON m.round_id = r.id
		WHERE r.season_id = $1
		GROUP BY r.id
		ORDER BY r.number, r.id`, id)
	if err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	defer rows.Close()
	var out []domain.Round
	for rows.Next() {
		var row domain.Round
		if err := rows.Scan(&row.Number, &row.Phase, &row.Name, &row.MatchCount); err != nil {
			return nil, domain.SourceMeta{}, unavailable(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.SourceMeta{}, unavailable(err)
	}
	return out, meta, nil
}

// ListMatches returns one stable page of fixtures.
func (s *Store) ListMatches(ctx context.Context, code string, year int, filter domain.MatchFilter) (domain.Page[domain.Match], error) {
	id, meta, err := s.requireSeason(ctx, code, year)
	if err != nil {
		return domain.Page[domain.Match]{}, err
	}
	where, args := matchFilterSQL(id, filter)
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM matches m JOIN rounds r ON r.id = m.round_id WHERE `+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.Match]{}, unavailable(err)
	}
	offset := int64(filter.Page-1) * int64(filter.PageSize)
	args = append(args, filter.PageSize, offset)
	limit := len(args) - 1
	query := matchSelect + ` WHERE ` + where + fmt.Sprintf(` ORDER BY m.kickoff_at ASC NULLS LAST, m.id ASC LIMIT $%d OFFSET $%d`, limit, limit+1)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.Match]{}, unavailable(err)
	}
	defer rows.Close()
	items, err := scanMatches(rows)
	if err != nil {
		return domain.Page[domain.Match]{}, err
	}
	if items == nil {
		items = []domain.Match{}
	}
	return domain.Page[domain.Match]{
		Items:    items,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
		Source:   meta,
	}, nil
}

// GetMatch returns one fixture by its internal id.
func (s *Store) GetMatch(ctx context.Context, id int64) (domain.Match, domain.SourceMeta, error) {
	rows, err := s.pool.Query(ctx, matchSelect+` WHERE m.id = $1`, id)
	if err != nil {
		return domain.Match{}, domain.SourceMeta{}, unavailable(err)
	}
	defer rows.Close()
	items, err := scanMatches(rows)
	if err != nil {
		return domain.Match{}, domain.SourceMeta{}, err
	}
	if len(items) == 0 {
		return domain.Match{}, domain.SourceMeta{}, domain.ErrNotFound
	}
	var seasonID int64
	if err := s.pool.QueryRow(ctx, `SELECT season_id FROM matches WHERE id = $1`, id).Scan(&seasonID); err != nil {
		return domain.Match{}, domain.SourceMeta{}, unavailable(err)
	}
	meta, err := s.seasonMeta(ctx, seasonID)
	if err != nil {
		return domain.Match{}, domain.SourceMeta{}, err
	}
	return items[0], meta, nil
}

func (s *Store) requireSeason(ctx context.Context, code string, year int) (int64, domain.SourceMeta, error) {
	id, err := s.seasonID(ctx, code, year)
	if err != nil {
		return 0, domain.SourceMeta{}, err
	}
	meta, err := s.seasonMeta(ctx, id)
	if err != nil {
		return 0, domain.SourceMeta{}, err
	}
	return id, meta, nil
}

func (s *Store) seasonID(ctx context.Context, code string, year int) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		SELECT s.id
		FROM seasons s
		JOIN competitions c ON c.id = s.competition_id
		WHERE c.code = $1 AND s.year = $2`, code, year).Scan(&id)
	if err != nil {
		return 0, unavailable(err)
	}
	return id, nil
}

func (s *Store) seasonMeta(ctx context.Context, seasonID int64) (domain.SourceMeta, error) {
	var lastSuccess *time.Time
	var lastError *string
	var collected *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT st.last_success_at,
		       NULLIF(st.last_error, ''),
		       GREATEST(
		           (SELECT MAX(collected_at) FROM matches WHERE season_id = $1),
		           (SELECT MAX(collected_at) FROM standings WHERE season_id = $1)
		       )
		FROM season_sync_state st
		WHERE st.season_id = $1`, seasonID).Scan(&lastSuccess, &lastError, &collected)
	if err != nil {
		if unavailable(err) == domain.ErrNotFound {
			return s.meta(nil, nil, nil), nil
		}
		return domain.SourceMeta{}, unavailable(err)
	}
	return s.meta(lastSuccess, lastError, collected), nil
}

func (s *Store) competitionMeta(ctx context.Context, competitionID int64) (domain.SourceMeta, error) {
	var lastSuccess *time.Time
	var lastError *string
	var collected *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT st.last_success_at,
		       NULLIF(st.last_error, ''),
		       (SELECT MAX(m.collected_at) FROM matches m JOIN seasons s2 ON s2.id = m.season_id WHERE s2.competition_id = $1)
		FROM seasons s
		JOIN season_sync_state st ON st.season_id = s.id
		WHERE s.competition_id = $1
		ORDER BY st.last_attempt_at DESC NULLS LAST
		LIMIT 1`, competitionID).Scan(&lastSuccess, &lastError, &collected)
	if err != nil {
		if unavailable(err) == domain.ErrNotFound {
			return s.meta(nil, nil, nil), nil
		}
		return domain.SourceMeta{}, unavailable(err)
	}
	return s.meta(lastSuccess, lastError, collected), nil
}

func (s *Store) latestMeta(ctx context.Context) (domain.SourceMeta, error) {
	var lastSuccess *time.Time
	var lastError *string
	var collected *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT st.last_success_at,
		       NULLIF(st.last_error, ''),
		       (SELECT MAX(collected_at) FROM matches)
		FROM season_sync_state st
		ORDER BY st.last_attempt_at DESC NULLS LAST
		LIMIT 1`).Scan(&lastSuccess, &lastError, &collected)
	if err != nil {
		if unavailable(err) == domain.ErrNotFound {
			return s.meta(nil, nil, nil), nil
		}
		return domain.SourceMeta{}, unavailable(err)
	}
	return s.meta(lastSuccess, lastError, collected), nil
}

func (s *Store) meta(lastSuccess *time.Time, lastError *string, collected *time.Time) domain.SourceMeta {
	meta := domain.SourceMeta{
		Name:          domain.SourceCBF,
		CollectedAt:   collected,
		LastSuccessAt: lastSuccess,
	}
	if lastSuccess == nil {
		meta.Stale = true
		meta.CollectionStatus = domain.CollectionNeverCollected
		return meta
	}
	meta.CollectionStatus = domain.CollectionOK
	if (lastError != nil && *lastError != "") || s.now().Sub(*lastSuccess) > s.staleAfter {
		meta.Stale = true
	}
	return meta
}

func matchFilterSQL(seasonID int64, filter domain.MatchFilter) (string, []any) {
	args := []any{seasonID, argPtr(filter.Round), argPtr(filter.TeamID), statusArg(filter.Status), argTime(filter.From), argTime(filter.To)}
	where := `m.season_id = $1
		AND ($2::integer IS NULL OR r.number = $2)
		AND ($3::bigint IS NULL OR m.home_team_id = $3 OR m.away_team_id = $3)
		AND ($4::text IS NULL OR m.status = $4)
		AND ($5::timestamptz IS NULL OR m.kickoff_at >= $5)
		AND ($6::timestamptz IS NULL OR m.kickoff_at <= $6)`
	return where, args
}

func statusArg(status *domain.MatchStatus) any {
	if status == nil {
		return nil
	}
	return string(*status)
}

func argPtr[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}

func argTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

const matchSelect = `
SELECT m.id, m.match_number, m.kickoff_at, m.status, m.home_score, m.away_score,
       m.home_penalty_score, m.away_penalty_score, COALESCE(m.venue, ''), COALESCE(m.city, ''), COALESCE(m.state, ''),
       r.number, r.phase, r.name,
       h.id, h.name, h.short_name, COALESCE(h.state, ''),
       a.id, a.name, a.short_name, COALESCE(a.state, ''),
       m.collected_at, m.updated_at
FROM matches m
JOIN rounds r ON r.id = m.round_id
JOIN teams h ON h.id = m.home_team_id
JOIN teams a ON a.id = m.away_team_id`

func scanMatches(rows pgx.Rows) ([]domain.Match, error) {
	var out []domain.Match
	for rows.Next() {
		var item domain.Match
		var number *int
		var status string
		if err := rows.Scan(
			&item.ID, &number, &item.Kickoff, &status, &item.HomeScore, &item.AwayScore,
			&item.HomePenalties, &item.AwayPenalties, &item.Venue, &item.City, &item.State,
			&item.RoundNumber, &item.Phase, &item.RoundName,
			&item.Home.ID, &item.Home.Name, &item.Home.ShortName, &item.Home.State,
			&item.Away.ID, &item.Away.Name, &item.Away.ShortName, &item.Away.State,
			&item.CollectedAt, &item.UpdatedAt,
		); err != nil {
			return nil, unavailable(err)
		}
		if number != nil {
			item.MatchNumber = *number
		}
		item.Status = domain.MatchStatus(status)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, unavailable(err)
	}
	return out, nil
}

func scanTeams(rows pgx.Rows) ([]domain.Team, error) {
	var out []domain.Team
	for rows.Next() {
		var item domain.Team
		if err := rows.Scan(&item.ID, &item.Name, &item.ShortName, &item.State); err != nil {
			return nil, unavailable(err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, unavailable(err)
	}
	return out, nil
}
