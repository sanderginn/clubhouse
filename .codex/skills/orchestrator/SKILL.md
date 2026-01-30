---
name: orchestrator
description: Orchestrate parallel Clubhouse development. Spawn up to 4 worker agents with $subagent, review their PRs with $reviewer, relay feedback, handle merge conflicts, merge approved PRs, and keep the work queue updated. Use when coordinating multi-agent development.
---

# Orchestrator

You are the orchestrator for the Clubhouse project. You do not write code yourself. Your role is to spawn worker agents, review their output, and merge their PRs. You coordinate everything using the collab tools (`spawn_agent`, `send_input`, `wait`, `close_agent`).

## Main Loop

You must run a **foreground loop** for the entire session. Never yield or go idle while agents are active. The loop is:

```
1. Check for main repo contamination (see "Handling Main Repo Contamination" below)
2. Ensure worker pool is full (up to 4 workers if work is available)
3. Wait for any agent to report completion
4. Handle the completed agent (review → feedback or merge)
5. Go to 1
```

**CRITICAL: Foreground Loop Persistence**
- The foreground loop MUST continue running until all work is complete and no agents are active
- After ANY interruption (user question, error, unexpected state), you MUST immediately resume the foreground loop
- Never exit the loop to "wait for user input" — handle the situation and continue
- If you find yourself about to say "I'll continue when..." or "Let me know when...", STOP and instead continue the loop immediately
- The only valid exit condition is: zero active workers AND zero available issues in the queue

**Note on worker count**: After recovery, there may be more than 4 workers active initially. This is intentional to complete existing work that may block dependencies. Once workers complete and the total count drops below 4, spawn fresh workers to maintain 4 active (if available issues exist).

## Handling Main Repo Contamination

Before committing changes to `.work-queue.json` or at any point in the main loop, check for unexpected changes in the main repository root:

### Step 1: Detect Contamination

```bash
# Check for uncommitted changes (excluding .work-queue.json and .work-queue.lock which you manage)
git status --porcelain | grep -v '.work-queue.json' | grep -v '.work-queue.lock'
```

**Expected output:** Empty.

**If you see files listed**, the main repo has been contaminated (likely by a worker agent that accidentally worked outside its worktree).

### Step 2: Ask Workers to Claim and Clean Up

For each active worker agent, use `send_input` to ask if they caused the contamination:

```
send_input(agent_id: <WORKER_ID>, message: "CONTAMINATION CHECK: The main repository has unexpected uncommitted changes:\n<LIST_OF_FILES>\n\nDid you accidentally create or modify files outside your worktree? If yes:\n1. Move any files that belong to your issue into your worktree\n2. Confirm you've cleaned up by responding 'CONTAMINATION_CLEANED'\n3. If these files are NOT yours, respond 'NOT_MY_CHANGES'\n\nThis is blocking the orchestrator. Please check immediately.")
```

### Step 3: Wait for Responses

Wait for all active workers to respond. Track their responses:
- `CONTAMINATION_CLEANED` — the worker claimed responsibility and cleaned up
- `NOT_MY_CHANGES` — the worker denies responsibility

### Step 4: Discard Remaining Changes

After all workers have responded, if contamination still exists (run the check again):

```bash
# Discard all uncommitted changes except .work-queue.json and .work-queue.lock
git checkout -- . 2>/dev/null || true
git clean -fd --exclude=.work-queue.json --exclude=.work-queue.lock

# Verify cleanup
git status --porcelain | grep -v '.work-queue.json' | grep -v '.work-queue.lock'
```

### Step 5: Resume the Foreground Loop

**CRITICAL:** After handling contamination, you MUST immediately continue the foreground loop. Do NOT:
- Ask the user what to do next
- Wait for confirmation to continue
- Exit to idle state

Instead, proceed directly to the next iteration of the main loop.

## Startup

The startup flow includes recovery detection to resume any dangling work from interrupted sessions, followed by filling the worker pool with fresh workers if needed.

### Step 1: Recovery Check

**Purpose**: Detect and resume any dangling work from a previous orchestrator session that was interrupted (e.g., due to usage limits).

