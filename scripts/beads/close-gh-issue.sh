#!/usr/bin/env bash

# close-gh-issue.sh - Close a beads issue by GitHub issue number (external_ref gh-<n>)
#
# Usage:
#   ./scripts/beads/close-gh-issue.sh <issue_number> [pr_number]
#
# This is useful for orchestrators that primarily reason in GitHub issue numbers.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

usage() {
  echo "Usage: ./scripts/beads/close-gh-issue.sh <issue_number> [pr_number]" >&2
}

if [[ $# -lt 1 ]]; then
  usage
  exit 2
fi

ISSUE_NUMBER="$1"
PR_NUMBER="${2:-}"

if ! command -v bd >/dev/null 2>&1; then
  echo "Missing required command: bd" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "Missing required command: jq" >&2
  exit 1
fi

cd "$REPO_ROOT"

external_ref="gh-${ISSUE_NUMBER}"

resolve_id() {
  local status="$1"
  bd --no-daemon list --status "$status" --json 2>/dev/null \
    | jq -r --arg ref "$external_ref" '.[] | select(.external_ref == $ref) | .id' \
    | head -n 1
}

beads_id="$(resolve_id open || true)"
if [[ -z "${beads_id:-}" ]]; then
  beads_id="$(resolve_id in_progress || true)"
fi
if [[ -z "${beads_id:-}" ]]; then
  beads_id="$(resolve_id blocked || true)"
fi
if [[ -z "${beads_id:-}" ]]; then
  beads_id="$(resolve_id deferred || true)"
fi
if [[ -z "${beads_id:-}" ]]; then
  beads_id="$(resolve_id closed || true)"
fi

if [[ -z "${beads_id:-}" ]]; then
  echo "No beads issue found with external_ref=${external_ref}" >&2
  exit 1
fi

reason="Merged GitHub issue #${ISSUE_NUMBER}"
if [[ -n "${PR_NUMBER:-}" ]]; then
  reason+=" (PR #${PR_NUMBER})"
fi

BD_ACTOR="${BD_ACTOR:-orchestrator}" bd --no-daemon close "$beads_id" --reason "$reason" --json >/dev/null
echo "Closed ${beads_id} (${external_ref})"
