-- name: UpsertLeague :one
INSERT INTO leagues (
    sleeper_league_id,
    season,
    name,
    status,
    sport,
    total_rosters,
    draft_id,
    avatar,
    roster_positions,
    scoring_settings,
    league_settings,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now()
)
ON CONFLICT (sleeper_league_id) DO UPDATE SET
    season = EXCLUDED.season,
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    sport = EXCLUDED.sport,
    total_rosters = EXCLUDED.total_rosters,
    draft_id = EXCLUDED.draft_id,
    avatar = EXCLUDED.avatar,
    roster_positions = EXCLUDED.roster_positions,
    scoring_settings = EXCLUDED.scoring_settings,
    league_settings = EXCLUDED.league_settings,
    updated_at = now()
RETURNING *;

-- name: GetLeagueBySleeperID :one
SELECT * FROM leagues
WHERE sleeper_league_id = $1;

-- name: ListLeaguesBySeason :many
SELECT * FROM leagues
WHERE season = $1
ORDER BY name ASC;
