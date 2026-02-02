#!/bin/sh
set -e

# Grafana expects the bundled plugins directory to exist; create it to avoid startup warnings.
if [ ! -d /usr/share/grafana/plugins-bundled ]; then
  mkdir -p /usr/share/grafana/plugins-bundled || true
fi

# Grafana 11+ bundles xychart; remove any stale external plugin to avoid double registration.
if [ -d /var/lib/grafana/plugins ]; then
  for plugin_dir in /var/lib/grafana/plugins/*; do
    if [ -f "$plugin_dir/plugin.json" ] && grep -q '"id"[[:space:]]*:[[:space:]]*"xychart"' "$plugin_dir/plugin.json"; then
      rm -rf "$plugin_dir"
    fi
  done
fi

# Provision Sentry only when fully configured to avoid invalid org slug errors.
if [ -z "$GRAFANA_SENTRY_URL" ] || [ -z "$GRAFANA_SENTRY_ORG" ] || [ -z "$GRAFANA_SENTRY_AUTH_TOKEN" ]; then
  rm -f /etc/grafana/provisioning/datasources/sentry.yml
  rm -f /etc/grafana/provisioning/dashboards/frontend-errors.json
else
  # Ensure the Sentry datasource plugin is available for the frontend errors dashboard.
  if [ -z "$GF_PLUGINS_PREINSTALL" ]; then
    export GF_PLUGINS_PREINSTALL="grafana-sentry-datasource"
  else
    case ",$GF_PLUGINS_PREINSTALL," in
      *,grafana-sentry-datasource,*) ;;
      *) export GF_PLUGINS_PREINSTALL="$GF_PLUGINS_PREINSTALL,grafana-sentry-datasource" ;;
    esac
  fi
fi

exec /run.sh "$@"
