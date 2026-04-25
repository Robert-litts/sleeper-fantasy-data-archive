#!/usr/bin/env bash
set -euo pipefail

if [[ -f ".env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source ".env"
  set +a
fi

DUMP_FILE="${1:-${ESPN_DUMP_FILE:-}}"
DB_NAME="${DB_NAME:-fantasy_espn_clone}"
DB_USER="${DB_USER:-sleeper}"
CONTAINER_NAME="${CONTAINER_NAME:-sleeper-postgres}"

if [[ -z "$DUMP_FILE" ]]; then
  echo "Set ESPN_DUMP_FILE in .env or pass the dump path as the first argument." >&2
  exit 1
fi

if [[ ! -f "$DUMP_FILE" ]]; then
  echo "Dump file not found: $DUMP_FILE" >&2
  exit 1
fi

if [[ ! -s "$DUMP_FILE" ]]; then
  echo "Dump file is empty: $DUMP_FILE" >&2
  exit 1
fi

case "$(file -b "$DUMP_FILE")" in
  *"PostgreSQL custom database dump"*) ;;
  *)
    echo "Expected a PostgreSQL custom dump, got: $(file -b "$DUMP_FILE")" >&2
    exit 1
    ;;
esac

echo "Restoring $DUMP_FILE into $DB_NAME on $CONTAINER_NAME"

docker exec "$CONTAINER_NAME" dropdb --if-exists -U "$DB_USER" "$DB_NAME"
docker exec "$CONTAINER_NAME" createdb -U "$DB_USER" "$DB_NAME"
docker exec -i "$CONTAINER_NAME" pg_restore \
  --dbname "$DB_NAME" \
  --username "$DB_USER" \
  --no-owner \
  --no-privileges \
  < "$DUMP_FILE"

echo "Restore complete."
echo "Restored tables:"
docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -Atc \
  "select schemaname || '.' || tablename from pg_tables where schemaname not in ('pg_catalog', 'information_schema') order by schemaname, tablename;"
