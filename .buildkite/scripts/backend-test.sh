#!/usr/bin/env bash
set -euo pipefail

cd backend

# Run tests with database if available
if [[ -n "${CLUBHOUSE_TEST_DATABASE_URL:-}" ]]; then
  echo "Running tests with database..."
  go test -v ./...
else
  echo "Running tests without database (DB-backed tests will be skipped)..."
  go test ./...
fi

go build ./...
