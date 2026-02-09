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
# 2. Atomically claims it (prefers beads `bd update --claim` when configured; otherwise uses the legacy work queue lock file)
# 3. Creates or reuses a git worktree for the agent
# 4. Outputs the issue number and worktree path
#
# Usage: .codex/skills/subagent/scripts/start-agent.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
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

have_beads() {
    command -v bd >/dev/null 2>&1 && [[ -d "$REPO_ROOT/.beads" ]] && [[ -f "$REPO_ROOT/.beads/beads.db" ]]
}

create_branch_name_beads() {
    local issue_number="$1"
    local beads_id="$2"
    local title="$3"
    local issue_type="$4"

    local prefix="feat"
    if [[ "$issue_type" == "bug" ]]; then
        prefix="fix"
    fi

    local base_ref="bead-${beads_id}"
    if [[ -n "${issue_number:-}" && "$issue_number" != "0" ]]; then
        base_ref="issue-${issue_number}"
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

    echo "${prefix}/${base_ref}-${slug}"
}

beads_main() {
    log_info "Starting Clubhouse Agent (beads mode)..."

    cd "$REPO_ROOT"

    log_info "Fetching origin/main..."
    git fetch origin main >/dev/null 2>&1 || true

    AGENT_ID="$(generate_agent_id)"

    log_info "Querying ready beads work..."
    READY_JSON="$(bd --no-daemon ready --json 2>/dev/null || true)"
    if [[ -z "${READY_JSON:-}" ]]; then
        log_error "Failed to query beads ready work. Is beads initialized (bd init ...)?"
        exit 1
    fi

    # Prefer bugs, then legacy ordering label (queue_prio:<n>), then beads priority, then created_at.
    CANDIDATE_IDS="$(
        echo "$READY_JSON" | jq -r '
          map(select(.external_ref != null and (.external_ref | test("^gh-[0-9]+$"))))
          | map(
              . as $i
              | ($i.labels // []) as $labels
              | ($labels | map(select(startswith("queue_prio:"))) | .[0] // "") as $q
              | ($q | sub("^queue_prio:";"") | tonumber? // 999999) as $qprio
              | (($i.issue_type == "bug") or ($labels | index("bug") != null)) as $isBug
              | {
                  id: $i.id,
                  bug_group: (if $isBug then 0 else 1 end),
                  qprio: $qprio,
                  prio: ($i.priority // 999),
                  created_at: ($i.created_at // "")
                }
            )
          | sort_by(.bug_group, .qprio, .prio, .created_at)
          | .[].id
        '
    )"

    if [[ -z "${CANDIDATE_IDS:-}" ]]; then
        log_warning "No ready beads issues."
        exit 0
    fi

    log_info "Claiming next beads issue..."
    CLAIMED_ID=""
    while IFS= read -r candidate_id; do
        [[ -z "$candidate_id" ]] && continue
        if BD_ACTOR="$AGENT_ID" bd --no-daemon update "$candidate_id" --claim --json >/dev/null 2>&1; then
            CLAIMED_ID="$candidate_id"
            break
        fi
    done <<<"$CANDIDATE_IDS"

    if [[ -z "${CLAIMED_ID:-}" ]]; then
        log_warning "No ready beads issues were claimable (race with other agents)."
        exit 0
    fi

    ISSUE_JSON="$(bd --no-daemon show "$CLAIMED_ID" --json)"
    ISSUE_TITLE="$(jq -r '.title' <<<"$ISSUE_JSON")"
    ISSUE_TYPE="$(jq -r '.issue_type // "task"' <<<"$ISSUE_JSON")"
    EXTERNAL_REF="$(jq -r '.external_ref // empty' <<<"$ISSUE_JSON")"

    ISSUE_NUMBER=""
    if [[ "$EXTERNAL_REF" =~ ^gh-([0-9]+)$ ]]; then
        ISSUE_NUMBER="${BASH_REMATCH[1]}"
    fi
    if [[ -z "${ISSUE_NUMBER:-}" ]]; then
        log_error "Claimed issue ${CLAIMED_ID} but external_ref is not in expected format (gh-<n>): ${EXTERNAL_REF:-<empty>}"
        log_error "Fix the issue external_ref in beads and retry."
        exit 1
    fi

    if [[ "$ISSUE_TYPE" == "bug" ]]; then
        log_success "Claimed BUG issue #$ISSUE_NUMBER as $CLAIMED_ID: $ISSUE_TITLE"
    else
        log_success "Claimed issue #$ISSUE_NUMBER as $CLAIMED_ID: $ISSUE_TITLE"
    fi

    BRANCH_NAME="$(create_branch_name_beads "$ISSUE_NUMBER" "$CLAIMED_ID" "$ISSUE_TITLE" "$ISSUE_TYPE")"
    WORKTREE_PATH="$WORKTREES_DIR/$AGENT_ID"

    log_info "Creating worktree at $WORKTREE_PATH..."
    mkdir -p "$WORKTREES_DIR"

    if git ls-remote --heads origin "$BRANCH_NAME" 2>/dev/null | grep -q "$BRANCH_NAME"; then
        log_info "Branch $BRANCH_NAME exists, checking it out..."
        git fetch origin "$BRANCH_NAME" >/dev/null 2>&1 || true
        git worktree add "$WORKTREE_PATH" "$BRANCH_NAME" >/dev/null 2>&1 || \
            git worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" "origin/$BRANCH_NAME" >/dev/null 2>&1
    else
        git worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" origin/main >/dev/null 2>&1
    fi

    log_success "Worktree created"

    # Persist worktree/branch info for recovery (merge with existing metadata).
    EXISTING_META="$(jq -c '.metadata // {}' <<<"$ISSUE_JSON")"
    UPDATED_META="$(jq -c \
      --argjson m "$EXISTING_META" \
      --arg agent_id "$AGENT_ID" \
      --arg branch "$BRANCH_NAME" \
      --arg worktree "$WORKTREE_PATH" \
      --arg ext "$EXTERNAL_REF" \
      --arg issue_number "$ISSUE_NUMBER" \
      '$m + {
        clubhouse: (($m.clubhouse // {}) + {
          agent_id: $agent_id,
          branch: $branch,
          worktree: $worktree,
          external_ref: $ext,
          gh_issue_number: ($issue_number | tonumber)
        })
      }' <<<"$ISSUE_JSON")"

    BD_ACTOR="$AGENT_ID" bd --no-daemon update "$CLAIMED_ID" --metadata "$UPDATED_META" --json >/dev/null 2>&1 || true

    # Output only machine-readable results (parse these values in the subagent workflow).
    echo "BEADS_ID=$CLAIMED_ID"
    echo "ISSUE_NUMBER=$ISSUE_NUMBER"
    echo "WORKTREE_PATH=$WORKTREE_PATH"
}

legacy_main() {
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

    if is_bug_issue "$ISSUE_NUMBER"; then
        ISSUE_PRIORITY="$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | (.priority // \"\")" "$WORK_QUEUE")"
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

    log_success "Worktree created"

    # Explicitly release lock early
    rm -f "$LOCK_FILE" || true

    # Output only machine-readable results
    echo "ISSUE_NUMBER=$ISSUE_NUMBER"
    echo "WORKTREE_PATH=$WORKTREE_PATH"
}

main() {
    if have_beads; then
        beads_main "$@"
        return 0
    fi
    legacy_main "$@"
}

main "$@"
