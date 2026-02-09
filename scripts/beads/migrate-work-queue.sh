#!/usr/bin/env bash

# migrate-work-queue.sh - One-time importer from .work-queue.json to beads issues
#
# Usage:
#   ./scripts/beads/migrate-work-queue.sh [--dry-run] [--force]
#
# Preconditions:
# - bd is installed and initialized in this repo (bd init ...)
# - jq is installed
#
# Notes:
# - Creates beads issues with external_ref="gh-<issue_number>"
# - Converts .dependencies -> blocks edges
# - Preserves legacy order via label queue_prio:<n>
#
# This script is intentionally conservative and will refuse to run if the beads
# database already contains issues (unless --force is provided).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_QUEUE="$REPO_ROOT/.work-queue.json"

DRY_RUN=false
FORCE=false

usage() {
  cat <<'EOF'
Usage: ./scripts/beads/migrate-work-queue.sh [--dry-run] [--force]

  --dry-run  Print planned operations without modifying beads
  --force    Allow running even if beads already has issues
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --force)
      FORCE=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd jq
require_cmd bd

if [[ ! -f "$WORK_QUEUE" ]]; then
  echo "Work queue not found: $WORK_QUEUE" >&2
  exit 1
fi

cd "$REPO_ROOT"

if [[ ! -d "$REPO_ROOT/.beads" ]]; then
  echo "Beads is not initialized in this repo (.beads/ missing)." >&2
  echo "Run: bd init --branch beads-sync" >&2
  exit 1
fi

# Refuse to run on a non-empty beads DB unless --force.
existing_count="$(bd --no-daemon list --json 2>/dev/null | jq 'length' 2>/dev/null || echo "0")"
if [[ "$existing_count" != "0" && "$FORCE" != "true" ]]; then
  echo "Beads already has $existing_count issues. Refusing to import without --force." >&2
  echo "If this is expected, rerun with: --force" >&2
  exit 1
fi

echo "Importing .work-queue.json into beads (dry_run=$DRY_RUN, force=$FORCE)"

TMP_DIR="$(mktemp -d)"
MAP_FILE="$TMP_DIR/work_queue_map.tsv"

cleanup_tmp() {
  rm -rf "$TMP_DIR"
}
trap cleanup_tmp EXIT

lookup_map_field() {
  local issue_number="$1"
  local field_index="$2"
  awk -F'\t' -v n="$issue_number" -v idx="$field_index" '$1 == n {print $idx; exit 0}' "$MAP_FILE"
}

