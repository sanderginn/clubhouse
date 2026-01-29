#!/bin/bash

# claim-issue.sh - Atomically claim the next available issue from work queue
# Usage: ./scripts/claim-issue.sh [--dry-run] <agent_name>

set -e

# Parse flags
DRY_RUN=false
if [ "$1" == "--dry-run" ]; then
  DRY_RUN=true
  shift
fi

if [ -z "$1" ]; then
  echo "Usage: ./scripts/claim-issue.sh [--dry-run] <agent_name>"
  echo "Example: ./scripts/claim-issue.sh agent-1"
  echo "         ./scripts/claim-issue.sh --dry-run agent-1"
  exit 1
fi

AGENT_NAME=$1
WORK_QUEUE=".work-queue.json"

# Ensure we're on main and up to date (skip in dry run)
if [ "$DRY_RUN" = false ]; then
  echo "Syncing with main branch..."
  git checkout main
  git pull origin main
else
  echo "[DRY RUN] Skipping git sync..."
fi

# Find first available issue (sorted by priority - lower number = higher priority)
# Issues without a priority field are assigned priority 999 (picked last)
# Only select issues whose dependencies are all completed
echo "Searching for available issues..."

# Build list of completed issue numbers
COMPLETED_ISSUES=$(jq -r '[.completed_issues[], (.issues[] | select(.status == "completed") | .issue_number)] | unique' "$WORK_QUEUE")

# Find available issues where all dependencies are in the completed list
AVAILABLE_ISSUE=$(jq -r --argjson completed "$COMPLETED_ISSUES" '
  [.issues[] | select(
    .status == "available" and
    ((.dependencies // []) | all(. as $dep | $completed | index($dep) != null))
  )] | sort_by(.priority // 999) | .[0].issue_number // empty
' "$WORK_QUEUE")

if [ -z "$AVAILABLE_ISSUE" ]; then
  echo "No available issues found in work queue"
  echo ""
  # Show issues that are blocked by dependencies
  BLOCKED_ISSUES=$(jq -r --argjson completed "$COMPLETED_ISSUES" '
    [.issues[] | select(
      .status == "available" and
      ((.dependencies // []) | any(. as $dep | $completed | index($dep) == null))
    )] | if length > 0 then
      "Blocked issues (waiting for dependencies):\n" +
      (map("  #\(.issue_number) - blocked by: \([.dependencies[] | select(. as $dep | $completed | index($dep) == null)] | map("#\(.)") | join(", "))") | join("\n"))
    else
      empty
    end
  ' "$WORK_QUEUE")
  if [ -n "$BLOCKED_ISSUES" ]; then
    echo -e "$BLOCKED_ISSUES"
  fi
  exit 1
fi

echo "Found available issue: #$AVAILABLE_ISSUE"

# Check if someone else claimed it in the meantime (race condition protection)
# by re-pulling and checking again (skip in dry run)
if [ "$DRY_RUN" = false ]; then
  git pull origin main 2>/dev/null || true
  STILL_AVAILABLE=$(jq -r ".issues[] | select(.issue_number == $AVAILABLE_ISSUE) | .status" "$WORK_QUEUE")

  if [ "$STILL_AVAILABLE" != "available" ]; then
    echo "Issue #$AVAILABLE_ISSUE was claimed by another agent. Trying again..."
    ./scripts/claim-issue.sh "$AGENT_NAME"
    exit 0
  fi
fi

# Get issue details
ISSUE_TITLE=$(jq -r ".issues[] | select(.issue_number == $AVAILABLE_ISSUE) | .title" "$WORK_QUEUE")
BRANCH_NAME=$(jq -r ".issues[] | select(.issue_number == $AVAILABLE_ISSUE) | .branch" "$WORK_QUEUE")

if [ "$DRY_RUN" = true ]; then
  # Dry run mode - just show what would be claimed
  echo ""
  echo "[DRY RUN] Would claim issue #$AVAILABLE_ISSUE"
  echo ""
  echo "Issue: $ISSUE_TITLE"
  echo "Branch: $BRANCH_NAME"
  echo "Agent: $AGENT_NAME"
  echo ""
  echo "No changes made to work queue or git repository."
  echo ""
  exit 0
fi

# Update work queue
echo "Claiming issue #$AVAILABLE_ISSUE as $AGENT_NAME..."
jq ".issues[] |= if .issue_number == $AVAILABLE_ISSUE then .status = \"in_progress\" | .assigned_to = \"$AGENT_NAME\" else . end" "$WORK_QUEUE" > "$WORK_QUEUE.tmp"
mv "$WORK_QUEUE.tmp" "$WORK_QUEUE"

# Commit and push
git add "$WORK_QUEUE"
git commit -m "chore: claim issue #$AVAILABLE_ISSUE as $AGENT_NAME"

# Try to push - if it fails (race condition), reset and retry
if ! git push origin main; then
  echo "Push failed (likely race condition). Resetting and retrying..."
  git reset --hard origin/main
  ./scripts/claim-issue.sh "$AGENT_NAME"
  exit 0
fi

echo ""
echo "✓ Successfully claimed issue #$AVAILABLE_ISSUE"
echo ""
echo "Issue: $ISSUE_TITLE"
echo "Branch: $BRANCH_NAME"
echo ""
echo "Next steps:"
echo "  git checkout -b $BRANCH_NAME"
echo "  # ... implement the feature ..."
echo "  git commit -m \"feat: ...\""
echo "  git push -u origin $BRANCH_NAME"
echo "  gh pr create --title \"...\" --body \"Closes #$AVAILABLE_ISSUE\""
echo ""
