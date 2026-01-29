---
name: subagent
description: Work as a subagent on Clubhouse project issues. Claim available issues, create git worktrees, implement features following project patterns, run targeted tests, create pull requests, and address review feedback.
---

# Subagent Instructions

You are a subagent for the Clubhouse project. Your job is to claim an issue, implement it in a dedicated worktree, open a PR, and then address any review feedback until the PR is merged.

## Step 1: Claim an Issue and Create a Worktree

From the **main repository root**, run the start script:

```bash
.codex/skills/subagent/scripts/start-agent.sh
```

The script will:
1. Find the next available issue (dependencies satisfied, bugs prioritized)
2. Atomically claim it in the work queue
3. Create a dedicated git worktree

It outputs two lines:
```
ISSUE_NUMBER=<number>
WORKTREE_PATH=<path>
```

Parse these values. You will use them throughout the rest of the workflow.

## Step 2: Change to the Worktree

```bash
cd <WORKTREE_PATH>
```

**Mandatory:** Do all work from the designated worktree. Never edit or run commands from the main repo root once the worktree is created.

**Autonomy:** After entering the worktree, start the task immediately. Do not wait for any further input until the PR is opened.

## Step 3: Fetch Issue Context

```bash
gh issue view <ISSUE_NUMBER>
```

Read the issue description and acceptance criteria. This is your specification.

## Step 4: Get Code Context (Efficiently)

Use the reference tables below to go directly to the files you need. Do not explore broadly.

### Key File Locations

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

### Schema Quick Reference

**Foreign keys to users always use `user_id`** (NOT `author_id`):
- `posts.user_id`, `comments.user_id`, `reactions.user_id`
- `mentions.mentioned_user_id`, `notifications.user_id`
- `audit_logs.admin_user_id`

**Soft deletes:** `deleted_at` timestamp + `deleted_by_user_id`

**All IDs are UUIDs**

Check `backend/migrations/` only for the specific table you need.

Only read full files if you need more detail:
- `AGENTS.md` - Code standards (skim relevant sections)
- `DESIGN.md` - Only if you need API spec details

### What NOT to Explore

- **Backend-only issues:** Don't read frontend code
- **Frontend-only issues:** Don't read backend code
- **Schema lookups:** Don't read all migrations — just the table you need
- **Don't read DESIGN.md fully** — use the schema reference above
- **Don't read all of AGENTS.md** — skim for your specific domain

## Step 5: Implement the Feature

Follow the issue description and acceptance criteria.

### Pattern Examples

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

### Add Audit Logging (When Required)

**All state-changing operations must emit an audit event.** This applies to:
- Admin actions (approve/reject user, hard delete, restore, toggle settings)
- User actions that mutate persistent state (delete post/comment, restore own content)

**Audit event format:**
- **action**: snake_case action name (e.g., `delete_post`, `approve_user`, `toggle_link_metadata`)
- **target_user_id**: user affected by the action (nullable; use the closest related user)
- **metadata**: JSON map with relevant IDs and context

**Common action names (use these for consistency):**
`approve_user`, `reject_user`, `delete_post`, `hard_delete_post`, `restore_post`, `delete_comment`, `hard_delete_comment`, `restore_comment`, `generate_password_reset_token`, `toggle_link_metadata`, `update_section`, `delete_section`

**Example:**
```go
auditQuery := `
    INSERT INTO audit_logs (admin_user_id, action, related_comment_id, related_user_id, created_at)
    VALUES ($1, 'delete_comment', $2, $3, now())`
_, err := tx.ExecContext(ctx, auditQuery, adminUserID, commentID, commentUserID)
```

See AGENTS.md "Audit Logging" section for full details.

### Add Observability (When Required)

**New endpoints and operations must include OpenTelemetry signals** per DESIGN.md Section 9:

**Traces:**
- Every new HTTP endpoint should be traced
- Include span attributes: `user_id`, `section_id`, `post_id`, error details
- Database queries should be traced

**Metrics:**
- Business operations (posts created, comments added, deletes, etc.) should emit metrics
- HTTP request counts and durations (usually handled by middleware)

**Logs:**
- Use `internal/observability` functions: `LogDebug`, `LogInfo`, `LogWarn`, `LogError`
- Never use `fmt.Println` or standard `log` package
- Include structured key-value pairs

If your implementation adds new endpoints or business operations, follow existing patterns in handlers/services for observability.

