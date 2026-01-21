# AGENTS.md - Clubhouse Development Guide

This document guides AI agents on how to work with the Clubhouse codebase effectively.

## Project Overview

**Clubhouse** is a self-hosted, lightweight social platform for sharing music links, photos, events, recipes, books, and movies within small-to-medium communities (5-500 people).

### Key Principles
- **Lightweight & self-hosted** — Minimal resource footprint, runs via Docker Compose
- **Private community** — All content visible to members only
- **Simple architecture** — Monolith backend, avoid unnecessary complexity
- **Observable** — OpenTelemetry integration from day one

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.21+ |
| Frontend | Svelte (PWA) |
| Database | PostgreSQL 14+ |
| Cache/Pub-Sub | Redis 7+ |
| Deployment | Docker Compose |
| Observability | OpenTelemetry + Grafana Stack (Loki, Prometheus, Tempo) |
| Authentication | JWT sessions (Redis-backed, httpOnly cookies) |

## Architecture

```
Svelte PWA (Web Push API for notifications)
    ↓
Go HTTP Server (monolith)
├── REST API (/api/v1/*)
├── WebSocket handler (/api/v1/ws)
└── Link metadata fetcher (synchronous)
    ↓
├── PostgreSQL (persistence)
├── Redis (session storage, pub/sub)
└── OpenTelemetry exporters (OTLP)
    ↓
Grafana Stack (local observability)
├── Loki (logs)
├── Prometheus (metrics)
└── Tempo (traces)
```

## Directory Structure

```
clubhouse/
├── backend/
│   ├── cmd/server/           # Main application entry point
│   ├── internal/
│   │   ├── models/           # Data models and request/response types
│   │   ├── handlers/         # HTTP handlers (one per domain)
│   │   ├── services/         # Business logic (one per domain)
│   │   ├── middleware/       # Auth, observability, etc.
│   │   └── db/               # Database initialization
│   ├── migrations/           # SQL migration files (11 tables)
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── routes/           # SvelteKit routes
│   │   ├── lib/
│   │   │   ├── components/   # Reusable Svelte components
│   │   │   ├── stores/       # Svelte stores (state)
│   │   │   └── api/          # API client
│   │   └── app.css           # Global styles (Tailwind)
│   └── package.json
├── scripts/                  # Orchestration scripts
│   ├── start-agent.sh        # Claim issue and create worktree
│   ├── complete-issue.sh     # Mark issue done after PR merge
│   └── show-queue.sh         # Display work queue status
├── .worktrees/               # Git worktrees for parallel agents
├── .work-queue.json          # Issue tracking with dependencies
├── docker-compose.yml        # Local dev environment
├── AGENTS.md                 # This file (code standards)
├── DESIGN.md                 # System architecture
├── ORCHESTRATOR_INSTRUCTIONS.md  # For the orchestrator agent
└── SUBAGENT_INSTRUCTIONS.md  # For subagents
```

## Core Design Decisions

### 1. API Design
- **REST API** (not GraphQL) — simplicity over flexibility
- **Versioned endpoints** (`/api/v1/*`)
- **Standard error format**: `{error: "message", code: "ERROR_CODE"}`
- **Cursor-based pagination** for feeds/comments

### 2. Database
- **9 core tables**: users, sections, posts, comments, reactions, links, mentions, audit_logs, notifications
- **Soft deletes** with 7-day retention before hard purge
- **Audit logging** for admin actions
- **Full-text search** on posts/comments/link metadata

### 3. Real-Time Communication
- **WebSocket via Redis pub/sub** — allows multi-server scaling
- **Events**: new posts, comments, mentions, reactions
- **Push notifications** via Web Push API

### 4. Authentication
- **Session-based** (stateful in Redis)
- **30-day session duration**
- **Admin approval required** for registration
- **bcrypt** for password hashing

### 5. Link Metadata
- **Synchronous fetching** during post/comment creation
- **Stored as JSONB** in `links` table
- **Admin toggle** to enable/disable
- **Rich embeds** where possible (Spotify, YouTube, etc.)

### 6. Observability
- **All three OTel signals**: traces, metrics, logs
- **Trace every request** (no sampling for small scale)
- **Direct OTLP export** to Grafana Stack
- **30-day retention** minimum

## Common Tasks

### Running Locally
```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f backend
docker-compose logs -f frontend

# Stop
docker-compose down
```

### Database Migrations
```bash
# Create new migration
cd backend && migrate create -ext sql -dir migrations -seq <name>

# Run migrations
migrate -path migrations -database "postgres://..." up
```

