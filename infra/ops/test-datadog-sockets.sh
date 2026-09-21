#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
COMPOSE_FILE="$PROJECT_ROOT/infra/docker-compose.yml"
PROD_COMPOSE_FILE="$PROJECT_ROOT/infra/docker-compose.prod.yml"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

docker compose --file "$COMPOSE_FILE" --file "$PROD_COMPOSE_FILE" config --format json >"$tmp"

jq -e '
  . as $config
  | ["server", "email-compose", "airflow-webserver", "airflow-scheduler", "airflow-dag-processor"] as $traced
  | all($traced[] as $service |
      $config.services[$service].environment.DD_TRACE_AGENT_URL == "unix:///var/run/datadog/apm.socket"
      and $config.services[$service].environment.DD_DOGSTATSD_URL == "unix:///var/run/datadog/dsd.socket"
      and any($config.services[$service].volumes[];
        .type == "bind"
        and .source == "/var/run/datadog"
        and .target == "/var/run/datadog"
        and .read_only == true))
' "$tmp" >/dev/null

# Agent transport must remain private; only Unix sockets are used by services.
jq -e '
  all(.services[];
    all((.ports // [])[];
      (.target != 8125 and .target != 8126)
      and (.published != "8125" and .published != "8126")))
' "$tmp" >/dev/null

echo "Datadog socket policy passed"
