-- name: UpsertMatchup :one
INSERT INTO matchups (
    league_id,
    week,
    matchup_id,
    roster_id,
    opponent_roster_id,
    points,
    custom_points,
    is_playoff,
    matchup_type,
    starters,
    players
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
ON CONFLICT (league_id, week, matchup_id, roster_id) DO UPDATE SET
    opponent_roster_id = EXCLUDED.opponent_roster_id,
    points = EXCLUDED.points,
    custom_points = EXCLUDED.custom_points,
    is_playoff = EXCLUDED.is_playoff,
    matchup_type = EXCLUDED.matchup_type,
    starters = EXCLUDED.starters,
    players = EXCLUDED.players
RETURNING *;

-- name: ListMatchupsByLeague :many
SELECT * FROM matchups
WHERE league_id = $1
ORDER BY week ASC, matchup_id ASC, roster_id ASC;
