-- name: UpsertPlayoffBracketMatchup :one
INSERT INTO playoff_bracket_matchups (
    league_id,
    bracket_type,
    round_num,
    matchup_id,
    placement,
    slot1_roster_id,
    slot2_roster_id,
    slot1_source_matchup_id,
    slot1_source_result,
    slot2_source_matchup_id,
    slot2_source_result,
    winner_roster_id,
    loser_roster_id,
    raw_payload
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
ON CONFLICT (league_id, bracket_type, matchup_id) DO UPDATE SET
    round_num = EXCLUDED.round_num,
    placement = EXCLUDED.placement,
    slot1_roster_id = EXCLUDED.slot1_roster_id,
    slot2_roster_id = EXCLUDED.slot2_roster_id,
    slot1_source_matchup_id = EXCLUDED.slot1_source_matchup_id,
    slot1_source_result = EXCLUDED.slot1_source_result,
    slot2_source_matchup_id = EXCLUDED.slot2_source_matchup_id,
    slot2_source_result = EXCLUDED.slot2_source_result,
    winner_roster_id = EXCLUDED.winner_roster_id,
    loser_roster_id = EXCLUDED.loser_roster_id,
    raw_payload = EXCLUDED.raw_payload
RETURNING *;

-- name: ListPlayoffBracketMatchupsByLeague :many
SELECT * FROM playoff_bracket_matchups
WHERE league_id = $1
ORDER BY bracket_type ASC, round_num ASC, matchup_id ASC;
