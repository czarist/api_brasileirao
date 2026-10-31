CREATE TABLE competitions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code text NOT NULL UNIQUE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE seasons (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    competition_id bigint NOT NULL REFERENCES competitions (id),
    year integer NOT NULL,
    name text NOT NULL,
    external_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (competition_id, year)
);

CREATE TABLE teams (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    short_name text NOT NULL,
    state text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE team_aliases (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    team_id bigint NOT NULL REFERENCES teams (id),
    alias text NOT NULL,
    UNIQUE (team_id, alias)
);

CREATE INDEX team_aliases_team_idx ON team_aliases (team_id);

CREATE TABLE season_teams (
    season_id bigint NOT NULL REFERENCES seasons (id),
    team_id bigint NOT NULL REFERENCES teams (id),
    PRIMARY KEY (season_id, team_id)
);

CREATE INDEX season_teams_team_idx ON season_teams (team_id);

CREATE TABLE rounds (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    season_id bigint NOT NULL REFERENCES seasons (id),
    phase text NOT NULL,
    number integer NOT NULL,
    name text NOT NULL,
    UNIQUE (season_id, phase, number)
);

CREATE TABLE matches (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    season_id bigint NOT NULL REFERENCES seasons (id),
    round_id bigint NOT NULL REFERENCES rounds (id),
    home_team_id bigint NOT NULL REFERENCES teams (id),
    away_team_id bigint NOT NULL REFERENCES teams (id),
    source text NOT NULL,
    external_id text NOT NULL,
    match_number integer,
    kickoff_at timestamptz,
    status text NOT NULL,
    home_score integer,
    away_score integer,
    home_penalty_score integer,
    away_penalty_score integer,
    venue text,
    city text,
    state text,
    collected_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT matches_status_chk CHECK (
        status IN ('scheduled', 'in_progress', 'finished', 'postponed', 'suspended', 'cancelled')
    ),
    CONSTRAINT matches_score_pair_chk CHECK ((home_score IS NULL) = (away_score IS NULL)),
    CONSTRAINT matches_score_nonneg_chk CHECK (
        (home_score IS NULL OR home_score >= 0) AND (away_score IS NULL OR away_score >= 0)
    ),
    CONSTRAINT matches_penalty_pair_chk CHECK ((home_penalty_score IS NULL) = (away_penalty_score IS NULL)),
    UNIQUE (source, external_id)
);

CREATE INDEX matches_season_kickoff_idx ON matches (season_id, kickoff_at, id);
CREATE INDEX matches_season_status_idx ON matches (season_id, status);
CREATE INDEX matches_round_idx ON matches (round_id);
CREATE INDEX matches_home_idx ON matches (home_team_id);
CREATE INDEX matches_away_idx ON matches (away_team_id);

CREATE TABLE standings (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    season_id bigint NOT NULL REFERENCES seasons (id),
    team_id bigint NOT NULL REFERENCES teams (id),
    position integer NOT NULL,
    played integer NOT NULL,
    wins integer NOT NULL,
    draws integer NOT NULL,
    losses integer NOT NULL,
    goals_for integer NOT NULL,
    goals_against integer NOT NULL,
    goal_difference integer NOT NULL,
    points integer NOT NULL,
    yellow_cards integer NOT NULL,
    red_cards integer NOT NULL,
    win_percentage integer NOT NULL,
    source text NOT NULL,
    collected_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (season_id, team_id),
    UNIQUE (season_id, position)
);

CREATE INDEX standings_team_idx ON standings (team_id);

CREATE TABLE source_refs (
    source text NOT NULL,
    entity_type text NOT NULL,
    external_id text NOT NULL,
    entity_id bigint NOT NULL,
    PRIMARY KEY (source, entity_type, external_id)
);

CREATE INDEX source_refs_entity_idx ON source_refs (entity_type, entity_id);

CREATE TABLE sync_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    competition_code text NOT NULL,
    season_year integer NOT NULL,
    source text NOT NULL,
    status text NOT NULL,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    records_processed integer NOT NULL DEFAULT 0,
    error_message text,
    CONSTRAINT sync_runs_status_chk CHECK (status IN ('running', 'success', 'failed'))
);

CREATE INDEX sync_runs_lookup_idx ON sync_runs (competition_code, season_year, started_at DESC);

CREATE TABLE season_sync_state (
    season_id bigint PRIMARY KEY REFERENCES seasons (id),
    source text NOT NULL,
    last_attempt_at timestamptz,
    last_success_at timestamptz,
    last_error text
);
