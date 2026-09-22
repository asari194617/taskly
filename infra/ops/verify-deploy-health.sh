#!/usr/bin/env bash
set -Eeuo pipefail

# Run from infra/ after `docker compose up --wait`. The Compose healthchecks
# cover every container; these probes also cover the host port mappings and
# Airflow components whose health endpoint returns 200 even when unhealthy.
services=(db redis server email-compose mcp airflow-webserver airflow-scheduler airflow-dag-processor prometheus alertmanager blackbox grafana)

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
  expected='running healthy'
  case "$service" in prometheus|alertmanager|blackbox|grafana) expected='running missing' ;; esac
  if [[ "$state" != "$expected" ]]; then
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
probe 'Prometheus host port' http://127.0.0.1:3068/-/ready
probe 'Alertmanager host port' http://127.0.0.1:3069/-/ready
probe 'Grafana host port' http://127.0.0.1:3067/api/health

failure='API metrics endpoint is unavailable or missing required series'
curl --fail --silent --show-error --max-time 10 http://127.0.0.1:3064/metrics | python3 -c '
import sys
body = sys.stdin.read()
required = ("go_goroutines", "taskly_http_requests_total", "taskly_integration_jobs", "taskly_notification_deliveries")
missing = [name for name in required if name not in body]
if missing:
    print(f"Missing API metrics: {missing}", file=sys.stderr)
    sys.exit(1)
print("API metrics endpoint: required series present")
'
failure='Airflow metrics exporter is unavailable or missing DAG metrics'
docker compose exec -T airflow-scheduler curl --fail --silent --show-error --max-time 10 \
  http://127.0.0.1:9470/metrics | grep -q '^taskly_airflow_dag_last_run_timestamp_seconds'
echo 'Airflow metrics exporter: required series present'

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
  expected='running healthy'
  case "$service" in prometheus|alertmanager|blackbox|grafana) expected='running missing' ;; esac
  if [[ "$state" != "$expected" || "$current_restarts" != "${restarts[$index]}" ]]; then
    failure="$service changed during stability check: $state, restarts ${restarts[$index]} -> $current_restarts"
    false
  fi
done
echo 'All production services remained healthy for 20 seconds.'
