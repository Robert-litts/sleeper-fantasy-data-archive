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
- Optional restore script for a local ESPN clone database
- Weekly matchup archive
- Draft archive
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
| `players` | Fetches `/players/nfl` and upserts player records. Stores Sleeper IDs and optional ESPN ID cross-references. |
| `leagues` | Fetches leagues for each configured season and upserts league records. |
| `teams` | Fetches league users and rosters, upserts team records, and stores current roster entries at `week = 0`. |
| `backfill-basic` | Runs players, leagues, teams, and current roster archiving in sequence. |
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
- Current roster entries are stored with `week = 0` until weekly snapshots are implemented.

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