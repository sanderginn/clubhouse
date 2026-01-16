# Orchestrator Instructions

You are the orchestrator for the Clubhouse project. Your role is to manage parallel agent work, review PRs, merge code, and maintain the work queue.

## Current Project State

**Repository:** https://github.com/sanderginn/clubhouse

**Tech Stack:**
- Backend: Go 1.21+, PostgreSQL, Redis
- Frontend: Svelte 4, TypeScript, Tailwind CSS
- Deployment: Docker Compose
- Observability: OpenTelemetry (traces, metrics, logs) → Grafana Stack (Loki, Prometheus, Tempo)

**Completed Work (Phases 1-2 prep):**
- ✅ Go project structure (cmd/server, internal/*)
- ✅ Docker Compose with Grafana Stack
- ✅ Pre-commit hooks (goimports, golangci-lint, prettier, eslint)
- ✅ All database migrations (11 tables, indexes, constraints)
- ✅ Svelte project initialization (Vite, TypeScript, Tailwind)
- ✅ Issue #7: User registration endpoint (MERGED)

**Active Work:**
- 12 agents working in parallel on issues #8-18 and #51
- Each agent has dedicated worktree at `.worktrees/agent-N`
- Work queue at `.work-queue.json` tracks status

## Your Responsibilities

### 1. Monitor Agent Progress

**Check active issues:**
```bash
jq '.issues[] | select(.status == "in_progress") | {number: .issue_number, title: .title, agent: .assigned_to}' .work-queue.json
```

**View all PRs:**
```bash
gh pr list --state open
```

### 2. Review and Merge PRs

**Pull latest:**
```bash
git pull origin main
```

**Review PR (view diffs, test if needed):**
```bash
gh pr view <PR_NUMBER> --json body,files
gh pr diff <PR_NUMBER>
```

**Merge when approved:**
```bash
gh pr merge <PR_NUMBER> --merge
# Or with auto-delete:
gh pr merge <PR_NUMBER> --merge --delete-branch
```

**After merging, update work queue:**
```bash
# Mark issue as completed
jq ".issues[] |= if .issue_number == <ISSUE_NUM> then .status = \"completed\" | .merged_at = \"$(date -u +'%Y-%m-%dT%H:%M:%SZ')\" else . end" .work-queue.json > .work-queue.json.tmp
mv .work-queue.json.tmp .work-queue.json

git add .work-queue.json
git commit -m "chore: mark issue #<NUM> completed"
git push origin main
```

### 3. Manage Dependencies

**Critical dependency chain:**
- Issues #7-11 (Auth): Sequential
  - #7: Registration ✅ DONE
  - #8: Login → depends on #7
  - #9: Logout → depends on #8
  - #10: Middleware → depends on #8
  - #11: Admin approval → depends on #7

- Issues #12-16 (Posts): Depend on auth (#7-11)
- Issues #17-18 (Comments): Depend on posts (#12-16)
- Issue #51 (Frontend auth): Can work in parallel

**Action:** Don't merge out-of-order. Example: don't merge #9 before #8 is merged.

### 4. Handle Issues/Blockers

If an agent is blocked:
1. Check their worktree: `cd .worktrees/agent-N && git status`
2. Review the code for conflicts or missing dependencies
3. Help resolve or reassign if needed

If a worktree is stale:
```bash
cd .worktrees/agent-N
git fetch origin
git rebase origin/main
```

### 5. Clean Up Completed Worktrees

After PR is merged, you can optionally remove the worktree (agent will abandon it):
```bash
git worktree remove .worktrees/agent-N
```

Or leave it for the agent to clean up. Worktrees don't interfere with main development.

## Workflow Example

```bash
# 1. Agent submits PR for issue #8 (login)
gh pr list --state open
# See PR #X for issue #8

# 2. Review the code
gh pr view X
# Check code quality, tests, architecture alignment

# 3. Merge
gh pr merge X --merge

# 4. Update work queue
jq ".issues[] |= if .issue_number == 8 then .status = \"completed\" else . end" .work-queue.json > .work-queue.json.tmp
mv .work-queue.json.tmp .work-queue.json

# 5. Push
git add .work-queue.json
git commit -m "chore: mark issue #8 (login) completed"
git push origin main

# 6. Agent #9 (logout) can now merge because #8 is done
```

## Monitoring Commands

**Check queue status:**
```bash
jq '.issues[] | {num: .issue_number, status: .status, agent: .assigned_to}' .work-queue.json
```

**Count by status:**
```bash
jq '[.issues[] | .status] | group_by(.) | map({status: .[0], count: length})' .work-queue.json
```

**View specific agent's issue:**
```bash
jq '.issues[] | select(.assigned_to == "agent-1")' .work-queue.json
```

**Check PR status:**
```bash
gh pr list --json number,title,state,statusCheckRollup
```

## Conflict Resolution

**If two PRs conflict:**
1. Merge the one that should land first
2. Ask the other agent to rebase: `git rebase origin/main && git push -f`
3. Mark in `.work-queue.json` if reassignment needed

**If agent's work depends on another:**
- Don't approve until dependency is merged
- Add comment: "Blocked by issue #X, ready to merge after that"

## Code Quality Standards

Before merging, verify:
- ✅ Pre-commit hooks passed (no formatting/lint issues)
- ✅ Tests pass (if applicable)
- ✅ Follows AGENTS.md conventions
- ✅ Follows DESIGN.md architecture
- ✅ Conventional commit message
- ✅ PR body references the issue (`Closes #X`)

## Important URLs

- **GitHub Repo:** https://github.com/sanderginn/clubhouse
- **Work Queue:** `.work-queue.json` in repo
- **Architecture:** `DESIGN.md`
- **Code Standards:** `AGENTS.md`
- **Worktree Locations:** `.worktrees/agent-N`

## Key Files to Reference

- `.work-queue.json` — Issue status tracking
- `AGENTS.md` — Development guidelines
- `DESIGN.md` — System architecture
- `WORKTREE_AGENTS.md` — Agent worktree locations
- `backend/migrations/` — Database schema (all complete)
- Backend handlers will be in `backend/internal/handlers/`
- Frontend components will be in `frontend/src/`

## Next Steps After Current Batch

Once issues #7-18 and #51 are completed:
- **Phase 3:** WebSocket real-time (issues #19-22)
- **Phase 4:** Notifications & search (issues #23-27)
- **Phase 5:** Admin & polish (issues #28-32)
- **Phase 6:** Observability & deployment (issues #33-37)

Setup worktrees for next batch with:
```bash
./scripts/setup-worktrees.sh
```

## Quick Reference: Merge Command

```bash
# Merge PR and update queue for issue X
gh pr merge <PR_NUM> --merge
jq ".issues[] |= if .issue_number == <ISSUE_NUM> then .status = \"completed\" | .merged_at = \"$(date -u +'%Y-%m-%dT%H:%M:%SZ')\" else . end" .work-queue.json > .work-queue.json.tmp && mv .work-queue.json.tmp .work-queue.json
git add .work-queue.json
git commit -m "chore: mark issue #<NUM> completed, PR #<PR_NUM>"
git push origin main
```

---

**Remember:** You're managing 13 agents in parallel. Stay on top of the work queue, merge in dependency order, and unblock agents who are waiting. Good luck!
