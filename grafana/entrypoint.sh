#!/bin/sh
set -e

# Grafana 11+ bundles xychart; remove stale external plugin to avoid double registration.
if [ -d /var/lib/grafana/plugins/xychart ]; then
  rm -rf /var/lib/grafana/plugins/xychart
fi

exec /run.sh "$@"
