#!/bin/sh
set -e

# Grafana 11+ bundles xychart; remove any stale external plugin to avoid double registration.
if [ -d /var/lib/grafana/plugins ]; then
  for plugin_dir in /var/lib/grafana/plugins/*; do
    if [ -f "$plugin_dir/plugin.json" ] && grep -q '"id"[[:space:]]*:[[:space:]]*"xychart"' "$plugin_dir/plugin.json"; then
      rm -rf "$plugin_dir"
    fi
  done
fi

exec /run.sh "$@"
