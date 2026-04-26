-- +goose Up
-- +goose StatementBegin
ALTER TABLE leagues
    ADD COLUMN previous_league_id text,
    ADD COLUMN canonical_league_id text;

CREATE INDEX idx_leagues_previous_league_id
ON leagues(previous_league_id)
WHERE previous_league_id IS NOT NULL;

CREATE INDEX idx_leagues_canonical_league_id
ON leagues(canonical_league_id)
WHERE canonical_league_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_leagues_canonical_league_id;
DROP INDEX IF EXISTS idx_leagues_previous_league_id;

ALTER TABLE leagues
    DROP COLUMN IF EXISTS canonical_league_id,
    DROP COLUMN IF EXISTS previous_league_id;
-- +goose StatementEnd