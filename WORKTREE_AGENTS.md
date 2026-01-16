# Parallel Agent Worktrees

Git worktrees have been created for all available issues. Each agent gets an isolated working directory from the main repository.

## Agent Commands

Run each command in a separate terminal to start work on an issue:

### Agent 1 - Issue #7: User Registration
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-1' && git status
```

### Agent 2 - Issue #8: User Login
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-2' && git status
```

### Agent 3 - Issue #9: User Logout
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-3' && git status
```

### Agent 4 - Issue #10: Auth Middleware
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-4' && git status
```

### Agent 5 - Issue #11: Admin Approval Flow
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-5' && git status
```

### Agent 6 - Issue #12: Create Post Endpoint
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-6' && git status
```

### Agent 7 - Issue #13: Get Post Endpoint
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-7' && git status
```

### Agent 8 - Issue #14: Get Feed Endpoint
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-8' && git status
```

### Agent 9 - Issue #15: Soft Delete Post Endpoint
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-9' && git status
```

### Agent 10 - Issue #16: Restore Post Endpoint
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-10' && git status
```

### Agent 11 - Issue #17: Create Comment Endpoint
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-11' && git status
```

### Agent 12 - Issue #18: Get Thread Endpoint
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-12' && git status
```

### Agent 13 - Issue #51: Frontend Auth Pages
```bash
cd '/Users/sanderginn/repos/github/sanderginn/clubhouse/.worktrees/agent-13' && git status
```

## Workflow for Each Agent

1. **Enter worktree directory** (from command above)
2. **Implement the feature** following AGENTS.md guidelines
3. **Commit code:**
   ```bash
   git add .
   git commit -m "feat: description of what you implemented"
   ```
4. **Push branch:**
   ```bash
   git push -u origin <branch_name>
   ```
5. **Create PR:**
   ```bash
   gh pr create --title "feat: ..." --body "Closes #<issue_number>"
   ```
6. **Wait for orchestrator** to review and merge

## Important Notes

- **Do not delete worktrees manually** — orchestrator will clean up after merging
- **All worktrees share the same `.git` directory** — changes to main affect all worktrees
- **Dependencies matter** — some issues depend on others being completed first:
  - Auth endpoints (#7-11) must be sequential
  - Posts endpoints (#12-16) depend on auth (#7-11)
  - Comments endpoints (#17-18) depend on posts (#12-16)
  - Frontend auth (#51) can work in parallel with backend
- **Status tracked in `.work-queue.json`** — orchestrator uses this to manage merges

## Viewing Active Issues

```bash
cd /Users/sanderginn/repos/github/sanderginn/clubhouse
jq '.issues[] | select(.status == "in_progress") | {number: .issue_number, title: .title, agent: .assigned_to}' .work-queue.json
```

## Cleaning Up Worktrees

After all work is merged, clean up worktrees:

```bash
git worktree list
git worktree remove .worktrees/agent-<N>
```

Or remove all at once:

```bash
rm -rf .worktrees
```
