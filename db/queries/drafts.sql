-- name: UpsertDraftPick :one
INSERT INTO drafts (
    league_id,
    draft_id,
    team_id,
    player_id,
    overall_pick,
    round_num,
    round_pick,
    draft_slot,
    picked_by,
    keeper_status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (draft_id, overall_pick) DO UPDATE SET
    team_id = EXCLUDED.team_id,
    player_id = EXCLUDED.player_id,
    round_num = EXCLUDED.round_num,
    round_pick = EXCLUDED.round_pick,
    draft_slot = EXCLUDED.draft_slot,
    picked_by = EXCLUDED.picked_by,
    keeper_status = EXCLUDED.keeper_status
RETURNING *;

-- name: ListDraftPicksByLeague :many
SELECT * FROM drafts
WHERE league_id = $1
ORDER BY overall_pick ASC;
