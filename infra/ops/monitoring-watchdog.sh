#!/usr/bin/env bash
set -euo pipefail

STATE_DIR="${TASKLY_MONITOR_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/taskly}"
STATE_FILE="$STATE_DIR/monitoring-health"
ENV_FILE="${TASKLY_ENV_FILE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.env}"
PROMETHEUS_URL="${MONITORING_PROMETHEUS_URL:-http://127.0.0.1:3068}"
ALERTMANAGER_URL="${MONITORING_ALERTMANAGER_URL:-http://127.0.0.1:3069}"

healthy=true
for url in "$PROMETHEUS_URL/-/ready" "$ALERTMANAGER_URL/-/ready"; do
  if ! curl --fail --silent --show-error --max-time 5 "$url" >/dev/null; then
    healthy=false
  fi
done

if ! curl --fail --silent --show-error --max-time 5 \
  "$PROMETHEUS_URL/api/v1/query?query=up" | python3 -c '
import json, sys
data = json.load(sys.stdin)
if data.get("status") != "success":
    sys.exit(1)
samples = data.get("data", {}).get("result", [])
expected = {"taskly-api": 1, "taskly-airflow": 1, "taskly-services": 3, "prometheus": 1, "alertmanager": 1}
for job, count in expected.items():
    matches = [x for x in samples if x.get("metric", {}).get("job") == job]
    if len(matches) != count or any(float(x["value"][1]) != 1 for x in matches):
        sys.exit(1)
'; then
  healthy=false
fi

mkdir -p "$STATE_DIR"
previous=$(cat "$STATE_FILE" 2>/dev/null || true)
current=down
[[ "$healthy" == true ]] && current=ok
[[ "$current" == "$previous" ]] && exit 0

if [[ -z "$previous" && "$current" == ok ]]; then
  printf '%s\n' "$current" >"$STATE_FILE"
  exit 0
fi

webhook="${MONITORING_ALERT_WEBHOOK_URL:-}"
if [[ -z "$webhook" && -r "$ENV_FILE" ]]; then
  webhook=$(awk '$0 !~ /^[[:space:]]*#/ && index($0,"DISCORD_WEBHOOK_URL=")==1 {print substr($0,21); exit}' "$ENV_FILE" | sed -e 's/^['"'"'"]//' -e 's/['"'"'"]$//')
fi
if [[ -z "$webhook" ]]; then
  echo 'Monitoring watchdog: DISCORD_WEBHOOK_URL is missing' >&2
  exit 1
fi

message='Taskly monitoring watchdog: Prometheus, Alertmanager, or a scrape target is unavailable.'
[[ "$current" == ok ]] && message='Taskly monitoring watchdog: monitoring collection recovered.'
printf '%s' "$message" | python3 -c 'import json,sys; print(json.dumps({"content": sys.stdin.read()}))' | \
  curl --fail --silent --show-error --max-time 10 -H 'Content-Type: application/json' --data-binary @- "$webhook" >/dev/null
printf '%s\n' "$current" >"$STATE_FILE"
echo "Monitoring watchdog: $current"
