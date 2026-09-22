#!/usr/bin/env bash
set -euo pipefail

# This script deliberately never prunes Docker volumes. Named volumes contain
# MySQL, Redis, Airflow logs, and backups and must remain recoverable.

MODE="${1:-status}"
MOUNT_POINT="${DISK_MOUNT_POINT:-/}"
WARN_PERCENT="${DISK_WARN_PERCENT:-70}"
CRITICAL_PERCENT="${DISK_CRITICAL_PERCENT:-80}"
EMERGENCY_PERCENT="${DISK_EMERGENCY_PERCENT:-90}"
DEPLOY_MAX_PERCENT="${DISK_DEPLOY_MAX_PERCENT:-85}"
DEPLOY_MIN_FREE_GIB="${DISK_DEPLOY_MIN_FREE_GIB:-10}"
DOCKER_RETENTION_HOURS="${DOCKER_RETENTION_HOURS:-168}"
AIRFLOW_LOG_RETENTION_DAYS="${AIRFLOW_LOG_RETENTION_DAYS:-14}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_MIN_COPIES="${BACKUP_MIN_COPIES:-3}"
AIRFLOW_CONTAINER="${AIRFLOW_CONTAINER:-taskly-airflow-scheduler}"
STATE_DIR="${TASKLY_DISK_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/taskly}"
STATE_FILE="$STATE_DIR/disk-alert-level"
ENV_FILE="${TASKLY_ENV_FILE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.env}"

usage() {
  echo "Usage: $0 {status|pre-deploy|cleanup}" >&2
  exit 2
}

is_uint() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

for value in "$WARN_PERCENT" "$CRITICAL_PERCENT" "$EMERGENCY_PERCENT" \
  "$DEPLOY_MAX_PERCENT" "$DEPLOY_MIN_FREE_GIB" "$DOCKER_RETENTION_HOURS" \
  "$AIRFLOW_LOG_RETENTION_DAYS" "$BACKUP_RETENTION_DAYS" "$BACKUP_MIN_COPIES"; do
  is_uint "$value" || { echo "Configuration values must be non-negative integers" >&2; exit 2; }
done

read_disk() {
  local line
  line=$(df -Pk "$MOUNT_POINT" | awk 'NR == 2 { print $2, $3, $4, $5 }')
  read -r total_kib used_kib free_kib used_raw <<<"$line"
  used_percent="${used_raw%%%}"
  [[ -n "${total_kib:-}" && -n "${used_percent:-}" ]] || {
    echo "Could not read filesystem usage for $MOUNT_POINT" >&2
    exit 1
  }
}

level_for() {
  local percent="$1"
  if (( percent >= EMERGENCY_PERCENT )); then echo emergency
  elif (( percent >= CRITICAL_PERCENT )); then echo critical
  elif (( percent >= WARN_PERCENT )); then echo warning
  else echo ok
  fi
}

env_value() {
  local key="$1"
  [[ -r "$ENV_FILE" ]] || return 0
  awk -v key="$key" '
    $0 !~ /^[[:space:]]*#/ && index($0, key "=") == 1 {
      print substr($0, length(key) + 2); exit
    }
  ' "$ENV_FILE" | sed -e 's/^['"'"'"]//' -e 's/['"'"'"]$//'
}

send_alert_if_changed() {
  local level="$1" previous="" webhook="" message=""
  mkdir -p "$STATE_DIR"
  [[ -r "$STATE_FILE" ]] && previous=$(cat "$STATE_FILE")
  [[ "$level" == "$previous" ]] && return 0
  if [[ -z "$previous" && "$level" == ok ]]; then
    printf '%s\n' "$level" >"$STATE_FILE"
    return 0
  fi

  webhook="${DISK_ALERT_WEBHOOK_URL:-}"
  [[ -n "$webhook" ]] || webhook=$(env_value DISK_ALERT_WEBHOOK_URL)
  [[ -n "$webhook" ]] || webhook=$(env_value DISCORD_WEBHOOK_URL)
  message="Taskly production disk: level=$level usage=${used_percent}% free=$((free_kib / 1024))MiB mount=$MOUNT_POINT host=$(hostname)"

  if [[ -n "$webhook" ]]; then
    escaped=${message//\\/\\\\}
    escaped=${escaped//\"/\\\"}
    if ! curl --fail --silent --show-error --max-time 10 \
      -H 'Content-Type: application/json' -d "{\"content\":\"$escaped\"}" "$webhook" >/dev/null; then
      echo "Warning: disk alert webhook delivery failed" >&2
    fi
  else
    logger -t taskly-disk "$message" 2>/dev/null || true
  fi
  printf '%s\n' "$level" >"$STATE_FILE"
}

report_status() {
  read_disk
  level=$(level_for "$used_percent")
  echo "disk mount=$MOUNT_POINT level=$level used=${used_percent}% free=$((free_kib / 1024))MiB"
  send_alert_if_changed "$level"
}

pre_deploy() {
  report_status
  local min_free_kib=$((DEPLOY_MIN_FREE_GIB * 1024 * 1024))
  if (( used_percent >= DEPLOY_MAX_PERCENT || free_kib < min_free_kib )); then
    echo "Deployment blocked: require usage < ${DEPLOY_MAX_PERCENT}% and at least ${DEPLOY_MIN_FREE_GIB}GiB free" >&2
    exit 1
  fi
}

cleanup_airflow_files() {
  if ! docker inspect "$AIRFLOW_CONTAINER" >/dev/null 2>&1; then
    echo "Airflow cleanup skipped: container $AIRFLOW_CONTAINER not found"
    return 0
  fi

  docker exec "$AIRFLOW_CONTAINER" find /opt/airflow/logs -xdev -type f \
    -mtime "+$AIRFLOW_LOG_RETENTION_DAYS" -delete

  # Keep at least the newest BACKUP_MIN_COPIES old backups even when all
  # backups exceed the retention age. Recent backups are always retained.
  docker exec \
    -e BACKUP_RETENTION_DAYS="$BACKUP_RETENTION_DAYS" \
    -e BACKUP_MIN_COPIES="$BACKUP_MIN_COPIES" \
    "$AIRFLOW_CONTAINER" bash -euo pipefail -c '
      for directory in /opt/airflow/backups /opt/airflow/backups/airflow; do
        test -d "$directory" || continue
        mapfile -d "" old < <(find "$directory" -maxdepth 1 -type f -name "backup-*.sql.gz.age" \
          -mtime "+$BACKUP_RETENTION_DAYS" -print0 | sort -zr)
        for ((i=BACKUP_MIN_COPIES; i<${#old[@]}; i++)); do rm -f -- "${old[$i]}"; done
      done
    '
}

cleanup() {
  report_status
  echo "Pruning unused Docker images and build cache older than ${DOCKER_RETENTION_HOURS}h"
  docker image prune --all --force --filter "until=${DOCKER_RETENTION_HOURS}h"
  docker builder prune --all --force --filter "until=${DOCKER_RETENTION_HOURS}h"
  cleanup_airflow_files
  echo "Docker volumes were not pruned"
  report_status
}

case "$MODE" in
  status) report_status ;;
  pre-deploy) pre_deploy ;;
  cleanup) cleanup ;;
  *) usage ;;
esac
