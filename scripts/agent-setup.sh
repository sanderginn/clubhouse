#!/bin/bash

# agent-setup.sh - One-command setup for subagents
# Usage: ./scripts/agent-setup.sh N
# Where N is the agent number (1-12)

set -e

AGENT_NUM=$1
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_QUEUE="$REPO_ROOT/.work-queue.json"
WORKTREE_PATH="$REPO_ROOT/.worktrees/agent-$AGENT_NUM"

if [ -z "$AGENT_NUM" ]; then
  echo "Usage: ./scripts/agent-setup.sh N"
  echo "Where N is the agent number (1-12)"
  exit 1
fi

# Verify worktree exists
if [ ! -d "$WORKTREE_PATH" ]; then
  echo "❌ Error: Worktree not found at $WORKTREE_PATH"
  echo "Run './scripts/setup-worktrees.sh' first"
  exit 1
fi

# Get assigned issue from work queue
ISSUE_INFO=$(jq -r ".issues[] | select(.assigned_to == \"agent-$AGENT_NUM\") | \"\(.issue_number)|\(.title)|\(.branch)|\(.status)\"" "$WORK_QUEUE" | head -1)

if [ -z "$ISSUE_INFO" ]; then
  echo "❌ Error: No issue assigned to agent-$AGENT_NUM"
  echo ""
  echo "Available assignments:"
  jq -r '.issues[] | select(.assigned_to != null) | "  \(.assigned_to): issue #\(.issue_number) - \(.title)"' "$WORK_QUEUE"
  exit 1
fi

IFS='|' read -r ISSUE_NUM ISSUE_TITLE BRANCH_NAME STATUS <<< "$ISSUE_INFO"

if [ "$STATUS" != "in_progress" ]; then
  echo "⚠️  Warning: Issue #$ISSUE_NUM is marked as '$STATUS', not 'in_progress'"
fi

# Output setup info
clear
echo "╔════════════════════════════════════════════════════════════════════════════╗"
echo "║                     CLUBHOUSE AGENT SETUP - AGENT-$AGENT_NUM                      ║"
echo "╚════════════════════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 Issue Assignment:"
echo "   Issue #$ISSUE_NUM: $ISSUE_TITLE"
echo "   Status: $STATUS"
echo ""
echo "📂 Worktree Location:"
echo "   $WORKTREE_PATH"
echo ""
echo "🌿 Git Branch:"
echo "   $BRANCH_NAME"
echo ""
echo "✅ Your working directory is ready!"
echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""
echo "📖 NEXT STEPS:"
echo ""
echo "1. Read the guidelines:"
echo "   - AGENTS.md (code standards)"
echo "   - DESIGN.md (architecture)"
echo ""
echo "2. Implement the feature in this worktree"
echo ""
echo "3. Commit your work:"
echo "   git add ."
echo "   git commit -m \"feat: implement <feature>\""
echo ""
echo "4. Push and create PR:"
echo "   git push -u origin $BRANCH_NAME"
echo "   gh pr create --title \"feat: ...\" --body \"Closes #$ISSUE_NUM\""
echo ""
echo "5. Wait for orchestrator to review and merge"
echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""

# Change to worktree directory
cd "$WORKTREE_PATH"

# Show git status
echo "📊 Current git status:"
git status --short

echo ""
echo "You are now in: $WORKTREE_PATH"
echo "Ready to start coding!"
