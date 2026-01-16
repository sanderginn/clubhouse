#!/bin/bash

# setup-worktrees.sh - Create git worktrees for all available issues
# This allows multiple agents to work in parallel on different issues

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_QUEUE="$REPO_ROOT/.work-queue.json"
WORKTREES_DIR="$REPO_ROOT/.worktrees"

echo "Setting up git worktrees for available issues..."
mkdir -p "$WORKTREES_DIR"

# Get all available issues
AVAILABLE_ISSUES=$(jq -r '.issues[] | select(.status == "available") | "\(.issue_number)|\(.branch)|\(.title)"' "$WORK_QUEUE")

if [ -z "$AVAILABLE_ISSUES" ]; then
  echo "No available issues found"
  exit 1
fi

# Create worktrees and collect agent commands
AGENT_COMMANDS=""
AGENT_NUM=1

while IFS='|' read -r ISSUE_NUM BRANCH_NAME ISSUE_TITLE; do
  WORKTREE_PATH="$WORKTREES_DIR/agent-$AGENT_NUM"
  AGENT_NAME="agent-$AGENT_NUM"
  
  echo ""
  echo "Creating worktree for issue #$ISSUE_NUM: $ISSUE_TITLE"
  echo "  Branch: $BRANCH_NAME"
  echo "  Worktree: $WORKTREE_PATH"
  
  # Create worktree with new branch
  git -C "$REPO_ROOT" worktree add "$WORKTREE_PATH" -b "$BRANCH_NAME" main
  
  # Update work queue for this issue
  jq ".issues[] |= if .issue_number == $ISSUE_NUM then .status = \"in_progress\" | .assigned_to = \"$AGENT_NAME\" else . end" "$WORK_QUEUE" > "$WORK_QUEUE.tmp"
  mv "$WORK_QUEUE.tmp" "$WORK_QUEUE"
  
  # Generate command for agent
  AGENT_CMD="cd '$WORKTREE_PATH' && bash -c 'source /dev/stdin' << 'EOF'
# Issue #$ISSUE_NUM: $ISSUE_TITLE
echo \"Working on issue #$ISSUE_NUM: $ISSUE_TITLE\"
echo \"Branch: $BRANCH_NAME\"
echo \"Worktree path: $WORKTREE_PATH\"
echo \"\"
echo \"Next steps:\"
echo \"  1. Implement the feature in this worktree\"
echo \"  2. Run: git add . && git commit -m \\\"feat: ...\\\"\"
echo \"  3. Run: git push -u origin $BRANCH_NAME\"
echo \"  4. Run: gh pr create --title \\\"...\\\" --body \\\"Closes #$ISSUE_NUM\\\"\"
echo \"\"
echo \"After PR is merged by orchestrator, your work is done!\"
echo \"\"
EOF
"
  
  AGENT_COMMANDS="$AGENT_COMMANDS$AGENT_CMD
"
  
  AGENT_NUM=$((AGENT_NUM + 1))
done <<< "$AVAILABLE_ISSUES"

# Commit work queue changes
cd "$REPO_ROOT"
git add "$WORK_QUEUE"
git commit -m "chore: setup worktrees for $(echo "$AVAILABLE_ISSUES" | wc -l) available issues"
git push origin main

echo ""
echo "========================================"
echo "✓ Worktrees created successfully!"
echo "========================================"
echo ""
echo "Run these commands in new terminals to start work:"
echo ""
echo "$AGENT_COMMANDS"
echo ""
echo "Summary of active issues:"
jq -r '.issues[] | select(.status == "in_progress") | "  Issue #\(.issue_number): \(.title) (assigned to: \(.assigned_to))"' "$WORK_QUEUE"
echo ""
