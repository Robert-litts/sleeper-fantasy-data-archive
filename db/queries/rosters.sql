-- name: UpsertRosterEntry :one
INSERT INTO rosters (
    league_id,
    week,
    roster_id,
    player_id,
    roster_slot,
    is_starter
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (league_id, week, roster_id, player_id) DO UPDATE SET
    roster_slot = EXCLUDED.roster_slot,
    is_starter = EXCLUDED.is_starter
RETURNING *;

-- name: ListRosterEntriesByLeague :many
SELECT * FROM rosters
WHERE league_id = $1
ORDER BY week ASC, roster_id ASC, player_id ASC;
