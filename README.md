# Clubhouse

A self-hosted, lightweight social platform for sharing links within small-to-medium communities (5-500 people).

## Quick Start

### Prerequisites
- Docker & Docker Compose 2.0+
- Go 1.24+ (for local backend development)
- Node.js 20+ (for frontend development)

### Running Locally

1. **Clone the repository:**
   ```bash
   git clone https://github.com/sanderginn/clubhouse.git
   cd clubhouse
   ```

2. **Setup environment:**
   ```bash
   cp .env.example .env
   ```

3. **Start all services:**
   ```bash
   docker compose up -d
   ```

4. **Verify services are healthy:**
   ```bash
   docker compose ps
   ```

   Look for `healthy` in the STATUS column for Postgres/Redis (and backend once it's started).

5. **Bootstrap the first admin (first run only):**
   - Set `CLUBHOUSE_BOOTSTRAP_ADMIN_USERNAME`, `CLUBHOUSE_BOOTSTRAP_ADMIN_PASSWORD`, and optional `CLUBHOUSE_BOOTSTRAP_ADMIN_EMAIL` in `.env`.
   - Restart the stack (`docker compose up -d`) to create the admin, then remove the bootstrap values and restart.

6. **Access services:**
   - Grafana (Observability): http://localhost:3000 (admin/admin)
   - Prometheus: http://localhost:9090
   - Loki: http://localhost:3100
   - Tempo: http://localhost:3200
   - Backend API: http://localhost:8080 (health: `/health`)
   - Frontend: http://localhost:5173
   - PostgreSQL: localhost:5432
   - Redis: localhost:6379

### Viewing Logs

```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f postgres
docker compose logs -f redis
docker compose logs -f grafana
```

### Stopping Services

```bash
docker compose down
```

To also remove volumes:
```bash
docker compose down -v
```

## Production Deployment

See `docs/production-deployment.md` for the production checklist, TLS setup, backups, and rollback steps.

## Development

### Backend

```bash
cd backend

# Download dependencies
go mod download

# Build
go build -o clubhouse-server ./cmd/server

# Run migrations (if database is running)
migrate -path migrations -database "$DATABASE_URL" up

# Run (ensure .env is configured)
./clubhouse-server
```

### Frontend

```bash
cd frontend

# Install dependencies
npm install

# Run dev server
npm run dev
```

Optional frontend environment variables:
- `VITE_SENTRY_DSN`: Sentry DSN for client error tracking (leave unset to disable).
- `VITE_APP_VERSION`: Release identifier for error tracking.
- `VITE_OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP/HTTP traces endpoint for browser telemetry (in dev defaults to `http://localhost:4318/v1/traces`; leave unset in prod to disable).
- `VITE_OTEL_SERVICE_NAME`: Service name used in frontend traces (defaults to `clubhouse-frontend`).
- `VITE_PROXY_LOG_LEVEL`: Vite proxy logging level (`none`/`silent`, `error`, `warn`, `info`, `debug`; defaults to `warn`).

## Architecture

See [DESIGN.md](DESIGN.md) for complete system design.

## Authentication

Auth is username-based. Registration requires a username and password, with email optional. Login uses `username` + `password` and sets an httpOnly session cookie.
Admins can enroll TOTP MFA; set `CLUBHOUSE_TOTP_ENCRYPTION_KEY` (base64-encoded 32-byte key) before enabling.

## Development Guidelines

See [AGENTS.md](AGENTS.md) for AI agent development guidelines.

## CI (Buildkite)

Pull requests targeting `main` run in Buildkite and must pass all checks before merge. The pipeline lives in `.buildkite/pipeline.yml` and runs:
- Backend format/lint (gofmt + golangci-lint)
- Backend tests + build
- Frontend lint + typecheck + tests

The pipeline starts with a hosted selector step that checks for connected self-hosted agents and uploads the real pipeline to either the self-hosted or hosted queue.

Required Buildkite environment variables (set as secrets):
- `DYNAMIC_PIPELINE_GRAPHQL_TOKEN` (Buildkite GraphQL API access token)
- `BUILDKITE_CLUSTER_ID`
- `BUILDKITE_SELF_HOSTED_QUEUE_KEY` (default: `local-agents`)
- `BUILDKITE_HOSTED_QUEUE_KEY` (default: `default`)

Optional:
- `BUILDKITE_SELF_HOSTED_QUEUE_ID` (skip queue lookup if you already know the queue ID)

Note: the selector step uses the built-in `BUILDKITE_ORGANIZATION_SLUG` and requires `jq` on the hosted agent image.

## Pre-commit Checks

Before committing and pushing, run:
```bash
# Run checks only for files changed in the current changeset
prek run

# Or run everything
prek run --all-files
```

## License

MIT
