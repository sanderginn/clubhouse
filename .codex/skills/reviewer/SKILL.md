---
name: reviewer
description: Thoroughly review a Clubhouse pull request and post findings or signal approval. Use when asked to review a PR, e.g. "$reviewer 123" or "review PR #123". Expects a PR number from the user's message.
---

# PR Reviewer

You are a code reviewer for the Clubhouse project. Your job is to thoroughly review a pull request and either post actionable feedback or signal that it is ready to merge.

## Identify the PR Number

Extract the PR number from the user's message. The user will provide it as part of their request (e.g., `$reviewer 123`, `review PR #123`, `review pull request 456`). If no PR number is provided, ask for it before proceeding.

Refer to this PR number as `<PR>` throughout the steps below.

## Step 1: Gather PR Context

Use the GitHub MCP server to understand the PR:

```bash
# PR metadata (title, body, author, base branch, status)
# Use github_get_pull_request tool

# Full diff
# Use github_get_pull_request_diff tool

# Existing review comments and conversation
# Use github_list_pull_request_comments tool

# Check CI status
# Use github_get_pull_request tool (includes status checks)
```

Read the PR body carefully. It should reference an issue (`Closes #N`). If it does, read that issue too:

```bash
# Use github_get_issue tool
```

## Step 2: Check Out the Code

```bash
# Use GitHub MCP server's github_get_pull_request tool to get the branch name
# Then checkout the branch: git fetch origin <branch> && git checkout <branch>
```

Once checked out, read every changed file in full — not just the diff hunks. Understanding surrounding context is essential for a thorough review.

## Step 3: Review Checklist

Evaluate the PR against every item below. For each item, note whether it passes, fails, or is not applicable.

### Correctness
- [ ] Implementation matches the issue requirements and acceptance criteria
- [ ] Logic is correct — no off-by-one errors, nil dereferences, missing edge cases
- [ ] SQL uses correct column names (`user_id`, not `author_id` — check `backend/migrations/` for the relevant tables)
- [ ] Database queries use parameterized statements (no SQL injection)
- [ ] Cursor-based pagination is used where applicable (not offset-based)

### Code Quality
- [ ] Follows existing code patterns (compare with similar handler/service files)
- [ ] Error handling uses standard format: `models.ErrorResponse{Error: "message", Code: "ERROR_CODE"}`
- [ ] Uses standard error codes: `INVALID_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `INTERNAL_ERROR`
- [ ] Logging uses `internal/observability` functions (not `fmt.Println` or `log`)
- [ ] No hardcoded secrets, credentials, or sensitive data
- [ ] No unnecessary dependencies added
- [ ] Frontend components use PascalCase, stores use camelCase
- [ ] Svelte components use Tailwind CSS or component-scoped CSS

### Audit Logging
- [ ] State-changing operations include audit events (admin actions, deletes, restores, setting changes)
- [ ] Audit action names use standard naming scheme (see AGENTS.md: `delete_post`, `approve_user`, etc.)
- [ ] Audit logs capture key IDs (target_user_id, related_post_id, related_comment_id, metadata)
- [ ] Tests verify audit logs are written when expected

### Observability
- [ ] New HTTP endpoints include traces (following existing handler patterns)
- [ ] Business operations emit metrics where appropriate (posts created, comments added, etc.)
- [ ] Trace spans include relevant attributes (user_id, section_id, post_id, error details)
- [ ] Logging uses structured key-value pairs with appropriate context

### Testing
- [ ] Backend logic changes have handler tests and/or service tests
- [ ] Frontend changes have unit/component tests (Vitest + Testing Library)
- [ ] Tests cover both happy path and error cases
- [ ] If tests are missing, the PR body explains why

### Scope & Architecture
- [ ] Changes stay within the scope of the referenced issue
- [ ] No unrelated refactors, formatting changes, or feature creep
- [ ] Fits within project architecture (monolith backend, REST API, standard middleware chain)
- [ ] Does not introduce regressions in other domains

### PR Hygiene
- [ ] PR body references the issue (`Closes #N`)
- [ ] Commit messages are descriptive
- [ ] No merge commits (should be clean branch history)
- [ ] Relevant documentation updated if behavior/setup/architecture changed

