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

### 3. Read Guidelines

Before coding:
- `AGENTS.md` - Code standards and conventions
- `DESIGN.md` - System architecture

### 4. Implement the Feature

Follow the issue description and acceptance criteria.

### 4a. Add Tests (When It Makes Sense)

If you change or add backend logic, add or update **unit tests** (services/) and **handler tests** (handlers/) where applicable. If tests are not reasonable for the change, state why in the PR description.

### 5. Test Your Changes

```bash
# Backend
cd backend && go build ./... && go test ./...

# Frontend
cd frontend && npm run check
```

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
3. **Check existing code** - follow established patterns
4. **Add unit tests when it makes sense** - explain in PR if you didn’t add tests
5. **Don't skip dependencies** - the script handles this automatically
6. **Rebase if conflicts** - orchestrator will ask you to rebase, not fix it themselves

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
