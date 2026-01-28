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

Run these commands to understand the PR:

```bash
# PR metadata (title, body, author, base branch, status)
gh pr view <PR>

# Full diff
gh pr diff <PR>

# Existing review comments and conversation
gh api repos/{owner}/{repo}/pulls/<PR>/comments --paginate
gh api repos/{owner}/{repo}/issues/<PR>/comments --paginate

# Check CI status
gh pr checks <PR>
```

Read the PR body carefully. It should reference an issue (`Closes #N`). If it does, read that issue too:

```bash
gh issue view <ISSUE_NUMBER>
```

## Step 2: Check Out the Code

```bash
gh pr checkout <PR>
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

**Use `gh pr comment` with a heredoc** to ensure real newlines (never use literal `\n`):

```bash
gh pr comment <PR> --body "$(cat <<'EOF'
## Code Review Findings

### Must Fix
- **[file:line]** Description of critical issue

### Should Fix
- **[file:line]** Description of important issue

### Nit
- **[file:line]** Minor suggestion

---
*Automated review by Codex reviewer*
EOF
)"
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

## Important Rules

1. **Read every changed file in full** — not just the diff. Context matters.
2. **Check migrations** — if the PR touches SQL, verify column names against `backend/migrations/`.
3. **Never merge** — only review. Signal your verdict via the `REVIEW_VERDICT` line.
4. **Real newlines only** — never use literal `\n` in `gh` comment bodies. Always use heredocs.
5. **Be specific** — reference file paths and line numbers in findings.
6. **One comment** — consolidate all findings into a single PR comment, not multiple.
7. **Existing comments matter** — factor in prior review feedback and author responses.
8. **CI must pass** — if checks are failing or still running, note it. Do not approve with failing checks.
