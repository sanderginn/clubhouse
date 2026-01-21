# Clubhouse

A self-hosted, lightweight social platform for sharing links within small-to-medium communities (5-500 people).

## Quick Start

### Prerequisites
- Docker & Docker Compose 2.0+
- Go 1.21+ (for local backend development)
- Node.js 18+ (for frontend development)

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
   docker-compose up -d
   ```

4. **Verify services are healthy:**
   ```bash
   docker-compose ps
   ```

   Expected output:
   ```
   STATUS: healthy
   ```

5. **Access services:**
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
docker-compose logs -f

# Specific service
docker-compose logs -f postgres
docker-compose logs -f redis
docker-compose logs -f grafana
```

### Stopping Services

```bash
docker-compose down
```

To also remove volumes:
```bash
docker-compose down -v
```

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

## Architecture

See [DESIGN.md](DESIGN.md) for complete system design.

## Development Guidelines

See [AGENTS.md](AGENTS.md) for AI agent development guidelines.

## CI (Buildkite)

Pull requests targeting `main` run in Buildkite and must pass all checks before merge. The pipeline lives in `.buildkite/pipeline.yml` and runs:
- Backend format/lint (gofmt + golangci-lint)
- Backend tests + build
- Frontend lint + typecheck + tests

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
