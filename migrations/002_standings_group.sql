ALTER TABLE standings
    ADD COLUMN group_name text NOT NULL DEFAULT '',
    ADD COLUMN group_order integer NOT NULL DEFAULT 0;

ALTER TABLE standings
    DROP CONSTRAINT standings_season_id_position_key;

ALTER TABLE standings
    ADD CONSTRAINT standings_season_group_position_key UNIQUE (season_id, group_name, position);
