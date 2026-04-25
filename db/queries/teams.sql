-- name: UpsertTeam :one
INSERT INTO teams (
    league_id,
    roster_id,
    owner_id,
    user_id,
    display_name,
    username,
    team_name,
    avatar,
    wins,
    losses,
    ties,
    points_for,
    points_against,
    waiver_position,
    waiver_budget_used,
    total_moves,
    streak_type,
    streak_length,
    standing,
    final_standing
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
)
ON CONFLICT (league_id, roster_id) DO UPDATE SET
    owner_id = EXCLUDED.owner_id,
    user_id = EXCLUDED.user_id,
    display_name = EXCLUDED.display_name,
    username = EXCLUDED.username,
    team_name = EXCLUDED.team_name,
    avatar = EXCLUDED.avatar,
    wins = EXCLUDED.wins,
    losses = EXCLUDED.losses,
    ties = EXCLUDED.ties,
    points_for = EXCLUDED.points_for,
    points_against = EXCLUDED.points_against,
    waiver_position = EXCLUDED.waiver_position,
    waiver_budget_used = EXCLUDED.waiver_budget_used,
    total_moves = EXCLUDED.total_moves,
    streak_type = EXCLUDED.streak_type,
    streak_length = EXCLUDED.streak_length,
    standing = EXCLUDED.standing,
    final_standing = EXCLUDED.final_standing
RETURNING *;

-- name: ListTeamsByLeague :many
SELECT * FROM teams
WHERE league_id = $1
ORDER BY final_standing ASC, roster_id ASC;