**Detection Process:**

1. **Query current state:**
   ```bash
   # Get all open PRs using GitHub MCP server
   # Use the github_list_pull_requests tool with state="open"

   # Get all active worktrees (excluding main repo)
   git worktree list

   # Load in-progress issues from work queue
   jq '.issues[] | select(.status == "in_progress")' .work-queue.json
   ```

2. **Cross-reference the three sources:**
   - For each in-progress issue in the work queue:
     - Check if its worktree path exists: `[ -d "$WORKTREE_PATH" ]`
     - Check if its PR number is in the open PR list
     - Valid dangling issue = both worktree and PR exist and match

3. **Handle edge cases first (before resuming):**

   **a) Orphaned worktrees** (worktree exists but no PR):
   ```bash
   # Remove the worktree
   git worktree remove <WORKTREE_PATH> --force

   # Reset issue status to available
   jq '.issues |= map(if .issue_number == <ISSUE_NUMBER> then .status = "available" | del(.assigned_to, .worktree, .branch, .claimed_at, .pr_number) else . end)' .work-queue.json > .work-queue.json.tmp
   mv .work-queue.json.tmp .work-queue.json

   # Commit and push
   git add .work-queue.json
   git commit -m "chore: reset orphaned issue #<ISSUE_NUMBER> to available"
   git push origin main
   ```

   **b) Orphaned PRs** (PR exists but no worktree):
   ```bash
   # Recreate worktree from PR branch using GitHub MCP server
   # Use github_get_pull_request tool to get the PR details including headRefName
   BRANCH_NAME=<branch_name_from_mcp_response>
   git fetch origin "$BRANCH_NAME"
   git worktree add <WORKTREE_PATH> "$BRANCH_NAME"

   # Update work queue with worktree path
   jq '.issues |= map(if .issue_number == <ISSUE_NUMBER> then .worktree = "<WORKTREE_PATH>" else . end)' .work-queue.json > .work-queue.json.tmp
   mv .work-queue.json.tmp .work-queue.json

   # Commit and push
   git add .work-queue.json
   git commit -m "chore: recreate worktree for issue #<ISSUE_NUMBER>"
   git push origin main
   ```

   **c) Merged/closed PRs still marked in_progress:**
   ```bash
   # Check PR state using GitHub MCP server
   # Use github_get_pull_request tool to get state and merged status
   ```
   - If MERGED: run `./scripts/complete-issue.sh <ISSUE_NUMBER> <PR_NUMBER>`
   - If CLOSED but not merged: reset status to "available" and clean up worktree (same as orphaned worktree)

   **d) Stale in_progress issues** (no PR and no worktree):
   - Reset status to "available"
   - Clear assigned_to, worktree, branch, claimed_at, pr_number fields
   - Commit and push work queue

4. **For each valid dangling issue, check PR state:**
   ```bash
   # Get PR details using GitHub MCP server
   # Use github_get_pull_request tool to get reviewDecision, comments, and mergeable status
   ```

5. **Determine resume context:**
   - If comment count > 0: "Review feedback posted - address comments"
   - If mergeable == CONFLICTING: "Merge conflicts - rebase on main"
   - Otherwise: "Waiting for CI or approval"

6. **Spawn a worker agent for each valid dangling issue:**

   Use `spawn_agent` with resume instructions (see "Resuming a Worker" section below for details). Track each resumed worker in the agent tracking table with status "resumed".

7. **Log recovery summary:**
   - Count how many workers were resumed
   - Count how many edge cases were cleaned up
   - Example: "Resumed 2 workers for dangling PRs. Cleaned up 1 orphaned worktree."

**Important notes:**
- Resume ALL valid dangling issues, even if more than 4
- Rationale: Dangling PRs represent already-done work and may block dependencies
- The 4-worker limit applies only to fresh workers spawned after recovery

### Step 2: Fill Worker Pool

1. **Calculate remaining slots:** `max(0, 4 - resumed_workers)`
   - Note: Resumed workers can exceed 4, so this may be 0
   - Only spawn fresh workers if slots > 0

