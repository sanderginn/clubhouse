#!/usr/bin/env bash
set -euo pipefail

cd backend

# Run tests with database if available
if [[ -n "${CLUBHOUSE_TEST_DATABASE_URL:-}" ]]; then
  echo "Running tests with database..."

  # Wait for database to be ready (in case healthcheck isn't enough)
  for i in {1..30}; do
    if pg_isready -h "${PGHOST:-postgres-test}" -U clubhouse_test 2>/dev/null; then
      break
    fi
    echo "Waiting for database... ($i/30)"
    sleep 1
  done

  # Run migrations
  echo "Running migrations..."
  cat migrations/*.up.sql | psql "${CLUBHOUSE_TEST_DATABASE_URL}" 2>&1 || {
    echo "Note: Some migrations may have already been applied or psql not available"
  }

  # Run tests sequentially (-p 1) to avoid deadlocks from parallel table truncation
  go test -v -p 1 ./...
else
  echo "Running tests without database (DB-backed tests will be skipped)..."
  go test ./...
fi

go build ./...
