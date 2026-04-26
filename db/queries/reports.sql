-- name: ListLeagueReports :many
SELECT
    l.id AS league_id,
    l.season,
    l.name,
    l.total_rosters,
    (
        SELECT count(*)
        FROM teams t
        WHERE t.league_id = l.id
    ) AS team_count,
    (
        SELECT count(*)
        FROM drafts d
        WHERE d.league_id = l.id
    ) AS draft_pick_count,
    coalesce((
        SELECT min(m.week)
        FROM matchups m
        WHERE m.league_id = l.id
    ), 0)::integer AS matchup_min_week,
    coalesce((
        SELECT max(m.week)
        FROM matchups m
        WHERE m.league_id = l.id
    ), 0)::integer AS matchup_max_week,
    (
        SELECT count(*)
        FROM matchups m
        WHERE m.league_id = l.id
    ) AS matchup_entry_count,
    (
        SELECT count(*)
        FROM rosters r
        WHERE r.league_id = l.id
          AND r.week = 0
    ) AS current_roster_entry_count,
    (
        SELECT count(*)
        FROM rosters r
        WHERE r.league_id = l.id
          AND r.week > 0
    ) AS weekly_roster_entry_count,
    (
        SELECT count(*)
        FROM playoff_bracket_matchups pbm
        WHERE pbm.league_id = l.id
    ) AS playoff_bracket_matchup_count,
    coalesce((
        SELECT t.team_name
        FROM teams t
        WHERE t.league_id = l.id
          AND t.final_standing = 1
        ORDER BY t.roster_id ASC
        LIMIT 1
    ), '')::text AS champion_team_name,
    coalesce((
        SELECT t.team_name
        FROM teams t
        WHERE t.league_id = l.id
          AND t.final_standing = 2
        ORDER BY t.roster_id ASC
        LIMIT 1
    ), '')::text AS runner_up_team_name,
    (
        SELECT count(*)
        FROM teams t
        WHERE t.league_id = l.id
          AND t.final_standing = 0
    ) AS missing_final_standing_count,
    (
        SELECT count(*)
        FROM (
            SELECT t.final_standing
            FROM teams t
            WHERE t.league_id = l.id
              AND t.final_standing > 0
            GROUP BY t.final_standing
            HAVING count(*) > 1
        ) duplicate_standings
    ) AS duplicate_final_standing_count
FROM leagues l
ORDER BY l.season ASC, l.name ASC;
