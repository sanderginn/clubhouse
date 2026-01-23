#!/bin/sh
set -e

DB_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

echo "Running database migrations..."
migrate -path /app/migrations -database "$DB_URL" up

echo "Starting server..."
exec ./clubhouse-server
