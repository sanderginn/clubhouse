#!/bin/sh
set -eu

if [ -f ./.env.production ]; then
  set -a
  . ./.env.production
  set +a
fi

BACKUP_DIR=${BACKUP_DIR:-./backups}
RETENTION_DAYS=${BACKUP_RETENTION_DAYS:-7}
TIMESTAMP=$(date -u "+%Y%m%dT%H%M%SZ")

mkdir -p "$BACKUP_DIR"
BACKUP_FILE="$BACKUP_DIR/clubhouse_${TIMESTAMP}.sql.gz"

# Run pg_dump inside the postgres container and compress locally.
docker compose -f docker-compose.prod.yml exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > "$BACKUP_FILE"

# Prune old backups.
find "$BACKUP_DIR" -type f -name "clubhouse_*.sql.gz" -mtime "+$RETENTION_DAYS" -print -delete

echo "Backup written to $BACKUP_FILE"
