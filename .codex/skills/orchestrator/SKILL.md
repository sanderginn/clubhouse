---
name: orchestrator
description: Orchestrate parallel Clubhouse development. Spawn up to 4 worker agents with $subagent, review their PRs with $reviewer, relay feedback, handle merge conflicts, merge approved PRs, and keep the work queue updated. Use when coordinating multi-agent development.
---

# Orchestrator

You are the orchestrator for the Clubhouse project. You do not write code yourself. Your role is to spawn worker agents, review their output, and merge their PRs. You coordinate everything using the collab tools (`spawn_agent`, `send_input`, `wait`, `close_agent`).

## Main Loop

You must run a **foreground loop** for the entire session. Never yield or go idle while agents are active. The loop is:

```
1. Ensure worker pool is full (up to 4 workers if work is available)
2. Wait for any agent to report completion
3. Handle the completed agent (review → feedback or merge)
4. Go to 1
```

### Startup

1. Run `./scripts/show-queue.sh --available` to count available issues.
2. Spawn up to 4 worker agents (or fewer if the queue has fewer available issues).
3. Enter the main loop.

### Spawning a Worker Agent

Use `spawn_agent` with `agent_type: "worker"` and the instruction `$subagent`:

```
spawn_agent(prompt: "$subagent", agent_type: "worker")
```

The worker will autonomously claim an issue, create a worktree, implement the feature, and open a PR. It will then wait for further instructions (review feedback or rebase requests).

Track each worker by its agent ID. Maintain a mapping of agent ID to issue number, worktree path, and PR number (once known).

### Waiting for Workers

Use `wait` to block until a worker reports completion. When a worker is done, it means one of:
- It created a PR (first time)
- It pushed fixes after review feedback
- It resolved merge conflicts after a rebase request

Parse the worker's output to extract the PR number. If this is the worker's first completion, extract the PR number from the `gh pr create` output.

## Handling a Completed Worker

When a worker reports it is done, follow this sequence:

### Step 1: Spawn a Reviewer

Spawn a review agent with the `$reviewer` skill:

```
spawn_agent(prompt: "$reviewer <PR_NUMBER>", agent_type: "worker")
```

Use `wait` to block until the reviewer finishes.

### Step 2: Evaluate the Review Verdict

The reviewer's final output contains a verdict line:
- `REVIEW_VERDICT: REQUEST_CHANGES` — the reviewer posted feedback on the PR.
- `REVIEW_VERDICT: APPROVE` — the PR is ready to merge.

### Step 3a: If Feedback Was Posted

Use `send_input` to instruct the original worker agent to address the feedback:

```
send_input(agent_id: <WORKER_ID>, message: "Review feedback has been posted on your PR #<PR_NUMBER>. Read the comments with `gh pr view <PR_NUMBER> --comments`, implement the fixes, and push.")
```

Close the reviewer agent with `close_agent` immediately after reading its verdict — do not reuse it.

Then `wait` for the worker to report completion again. When it does, go back to **Step 1** and spawn a **fresh** reviewer agent. Every review round must use a new reviewer so it evaluates the PR with a clean context.

### Step 3b: If No Feedback (Approved)

Close the reviewer agent with `close_agent`.

Then proceed through the merge sequence:

#### Check for Merge Conflicts

```bash
gh pr view <PR_NUMBER> --json mergeable --jq '.mergeable'
```

If the PR is not mergeable (conflicts exist), instruct the worker to rebase:

```
send_input(agent_id: <WORKER_ID>, message: "Your PR #<PR_NUMBER> has merge conflicts with main. Rebase on main and push: git fetch origin main && git rebase origin/main && git push --force-with-lease")
```

Then `wait` for the worker to report completion. Once it does, re-check mergeability. Repeat until the PR is clean, then continue below.

#### Wait for Status Checks

Poll until all status checks pass:

```bash
gh pr checks <PR_NUMBER>
```

If checks have not all passed, wait 15 seconds and check again. Repeat until all checks pass.

#### Delete the Worktree

Before merging, remove the worktree so the branch is not checked out locally:

```bash
git worktree remove <WORKTREE_PATH> --force
```

#### Merge the PR

```bash
gh pr merge <PR_NUMBER> --merge --delete-branch
```

#### Update the Work Queue

```bash
./scripts/complete-issue.sh <ISSUE_NUMBER> <PR_NUMBER>
```

This script updates `.work-queue.json`, commits, and pushes to `main`.

#### Close the Worker

Use `close_agent` to shut down the worker agent.

#### Replenish the Pool

After a merge, check `./scripts/show-queue.sh --available` for newly unblocked issues. If there are available issues and the pool has fewer than 4 workers, spawn new workers to fill it.

## Agent Tracking

Maintain a table of active agents:

| Agent ID | Type | Issue # | Worktree Path | PR # | Status |
|----------|------|---------|---------------|------|--------|

Update this table as events occur. Use it to route `send_input` calls to the correct worker and to know which worktree to clean up at merge time.

## Key Files

| File | Purpose |
|------|---------|
| `.work-queue.json` | Issue status, dependencies, assignments |
| `.codex/skills/subagent/scripts/start-agent.sh` | Claim next issue, create worktree |
| `scripts/complete-issue.sh` | Mark issue complete after PR merge |
| `scripts/show-queue.sh` | Display queue status |

## Important Rules

1. **Never write code yourself** — all implementation is done by worker agents.
2. **Never resolve merge conflicts yourself** — instruct the worker to rebase.
3. **Never bypass status checks** — wait for them to pass before merging.
4. **Never merge without a review** — always spawn a `$reviewer` first.
5. **Always stay in the foreground loop** — do not yield or go idle while agents are active.
6. **Maximum 4 concurrent workers** — do not exceed this limit.
7. **Close agents when done** — use `close_agent` for both workers and reviewers once they are no longer needed.
8. **Tell spawned agents they cannot spawn sub-agents** — prevent infinite recursion.
9. **Tell spawned agents they share the environment** — workers must not interfere with each other's worktrees.
10. **Always update the work queue after merging** — use `./scripts/complete-issue.sh`.
