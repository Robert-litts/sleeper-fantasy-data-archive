-- +goose Up
-- +goose StatementBegin
CREATE TABLE playoff_bracket_matchups (
    id bigserial PRIMARY KEY,
    league_id bigint NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    bracket_type text NOT NULL,
    round_num integer NOT NULL,
    matchup_id integer NOT NULL,
    placement integer,
    slot1_roster_id integer,
    slot2_roster_id integer,
    slot1_source_matchup_id integer,
    slot1_source_result text,
    slot2_source_matchup_id integer,
    slot2_source_result text,
    winner_roster_id integer,
    loser_roster_id integer,
    raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    unique(league_id, bracket_type, matchup_id)
);

CREATE INDEX idx_playoff_bracket_matchups_league_type
ON playoff_bracket_matchups(league_id, bracket_type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS playoff_bracket_matchups;
-- +goose StatementEnd
