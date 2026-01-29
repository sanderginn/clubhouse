#!/bin/bash

# show-queue.sh - Display the current state of the work queue
#
# Usage: ./scripts/show-queue.sh [--available | --in-progress | --blocked | --all]

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_QUEUE="$REPO_ROOT/.work-queue.json"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Get completed issues for dependency checking
get_completed() {
    local completed_array
    completed_array=$(jq -r '.completed_issues // [] | .[]' "$WORK_QUEUE" 2>/dev/null || echo "")
    local completed_status
    completed_status=$(jq -r '.issues[] | select(.status == "completed") | .issue_number' "$WORK_QUEUE" 2>/dev/null || echo "")
    echo "$completed_array"
    echo "$completed_status"
}

# Check if all dependencies are satisfied
deps_satisfied() {
    local issue_number=$1
    local deps
    deps=$(jq -r ".issues[] | select(.issue_number == $issue_number) | .dependencies | .[]" "$WORK_QUEUE" 2>/dev/null || echo "")

    if [ -z "$deps" ]; then
        return 0
    fi

    local completed
    completed=$(get_completed)

    for dep in $deps; do
        if ! echo "$completed" | grep -q "^${dep}$"; then
            return 1
        fi
    done

    return 0
}

show_available() {
    echo -e "${GREEN}═══ AVAILABLE (ready to pick up) ═══${NC}"
    local count=0
    while IFS='|' read -r num title phase labels; do
        if [ -n "$num" ] && deps_satisfied "$num"; then
            echo -e "  ${GREEN}#$num${NC} [Phase $phase] $title"
            echo -e "       Labels: $labels"
            count=$((count + 1))
        fi
    done < <(jq -r '.issues[] | select(.status == "available") | "\(.issue_number)|\(.title)|\(.phase)|\(.labels | join(", "))"' "$WORK_QUEUE")
    echo -e "  ${CYAN}Total: $count issues${NC}"
    echo ""
}

show_blocked() {
    echo -e "${YELLOW}═══ BLOCKED (waiting on dependencies) ═══${NC}"
    local count=0
    while IFS='|' read -r num title phase deps; do
        if [ -n "$num" ] && ! deps_satisfied "$num"; then
            echo -e "  ${YELLOW}#$num${NC} [Phase $phase] $title"
            # Show which deps are missing
            local missing=""
            for dep in $(echo "$deps" | tr ',' ' '); do
                if ! get_completed | grep -q "^${dep}$"; then
                    missing="$missing #$dep"
                fi
            done
            echo -e "       Waiting for:$missing"
            count=$((count + 1))
        fi
    done < <(jq -r '.issues[] | select(.status == "available") | "\(.issue_number)|\(.title)|\(.phase)|\(.dependencies | join(","))"' "$WORK_QUEUE")
    echo -e "  ${CYAN}Total: $count issues${NC}"
    echo ""
}

show_in_progress() {
    echo -e "${BLUE}═══ IN PROGRESS ═══${NC}"
    local count=0
    while IFS='|' read -r num title agent worktree branch; do
        if [ -n "$num" ]; then
            echo -e "  ${BLUE}#$num${NC} $title"
            echo -e "       Agent: $agent"
            echo -e "       Branch: $branch"
            count=$((count + 1))
        fi
    done < <(jq -r '.issues[] | select(.status == "in_progress") | "\(.issue_number)|\(.title)|\(.assigned_to)|\(.worktree)|\(.branch)"' "$WORK_QUEUE")
    echo -e "  ${CYAN}Total: $count issues${NC}"
    echo ""
}

show_completed() {
    echo -e "${GREEN}═══ COMPLETED ═══${NC}"
    local completed_count
    completed_count=$(jq '.completed_issues | length' "$WORK_QUEUE")
    echo -e "  ${CYAN}$completed_count issues completed${NC}"
    echo ""
}

show_summary() {
    local total in_progress completed ready blocked
    total=$(jq '.issues | length' "$WORK_QUEUE")
    in_progress=$(jq '[.issues[] | select(.status == "in_progress")] | length' "$WORK_QUEUE")
    completed=$(jq '.completed_issues | length' "$WORK_QUEUE")

    # Get completed issues for dependency checking
    local completed_issues
    completed_issues=$(jq -r '[.completed_issues[], (.issues[] | select(.status == "completed") | .issue_number)] | unique' "$WORK_QUEUE")

    # Count ready (available with all deps satisfied) and blocked (available with unmet deps)
    ready=$(jq -r --argjson completed "$completed_issues" '
      [.issues[] | select(
        .status == "available" and
        ((.dependencies // []) | all(. as $dep | $completed | index($dep) != null))
      )] | length
    ' "$WORK_QUEUE")

    blocked=$(jq -r --argjson completed "$completed_issues" '
      [.issues[] | select(
        .status == "available" and
        ((.dependencies // []) | any(. as $dep | $completed | index($dep) == null))
      )] | length
    ' "$WORK_QUEUE")

    echo "════════════════════════════════════════════════════════════════"
    echo "                    CLUBHOUSE WORK QUEUE STATUS"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo -e "  ${GREEN}Completed:${NC}   $completed"
    echo -e "  ${BLUE}In Progress:${NC} $in_progress"
    echo -e "  ${GREEN}Ready:${NC}       $ready"
    echo -e "  ${YELLOW}Blocked:${NC}     $blocked"
    echo -e "  ${CYAN}Total:${NC}       $total"
    echo ""
}

case "${1:-}" in
    --available|-a)
        show_available
        ;;
    --in-progress|-p)
        show_in_progress
        ;;
    --blocked|-b)
        show_blocked
        ;;
    --all)
        show_summary
        show_available
        show_in_progress
        show_blocked
        show_completed
        ;;
    *)
        show_summary
        show_available
        show_in_progress
        ;;
esac
