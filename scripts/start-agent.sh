#!/bin/bash

# start-agent.sh - Start a subagent that claims and works on the next issue
#
# Priority rules:
# 1) Prefer BUG issues first (labels include "bug" OR phase starts with "bug-" OR title starts with "Bug:")
# 2) Among bugs: lowest .priority wins (1 is most critical), then phase severity (critical > high > medium > low)
# 3) Then lowest issue_number
# 4) If no bugs are available, fall back to next available issue with satisfied dependencies
#
# This script:
# 1. Finds the next best available issue whose dependencies are all complete
# 2. Atomically claims it (updates work queue with lock file)
# 3. Creates or reuses a git worktree for the agent
# 4. Outputs instructions for the Amp subagent to continue
#
# Usage: ./scripts/start-agent.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_QUEUE="$REPO_ROOT/.work-queue.json"
LOCK_FILE="$REPO_ROOT/.work-queue.lock"
WORKTREES_DIR="$REPO_ROOT/.worktrees"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}ℹ${NC} $1"; }
log_success() { echo -e "${GREEN}✓${NC} $1"; }
log_warning() { echo -e "${YELLOW}⚠${NC} $1"; }
log_error() { echo -e "${RED}✗${NC} $1"; }

cleanup() {
    # Only remove the lock if we own it
    if [[ -f "$LOCK_FILE" ]]; then
        local owner
        owner="$(cat "$LOCK_FILE" 2>/dev/null || true)"
        if [[ "$owner" == "$$" ]]; then
            rm -f "$LOCK_FILE"
        fi
    fi
}
trap cleanup EXIT

acquire_lock() {
    local timeout=30
    local waited=0

    while [[ -f "$LOCK_FILE" ]]; do
        if [[ $waited -ge $timeout ]]; then
            log_error "Timeout waiting for lock. Another agent may be stuck."
            log_info "Remove $LOCK_FILE manually if needed."
            exit 1
        fi
        sleep 1
        waited=$((waited + 1))
    done

    echo "$$" > "$LOCK_FILE"
}

# Get list of completed issue numbers (from both completed_issues and issues with status=completed)
get_completed_issues() {
    local completed_array
    completed_array="$(jq -r '.completed_issues // [] | .[]' "$WORK_QUEUE" 2>/dev/null || true)"
    local completed_status
    completed_status="$(jq -r '.issues[] | select(.status == "completed") | .issue_number' "$WORK_QUEUE" 2>/dev/null || true)"

    printf "%s\n" "$completed_array"
    printf "%s\n" "$completed_status"
}

dependencies_satisfied() {
    local issue_number="$1"
    local deps
    deps="$(jq -r ".issues[] | select(.issue_number == $issue_number) | .dependencies // [] | .[]" "$WORK_QUEUE" 2>/dev/null || true)"

    if [[ -z "${deps:-}" ]]; then
        return 0
    fi

    local completed
    completed="$(get_completed_issues)"

    local dep
    for dep in $deps; do
        if ! echo "$completed" | grep -q "^${dep}$"; then
            return 1
        fi
    done
    return 0
}