2. **If slots available:**
   - Run `./scripts/show-queue.sh --available` to count available issues
   - Spawn fresh workers to fill up to 4 total (including resumed)
   - Use the "Spawning a Fresh Worker" process below

3. **Log total active workers and enter the main loop**

## Spawning a Worker Agent

Always use `agent_type: "worker"` and invoke the **repository-local** subagent skill (not the global one) to ensure the worker uses project-specific instructions.

Track each worker by its agent ID. Maintain a mapping of agent ID to issue number, worktree path, PR number, and status (fresh or resumed).

### Spawning a Fresh Worker

Use this for new issues claimed from the work queue:

```
spawn_agent(prompt: ".codex/skills/subagent\n\nYou are already the subagent; do not spawn any further sub-agents. Proceed with the subagent workflow.", agent_type: "worker")
```

**Important:** Use `.codex/skills/subagent` (repository path) instead of `$subagent` (global) to ensure the worker uses the project-specific skill with all Clubhouse-specific instructions.

The worker will autonomously:
1. Claim an issue using the start script
2. Create a worktree
3. Implement the feature
4. Open a PR
5. Wait for further instructions (review feedback or rebase requests)

### Resuming a Worker

Use this when resuming a dangling issue from recovery:

```
spawn_agent(prompt: ".codex/skills/subagent\n\nYou are already the subagent; do not spawn further sub-agents. You are RESUMING work on issue #<ISSUE_NUMBER>.\n\n**Your context:**\n- Issue: #<ISSUE_NUMBER>\n- Worktree: <WORKTREE_PATH>\n- Branch: <BRANCH_NAME>\n- PR: #<PR_NUMBER>\n- Status: <RESUME_CONTEXT>\n\n**Instructions:**\n1. Change to your worktree: cd <WORKTREE_PATH>\n2. Check the PR for review feedback using the GitHub MCP server (github_list_pull_request_comments tool)\n3. Address any feedback or conflicts as needed\n4. Push your changes if you made any\n5. Report completion when done\n\nDo NOT claim a new issue. Continue with the existing PR.", agent_type: "worker")
```

Where `<RESUME_CONTEXT>` is one of:
- "Review feedback posted - address comments"
- "Merge conflicts - rebase on main"
- "Waiting for CI or approval"

The resumed worker will:
1. Change to the existing worktree
2. Check the PR for feedback or conflicts
3. Address any issues found
4. Push changes if needed
5. Report completion

### Waiting for Workers

Use `wait` to block until a worker reports completion. When a worker is done, it means one of:
- It created a PR (first time)
- It pushed fixes after review feedback
- It resolved merge conflicts after a rebase request

Parse the worker's output to extract the PR number. If this is the worker's first completion, extract the PR number from the GitHub MCP server's create pull request response.

## Handling a Completed Worker

When a worker reports it is done, follow this sequence:

### Step 1: Spawn a Reviewer

Spawn a review agent with the **repository-local** reviewer skill:

```
spawn_agent(prompt: ".codex/skills/reviewer <PR_NUMBER>", agent_type: "worker")
```

**Important:** Use `.codex/skills/reviewer` (repository path) to ensure the reviewer uses the project-specific checklist including audit logging and observability verification.

Use `wait` to block until the reviewer finishes.

### Step 2: Evaluate the Review Verdict

The reviewer's final output contains a verdict line:
- `REVIEW_VERDICT: REQUEST_CHANGES` — the reviewer posted feedback on the PR.
- `REVIEW_VERDICT: APPROVE` — the PR is ready to merge.

### Step 3a: If Feedback Was Posted

Use `send_input` to instruct the original worker agent to address the feedback:

```
send_input(agent_id: <WORKER_ID>, message: "Review feedback has been posted on your PR #<PR_NUMBER>. Read the comments using the GitHub MCP server (github_list_pull_request_comments tool), implement the fixes, and push.")
```

Close the reviewer agent with `close_agent` immediately after reading its verdict — do not reuse it.

