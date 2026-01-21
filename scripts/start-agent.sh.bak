#!/bin/bash

# start-agent.sh - Start a subagent that claims and works on the next available issue
#
# This script:
# 1. Finds the next available issue whose dependencies are all complete
# 2. Atomically claims it (updates work queue with lock file)
# 3. Creates or reuses a git worktree for the agent
# 4. Outputs instructions for the Amp subagent to continue
#
# Usage: ./scripts/start-agent.sh
#
# The script outputs the worktree path and issue details which can be used
# to start an Amp session in that directory.

set -e

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

# Cleanup function
cleanup() {
    rm -f "$LOCK_FILE"
}
trap cleanup EXIT

# Acquire lock with timeout
acquire_lock() {
    local timeout=30
    local waited=0
    
    while [ -f "$LOCK_FILE" ]; do
        if [ $waited -ge $timeout ]; then
            log_error "Timeout waiting for lock. Another agent may be stuck."
            log_info "Remove $LOCK_FILE manually if needed."
            exit 1
        fi
        sleep 1
        waited=$((waited + 1))
    done
    
    echo $$ > "$LOCK_FILE"
}

# Get list of completed issue numbers (from both completed_issues and issues with status=completed)
get_completed_issues() {
    local completed_array
    completed_array=$(jq -r '.completed_issues // [] | .[]' "$WORK_QUEUE" 2>/dev/null || echo "")
    local completed_status
    completed_status=$(jq -r '.issues[] | select(.status == "completed") | .issue_number' "$WORK_QUEUE" 2>/dev/null || echo "")
    
    echo "$completed_array"
    echo "$completed_status"
}

# Check if all dependencies for an issue are satisfied
dependencies_satisfied() {
    local issue_number=$1
    local deps
    deps=$(jq -r ".issues[] | select(.issue_number == $issue_number) | .dependencies | .[]" "$WORK_QUEUE" 2>/dev/null || echo "")
    
    if [ -z "$deps" ]; then
        return 0  # No dependencies
    fi
    
    local completed
    completed=$(get_completed_issues)
    
    for dep in $deps; do
        if ! echo "$completed" | grep -q "^${dep}$"; then
            return 1  # Dependency not satisfied
        fi
    done
    
    return 0  # All dependencies satisfied
}

# Find next available issue with satisfied dependencies
find_next_issue() {
    local available_issues
    available_issues=$(jq -r '.issues[] | select(.status == "available") | .issue_number' "$WORK_QUEUE")
    
    for issue_number in $available_issues; do
        if dependencies_satisfied "$issue_number"; then
            echo "$issue_number"
            return 0
        fi
    done
    
    return 1
}

# Generate a unique agent ID
generate_agent_id() {
    echo "agent-$(date +%s)-$$"
}

# Create branch name from issue
create_branch_name() {
    local issue_number=$1
    local title=$2
    
    # Convert title to kebab-case, remove special chars
    local slug
    slug=$(echo "$title" | \
        sed 's/Phase [0-9]*: //' | \
        tr '[:upper:]' '[:lower:]' | \
        sed 's/[^a-z0-9 ]//g' | \
        sed 's/  */ /g' | \
        sed 's/ /-/g' | \
        cut -c1-40)
    
    echo "feat/issue-${issue_number}-${slug}"
}

# Main execution
main() {
    log_info "Starting Clubhouse Agent..."
    
    # Ensure we're in the repo root
    cd "$REPO_ROOT"
    
    # Sync with remote
    log_info "Syncing with main branch..."
    git checkout main 2>/dev/null || true
    git pull origin main 2>/dev/null || true
    
    # Acquire lock
    log_info "Acquiring work queue lock..."
    acquire_lock
    
    # Find next available issue
    log_info "Finding next available issue..."
    ISSUE_NUMBER=$(find_next_issue)
    
    if [ -z "$ISSUE_NUMBER" ]; then
        log_warning "No available issues with satisfied dependencies."
        echo ""
        log_info "Blocked issues and their missing dependencies:"
        jq -r '.issues[] | select(.status == "available") | "  #\(.issue_number): \(.title)\n    Waiting for: \(.dependencies | map("#\(.)") | join(", "))"' "$WORK_QUEUE"
        exit 0
    fi
    
    # Get issue details
    ISSUE_TITLE=$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | .title" "$WORK_QUEUE")
    ISSUE_PHASE=$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | .phase" "$WORK_QUEUE")
    ISSUE_LABELS=$(jq -r ".issues[] | select(.issue_number == $ISSUE_NUMBER) | .labels | join(\", \")" "$WORK_QUEUE")
    
    log_success "Found issue #$ISSUE_NUMBER: $ISSUE_TITLE"
    
    # Generate agent ID and branch name
    AGENT_ID=$(generate_agent_id)
    BRANCH_NAME=$(create_branch_name "$ISSUE_NUMBER" "$ISSUE_TITLE")
    WORKTREE_PATH="$WORKTREES_DIR/$AGENT_ID"
    
    # Update work queue
    log_info "Claiming issue #$ISSUE_NUMBER..."
    jq ".issues |= map(if .issue_number == $ISSUE_NUMBER then . + {\"status\": \"in_progress\", \"assigned_to\": \"$AGENT_ID\", \"worktree\": \"$WORKTREE_PATH\", \"branch\": \"$BRANCH_NAME\", \"claimed_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"} else . end)" "$WORK_QUEUE" > "$WORK_QUEUE.tmp"
    mv "$WORK_QUEUE.tmp" "$WORK_QUEUE"
    
    # Commit the claim
    git add "$WORK_QUEUE"
    git commit -m "chore: agent $AGENT_ID claims issue #$ISSUE_NUMBER" 2>/dev/null || true
    
    # Push with retry on conflict
    if ! git push origin main 2>/dev/null; then
        log_warning "Push conflict detected, pulling and retrying..."
        git pull --rebase origin main 2>/dev/null || true
        git push origin main 2>/dev/null || true
    fi
    
    # Create worktree
    log_info "Creating worktree at $WORKTREE_PATH..."
    mkdir -p "$WORKTREES_DIR"
    
    # Check if branch already exists remotely
    if git ls-remote --heads origin "$BRANCH_NAME" | grep -q "$BRANCH_NAME"; then
        log_info "Branch $BRANCH_NAME exists, checking it out..."
        git worktree add "$WORKTREE_PATH" "$BRANCH_NAME" 2>/dev/null || \
            git worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" "origin/$BRANCH_NAME" 2>/dev/null || \
            git worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" main
    else
        git worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" main
    fi
    
    log_success "Worktree created at $WORKTREE_PATH"
    
    # Release lock (cleanup will handle this, but be explicit)
    rm -f "$LOCK_FILE"
    
    # Fetch issue body from GitHub
    log_info "Fetching issue details from GitHub..."
    ISSUE_BODY=$(gh issue view "$ISSUE_NUMBER" --json body --jq '.body' 2>/dev/null || echo "Could not fetch issue body")
    
    # Output summary
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