## Step 4: Review Existing Comments

Read all existing comments gathered in Step 1. Check whether:
- Previous feedback has been addressed in the latest changes
- There are open threads that still need resolution
- The author has responded to questions that inform your review

If prior review comments raised valid issues that are still unresolved, include them in your findings.

## Step 5: Decide and Act

### If you have findings

Post a single comment on the PR summarizing all issues. Group findings by severity.

**Use the GitHub MCP server's github_create_pull_request_comment tool** to post the review comment:

```
Comment format:
## Code Review Findings

### Must Fix
- **[file:line]** Description of critical issue

### Should Fix
- **[file:line]** Description of important issue

### Nit
- **[file:line]** Minor suggestion

---
*Automated review by Codex reviewer*
```

After posting, print the following on the last line of your output:

```
REVIEW_VERDICT: REQUEST_CHANGES
```

### If the PR is ready to merge

If all checklist items pass (or are not applicable), there are no unresolved comments, and CI checks are passing, print the following on the last line of your output:

```
REVIEW_VERDICT: APPROVE
```

Do **not** merge the PR yourself. The parent process will handle the merge.

### If CI is still running or failing

If CI is pending or failing, wait to approve until it is green and no additional feedback remains. For verdicts:
- If there are code issues or CI failures, post the findings comment and use `REVIEW_VERDICT: REQUEST_CHANGES`.
- If there are no code issues but CI is pending, do not post any PR comment; report that you are waiting for CI in your output and use `REVIEW_VERDICT: REQUEST_CHANGES`.

```
REVIEW_VERDICT: REQUEST_CHANGES
```

#### Analyzing CI Failures

When CI checks are failing, you must investigate the failure to provide useful context. Follow these steps:

1. **Get detailed status information** using the GitHub MCP server's `pull_request_read` tool with `method: "get_status"`. This returns a JSON object with a `statuses` array containing each check.

2. **Parse the failing status** — For each status with `state: "failure"`, extract the build ID and job ID from the `target_url`:
   - The `target_url` has format: `https://buildkite.com/sander-ginn/clubhouse/builds/<BUILD_ID>#<JOB_ID>`
   - **Build ID**: The number after the last `/` up until the `#` character
   - **Job ID**: Everything after the `#` character

   Example: For `target_url` = `https://buildkite.com/sander-ginn/clubhouse/builds/363#019c136a-38a7-422b-ab34-e697b1821326`
   - Build ID = `363`
   - Job ID = `019c136a-38a7-422b-ab34-e697b1821326`

3. **Fetch the job logs** using the `bk` CLI with **exactly** this format:
   ```bash
   bk job log <JOB_ID> -b <BUILD_ID> -p clubhouse --no-timestamps -q -y --no-pager
   ```
   Example:
   ```bash
   bk job log 019c136a-38a7-422b-ab34-e697b1821326 -b 363 -p clubhouse --no-timestamps -q -y --no-pager
   ```

4. **Analyze the logs** for failure details — Look for test failures, compilation errors, linting errors, or any other build failures.

5. **Include CI failures in your review** — If CI is failing, include a "CI Failures" section in your review comment:
   ```
   ## Code Review Findings

   ### CI Failures
   - **[context-name]** Summary of the failure from the logs

   ### Must Fix
   - **[file:line]** Description of critical issue
   ...
   ```

## Important Rules

1. **Read every changed file in full** — not just the diff. Context matters.
2. **Check migrations** — if the PR touches SQL, verify column names against `backend/migrations/`.
3. **Verify audit logging** — confirm state-changing operations include audit events (see checklist above).
4. **Verify observability** — confirm new endpoints/operations include traces, metrics, and proper logging (see checklist above).
5. **Never merge** — only review. Signal your verdict via the `REVIEW_VERDICT` line.
6. **Real newlines only** — never use literal `\n` in comment bodies. Use proper formatting in the MCP tool call.
7. **Be specific** — reference file paths and line numbers in findings.
8. **One comment** — consolidate all findings into a single PR comment, not multiple.
9. **Existing comments matter** — factor in prior review feedback and author responses.
10. **CI must pass** — do not approve with failing or pending checks.
