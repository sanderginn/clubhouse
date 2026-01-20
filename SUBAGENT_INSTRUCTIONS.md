# Subagent Instructions

This document explains how to work as a subagent on the Clubhouse project.

## Quick Start

From the **main repository root** (not a worktree), run:

```bash
./scripts/start-agent.sh
```

This script will:
1. Find the next available issue (dependencies satisfied)
2. Atomically claim it in the work queue
3. Create a dedicated git worktree
4. Output the worktree path and issue details

## Key File Locations (Read These First)

All paths relative to `backend/internal/`:

| Domain | Handler | Service | Handler Tests |
|--------|---------|---------|---------------|
| Auth | handlers/auth.go | services/session.go | - |
| Posts | handlers/post.go | services/post.go | handlers/post_test.go |
| Comments | handlers/comment.go | services/comment.go | handlers/comment_test.go |
| Reactions | handlers/reaction.go | services/reaction.go | handlers/reaction_test.go |
| Users | handlers/user.go | services/user.go | handlers/user_test.go |
| Sections | handlers/section.go | services/section.go | handlers/section_test.go |
| Admin | handlers/admin.go | - | handlers/admin_test.go |
| Search | handlers/search.go | services/search.go | handlers/search_test.go |
| Notifications | handlers/notification.go | services/notification.go | - |
| WebSocket | handlers/websocket.go | - | - |
| Pub/Sub | handlers/pubsub.go | - | handlers/pubsub_test.go |

**Other key files:**
- `models/` - Request/response types, `ErrorResponse` struct
- `middleware/` - Auth, logging, etc.
- `db/` - Database initialization

## Schema Quick Reference

**Foreign keys to users always use `user_id`** (NOT `author_id`):
- `posts.user_id`, `comments.user_id`, `reactions.user_id`
- `mentions.mentioned_user_id`, `notifications.user_id`
- `audit_logs.admin_user_id`

**Soft deletes:** `deleted_at` timestamp + `deleted_by_user_id`

**All IDs are UUIDs**

Check `backend/migrations/` only for the specific table you need.

## Pattern Examples

**New endpoint handler:**
```go
// Copy pattern from handlers/post.go
func (h *Handler) YourEndpoint(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    userID := ctx.Value(middleware.UserIDKey).(string)
    // ... parse request, call service, return JSON
}
```

**Error responses:**
```go
models.ErrorResponse{Error: "message", Code: "ERROR_CODE"}
```

**Standard error codes:** `INVALID_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `INTERNAL_ERROR`

## Efficient Testing

Run only tests for your domain instead of the full suite:

```bash
# Instead of: go test ./...
cd backend
go test ./internal/handlers/admin_test.go -v        # Single test file
go test ./internal/services/user_test.go -v         # Single test file
go test ./internal/handlers/... -run TestCreate     # Pattern match
```

## What NOT to Explore

- **Backend-only issues:** Don't read frontend code
- **Frontend-only issues:** Don't read backend code
- **Schema lookups:** Don't read all migrations - just the table you need
- **Don't read DESIGN.md fully** - use the schema reference above
- **Don't read all of AGENTS.md** - skim for your specific domain

---

## Workflow

### 1. Run the Start Script

```bash
./scripts/start-agent.sh
```

The output will show:
- Issue number and full description
- Worktree path (e.g., `.worktrees/agent-1737300000-12345`)
- Branch name

### 2. Change to Worktree

```bash
cd <WORKTREE_PATH>  # Path from script output
```

### 3. Get Context (Efficiently)

Use the **Key File Locations** and **Schema Quick Reference** sections above first.

Only read full files if you need more detail:
- `AGENTS.md` - Code standards (skim relevant sections)
- `DESIGN.md` - Only if you need API spec details

### 4. Implement the Feature

Follow the issue description and acceptance criteria.

### 4a. Add Tests (When It Makes Sense)

If you change or add backend logic, add or update **unit tests** (services/) and **handler tests** (handlers/) where applicable. If you change frontend logic, add **frontend unit tests** (stores/services) and **component tests** (Svelte) where reasonable. If tests are not reasonable for the change, state why in the PR description.
Tests should pass before finalizing an issue. If you’re explicitly instructed that tests may fail, **create follow-up issues per failing domain** and link them in the PR description.

### 5. Test Your Changes

See **Efficient Testing** section above. Prefer targeted tests:

```bash
# Backend - build first, then test your domain only
cd backend && go build ./...
go test ./internal/handlers/your_handler_test.go -v
go test ./internal/services/your_service_test.go -v

# Frontend (if applicable)
cd frontend && npm run check
npm run test -- --grep "YourComponent"
```

Only run full test suite (`go test ./...`) before creating PR.

### 6. Commit and Push

```bash
git add .
git commit -m "feat(issue-NN): description"
git push -u origin <BRANCH_NAME>
```

### 7. Create Pull Request

```bash
gh pr create --title "Issue Title" --body "Summary:\n- ...\n\nCloses #NN"
```

### 8. Wait for Review

The orchestrator will review your PR and either:
- Merge it (done!)
- Request changes (fix and push again)

## Scripts Reference

| Script | Purpose |
|--------|---------|
| `./scripts/start-agent.sh` | Claim next issue, create worktree |
| `./scripts/show-queue.sh` | View queue status |
| `./scripts/complete-issue.sh` | (Orchestrator only) Mark issue done |

## Important Notes

1. **Always use `start-agent.sh`** - it handles claiming atomically
2. **Work in the worktree** - not the main repo
3. **Use the file location table above** - don't explore; go directly to the right files
4. **Copy existing patterns** - check one similar handler/service, not multiple
5. **Add tests when it makes sense** - include frontend tests; explain in PR if you didn't add tests
6. **Failing tests policy** - tests must pass unless explicitly told otherwise; if allowed to fail, file follow-up issues and link them in the PR
7. **Don't skip dependencies** - the script handles this automatically
8. **Rebase if conflicts** - orchestrator will ask you to rebase, not fix it themselves
9. **Minimize exploration** - see "What NOT to Explore" section

## Troubleshooting

### Lock timeout
```bash
rm .work-queue.lock
```

### No available issues
```bash
./scripts/show-queue.sh --blocked
```

### Merge conflicts
```bash
git fetch origin main
git rebase origin/main
git push --force-with-lease
```

---

**Questions?** Check `AGENTS.md` and `DESIGN.md`.
