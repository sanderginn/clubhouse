#!/bin/sh
set -e

# Grafana expects the bundled plugins directory to exist; create it to avoid startup warnings.
mkdir -p /usr/share/grafana/plugins-bundled

# Grafana 11+ bundles xychart; remove stale external plugin to avoid double registration.
if [ -d /var/lib/grafana/plugins/xychart ]; then
  rm -rf /var/lib/grafana/plugins/xychart
fi

exec /run.sh "$@"