### API Endpoint Pattern
All endpoints:
1. Use `context.Context` for cancellation/observability
2. Log via OpenTelemetry (structured)
3. Return standard error format
4. Validate auth via middleware

Example:
```go
func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
    // 1. Extract context, user from middleware
    // 2. Parse request body
    // 3. Validate
    // 4. Call service layer
    // 5. Return JSON or error
}
```

### WebSocket Events
Format: `{type: "event_type", data: {...}}`

Examples:
- `{type: "post_created", data: {post}}`
- `{type: "comment_added", data: {comment, post_id}}`
- `{type: "mention", data: {mention}}`

## Code Style & Conventions

### Go
- **Package structure**: `internal/` for private code, `cmd/` for entry points
- **Error handling**: Wrap errors with context, don't ignore
- **Logging**: Use OpenTelemetry logs (structured, not fmt.Println)
- **Database queries**: Use parameterized queries (prevent SQL injection)
- **Middleware**: Chain via `http.Handler` wrapper pattern

### Svelte
- **Component naming**: PascalCase (PostCard.svelte)
- **Store naming**: camelCase (authStore.ts)
- **Styling**: Use Tailwind CSS or component-scoped CSS
- **API calls**: Centralize in `services/api.ts`

## Important Notes

### Lightweight Philosophy
- Avoid heavy dependencies; prefer stdlib when possible
- Single binary deployment (no separate processes initially)
- Monitor resource usage in observability

### Scalability Readiness
- Architecture supports multi-instance Go servers (via Redis pub/sub)
- Database designed for horizontal scaling (no sharding yet)
- Switch to OpenTelemetry Collector later if needed

### Content Type Specifics
Each section type (music, recipes, books, movies, events) will need custom:
- Link metadata extraction logic
- Rich embed rendering (Spotify player, Rotten Tomatoes ratings, etc.)
- Validation (e.g., valid Spotify URLs)

These belong in `internal/services/links/` with a strategy pattern.

### Admin Capabilities (MVP)
- Delete/restore any post or comment
- Manage user approvals
- View audit logs
- Toggle link metadata fetching

Later expansions: suspend users, configure sections, custom emojis.

## Testing Strategy

- **Unit tests** for business logic (services/)
- **Integration tests** for handlers (happy path + error cases)
- **Database tests** use transactions (rollback after each test)
- **WebSocket tests** mock Redis or use real connection
- **Frontend unit tests** for stores/services and component tests (Svelte) using Vitest + jsdom + Testing Library
- **Frontend typecheck**: Run `cd frontend && npm run check` for any frontend changes
- **Failing tests policy**: Tests should pass before finalizing an issue. If explicitly instructed that tests may fail, file follow-up issues for each failing domain and link them in the PR.

### Pre-commit Checks (prek)

These checks must pass before committing and pushing:
```bash
# Run checks only for files changed in the current changeset
prek run

# Or run everything
prek run --all-files
```

### Writing GitHub Comments/Issues/PRs

- Never use literal `\\n` in GitHub issues, PRs, or comments. Use real newlines instead.

### Writing GitHub Comments/Issues/PRs

- Never use literal `\\n` in GitHub issues, PRs, or comments. Use real newlines instead.

## Deployment

Production:
```bash
# Build Go binary
go build -o clubhouse-server ./cmd/server

# Push Docker image to registry
docker build -t myregistry/clubhouse:latest .
docker push myregistry/clubhouse:latest

# Deploy via docker-compose on server
docker-compose -f docker-compose.prod.yml up -d
```

## Questions for the Team

Before major feature additions, ask:
1. Does this add observable overhead?
2. Can we implement this with existing dependencies?
3. How does this scale to 500 concurrent users?

## Useful Commands

```bash
# Format Go code
go fmt ./...

# Lint
golangci-lint run ./...

# Build (check for compile errors)
cd backend && go build ./...

# Test
cd backend && go test -v ./...

# Frontend tests (unit + component)
cd frontend && npm run test

# Check dependencies
go mod tidy && git diff go.mod

# Frontend check
cd frontend && npm run check
```

## Database Schema Reference

The database uses `user_id` (not `author_id`) for foreign keys to users:

| Table | User FK Column |
|-------|---------------|
| posts | `user_id` |
| comments | `user_id` |
| reactions | `user_id` |
| mentions | `mentioned_user_id` |
| notifications | `user_id` |
| audit_logs | `admin_user_id` |

Always check `backend/migrations/` for the actual schema before writing queries.

## When to Contact the Developer

- Major architecture decisions
- Adding new external dependencies
- Changing database schema
- Security-related changes
- Performance optimizations

---

**Last Updated**: January 19, 2026
**Version**: 1.1