issues_stream() {
  # Create in legacy priority order (lower is earlier), then by issue_number.
  jq -c '
    .issues
    | sort_by(.priority // 999999, .issue_number)
    | .[]
  ' "$WORK_QUEUE"
}

is_bug_issue() {
  jq -e '
    ((.labels // []) | index("bug") != null)
    or ((.phase // "") | tostring | startswith("bug-"))
    or ((.title // "") | tostring | startswith("Bug:"))
  ' >/dev/null 2>&1
}

issue_type_for() {
  if is_bug_issue; then
    echo "bug"
    return 0
  fi
  if jq -e '((.labels // []) | index("enhancement") != null) or ((.labels // []) | index("feature") != null)' >/dev/null 2>&1; then
    echo "feature"
    return 0
  fi
  echo "task"
}

beads_priority_for() {
  if is_bug_issue; then
    case "$(jq -r '.phase // ""')" in
      bug-critical) echo 0 ;;
      bug-high) echo 1 ;;
      bug-medium) echo 2 ;;
      bug-low) echo 3 ;;
      *) echo 1 ;;
    esac
    return 0
  fi
  if jq -e '((.labels // []) | index("enhancement") != null) or ((.labels // []) | index("feature") != null)' >/dev/null 2>&1; then
    echo 2
    return 0
  fi
  echo 3
}

labels_csv_for() {
  local issue_number legacy_prio
  issue_number="$(jq -r '.issue_number')"
  legacy_prio="$(jq -r '.priority // 999')"

  # Start with existing labels.
  local labels
  labels="$(jq -r '.labels // [] | .[]' | tr '\n' ',' | sed 's/,$//')"

  # Add stable lookups and legacy ordering hints.
  if [[ -n "$labels" ]]; then
    labels+=","
  fi
  labels+="gh:${issue_number},queue_prio:${legacy_prio}"

  echo "$labels"
}

echo ""
echo "== Phase 1: Create issues =="

while IFS= read -r issue; do
  issue_number="$(jq -r '.issue_number' <<<"$issue")"
  title="$(jq -r '.title' <<<"$issue")"
  status="$(jq -r '.status' <<<"$issue")"
  assigned_to="$(jq -r '.assigned_to // empty' <<<"$issue")"
  pr_number="$(jq -r '.pr_number // empty' <<<"$issue")"

  issue_type="$(jq -c '.' <<<"$issue" | issue_type_for)"
  beads_prio="$(jq -c '.' <<<"$issue" | beads_priority_for)"
  labels_csv="$(jq -c '.' <<<"$issue" | labels_csv_for)"
  external_ref="gh-${issue_number}"

  status_by_issue_number["$issue_number"]="$status"
  assignee_by_issue_number["$issue_number"]="$assigned_to"
  pr_by_issue_number["$issue_number"]="$pr_number"

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "bd --no-daemon create \"${title}\" -t ${issue_type} -p ${beads_prio} --external-ref ${external_ref} -l \"${labels_csv}\" --json"
    echo -e "${issue_number}\tDRYRUN-${issue_number}\t${status}\t${assigned_to}\t${pr_number}" >>"$MAP_FILE"
    continue
  fi

  create_out="$(
    bd --no-daemon create "$title" \
      -t "$issue_type" \
      -p "$beads_prio" \
      --external-ref "$external_ref" \
      -l "$labels_csv" \
      --json
  )"

  beads_id="$(jq -r '.id' <<<"$create_out")"
  echo -e "${issue_number}\t${beads_id}\t${status}\t${assigned_to}\t${pr_number}" >>"$MAP_FILE"
  echo "Created gh-${issue_number} -> ${beads_id}"
done < <(issues_stream)

echo ""
echo "== Phase 2: Add dependencies (blocks) =="

while IFS= read -r issue; do
  issue_number="$(jq -r '.issue_number' <<<"$issue")"
  beads_id="$(lookup_map_field "$issue_number" 2 || true)"
  [[ -z "$beads_id" ]] && continue

  deps="$(jq -r '.dependencies // [] | .[]' <<<"$issue" || true)"
  [[ -z "${deps:-}" ]] && continue

  while IFS= read -r dep_issue_number; do
    [[ -z "$dep_issue_number" ]] && continue
    dep_beads_id="$(lookup_map_field "$dep_issue_number" 2 || true)"
    if [[ -z "$dep_beads_id" ]]; then
      echo "WARN: missing dependency issue #$dep_issue_number for #$issue_number (skipping)" >&2
      continue
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
      echo "bd --no-daemon dep add ${beads_id} ${dep_beads_id} --type blocks"
      continue
    fi

    bd --no-daemon dep add "$beads_id" "$dep_beads_id" --type blocks >/dev/null
    echo "Added blocks: ${dep_beads_id} blocks ${beads_id}"
  done <<<"$deps"
done < <(issues_stream)

echo ""
echo "== Phase 3: Restore legacy status =="

while IFS=$'\t' read -r issue_number beads_id legacy_status legacy_assignee legacy_pr; do
  [[ -z "${issue_number:-}" || -z "${beads_id:-}" ]] && continue

  case "$legacy_status" in
    in_progress)
      if [[ "$DRY_RUN" == "true" ]]; then
        echo "BD_ACTOR=\"${legacy_assignee:-legacy-in-progress}\" bd --no-daemon update ${beads_id} --claim --json"
        continue
      fi
      BD_ACTOR="${legacy_assignee:-legacy-in-progress}" bd --no-daemon update "$beads_id" --claim --json >/dev/null
      echo "Claimed ${beads_id} as ${legacy_assignee:-legacy-in-progress}"
      ;;
    completed)
      reason="Legacy completed (gh-${issue_number}"
      if [[ -n "${legacy_pr:-}" ]]; then
        reason+=", PR #${legacy_pr}"
      fi
      reason+=")"

      if [[ "$DRY_RUN" == "true" ]]; then
        echo "BD_ACTOR=legacy-migration bd --no-daemon close ${beads_id} --reason \"${reason}\" --json"
        continue
      fi
      BD_ACTOR="legacy-migration" bd --no-daemon close "$beads_id" --reason "$reason" --json >/dev/null
      echo "Closed ${beads_id}"
      ;;
    available|*)
      # Legacy "available" maps to beads "open" (default).
      :
      ;;
  esac
done <"$MAP_FILE"

echo ""
echo "Done."
echo ""
echo "Next steps:"
echo "  bd ready"
echo "  bd list --status in_progress"
echo "  bd dep cycles"
