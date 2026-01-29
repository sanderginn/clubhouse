#!/bin/bash

# prune-completed.sh - Archive completed issues that aren't blocking open work
#
# This script moves completed issues from .work-queue.json to .completed-work-items.json
# if they are not listed as dependencies of any open (available/in_progress) issue.
#
# Usage: ./scripts/prune-completed.sh [--dry-run]

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_QUEUE="$REPO_ROOT/.work-queue.json"
ARCHIVE_FILE="$REPO_ROOT/.completed-work-items.json"

# Parse flags
DRY_RUN=false
if [ "$1" == "--dry-run" ]; then
  DRY_RUN=true
  echo "[DRY RUN] No changes will be made"
  echo ""
fi

# Ensure work queue exists
if [ ! -f "$WORK_QUEUE" ]; then
  echo "Error: $WORK_QUEUE not found"
  exit 1
fi

# Initialize archive file if it doesn't exist
if [ ! -f "$ARCHIVE_FILE" ]; then
  if [ "$DRY_RUN" = false ]; then
    echo '{"metadata":{"description":"Archived completed work items","created_at":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"},"archived_issues":[],"archived_issue_numbers":[]}' | jq '.' > "$ARCHIVE_FILE"
  fi
  echo "Created new archive file: $ARCHIVE_FILE"
fi

