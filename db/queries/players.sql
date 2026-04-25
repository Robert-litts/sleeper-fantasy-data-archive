-- name: UpsertPlayer :one
INSERT INTO players (
    sleeper_id,
    espn_id,
    name,
    first_name,
    last_name,
    position,
    team,
    status,
    fantasy_positions,
    age
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (sleeper_id) DO UPDATE SET
    espn_id = EXCLUDED.espn_id,
    name = EXCLUDED.name,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    position = EXCLUDED.position,
    team = EXCLUDED.team,
    status = EXCLUDED.status,
    fantasy_positions = EXCLUDED.fantasy_positions,
    age = EXCLUDED.age
RETURNING *;

-- name: GetPlayerBySleeperID :one
SELECT * FROM players
WHERE sleeper_id = $1;

-- name: GetPlayerByESPNID :one
SELECT * FROM players
WHERE espn_id = $1;
