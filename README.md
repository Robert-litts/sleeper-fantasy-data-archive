# Sleeper Fantasy Data Archive

Go service for archiving historical Sleeper fantasy football data into Postgres.

This project is the Sleeper companion to my [ESPN Fantasy Data Archive](https://github.com/Robert-litts/ESPN-Fantasy-Data-Archive). My league moved from ESPN to Sleeper in 2022, so this service is designed to backfill Sleeper seasons into a local database with tables that are intentionally similar to the ESPN archive.

The long-term goal is for my fantasy football web app to read from both:

- an ESPN clone database containing preserved historical ESPN data
- a Sleeper archive database containing 2022+ Sleeper data

## Major project components

- Sleeper API client using the Go standard library
- Postgres schema managed with `goose`
- Typed database queries generated with `sqlc`
- Local Postgres 17 + Adminer through Docker Compose
- Player archive
- League archive
- Team and current-roster archive
- Draft archive
- Weekly matchup archive
- Playoff bracket archive
- Optional restore script for a local ESPN clone database
- Historical lineup/roster snapshots by week
- Web app integration across ESPN and Sleeper databases

## Project Structure

```text
cmd/sleeper-archive/      CLI entrypoint for archive commands
internal/config/          Environment loading and app configuration
internal/sleeper/         Sleeper API client and response types
internal/archive/         Archive orchestration and normalization logic
internal/postgres/        Postgres connection setup
internal/db/              Generated sqlc package
db/schema/                SQL schema source for sqlc
db/queries/               sqlc query definitions
migrations/               goose migrations
scripts/                  Local utility scripts
postgres/                 Local bind-mounted Postgres data directory
```

## Requirements

- Go 1.23+
- Docker and Docker Compose
- `goose`
- `sqlc`

## Local Setup

Start Postgres and Adminer:

```sh
docker compose up -d postgres adminer
```

Postgres runs on local port `5434`.

Adminer runs at:

```text
http://localhost:8081
```

Adminer login:

```text
System: PostgreSQL
Server: postgres
Username: sleeper
Password: sleeper
Database: sleeper_archive
```

Postgres uses a bind mount at:

```text
./postgres/data
```

That directory is ignored by git.

## Environment

Copy the example file:

```sh
cp .env.example .env
```

Then set your Sleeper user ID:

```env
DATABASE_URL=postgres://sleeper:sleeper@localhost:5434/sleeper_archive?sslmode=disable
SLEEPER_USER_ID=your_sleeper_user_id
SLEEPER_MAIN_LEAGUE_ID=your_main_sleeper_league_id
SLEEPER_SPORT=nfl
START_SEASON=2022
END_SEASON=2025
SLEEPER_API_BASE_URL=https://api.sleeper.app/v1
HTTP_TIMEOUT=30s
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=25
DB_MAX_IDLE_TIME=1h15m
# Optional ESPN dump file for restoring data from a backup.
# ESPN_DUMP_FILE=/path/to/espn_fantasy_postgres_backup.dump

```

`SLEEPER_MAIN_LEAGUE_ID` is the preferred lineage seed for the archive. The service uses it to walk `previous_league_id` backward and stamp a canonical lineage id across every season in that league family.

If your Sleeper account has more than one league family, pick the current-season league id for the family you want to treat as the main archive. `go run ./cmd/sleeper-archive -mode inspect` will list every league found for `SLEEPER_USER_ID` across the configured seasons, so you can compare the league ids and choose the chain you want. Leagues that do not belong to that seed chain stay outside the canonical lineage.

If you only know your Sleeper username, you can get your user ID from:

```text
https://api.sleeper.app/v1/user/YOUR_USERNAME
```

## Database Migrations

Run migrations against the Sleeper archive database:

```sh
DATABASE_URL=postgres://sleeper:sleeper@localhost:5434/sleeper_archive?sslmode=disable make migrate/up
```

Rollback one migration:

```sh
DATABASE_URL=postgres://sleeper:sleeper@localhost:5434/sleeper_archive?sslmode=disable make migrate/down
```

## Archive Commands

Run commands with:

```sh
go run ./cmd/sleeper-archive -mode <mode>
```

Available modes:

| Mode | What it does |
| --- | --- |
| `inspect` | Lists Sleeper leagues found for `SLEEPER_USER_ID` from `START_SEASON` through `END_SEASON`. Does not write archive data. |
| `report` | Prints archive health summaries by league, including teams, draft picks, matchup weeks, roster rows, playoff bracket rows, champion, runner-up, and warnings. |
| `players` | Fetches `/players/nfl` and upserts player records. Stores Sleeper IDs and optional ESPN ID cross-references. |
| `leagues` | Fetches leagues for each configured season and upserts league records. |
| `teams` | Fetches league users and rosters, upserts team records, and stores current roster entries at `week = 0`. |
| `drafts` | Fetches league drafts and draft picks, resolves picks to archived teams and players, and stores draft history. |
| `matchups` | Fetches weekly matchup entries until Sleeper returns the first empty week, stores scores, starters, players, and paired opponent roster IDs. |
| `weekly-rosters` | Fetches weekly matchup entries and normalizes each roster's players/starters into `rosters` rows for each week. |
| `brackets` | Fetches winners and losers playoff brackets, stores bracket progression metadata, and updates `teams.final_standing` from completed winners-bracket placement games. |
| `backfill-basic` | Runs players, leagues, teams, current rosters, drafts, matchups, weekly rosters, and playoff bracket archiving in sequence. |
| `state` | Fetches Sleeper NFL state metadata. Useful for API/debug checks. |

Example:

```sh
go run ./cmd/sleeper-archive -mode inspect
go run ./cmd/sleeper-archive -mode backfill-basic
```

## Generated Database Code

This project uses `sqlc`.

After changing files in `db/schema/` or `db/queries/`, regenerate:

```sh
make sqlc/generate
```

Generated code lives in:

```text
internal/db/
```

## Local ESPN Clone

This repo can also restore a local clone of the preserved ESPN database into the same Postgres instance. This is for development and comparison only; it does not mutate the original ESPN data source.

Set `ESPN_DUMP_FILE` in `.env`:

```env
ESPN_DUMP_FILE=/path/to/espn_fantasy_postgres_backup.dump
```

Then restore the dump:

```sh
./scripts/restore-espn-dev-db.sh
```

The script restores into:

```text
fantasy_espn_clone
```

You can also pass the dump path directly as the first argument.

Useful local database URLs:

```env
SLEEPER_DATABASE_URL=postgres://sleeper:sleeper@localhost:5434/sleeper_archive?sslmode=disable
ESPN_DATABASE_URL=postgres://sleeper:sleeper@localhost:5434/fantasy_espn_clone?sslmode=disable
```

## Schema Notes

- Sleeper IDs are stored as text.
- `players.sleeper_id` is the authoritative Sleeper player ID.
- `players.espn_id` is an optional indexed cross-reference.
- The table names mirror the ESPN archive where practical: `leagues`, `teams`, `players`, `drafts`, `matchups`, and `rosters`.
- `drafts` stores individual draft picks and links each pick to the archived `teams` and `players` records.
- `rosters.week = 0` stores the current roster snapshot from Sleeper's roster endpoint.
- `rosters.week >= 1` stores historical weekly lineup snapshots derived from matchup `players` and `starters`.
- `teams.standing` is regular-season standing.
- `teams.final_standing` starts as regular-season standing, then `brackets` updates it when Sleeper exposes completed winners-bracket placement games.
- `playoff_bracket_matchups` stores Sleeper winners and losers bracket metadata separately from weekly matchup scores. Losers-bracket placement values are archived but not used as overall league final standings.
- In `playoff_bracket_matchups`, `slot1_*` and `slot2_*` represent the two bracket positions in a playoff matchup. A slot can point directly to a roster with `slot1_roster_id`, or it can point to the winner/loser of an earlier bracket matchup with `slot1_source_matchup_id` and `slot1_source_result`.

### Playoff Bracket Example

Sleeper does not expose a single final standings endpoint. Instead, it exposes weekly scores through `matchups` and playoff bracket structure through `winners_bracket` and `losers_bracket`.

The `playoff_bracket_matchups` table preserves that bracket structure. Each row is one node in the bracket tree.

For a simple four-team winners bracket, the rows might look like this:

| bracket_type | round_num | matchup_id | placement | slot1_roster_id | slot2_roster_id | slot1_source_matchup_id | slot1_source_result | slot2_source_matchup_id | slot2_source_result | winner_roster_id | loser_roster_id |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- | ---: | --- | ---: | ---: |
| `WINNERS_BRACKET` | 1 | 1 |  | 3 | 6 |  |  |  |  | 3 | 6 |
| `WINNERS_BRACKET` | 1 | 2 |  | 4 | 5 |  |  |  |  | 5 | 4 |
| `WINNERS_BRACKET` | 2 | 3 | 1 | 3 | 5 | 1 | `WINNER` | 2 | `WINNER` | 5 | 3 |
| `WINNERS_BRACKET` | 2 | 4 | 3 | 6 | 4 | 1 | `LOSER` | 2 | `LOSER` | 4 | 6 |

Read the rows like this:

- `matchup_id = 1` is a first-round game between roster `3` and roster `6`; roster `3` won.
- `matchup_id = 2` is a first-round game between roster `4` and roster `5`; roster `5` won.
- `matchup_id = 3` is the championship game because `placement = 1`.
- `matchup_id = 3` has `slot1_source_matchup_id = 1` and `slot1_source_result = WINNER`, so slot 1 came from the winner of matchup `1`, which was roster `3`.
- `matchup_id = 3` has `slot2_source_matchup_id = 2` and `slot2_source_result = WINNER`, so slot 2 came from the winner of matchup `2`, which was roster `5`.
- Since roster `5` won `matchup_id = 3`, roster `5` finished `1st` and roster `3` finished `2nd`.
- `matchup_id = 4` has `placement = 3`, so it is the third-place game. Roster `4` won and finished `3rd`; roster `6` finished `4th`.

The archive uses completed `WINNERS_BRACKET` placement rows to update `teams.final_standing`. The original bracket rows remain available so the web app can later render playoff paths, byes, championship games, and third-place games without guessing from scores alone.

## Development

Run focused tests:

```sh
go test ./cmd/... ./internal/...
```

The narrower package list avoids walking into the bind-mounted `postgres/data` directory.

Format code:

```sh
make fmt
```

Build the archive binary:

```sh
make build
```
