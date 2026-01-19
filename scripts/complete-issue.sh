#!/bin/bash

# complete-issue.sh - Mark an issue as completed in the work queue
#
# Usage: ./scripts/complete-issue.sh <issue_number> [pr_number]
#
# This script:
# 1. Updates the issue status to "completed"
# 2. Moves the issue number to completed_issues array
# 3. Commits and pushes the change
# 4. Optionally cleans up the worktree

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_QUEUE="$REPO_ROOT/.work-queue.json"
LOCK_FILE="$REPO_ROOT/.work-queue.lock"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}ℹ${NC} $1"; }
log_success() { echo -e "${GREEN}✓${NC} $1"; }
log_warning() { echo -e "${YELLOW}⚠${NC} $1"; }
log_error() { echo -e "${RED}✗${NC} $1"; }

if [ -z "$1" ]; then
    echo "Usage: ./scripts/complete-issue.sh <issue_number> [pr_number]"
    echo "Example: ./scripts/complete-issue.sh 19 85"
    exit 1
fi

ISSUE_NUMBER=$1
PR_NUMBER=${2:-null}

# Cleanup function
cleanup() {
    rm -f "$LOCK_FILE"
}
trap cleanup EXIT

# Acquire lock
acquire_lock() {
    local timeout=30
    local waited=0
    
    while [ -f "$LOCK_FILE" ]; do
        if [ $waited -ge $timeout ]; then
            log_error "Timeout waiting for lock."
            exit 1
        fi
        sleep 1
        waited=$((waited + 1))
    done
    
    echo $$ > "$LOCK_FILE"
}

main() {
    cd "$REPO_ROOT"
    
    # Sync with remote
    log_info "Syncing with main branch..."
    git checkout main 2>/dev/null || true
    git pull origin main 2>/dev/null || true
    
    # Acquire lock
    acquire_lock
    
    # Check issue exists
    ISSUE_EXISTS=$(jq ".issues[] | select(.issue_number == $ISSUE_NUMBER)" "$WORK_QUEUE")
    if [ -z "$ISSUE_EXISTS" ]; then
        log_error "Issue #$ISSUE_NUMBER not found in work queue"
        exit 1
    fi
    
    # Get issue info
    ISSUE_TITLE=$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | .title" "$WORK_QUEUE")
    WORKTREE_PATH=$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | .worktree // empty" "$WORK_QUEUE")
    
    log_info "Marking issue #$ISSUE_NUMBER as completed: $ISSUE_TITLE"
    
    # Update issue status and add to completed_issues
    if [ "$PR_NUMBER" != "null" ]; then
        jq ".issues |= map(if .issue_number == $ISSUE_NUMBER then . + {\"status\": \"completed\", \"pr_number\": $PR_NUMBER, \"completed_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"} else . end) | .completed_issues += [$ISSUE_NUMBER] | .completed_issues |= unique" "$WORK_QUEUE" > "$WORK_QUEUE.tmp"
    else
        jq ".issues |= map(if .issue_number == $ISSUE_NUMBER then . + {\"status\": \"completed\", \"completed_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"} else . end) | .completed_issues += [$ISSUE_NUMBER] | .completed_issues |= unique" "$WORK_QUEUE" > "$WORK_QUEUE.tmp"
    fi
    mv "$WORK_QUEUE.tmp" "$WORK_QUEUE"
    
    # Commit and push
    git add "$WORK_QUEUE"
    git commit -m "chore: mark issue #$ISSUE_NUMBER as completed"
    
    if ! git push origin main 2>/dev/null; then
        log_warning "Push conflict, rebasing..."
        git pull --rebase origin main
        git push origin main
    fi
    
    log_success "Issue #$ISSUE_NUMBER marked as completed"
    
    # Clean up worktree if it exists
    if [ -n "$WORKTREE_PATH" ] && [ -d "$WORKTREE_PATH" ]; then
        log_info "Cleaning up worktree at $WORKTREE_PATH..."
        git worktree remove "$WORKTREE_PATH" --force 2>/dev/null || true
        log_success "Worktree removed"
    fi
    
    # Show newly unblocked issues
    echo ""
    log_info "Checking for newly unblocked issues..."
    
    # Show issues that were waiting on this one
    WAITING=$(jq -r ".issues[] | select(.status == \"available\") | select(.dependencies | index($ISSUE_NUMBER)) | \"  #\(.issue_number): \(.title)\"" "$WORK_QUEUE")
    if [ -n "$WAITING" ]; then
        echo "Issues that were depending on #$ISSUE_NUMBER:"
        echo "$WAITING"
    fi
    
    rm -f "$LOCK_FILE"
}

main "$@"
