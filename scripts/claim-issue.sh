#!/bin/bash

# claim-issue.sh - Atomically claim the next available issue from work queue
# Usage: ./scripts/claim-issue.sh <agent_name>

set -e

if [ -z "$1" ]; then
  echo "Usage: ./scripts/claim-issue.sh <agent_name>"
  echo "Example: ./scripts/claim-issue.sh agent-1"
  exit 1
fi

AGENT_NAME=$1
WORK_QUEUE=".work-queue.json"

# Ensure we're on main and up to date
echo "Syncing with main branch..."
git checkout main
git pull origin main

# Find first available issue (sorted by priority - lower number = higher priority)
# Issues without a priority field are assigned priority 999 (picked last)
echo "Searching for available issues..."
AVAILABLE_ISSUE=$(jq -r '[.issues[] | select(.status == "available")] | sort_by(.priority // 999) | .[0].issue_number // empty' "$WORK_QUEUE")

if [ -z "$AVAILABLE_ISSUE" ]; then
  echo "No available issues found in work queue"
  exit 1
fi

echo "Found available issue: #$AVAILABLE_ISSUE"

# Check if someone else claimed it in the meantime (race condition protection)
# by re-pulling and checking again
git pull origin main 2>/dev/null || true
STILL_AVAILABLE=$(jq -r ".issues[] | select(.issue_number == $AVAILABLE_ISSUE) | .status" "$WORK_QUEUE")

if [ "$STILL_AVAILABLE" != "available" ]; then
  echo "Issue #$AVAILABLE_ISSUE was claimed by another agent. Trying again..."
  ./scripts/claim-issue.sh "$AGENT_NAME"
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

# Get issue details
ISSUE_TITLE=$(jq -r ".issues[] | select(.issue_number == $AVAILABLE_ISSUE) | .title" "$WORK_QUEUE")
BRANCH_NAME=$(jq -r ".issues[] | select(.issue_number == $AVAILABLE_ISSUE) | .branch" "$WORK_QUEUE")

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