is_bug_issue() {
    local issue_number="$1"
    jq -e "
      .issues[]
      | select(.issue_number == $issue_number)
      | (
          ((.labels // []) | index(\"bug\") != null)
          or ((.phase // \"\") | tostring | startswith(\"bug-\"))
          or ((.title // \"\") | tostring | startswith(\"Bug:\"))
        )
    " "$WORK_QUEUE" >/dev/null 2>&1
}

# Build a sorted candidate list:
# Columns: bug_group(0 bugs first, 1 non-bugs), severity_rank, priority, issue_number
# severity_rank: critical=0, high=1, medium=2, low=3, else=9
candidate_issue_numbers_sorted() {
    jq -r '
      .issues[]
      | select(.status == "available")
      | . as $i
      | ($i.phase // "" | tostring) as $phase
      | ($i.title // "" | tostring) as $title
      | (($i.labels // []) | index("bug") != null) as $hasBugLabel
      | ($phase | startswith("bug-")) as $bugPhase
      | ($title | startswith("Bug:")) as $bugTitle
      | ($hasBugLabel or $bugPhase or $bugTitle) as $isBug
      | (if $phase == "bug-critical" then 0
         elif $phase == "bug-high" then 1
         elif $phase == "bug-medium" then 2
         elif $phase == "bug-low" then 3
         else 9 end) as $sev
      | ($i.priority // 999) as $prio
      | [(if $isBug then 0 else 1 end), $sev, $prio, $i.issue_number] | @tsv
    ' "$WORK_QUEUE" \
    | sort -n -k1,1 -k2,2 -k3,3 -k4,4 \
    | awk -F'\t' '{print $4}'
}

find_next_issue() {
    local issue_number

    while IFS= read -r issue_number; do
        [[ -z "$issue_number" ]] && continue
        if dependencies_satisfied "$issue_number"; then
            echo "$issue_number"
            return 0
        fi
    done < <(candidate_issue_numbers_sorted)

    return 1
}

generate_agent_id() {
    echo "agent-$(date +%s)-$$"
}

create_branch_name() {
    local issue_number="$1"
    local title="$2"

    local prefix="feat"
    if is_bug_issue "$issue_number"; then
        prefix="fix"
    fi

    # Convert title to kebab-case, remove special chars
    local slug
    slug="$(echo "$title" \
        | sed 's/^Bug:[[:space:]]*//; s/^Phase [0-9]*:[[:space:]]*//' \
        | tr '[:upper:]' '[:lower:]' \
        | sed 's/[^a-z0-9 ]//g' \
        | sed 's/  */ /g' \
        | sed 's/ /-/g' \
        | cut -c1-40)"

    echo "${prefix}/issue-${issue_number}-${slug}"
}

main() {
    log_info "Starting Clubhouse Agent..."

    cd "$REPO_ROOT"

    log_info "Syncing with main branch..."
    git checkout main >/dev/null 2>&1 || true
    git pull origin main >/dev/null 2>&1 || true

    log_info "Acquiring work queue lock..."
    acquire_lock

    log_info "Finding next issue (bugs prioritized)..."
    ISSUE_NUMBER="$(find_next_issue || true)"

    if [[ -z "${ISSUE_NUMBER:-}" ]]; then
        log_warning "No available issues with satisfied dependencies."
        echo ""
        log_info "Blocked available issues and their missing dependencies:"
        jq -r '.issues[]
          | select(.status == "available")
          | "  #\(.issue_number): \(.title)\n    Waiting for: \((.dependencies // []) | map("#\(.)") | join(", "))"
        ' "$WORK_QUEUE"
        exit 0
    fi

    ISSUE_TITLE="$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | .title" "$WORK_QUEUE")"
    ISSUE_PHASE="$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | .phase" "$WORK_QUEUE")"
    ISSUE_LABELS="$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | (.labels // []) | join(\", \")" "$WORK_QUEUE")"
    ISSUE_PRIORITY="$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | (.priority // \"\")" "$WORK_QUEUE")"

    if is_bug_issue "$ISSUE_NUMBER"; then
        log_success "Found BUG issue #$ISSUE_NUMBER (priority ${ISSUE_PRIORITY:-n/a}): $ISSUE_TITLE"
    else
        log_success "Found issue #$ISSUE_NUMBER: $ISSUE_TITLE"
    fi

    AGENT_ID="$(generate_agent_id)"
    BRANCH_NAME="$(create_branch_name "$ISSUE_NUMBER" "$ISSUE_TITLE")"
    WORKTREE_PATH="$WORKTREES_DIR/$AGENT_ID"

    log_info "Claiming issue #$ISSUE_NUMBER..."
    jq ".issues |= map(if .issue_number == $ISSUE_NUMBER then . + {
          \"status\": \"in_progress\",
          \"assigned_to\": \"$AGENT_ID\",
          \"worktree\": \"$WORKTREE_PATH\",
          \"branch\": \"$BRANCH_NAME\",
          \"claimed_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
        } else . end)" "$WORK_QUEUE" > "$WORK_QUEUE.tmp"
    mv "$WORK_QUEUE.tmp" "$WORK_QUEUE"

    git add "$WORK_QUEUE"
    git commit -m "chore: agent $AGENT_ID claims issue #$ISSUE_NUMBER" >/dev/null 2>&1 || true

    if ! git push origin main >/dev/null 2>&1; then
        log_warning "Push conflict detected, pulling and retrying..."
        git pull --rebase origin main >/dev/null 2>&1 || true
        git push origin main >/dev/null 2>&1 || true
    fi

    log_info "Creating worktree at $WORKTREE_PATH..."
    mkdir -p "$WORKTREES_DIR"

    if git ls-remote --heads origin "$BRANCH_NAME" | grep -q "$BRANCH_NAME"; then
        log_info "Branch $BRANCH_NAME exists, checking it out..."
        git worktree add "$WORKTREE_PATH" "$BRANCH_NAME" >/dev/null 2>&1 || \
            git worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" "origin/$BRANCH_NAME" >/dev/null 2>&1 || \
            git worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" main >/dev/null 2>&1
    else
        git worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" main >/dev/null 2>&1
    fi

    log_success "Worktree created at $WORKTREE_PATH"

    # Explicitly release lock early
    rm -f "$LOCK_FILE" || true

    log_info "Fetching issue details from GitHub..."
    ISSUE_BODY="$(gh issue view "$ISSUE_NUMBER" --json body --jq '.body' 2>/dev/null || echo "Could not fetch issue body")"

    echo ""
    echo "════════════════════════════════════════════════════════════════════════════"
    echo "                    CLUBHOUSE AGENT READY: $AGENT_ID"
    echo "════════════════════════════════════════════════════════════════════════════"
    echo ""
    echo "📋 ISSUE ASSIGNMENT"
    echo "   Issue:    #$ISSUE_NUMBER"
    echo "   Title:    $ISSUE_TITLE"
    echo "   Phase:    $ISSUE_PHASE"
    echo "   Labels:   $ISSUE_LABELS"
    if [[ -n "${ISSUE_PRIORITY:-}" ]]; then
        echo "   Priority: $ISSUE_PRIORITY"
    fi
    echo ""
    echo "📂 WORKTREE"
    echo "   Path:     $WORKTREE_PATH"
    echo "   Branch:   $BRANCH_NAME"
    echo ""
    echo "📖 ISSUE DESCRIPTION"
    echo "────────────────────────────────────────────────────────────────────────────"
    echo "$ISSUE_BODY"
    echo "────────────────────────────────────────────────────────────────────────────"
    echo ""
    echo "🚀 NEXT STEPS FOR SUBAGENT"
    echo ""
    echo "1. Change to the worktree directory:"
    echo "   cd $WORKTREE_PATH"
    echo ""
    echo "2. Read project guidelines:"
    echo "   - AGENTS.md for code standards"
    echo "   - DESIGN.md for architecture"
    echo ""
    echo "3. Implement the feature described above"
    echo ""
    echo "4. Test your changes"
    echo ""
    echo "5. Commit and push:"
    echo "   git add ."
    echo "   git commit -m \"feat(issue-$ISSUE_NUMBER): <description>\""
    echo "   git push -u origin $BRANCH_NAME"
    echo ""
    echo "6. Create PR:"
    echo "   gh pr create --title \"$ISSUE_TITLE\" --body \"Closes #$ISSUE_NUMBER\""
    echo ""
    echo "7. Mark complete in work queue (from repo root):"
    echo "   ./scripts/complete-issue.sh $ISSUE_NUMBER"
    echo ""
    echo "════════════════════════════════════════════════════════════════════════════"
    echo ""
    echo "WORKTREE_PATH=$WORKTREE_PATH"
    echo "ISSUE_NUMBER=$ISSUE_NUMBER"
    echo "BRANCH_NAME=$BRANCH_NAME"
    echo "AGENT_ID=$AGENT_ID"
    echo ""
    echo "═══════════════════════════════════════════════════════════════════════════"
    echo "CLAUDE: Run 'cd $WORKTREE_PATH' then read SUBAGENT_INSTRUCTIONS.md"
    echo "═══════════════════════════════════════════════════════════════════════════"
}

main "$@"
