#!/bin/sh
set -e

DB_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

echo "Running database migrations..."
migrate -path /app/migrations -database "$DB_URL" up

echo "Seeding default admin user..."
PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "${POSTGRES_HOST}" -p "${POSTGRES_PORT}" -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -f /app/migrations/seed_admin.sql

echo "Starting server..."
exec ./clubhouse-server
