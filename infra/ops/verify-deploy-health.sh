#!/usr/bin/env bash
set -Eeuo pipefail

# Run from infra/ after `docker compose up --wait`. The Compose healthchecks
# cover every container; these probes also cover the host port mappings and
# Airflow components whose health endpoint returns 200 even when unhealthy.
services=(db redis server email-compose mcp airflow-webserver airflow-scheduler airflow-dag-processor)

diagnose() {
  echo "Deployment health verification failed: ${failure:-unknown failure}" >&2
  docker compose ps >&2 || true
  for service in "${services[@]}"; do
    id=$(docker compose ps -aq "$service" 2>/dev/null) || continue
    if [[ -n "$id" ]]; then
      docker inspect --format "${service}: state={{.State.Status}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}} restarts={{.RestartCount}} last_error={{if .State.Health}}{{range .State.Health.Log}}{{.Output}}{{end}}{{end}}" "$id" >&2 || true
    fi
  done
  docker compose logs --tail=30 --no-color "${services[@]}" >&2 || true
}
trap diagnose ERR

for service in "${services[@]}"; do
  id=$(docker compose ps -q "$service")
  if [[ -z "$id" ]]; then
    failure="$service container is missing"
    false
  fi
  state=$(docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' "$id")
  if [[ "$state" != 'running healthy' ]]; then
    failure="$service is $state"
    false
  fi
done

probe() {
  local name=$1 url=$2
  local status
  if ! status=$(curl --fail --silent --show-error --max-time 10 --output /dev/null --write-out '%{http_code}' "$url"); then
    failure="$name request failed at $url"
    false
  fi
  if [[ "$status" != 200 ]]; then
    failure="$name returned HTTP $status from $url (expected 200)"
    false
  fi
  echo "$name: HTTP 200"
}

probe 'API host port' http://127.0.0.1:3063/healthz
probe 'MCP host port' http://127.0.0.1:3066/healthz
probe 'Airflow host port' http://127.0.0.1:3065/api/v2/monitor/health

for name in API MCP Airflow; do
  variable="PROD_PUBLIC_${name}_HEALTH_URL"
  if [[ -n "${!variable:-}" ]]; then
    probe "$name public route" "${!variable}"
  fi
done

failure='Airflow metadatabase, scheduler, or DAG processor is unhealthy'
curl --fail --silent --show-error --max-time 10 \
  http://127.0.0.1:3065/api/v2/monitor/health \
  | python3 -c '
import json, sys
health = json.load(sys.stdin)
failed = {name: health.get(name, {}).get("status") for name in ("metadatabase", "scheduler", "dag_processor") if health.get(name, {}).get("status") != "healthy"}
if failed:
    print(f"Airflow unhealthy components: {failed}", file=sys.stderr)
    sys.exit(1)
print("Airflow metadatabase, scheduler, and DAG processor: healthy")
'

ids=()
restarts=()
for index in "${!services[@]}"; do
  service=${services[$index]}
  id=$(docker compose ps -q "$service")
  ids[$index]=$id
  restarts[$index]=$(docker inspect --format '{{.RestartCount}}' "$id")
done
sleep 20
for index in "${!services[@]}"; do
  service=${services[$index]}
  id=$(docker compose ps -q "$service")
  if [[ -z "$id" || "$id" != "${ids[$index]}" ]]; then
    failure="$service was replaced or disappeared during stability check"
    false
  fi
  state=$(docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' "$id")
  current_restarts=$(docker inspect --format '{{.RestartCount}}' "$id")
  if [[ "$state" != 'running healthy' || "$current_restarts" != "${restarts[$index]}" ]]; then
    failure="$service changed during stability check: $state, restarts ${restarts[$index]} -> $current_restarts"
    false
  fi
done
echo 'All production services remained healthy for 20 seconds.'