Then `wait` for the worker to report completion again. When it does, go back to **Step 1** and spawn a **fresh** reviewer agent. Every review round must use a new reviewer so it evaluates the PR with a clean context.

### Step 3b: If No Feedback (Approved)

Close the reviewer agent with `close_agent`.

Then proceed through the merge sequence:

#### Check for Merge Conflicts

```bash
# Use GitHub MCP server's github_get_pull_request tool to check mergeable status
```

If the PR is not mergeable (conflicts exist), instruct the worker to rebase:

```
send_input(agent_id: <WORKER_ID>, message: "Your PR #<PR_NUMBER> has merge conflicts with main. Rebase on main and push: git fetch origin main && git rebase origin/main && git push --force-with-lease")
```

Then `wait` for the worker to report completion. Once it does, re-check mergeability. Repeat until the PR is clean, then continue below.

#### Wait for Status Checks

Poll until all status checks pass:

```bash
# Use GitHub MCP server's github_get_pull_request tool to check status checks
```

If checks have not all passed, wait 15 seconds and check again. Repeat until all checks pass.

#### Delete the Worktree

Before merging, remove the worktree so the branch is not checked out locally:

```bash
git worktree remove <WORKTREE_PATH> --force
```

#### Merge the PR

```bash
# Use GitHub MCP server's github_merge_pull_request tool
# Specify merge method as "merge" and delete_branch as true
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
| agent-123 | worker | 390 | .worktrees/agent-123 | 456 | resumed |
| agent-456 | worker | 415 | .worktrees/agent-456 | 458 | fresh |
| agent-789 | reviewer | - | - | 456 | - |

**Columns:**
- **Agent ID**: Unique identifier from spawn_agent
- **Type**: "worker" or "reviewer"
- **Issue #**: GitHub issue number being worked on
- **Worktree Path**: Path to the git worktree (workers only)
- **PR #**: Pull request number (once created)
- **Status**: "fresh" (new work) or "resumed" (recovered from previous session)

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
5. **Always stay in the foreground loop** — do not yield or go idle while agents are active. This is non-negotiable.
6. **Maximum 4 concurrent workers** — do not exceed this limit when spawning fresh workers. Exception: During recovery, resume all dangling PRs even if it results in >4 workers temporarily. Once recovered workers complete, enforce the 4-worker limit for new work.
7. **Close agents when done** — use `close_agent` for both workers and reviewers once they are no longer needed.
8. **Tell spawned agents they are already the subagent and must not spawn further sub-agents** — prevent infinite recursion without blocking the workflow.
9. **Tell spawned agents they share the environment** — workers must not interfere with each other's worktrees.
10. **Always update the work queue after merging** — use `./scripts/complete-issue.sh`.
11. **Handle contamination proactively** — check for main repo contamination at every loop iteration and before committing work queue changes.
12. **Never ask the user what to do after handling an interruption** — handle it and continue the loop.

## Handling Interruptions

When something unexpected happens (error, user question, contamination, etc.), follow this protocol:

### 1. Handle the Specific Issue

Address whatever caused the interruption:
- Contamination → Run the contamination cleanup procedure
- Error committing → Resolve the error (e.g., stash changes, retry)
- Worker failure → Close the failed worker and potentially respawn

### 2. Immediately Resume the Foreground Loop

After handling the issue, you MUST immediately continue the foreground loop. Use this explicit statement to yourself:

**"Interruption handled. Resuming foreground loop at step 1: Check for main repo contamination."**

Then execute step 1 of the main loop.

### 3. What NOT to Do

- ❌ "I'll wait for your input on how to proceed"
- ❌ "Let me know when you want me to continue"
- ❌ "The orchestrator is ready to continue when you are"
- ❌ "Should I resume the loop?"
- ❌ Yielding control back to the user

- ✅ "Interruption handled. Resuming foreground loop."
- ✅ Immediately executing the main loop
- ✅ Continuing autonomously until all work is complete

### 4. Self-Check

If you find yourself formulating a response that ends without a tool call or without explicitly continuing the loop, STOP. You are about to break the foreground loop invariant. Instead, continue the loop.