# Get all dependencies required by open issues (available or in_progress)
# This is the set of issue numbers we MUST keep in the work queue
REQUIRED_DEPS=$(jq -r '
  [.issues[] | select(.status == "available" or .status == "in_progress") | .dependencies // []]
  | flatten
  | unique
  | .[]
' "$WORK_QUEUE")

echo "=== Dependency Analysis ==="
echo ""

# Count open issues
OPEN_COUNT=$(jq '[.issues[] | select(.status == "available" or .status == "in_progress")] | length' "$WORK_QUEUE")
echo "Open issues (available/in_progress): $OPEN_COUNT"

# Show required dependencies
REQUIRED_COUNT=$(echo "$REQUIRED_DEPS" | grep -c . || echo 0)
echo "Dependencies required by open issues: $REQUIRED_COUNT"
if [ "$REQUIRED_COUNT" -gt 0 ] && [ "$REQUIRED_COUNT" -lt 20 ]; then
  echo "  Required: $(echo $REQUIRED_DEPS | tr '\n' ' ')"
fi
echo ""

# Find completed issues that can be archived
# These are issues with status "completed" that are NOT in the required deps list
ARCHIVABLE_ISSUES=$(jq -r --argjson required "$(echo "$REQUIRED_DEPS" | jq -R -s 'split("\n") | map(select(length > 0) | tonumber)')" '
  [.issues[] | select(
    .status == "completed" and
    ([.issue_number] | inside($required) | not)
  )]
' "$WORK_QUEUE")

ARCHIVABLE_COUNT=$(echo "$ARCHIVABLE_ISSUES" | jq 'length')

# Find completed_issues numbers that can be archived
ARCHIVABLE_NUMBERS=$(jq -r --argjson required "$(echo "$REQUIRED_DEPS" | jq -R -s 'split("\n") | map(select(length > 0) | tonumber)')" '
  [.completed_issues[] | select(. as $n | $required | index($n) == null)]
' "$WORK_QUEUE")

ARCHIVABLE_NUMBERS_COUNT=$(echo "$ARCHIVABLE_NUMBERS" | jq 'length')

echo "=== Archivable Items ==="
echo ""
echo "Completed issues (full records) to archive: $ARCHIVABLE_COUNT"
echo "Completed issue numbers to archive: $ARCHIVABLE_NUMBERS_COUNT"
echo ""

if [ "$ARCHIVABLE_COUNT" -eq 0 ] && [ "$ARCHIVABLE_NUMBERS_COUNT" -eq 0 ]; then
  echo "Nothing to archive. All completed issues are still required as dependencies."
  exit 0
fi

# Show what will be archived
if [ "$ARCHIVABLE_COUNT" -gt 0 ]; then
  echo "Issues to archive:"
  echo "$ARCHIVABLE_ISSUES" | jq -r '.[] | "  #\(.issue_number) - \(.title)"'
  echo ""
fi

if [ "$DRY_RUN" = true ]; then
  echo "[DRY RUN] Would archive $ARCHIVABLE_COUNT issues and $ARCHIVABLE_NUMBERS_COUNT issue numbers"
  echo ""

  # Show size reduction estimate
  CURRENT_SIZE=$(wc -c < "$WORK_QUEUE" | tr -d ' ')
  echo "Current .work-queue.json size: $(numfmt --to=iec-i --suffix=B $CURRENT_SIZE 2>/dev/null || echo "$CURRENT_SIZE bytes")"
  exit 0
fi

# Perform the archive operation
echo "Archiving..."

# 1. Add archivable issues to archive file
jq --argjson issues "$ARCHIVABLE_ISSUES" --argjson numbers "$ARCHIVABLE_NUMBERS" '
  .archived_issues += $issues |
  .archived_issue_numbers += $numbers |
  .metadata.last_updated = (now | todate)
' "$ARCHIVE_FILE" > "$ARCHIVE_FILE.tmp"
mv "$ARCHIVE_FILE.tmp" "$ARCHIVE_FILE"

# 2. Remove archived items from work queue
jq --argjson required "$(echo "$REQUIRED_DEPS" | jq -R -s 'split("\n") | map(select(length > 0) | tonumber)')" '
  # Keep only completed issues that are required as dependencies
  .issues = [.issues[] | select(
    .status != "completed" or
    ([.issue_number] | inside($required))
  )] |
  # Keep only completed_issues numbers that are required as dependencies
  .completed_issues = [.completed_issues[] | select(. as $n | $required | index($n) != null)]
' "$WORK_QUEUE" > "$WORK_QUEUE.tmp"
mv "$WORK_QUEUE.tmp" "$WORK_QUEUE"

# Validate both files
if ! jq empty "$WORK_QUEUE" 2>/dev/null; then
  echo "Error: Invalid JSON in $WORK_QUEUE after pruning!"
  exit 1
fi

if ! jq empty "$ARCHIVE_FILE" 2>/dev/null; then
  echo "Error: Invalid JSON in $ARCHIVE_FILE after archiving!"
  exit 1
fi

echo ""
echo "=== Results ==="
echo ""

# Show new counts
NEW_ISSUES_COUNT=$(jq '.issues | length' "$WORK_QUEUE")
NEW_COMPLETED_COUNT=$(jq '.completed_issues | length' "$WORK_QUEUE")
ARCHIVE_ISSUES_COUNT=$(jq '.archived_issues | length' "$ARCHIVE_FILE")
ARCHIVE_NUMBERS_COUNT=$(jq '.archived_issue_numbers | length' "$ARCHIVE_FILE")

echo ".work-queue.json:"
echo "  Issues remaining: $NEW_ISSUES_COUNT"
echo "  Completed numbers remaining: $NEW_COMPLETED_COUNT"
echo ""
echo ".completed-work-items.json:"
echo "  Archived issues: $ARCHIVE_ISSUES_COUNT"
echo "  Archived numbers: $ARCHIVE_NUMBERS_COUNT"
echo ""

# Show size comparison
QUEUE_SIZE=$(wc -c < "$WORK_QUEUE" | tr -d ' ')
ARCHIVE_SIZE=$(wc -c < "$ARCHIVE_FILE" | tr -d ' ')
echo "File sizes:"
echo "  .work-queue.json: $(numfmt --to=iec-i --suffix=B $QUEUE_SIZE 2>/dev/null || echo "$QUEUE_SIZE bytes")"
echo "  .completed-work-items.json: $(numfmt --to=iec-i --suffix=B $ARCHIVE_SIZE 2>/dev/null || echo "$ARCHIVE_SIZE bytes")"
echo ""
echo "Done! Archived $ARCHIVABLE_COUNT issues and $ARCHIVABLE_NUMBERS_COUNT issue numbers."
