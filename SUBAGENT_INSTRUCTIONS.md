# Subagent Work Queue Instructions

This document explains how new Amp sessions should pick up work.

## How to Claim an Issue

When you start a new Amp session for this project:

1. **Use the claim script** (atomic, race-condition safe):
   ```bash
   ./scripts/claim-issue.sh your-agent-name
   ```
   
   This script will:
   - Find the first available issue
   - Mark it as `in_progress` with your agent name
   - Commit and push the claim to main
   - Display the issue details and next steps
   - Handle race conditions if multiple agents claim simultaneously

**Important:** Always use this script first. It's atomic and prevents multiple agents from claiming the same issue.

## Workflow for Each Issue

1. **Create a feature branch:**
   ```bash
   git checkout -b <branch_name>
   ```

2. **Implement the feature** following AGENTS.md guidelines

3. **Commit with conventional commits:**
   ```bash
   git commit -m "feat: description"
   ```

4. **Push and create a PR:**
   ```bash
   git push -u origin <branch_name>
   gh pr create --title "..." --body "..."
   ```

5. **After opening PR:**
   - The orchestrator will review and merge
   - Once merged, orchestrator updates work queue status to `"completed"`
   - Next agent can then claim the next issue

## Dependency Chain

Some issues depend on others. **Do not start** an issue if its dependencies aren't completed:

- **Auth endpoints (7-11):** Sequential (register → login → logout → middleware → approval)
- **Posts endpoints (12-16):** Depend on auth (7-11)
- **Comments endpoints (17-18):** Depend on posts (12-16)
- **Frontend auth (51):** Can work in parallel with backend

## Current Status

Check `.work-queue.json` for real-time issue status:
- `available` — Ready to claim (use `./scripts/claim-issue.sh` to claim)
- `in_progress` — Another agent is actively working on it
- `completed` — Merged into main

## Orchestrator (Main Session)

The orchestrator session will:
- Monitor progress via `.work-queue.json`
- Merge approved PRs
- Update queue status
- Handle any blockers or conflicts

---

**Example Workflow**

**Agent 1 starts:**
```bash
./scripts/claim-issue.sh agent-1
# Output:
# ✓ Successfully claimed issue #7
# Issue: Implement user registration endpoint
# Branch: feat/auth-register
```

**Agent 1 implements:**
```bash
git checkout -b feat/auth-register
# ... write code ...
git commit -m "feat: implement user registration"
git push -u origin feat/auth-register
gh pr create --title "feat: implement user registration" --body "Closes #7"
```

**Agent 2 starts (while Agent 1 is working):**
```bash
./scripts/claim-issue.sh agent-2
# Finds issue #51 (frontend auth, can run in parallel)
# Starts work on that
```

**Orchestrator:**
```bash
# Reviews & merges PR from Agent 1
gh pr merge 65 --merge
# Updates work queue
git add .work-queue.json
git commit -m "chore: mark issue #7 completed, PR #65"
git push
```

**Agent 3 starts (after Agent 1 completes):**
```bash
./scripts/claim-issue.sh agent-3
# Finds issue #8 (user login, depends on #7)
# Starts work on that
```

---

Questions? Check AGENTS.md and DESIGN.md for architecture details.
