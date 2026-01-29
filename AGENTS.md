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
| Backend | Go 1.24+ |
| Frontend | Svelte 4 (PWA) |
| Database | PostgreSQL 16+ |
| Cache/Pub-Sub | Redis 7+ |
| Deployment | Docker Compose |
| Observability | OpenTelemetry + Grafana Stack (Loki, Prometheus, Tempo) |
| Authentication | Session-based (Redis-backed, httpOnly cookies) |

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
- **11 core tables**: users, sections, posts, comments, reactions, links, mentions, notifications, section_subscriptions, audit_logs, push_subscriptions
- **Soft deletes** with a 7-day owner restore window (no automated purge job in repo)
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
- **Retention** follows service configs (see `tempo.yml`, `prometheus.yml`, and `loki.yml`)

## Common Tasks

### Running Locally
```bash
# Start all services
docker compose up -d

# View logs
docker compose logs -f backend
docker compose logs -f frontend

# Stop
docker compose down
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
- **Logging**: Use the observability package functions (see below)
- **Database queries**: Use parameterized queries (prevent SQL injection)
- **Middleware**: Chain via `http.Handler` wrapper pattern

### Logging

Use the structured logging functions in `internal/observability/`. Never use `fmt.Println` or the standard `log` package.

**Available functions:**
```go
import "github.com/sanderginn/clubhouse/internal/observability"

// Debug - verbose information for development/troubleshooting
// Only emitted when LOG_LEVEL=debug
observability.LogDebug(ctx, "processing request", "user_id", userID, "action", "create_post")

// Info - general operational messages
// Emitted when LOG_LEVEL=debug or LOG_LEVEL=info
observability.LogInfo(ctx, "post created", "post_id", postID, "section_id", sectionID)

// Warn - potential issues that don't prevent operation
// Emitted when LOG_LEVEL=debug, info, or warn
observability.LogWarn(ctx, "rate limit approaching", "user_id", userID, "remaining", "5")

// Error - failures requiring attention
// Always emitted regardless of LOG_LEVEL
observability.LogError(ctx, observability.ErrorLog{
    Message:    "failed to create post",
    Code:       "POST_CREATE_FAILED",
    StatusCode: http.StatusInternalServerError,
    UserID:     userID,
    Err:        err,
})
```

**When to use each level:**
- **Debug**: Request/response details, SQL queries, cache hits/misses, detailed flow tracing
- **Info**: Successful operations worth noting (user actions, service startup, config changes)
- **Warn**: Recoverable issues, deprecated usage, approaching limits
- **Error**: Failures, exceptions, things requiring investigation

**Log level configuration:**
Set via `LOG_LEVEL` environment variable: `debug`, `info`, `warn`, `error` (defaults to `info`)

### Audit Logging

All state-changing operations must emit an audit event (create/update/delete/restore/approve/reject/toggle settings). This applies to admin actions and any user action that mutates persistent state.

**Audit event format:**
- **action**: snake_case action name (see common actions below)
- **target_user_id**: user affected by the action (nullable; use the closest related user)
- **metadata**: JSON map with relevant IDs and context (post_id, comment_id, section_id, reason, previous_state, etc.)

When recording audit logs, include the acting admin/user ID and map `target_user_id` to `related_user_id` in `audit_logs`. If metadata storage isn't available yet, capture the most important IDs in `related_*` columns and document what would go into metadata.

**Common audit action names (use these for consistency):**
- `approve_user`
- `reject_user`
- `suspend_user`
- `unsuspend_user`
- `delete_post`
- `hard_delete_post`
- `restore_post`
- `delete_comment`
- `hard_delete_comment`
- `restore_comment`
- `toggle_link_metadata`
- `update_section`
- `delete_section`

**Example (admin deletes a comment):**
```go
auditQuery := `
    INSERT INTO audit_logs (admin_user_id, action, related_comment_id, related_user_id, created_at)
    VALUES ($1, 'delete_comment', $2, $3, now())
`
_, err := tx.ExecContext(ctx, auditQuery, adminUserID, commentID, commentUserID)
```

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

### CI (Buildkite)

PRs targeting `main` run Buildkite using `.buildkite/pipeline.yml`. Ensure backend and frontend steps pass before merge. The required checks are the Buildkite pipeline steps (backend format/lint, backend tests/build, frontend lint/check/test).

### Writing GitHub Comments/Issues/PRs

- Never use literal `\\n` in GitHub issues, PRs, or comments. Use real newlines instead.

## Agent Checklists

### Subagent Checklist
- Add audit events for all state-changing operations (action, target_user_id, metadata).
- Add/extend tests that verify audit logs are written when expected.

### Reviewer Checklist
- Confirm every state-changing operation includes audit logging.
- Verify action names use the common naming scheme and metadata captures key IDs.

## Deployment

Production:
```bash
# Build Go binary
go build -o clubhouse-server ./cmd/server

# Push Docker image to registry
docker build -t myregistry/clubhouse:latest .
docker push myregistry/clubhouse:latest

# Deploy via Docker Compose on server
docker compose -f docker-compose.prod.yml up -d
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

**Last Updated**: January 29, 2026
**Version**: 1.3
