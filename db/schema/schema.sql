CREATE TABLE leagues (
    id bigserial PRIMARY KEY,
    sleeper_league_id text NOT NULL UNIQUE,
    previous_league_id text,
    canonical_league_id text,
    season integer NOT NULL,
    name text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT '',
    sport text NOT NULL DEFAULT 'nfl',
    total_rosters integer NOT NULL DEFAULT 0,
    draft_id text,
    avatar text,
    roster_positions jsonb NOT NULL DEFAULT '[]'::jsonb,
    scoring_settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    league_settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_leagues_previous_league_id
ON leagues(previous_league_id)
WHERE previous_league_id IS NOT NULL;

CREATE INDEX idx_leagues_canonical_league_id
ON leagues(canonical_league_id)
WHERE canonical_league_id IS NOT NULL;

CREATE TABLE teams (
    id bigserial PRIMARY KEY,
    league_id bigint NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    roster_id integer NOT NULL,
    owner_id text NOT NULL,
    user_id text,
    display_name text,
    username text,
    team_name text NOT NULL DEFAULT '',
    avatar text,
    wins integer NOT NULL DEFAULT 0,
    losses integer NOT NULL DEFAULT 0,
    ties integer NOT NULL DEFAULT 0,
    points_for numeric(12,2) NOT NULL DEFAULT 0,
    points_against numeric(12,2) NOT NULL DEFAULT 0,
    waiver_position integer NOT NULL DEFAULT 0,
    waiver_budget_used integer NOT NULL DEFAULT 0,
    total_moves integer NOT NULL DEFAULT 0,
    streak_type text NOT NULL DEFAULT '',
    streak_length integer NOT NULL DEFAULT 0,
    standing integer NOT NULL DEFAULT 0,
    final_standing integer NOT NULL DEFAULT 0,
    unique(league_id, roster_id)
);

CREATE TABLE players (
    id bigserial PRIMARY KEY,
    sleeper_id text NOT NULL UNIQUE,
    espn_id text,
    name text NOT NULL,
    first_name text,
    last_name text,
    position text,
    team text,
    status text,
    fantasy_positions jsonb NOT NULL DEFAULT '[]'::jsonb,
    age integer
);

CREATE INDEX idx_players_espn_id ON players(espn_id)
WHERE espn_id IS NOT NULL;

CREATE TABLE drafts (
    id bigserial PRIMARY KEY,
    league_id bigint NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    draft_id text NOT NULL,
    team_id bigint NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    player_id bigint NOT NULL REFERENCES players(id),
    overall_pick integer NOT NULL,
    round_num integer NOT NULL,
    round_pick integer NOT NULL,
    draft_slot integer,
    picked_by text,
    keeper_status boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    unique(draft_id, overall_pick)
);

CREATE TABLE matchups (
    id bigserial PRIMARY KEY,
    league_id bigint NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    week integer NOT NULL,
    matchup_id integer NOT NULL,
    roster_id integer NOT NULL,
    opponent_roster_id integer,
    points numeric(12,2) NOT NULL DEFAULT 0,
    custom_points numeric(12,2),
    is_playoff boolean NOT NULL DEFAULT false,
    matchup_type text NOT NULL DEFAULT 'regular',
    starters jsonb NOT NULL DEFAULT '[]'::jsonb,
    players jsonb NOT NULL DEFAULT '[]'::jsonb,
    unique(league_id, week, matchup_id, roster_id)
);

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

CREATE TABLE rosters (
    id bigserial PRIMARY KEY,
    league_id bigint NOT NULL REFERENCES leagues(id) ON DELETE CASCADE,
    week integer NOT NULL DEFAULT 0,
    roster_id integer NOT NULL,
    player_id bigint NOT NULL REFERENCES players(id),
    roster_slot text NOT NULL DEFAULT '',
    is_starter boolean NOT NULL DEFAULT false,
    unique(league_id, week, roster_id, player_id)
);
