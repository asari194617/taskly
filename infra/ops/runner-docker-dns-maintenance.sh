#!/usr/bin/env bash
set -Eeuo pipefail

# One-time maintenance for the Taskly CI runner, scheduled for a quiet period.
# Run as root. --check validates the guards and candidate DNS without changes.
mode=${1:-apply}
if [[ "$mode" != apply && "$mode" != --check ]]; then
  echo "usage: $0 [--check]" >&2
  exit 2
fi

quiet() {
  if [[ -n "$(who)" ]]; then
    echo 'Interactive users are logged in; deferring Docker restart.' >&2
    return 1
  fi
  if pgrep -f '[R]unner.Worker' >/dev/null; then
    echo 'A GitHub Actions job is running; deferring Docker restart.' >&2
    return 1
  fi
  local name
  while IFS= read -r name; do
    if [[ "$name" != taskly-ci-slot-[123] ]]; then
      echo "Unexpected running container $name; deferring Docker restart." >&2
      return 1
    fi
  done < <(docker ps --format '{{.Names}}')
}

# Docker's bridge must be able to reach the replacement DNS servers.
docker run --rm --network bridge --dns 1.1.1.1 --dns 8.8.8.8 \
  --pull never alpine:3.20 sh -ec \
  'nslookup registry-1.docker.io >/dev/null && nslookup auth.docker.io >/dev/null'

if [[ "$mode" == --check ]]; then
  quiet
  echo 'Candidate DNS and quiet-period checks passed.'
  exit 0
fi
if (( EUID != 0 )); then
  echo 'Root privileges are required to change Docker daemon settings.' >&2
  exit 1
fi

in_window() {
  local date time
  date=$(TZ=Asia/Tokyo date +%Y-%m-%d)
  time=$(TZ=Asia/Tokyo date +%H%M)
  [[ "$date" == 2026-10-08 && "$time" > 0359 && "$time" < 0500 ]]
}
if ! in_window; then
  echo 'Outside the approved 2026-10-08 04:00-05:00 JST window; no changes made.' >&2
  exit 1
fi

# Retry only between 04:00 and 05:00 JST. If the host never becomes quiet,
# leave Docker untouched and report failure in the systemd service journal.
for attempt in {1..12}; do
  if quiet; then break; fi
  if (( attempt == 12 )); then
    echo 'Quiet period was not available within one hour; no changes made.' >&2
    exit 1
  fi
  sleep 300
done

config=/etc/docker/daemon.json
backup="/etc/docker/daemon.json.taskly-backup.$(date -u +%Y%m%dT%H%M%SZ)"
staged=$(mktemp /etc/docker/.daemon.json.taskly.XXXXXXXX)
trap 'rm -f "$staged"' EXIT
cp -a "$config" "$backup"
python3 - "$config" "$staged" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    config = json.load(source)
config["dns"] = ["1.1.1.1", "8.8.8.8"]
with open(sys.argv[2], "w", encoding="utf-8") as target:
    json.dump(config, target, indent=2)
    target.write("\n")
PY
chmod --reference="$config" "$staged"
chown --reference="$config" "$staged"
dockerd --validate --config-file "$staged"
quiet
if ! in_window; then
  echo 'The approved maintenance window ended; no changes made.' >&2
  exit 1
fi
mv "$staged" "$config"

rollback() {
  trap - ERR
  echo "Docker did not recover; restoring $backup" >&2
  cp -a "$backup" "$config"
  systemctl restart docker || true
  systemctl restart taskly-runner@1.service taskly-runner@2.service taskly-runner@3.service || true
}
trap rollback ERR
systemctl restart docker
systemctl restart taskly-runner@1.service taskly-runner@2.service taskly-runner@3.service
systemctl is-active --quiet docker
for slot in 1 2 3; do
  systemctl is-active --quiet "taskly-runner@$slot.service"
  docker ps --format '{{.Names}}' | grep -qx "taskly-ci-slot-$slot"
done
docker run --rm --network bridge --pull never alpine:3.20 sh -ec \
  'nslookup registry-1.docker.io >/dev/null && nslookup auth.docker.io >/dev/null'
trap - ERR
echo "Docker DNS updated and runner slots recovered. Backup: $backup"
