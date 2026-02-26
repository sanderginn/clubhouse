#!/usr/bin/env bash
set -euo pipefail

report_json_path="${1:-}"
if [[ -z "$report_json_path" ]]; then
  echo "usage: $0 <fow-report.json>" >&2
  exit 2
fi
if [[ ! -f "$report_json_path" ]]; then
  echo "report file not found: $report_json_path" >&2
  exit 2
fi

max_violations="${MAX_VIOLATIONS:-0}"
max_non_complete_flows="${MAX_NON_COMPLETE_FLOWS:-0}"
min_flow_count="${MIN_FLOW_COUNT:-1}"

if [[ ! "$max_violations" =~ ^[0-9]+$ ]]; then
  echo "MAX_VIOLATIONS must be a non-negative integer" >&2
  exit 2
fi
if [[ ! "$max_non_complete_flows" =~ ^[0-9]+$ ]]; then
  echo "MAX_NON_COMPLETE_FLOWS must be a non-negative integer" >&2
  exit 2
fi
if [[ ! "$min_flow_count" =~ ^[0-9]+$ ]]; then
  echo "MIN_FLOW_COUNT must be a non-negative integer" >&2
  exit 2
fi

counts_json="$(jq -c '
{
  violation_count: (.violations | length),
  flow_count: (.stats.flow_count // (.flows | length) // 0),
  broken_flows: (.stats.broken_flows // ([.flows[]? | select(.status=="broken")] | length)),
  partial_flows: (.stats.partial_flows // ([.flows[]? | select(.status=="partial")] | length)),
  non_complete_flows: (.stats.non_complete_flows // ([.flows[]? | select(.status=="partial" or .status=="broken")] | length))
}
' "$report_json_path")"

violation_count="$(jq -r '.violation_count' <<<"$counts_json")"
flow_count="$(jq -r '.flow_count' <<<"$counts_json")"
broken_flows="$(jq -r '.broken_flows' <<<"$counts_json")"
partial_flows="$(jq -r '.partial_flows' <<<"$counts_json")"
non_complete_flows="$(jq -r '.non_complete_flows' <<<"$counts_json")"

echo "Gate inputs: total_flows=${flow_count}/${min_flow_count}, violations=${violation_count}/${max_violations}, non_complete_flows=${non_complete_flows}/${max_non_complete_flows}, partial=${partial_flows}, broken=${broken_flows}"

if (( flow_count < min_flow_count )); then
  echo "FAIL: flow count below minimum (${flow_count} < ${min_flow_count})" >&2
  exit 1
fi

if (( violation_count > max_violations )); then
  echo "FAIL: violation threshold exceeded (${violation_count} > ${max_violations})" >&2
  exit 1
fi

if (( non_complete_flows > max_non_complete_flows )); then
  echo "FAIL: flow completeness threshold exceeded (${non_complete_flows} > ${max_non_complete_flows})" >&2
  exit 1
fi

echo "PASS: golden-state thresholds satisfied"
