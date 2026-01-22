#!/usr/bin/env bash
set -euo pipefail

org_slug="${BUILDKITE_ORGANIZATION_SLUG:?BUILDKITE_ORGANIZATION_SLUG is required}"
cluster_id="${BUILDKITE_CLUSTER_ID:?BUILDKITE_CLUSTER_ID is required}"
graphql_token="${DYNAMIC_PIPELINE_GRAPHQL_TOKEN:?DYNAMIC_PIPELINE_GRAPHQL_TOKEN is required}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for Buildkite queue selection. Install Python 3 or update the script to use a different JSON parser."
  exit 1
fi

self_hosted_queue_key="${BUILDKITE_SELF_HOSTED_QUEUE_KEY:-local-agents}"
hosted_queue_key="${BUILDKITE_HOSTED_QUEUE_KEY:-hosted}"
self_hosted_queue_id="${BUILDKITE_SELF_HOSTED_QUEUE_ID:-}"

api_call() {
  local payload="$1"
  curl -sS -X POST \
    -H "Authorization: Bearer ${graphql_token}" \
    -H "Content-Type: application/json" \
    -d "${payload}" \
    "https://graphql.buildkite.com/v1"
}

if [[ -z "${self_hosted_queue_id}" ]]; then
  queue_payload=$(cat <<JSON
{"query":"query(\$org:String!,\$cluster:ID!){organization(slug:\$org){cluster(id:\$cluster){queues(first:100){edges{node{id key}}}}}}","variables":{"org":"${org_slug}","cluster":"${cluster_id}"}}
JSON
)

  queue_response=$(api_call "${queue_payload}")

  self_hosted_queue_id=$(python3 - <<'PY'
import json
import os
import sys

key = os.environ.get("BUILDKITE_SELF_HOSTED_QUEUE_KEY", "local-agents")

try:
    data = json.load(sys.stdin)
    queues = data["data"]["organization"]["cluster"]["queues"]["edges"]
except Exception as exc:
    raise SystemExit(f"Failed to read Buildkite queue response: {exc}")

for edge in queues:
    node = edge.get("node", {})
    if node.get("key") == key:
        print(node.get("id"))
        raise SystemExit(0)

raise SystemExit(f"No queue found for key '{key}'.")
PY
  <<<"${queue_response}")
fi

agents_payload=$(cat <<JSON
{"query":"query(\$org:String!,\$queue:[ID!]){organization(slug:\$org){agents(first:50,clusterQueue:\$queue){edges{node{id connectionState}}}}}","variables":{"org":"${org_slug}","queue":["${self_hosted_queue_id}"]}}
JSON
)

agents_response=$(api_call "${agents_payload}")

connected_count=$(python3 - <<'PY'
import json
import sys

try:
    data = json.load(sys.stdin)
    edges = data["data"]["organization"]["agents"]["edges"]
except Exception as exc:
    raise SystemExit(f"Failed to read Buildkite agents response: {exc}")

connected = 0
for edge in edges:
    node = edge.get("node", {})
    if node.get("connectionState") == "connected":
        connected += 1

print(connected)
PY
<<<"${agents_response}")

if [[ "${connected_count}" -gt 0 ]]; then
  target_queue="${self_hosted_queue_key}"
  echo "Found ${connected_count} connected self-hosted agent(s); using queue '${target_queue}'."
else
  target_queue="${hosted_queue_key}"
  echo "No connected self-hosted agents found; using queue '${target_queue}'."
fi

{
  cat <<YAML
env:
  CI: "true"

agents:
  queue: "${target_queue}"

YAML
  cat ".buildkite/pipeline.steps.yml"
} | buildkite-agent pipeline upload
