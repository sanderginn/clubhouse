# Beads Work Planning (Multi-Orchestrator Safe)

This repo historically used `.work-queue.json` + `scripts/` to plan and assign work. That works for a single orchestrator, but it becomes fragile when multiple orchestrators run at the same time (local lock files, frequent commits to `main`, push races).

[Beads](https://github.com/steveyegge/beads) is a git-backed, dependency-aware issue tracker designed for agents. It gives us:
- A real dependency graph (`blocks`) that powers `bd ready`
- Atomic claiming (`bd update <id> --claim`) to prevent double-assignment
- Merge-friendly JSONL storage (`.beads/issues.jsonl`) with an optional merge driver
- Optional "sync branch" mode (`bd init --branch beads-sync`) so planning commits never touch `main`

## Recommended Setup

1. Install `bd` (pick one):
```bash
brew install beads
# or: npm install -g @beads/bd
# or: go install github.com/steveyegge/beads/cmd/bd@latest
```

2. Initialize beads in this repo (recommended: separate sync branch):
```bash
cd /path/to/clubhouse
bd init --branch beads-sync
```

Notes:
- `bd init` can install hooks and configure the JSONL merge driver. Say yes if you want the most robust multi-agent experience.
- The sync-branch workflow is optional, but strongly recommended if multiple orchestrators are making frequent planning updates.

## One-Time Migration From `.work-queue.json`

Once beads is initialized, import the existing work queue:
```bash
./scripts/beads/migrate-work-queue.sh
```

What it does:
- Creates one beads issue per entry in `.work-queue.json`
- Sets `external_ref` to `gh-<issue_number>`
- Converts `.dependencies` to `blocks` edges
- Adds labels `gh:<issue_number>` and `queue_prio:<legacy_priority>`
- Marks legacy `in_progress` items as claimed (assignee preserved)
- Marks legacy `completed` items as closed (keeps dependency graph consistent)

Validate:
```bash
bd ready
bd list --status in_progress
bd dep cycles
```

## New Workflow (Beads-Backed)

### Workers (Subagents)

Workers should claim work via beads, not by editing `.work-queue.json`.

This repo’s subagent bootstrap script (`.codex/skills/subagent/scripts/start-agent.sh`) now prefers beads when it detects:
- `bd` is installed
- `.beads/` exists

It will:
- Pick from `bd ready --json`
- Atomically claim with `bd update <id> --claim`
- Create a git worktree for the branch
- Print:
  - `BEADS_ID=...`
  - `ISSUE_NUMBER=...` (parsed from `external_ref` when possible)
  - `WORKTREE_PATH=...`

### Orchestrators

Each orchestrator should run from its own git worktree (detached HEAD) to avoid local contention with other orchestrators:
```bash
REPO_ROOT=/path/to/clubhouse
ORCH_ID="orch-$(date +%s)-$$"
ORCH_WT="$REPO_ROOT/.worktrees/$ORCH_ID"

git -C "$REPO_ROOT" fetch origin main
git -C "$REPO_ROOT" worktree add --detach "$ORCH_WT" origin/main
cd "$ORCH_WT"
```

In beads mode, orchestration uses:
- `bd ready` to see unblocked work
- `bd list --status in_progress` to see claimed work
- `bd close <id> --reason "Merged PR #123"` to mark completion

## Deprecation Notes

After migration:
- Treat `.work-queue.json` as legacy. Keep it around only until you’re confident in the beads-backed flow.
- Avoid scripts that commit planning updates to `main` (`scripts/claim-issue.sh`, `scripts/complete-issue.sh`) when running multiple orchestrators in parallel.
