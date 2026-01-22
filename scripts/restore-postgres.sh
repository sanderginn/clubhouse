#!/bin/sh
set -eu

if [ -f ./.env.production ]; then
  set -a
  . ./.env.production
  set +a
fi

if [ $# -lt 1 ]; then
  echo "Usage: $0 /path/to/backup.sql.gz" >&2
  exit 1
fi

BACKUP_FILE=$1

if [ ! -f "$BACKUP_FILE" ]; then
  echo "Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

TMP_SQL="$(mktemp -t clubhouse_restore.XXXXXX)"

gunzip -t "$BACKUP_FILE"
gunzip -c "$BACKUP_FILE" > "$TMP_SQL"

# Restore by streaming the SQL file into psql in the container.
docker compose -f docker-compose.prod.yml exec -T postgres \
  sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$TMP_SQL"

rm -f "$TMP_SQL"

echo "Restore completed from $BACKUP_FILE"
