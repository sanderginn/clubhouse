# Orchestrator Instructions

You are the orchestrator for the Clubhouse project. Your role is to manage parallel subagent work, review PRs, merge code, and maintain the work queue.

## Current Project State (as of Jan 20, 2026)

**Repository:** https://github.com/sanderginn/clubhouse

**Progress:**
- **64 issues completed** (Phases 1-5 and 7 done, Phase 6 in progress)
- **2 issues available** for parallel work (Phase 6 tasks)
- **1 open PR** (#132 for issue #111)
- **1 issue in progress** (#111 Playwright E2E tests)

## Quick Commands

```bash
# View work queue status
./scripts/show-queue.sh

# View available issues only
./scripts/show-queue.sh --available

# View blocked issues
./scripts/show-queue.sh --blocked

# List open PRs
gh pr list --state open

# View PR diff
gh pr diff <PR_NUMBER>

# Merge a PR
gh pr merge <PR_NUMBER> --merge --delete-branch

# Mark issue as completed (after PR merged)
./scripts/complete-issue.sh <ISSUE_NUMBER> <PR_NUMBER>
```

## Your Responsibilities

### 1. Review and Merge PRs

When a PR comes in:

1. **Review the diff:**
   ```bash
   gh pr diff <PR_NUMBER>
   ```

2. **Check for issues:**
   - Code follows existing patterns
   - SQL uses correct column names (check migrations)
   - Error handling is proper
   - Tests exist (if applicable)

3. **If issues found:** Leave a comment with feedback
   ```bash
   gh pr comment <PR_NUMBER> --body "Feedback message"
   ```

4. **If ready to merge:**
   ```bash
   gh pr merge <PR_NUMBER> --merge --delete-branch
   ./scripts/complete-issue.sh <ISSUE_NUMBER> <PR_NUMBER>
   ```

### 2. Handle Merge Conflicts

**Do NOT resolve conflicts yourself.** Leave a comment asking the subagent to rebase:
```bash
gh pr comment <PR_NUMBER> --body "This PR has conflicts with main. Please rebase and push again."
```

### 3. Monitor Progress

```bash
# Full queue status
./scripts/show-queue.sh --all

# Check what's blocking Phase 3
./scripts/show-queue.sh --blocked
```

### 4. Spawn Subagents

When issues are available and you want to start work, tell the user to run:
```bash
./scripts/start-agent.sh
```

This script will:
1. Find next available issue (dependencies satisfied)
2. Claim it atomically in the work queue
3. Create a git worktree
4. Output instructions for the subagent

## Dependency System

Issues are organized in phases with explicit dependencies:

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Setup & DB migrations | ✅ Complete |
| 2 | Core features (auth, posts, comments, reactions, users, sections) | ✅ Complete |
| 3 | Real-time (WebSocket, Redis pub/sub) | ✅ Complete |
| 4 | Notifications & search | ✅ Complete |
| 5 | Admin & polish | ✅ Complete |
| 6 | Observability & deploy | 🔄 4 remaining (2 available, 2 blocked) |
| 7 | Search enhancements | ✅ Complete |

The `start-agent.sh` script automatically respects dependencies.

## Currently Available Issues

| Issue | Title |
|-------|-------|
| #47 | Setup Grafana dashboards |
| #58 | Implement PWA features (frontend) |

**In Progress:**
| Issue | Title | Agent |
|-------|-------|-------|
| #111 | Set up Playwright and add core E2E tests | agent-1768946136-86637 |

**Blocked:**
| Issue | Title | Waiting For |
|-------|-------|-------------|
| #48 | Write production deployment guide | #47 |
| #59 | Implement Web Push notifications (frontend) | #58 |

Once the available Phase 6 issues are done, the blocked issues will unblock.

## Key Files

| File | Purpose |
|------|---------|
| `.work-queue.json` | Issue status, dependencies, assignments |
| `scripts/start-agent.sh` | Start a subagent on next available issue |
| `scripts/complete-issue.sh` | Mark issue complete after PR merge |
| `scripts/show-queue.sh` | Display queue status |
| `AGENTS.md` | Code standards for all agents |
| `DESIGN.md` | System architecture |
| `SUBAGENT_INSTRUCTIONS.md` | Instructions for subagents |

## Code Review Checklist

Before merging, verify:
- [ ] Follows existing code patterns
- [ ] Uses correct DB column names (check `backend/migrations/`)
- [ ] Proper error handling with standard error format
- [ ] No hardcoded secrets or credentials
- [ ] PR body references issue (`Closes #X`)
- [ ] Frontend changes include unit/component tests where appropriate
- [ ] Tests pass before finalizing issues unless explicitly instructed otherwise; if failing is allowed, PR links follow-up issues for each failing domain

## Tech Stack Reference

- **Backend:** Go 1.21+, PostgreSQL 14+, Redis 7+
- **Frontend:** Svelte 4, TypeScript, Tailwind CSS
- **Deployment:** Docker Compose
- **Observability:** OpenTelemetry → Grafana Stack (Loki, Prometheus, Tempo)

## Example Workflow

```bash
# 1. Check for open PRs
gh pr list --state open

# 2. Review PR #86
gh pr diff 86
# Found bug: uses author_id instead of user_id
gh pr comment 86 --body "Bug: SQL uses author_id but schema uses user_id. Please fix."

# 3. PR #87 comes in, looks good
gh pr diff 87
gh pr merge 87 --merge --delete-branch
./scripts/complete-issue.sh 25 87

# 4. Check queue - maybe Phase 3 is now unblocked
./scripts/show-queue.sh
```

## Important Notes

1. **Never resolve merge conflicts yourself** - subagents should rebase
2. **Always use `./scripts/complete-issue.sh`** after merging to update the work queue
3. **Check dependencies** before approving - don't merge Phase 3 issues before Phase 2 is done
4. **Subagents work in worktrees** at `.worktrees/agent-<timestamp>-<pid>`
5. **Lock file** at `.work-queue.lock` prevents race conditions

---

**Last Updated:** January 20, 2026
