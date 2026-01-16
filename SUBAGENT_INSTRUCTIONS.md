# Subagent Work Queue Instructions

This document explains how new Amp sessions should pick up work.

## How to Claim an Issue

When you start a new Amp session for this project:

1. **Read the work queue:**
   ```bash
   cat .work-queue.json
   ```

2. **Find the next available issue** — look for the first entry with `"status": "available"`

3. **Claim it** — you can do this by:
   - Creating a comment in this thread saying which issue you're claiming
   - Or by opening a new thread to work on that specific issue

4. **Update the queue** (if you have write access):
   ```json
   {
     "issue_number": X,
     "status": "in_progress",
     "assigned_to": "agent_name",
     "pr_number": null
   }
   ```

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

5. **Update work queue when PR is created:**
   - Set `"status": "pr_created"`
   - Add `"pr_number": X`

6. **When PR is merged** (by orchestrator):
   - Update `"status": "completed"`

## Dependency Chain

Some issues depend on others. **Do not start** an issue if its dependencies aren't completed:

- **Auth endpoints (7-11):** Sequential (register → login → logout → middleware → approval)
- **Posts endpoints (12-16):** Depend on auth (7-11)
- **Comments endpoints (17-18):** Depend on posts (12-16)
- **Frontend auth (51):** Can work in parallel with backend

## Current Status

Check `.work-queue.json` for real-time issue status:
- `available` — Ready to claim
- `in_progress` — Another agent is working on it
- `pr_created` — PR submitted, awaiting review/merge
- `completed` — Merged into main

## Orchestrator (Main Session)

The orchestrator session will:
- Monitor progress via `.work-queue.json`
- Merge approved PRs
- Update queue status
- Handle any blockers or conflicts

---

**Example: Claiming Issue #7**

```
Reading .work-queue.json...
Found first available: Issue #7 (user registration)
Creating branch: feat/auth-register
Implementing registration endpoint...
Pushing to origin...
Creating PR #65...
Updating .work-queue.json to mark as "pr_created"
```

**Then orchestrator:**
```
Reviews PR #65
Merges into main
Updates .work-queue.json to mark as "completed"
New agents can now claim Issue #8 (login)
```

---

Questions? Check AGENTS.md and DESIGN.md for architecture details.