### Add Tests (When It Makes Sense)

If you change or add backend logic, add or update **unit tests** (services/) and **handler tests** (handlers/) where applicable. If you change frontend logic, add **frontend unit tests** (stores/services) and **component tests** (Svelte) where reasonable. If the change affects critical user flows, add or extend **E2E tests** (Playwright) to cover the new behavior. If tests are not reasonable for the change, state why in the PR description.

**If you added audit logging**, add/extend tests to verify audit logs are written when expected.

Tests should pass before finalizing an issue. If you're explicitly instructed that tests may fail, **create follow-up issues per failing domain** and link them in the PR description.

## Step 6: Test Your Changes

Run only tests for your domain instead of the full suite:

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

## Step 7: Commit and Push

```bash
git add .
git commit -m "feat(issue-<ISSUE_NUMBER>): description"
git push -u origin <BRANCH_NAME>
```

### Update Markdown Docs (When Relevant)

If your changes affect behavior, setup, or architecture, update relevant markdown docs (README, DESIGN, AGENTS, `docs/`, or these instructions) to reflect the new reality.

## Step 8: Create Pull Request

```bash
gh pr create --title "Issue Title" --body "$(cat <<'EOF'
## Summary
- ...

Closes #<ISSUE_NUMBER>
EOF
)"
```

## Step 9: Wait for Review and Address Feedback

After opening the PR, **wait for instructions**. The reviewer or orchestrator will either:
- **Merge it** — you are done.
- **Post feedback** — you must address it (see below).
- **Ask you to rebase** — see the rebase section below.

### Handling Review Feedback

When you receive review feedback on your PR:

1. **Read all comments** on the PR:
   ```bash
   gh pr view <PR_NUMBER> --comments
   ```

2. **Implement the requested fixes** in the worktree. Address every comment — do not skip or defer items unless explicitly told to.

3. **Test your fixes** the same way as Step 6 (targeted tests for the affected domain).

4. **Commit and push** the fixes:
   ```bash
   git add .
   git commit -m "fix(issue-<ISSUE_NUMBER>): address review feedback"
   git push
   ```

5. **Wait again** for the next round of review. Repeat this cycle until the PR is merged or you are told to stop.

### Rebasing on Main

If instructed to rebase (e.g., due to merge conflicts):

```bash
git fetch origin main
git rebase origin/main
# Resolve any conflicts, then:
git push --force-with-lease
```

After rebasing, wait for the next review cycle.

## Important Notes

1. **Always use the start script** (`.codex/skills/subagent/scripts/start-agent.sh`) — it handles claiming atomically
2. **Work in the designated worktree only** — never use the main repo for edits/commands once a worktree exists
3. **Use the file location table above** — don't explore; go directly to the right files
4. **Copy existing patterns** — check one similar handler/service, not multiple
5. **Add tests when it makes sense** — include frontend tests; explain in PR if you didn't add tests
6. **Add audit logging for state-changing operations** — see "Add Audit Logging" section above
7. **Add observability signals for new endpoints** — see "Add Observability" section above
8. **Failing tests policy** — tests must pass unless explicitly told otherwise; if allowed to fail, file follow-up issues and link them in the PR
9. **Don't skip dependencies** — the script handles this automatically
10. **Rebase if asked** — rebase on `main` when the reviewer requests it; never merge main into your branch
11. **Minimize exploration** — see "What NOT to Explore" section
12. **Start immediately** — once in the worktree, begin work without waiting for more user input; keep going until the PR is opened
13. **No confirmation prompts** — never ask whether to run extra tests or whether to commit/create a PR; decide and proceed without asking
14. **Worktree integrity checks** — after every change, verify you did not modify files outside your worktree. If you did, immediately move those changes into your worktree and undo the changes outside it.
15. **Stay alive after PR** — do not exit after creating the PR. Wait for review feedback and address it until the PR is merged.
16. **Do not spawn sub-agents** — you are already the subagent; proceed with this workflow directly.

## Troubleshooting

### Lock timeout
```bash
rm .work-queue.lock
```

### No available issues
```bash
./scripts/show-queue.sh --blocked
```

### Merge conflicts during rebase
```bash
# Fix conflicts in the affected files, then:
git add .
git rebase --continue
git push --force-with-lease
```

---

**Questions?** Check `AGENTS.md` and `DESIGN.md`.
