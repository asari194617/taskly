#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/state"

cat >"$tmp/bin/df" <<'EOF'
#!/usr/bin/env bash
echo 'Filesystem 1024-blocks Used Available Capacity Mounted on'
echo "/dev/test 104857600 83886080 ${MOCK_FREE_KIB:-20971520} ${MOCK_USED_PERCENT:-80}% /"
EOF
cat >"$tmp/bin/docker" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$MOCK_DOCKER_LOG"
if [[ "$1" == inspect ]]; then exit 1; fi
EOF
cat >"$tmp/bin/logger" <<'EOF'
#!/usr/bin/env bash
[[ "${MOCK_LOGGER_FAIL:-0}" == 0 ]] || exit 1
if [[ -n "${MOCK_LOGGER_LOG:-}" ]]; then
  if (( $# > 2 )); then printf '%s\n' "$*" >>"$MOCK_LOGGER_LOG"
  else cat >>"$MOCK_LOGGER_LOG"
  fi
fi
exit 0
EOF
cat >"$tmp/bin/curl" <<'EOF'
#!/usr/bin/env bash
printf 'call\n' >>"$MOCK_CURL_LOG"
case "${MOCK_CURL_RESULT:-success}" in
  success) exit 0 ;;
  5xx) exit 22 ;;
  timeout) exit 28 ;;
esac
exit 2
EOF
cat >"$tmp/bin/crontab" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == -l ]]; then
  [[ -r "$MOCK_CRONTAB" ]] || exit 1
  cat "$MOCK_CRONTAB"
else
  cp "$1" "$MOCK_CRONTAB"
fi
EOF
chmod +x "$tmp/bin/"*

export PATH="$tmp/bin:$PATH"
export HOME="$tmp"
export TASKLY_DISK_STATE_DIR="$tmp/state"
export TASKLY_ENV_FILE="$tmp/missing.env"
export MOCK_DOCKER_LOG="$tmp/docker.log"
export MOCK_CURL_LOG="$tmp/curl.log"
export MOCK_LOGGER_LOG="$tmp/logger.log"

alert_status() {
  MOCK_USED_PERCENT="$1" "$SCRIPT_DIR/disk-maintenance.sh" status >"$tmp/status.out" 2>"$tmp/status.err"
}

assert_calls() {
  local actual=0
  [[ ! -f "$MOCK_CURL_LOG" ]] || actual=$(wc -l <"$MOCK_CURL_LOG")
  (( actual == $1 )) || { echo "Expected $1 webhook calls, got $actual" >&2; exit 1; }
}

MOCK_USED_PERCENT=80 MOCK_FREE_KIB=20971520 "$SCRIPT_DIR/disk-maintenance.sh" status | grep -q 'level=critical'

if MOCK_USED_PERCENT=86 MOCK_FREE_KIB=20971520 "$SCRIPT_DIR/disk-maintenance.sh" pre-deploy >/dev/null 2>&1; then
  echo "Expected pre-deploy check to fail at 86%" >&2
  exit 1
fi

MOCK_USED_PERCENT=50 MOCK_FREE_KIB=52428800 "$SCRIPT_DIR/disk-maintenance.sh" cleanup >/dev/null
grep -q '^image prune --all --force --filter until=168h$' "$MOCK_DOCKER_LOG"
grep -q '^builder prune --all --force --filter until=168h$' "$MOCK_DOCKER_LOG"
if grep -q 'volume.*prune' "$MOCK_DOCKER_LOG"; then
  echo "Cleanup must never prune volumes" >&2
  exit 1
fi

# A failed alert must leave the old level in place for the next run.
export DISK_ALERT_WEBHOOK_URL=https://example.invalid/disk-alert
export MOCK_CURL_RESULT=5xx
if alert_status 80; then echo "Expected webhook 5xx to fail" >&2; exit 1; fi
grep -q 'disk alert webhook delivery failed' "$tmp/status.err"
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == ok ]]
assert_calls 1

export MOCK_CURL_RESULT=timeout
if alert_status 80; then echo "Expected webhook timeout to fail" >&2; exit 1; fi
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == ok ]]
assert_calls 2

export MOCK_CURL_RESULT=success
alert_status 80
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == critical ]]
assert_calls 3
alert_status 80
assert_calls 3
alert_status 90
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == emergency ]]
assert_calls 4

# Recovery also needs delivery before its state can be saved.
export MOCK_CURL_RESULT=5xx
if alert_status 50; then echo "Expected failed recovery notification" >&2; exit 1; fi
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == emergency ]]
assert_calls 5
export MOCK_CURL_RESULT=success
alert_status 50
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == ok ]]
assert_calls 6
alert_status 50
assert_calls 6

# Without a webhook, syslog delivery determines whether state can advance.
unset DISK_ALERT_WEBHOOK_URL
export MOCK_LOGGER_FAIL=1
if alert_status 80; then echo "Expected syslog delivery to fail" >&2; exit 1; fi
grep -q 'disk alert syslog delivery failed' "$tmp/status.err"
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == ok ]]
export MOCK_LOGGER_FAIL=0
alert_status 80
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == critical ]]

export MOCK_CRONTAB="$tmp/crontab"
PROJECT_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
"$SCRIPT_DIR/install-user-cron.sh" "$PROJECT_ROOT" >/dev/null
"$SCRIPT_DIR/install-user-cron.sh" "$PROJECT_ROOT" >/dev/null
[[ $(grep -c '^# BEGIN taskly disk maintenance$' "$MOCK_CRONTAB") == 1 ]]
[[ $(grep -c 'disk-maintenance.sh status' "$MOCK_CRONTAB") == 1 ]]
[[ $(grep -c 'disk-maintenance.sh cleanup' "$MOCK_CRONTAB") == 1 ]]

# The cron pipeline must report the script's failure even when logger succeeds.
export DISK_ALERT_WEBHOOK_URL=https://example.invalid/disk-alert
export MOCK_CURL_RESULT=5xx
export MOCK_USED_PERCENT=90
cron_status=$(sed -n '/^\*\/5 /{s/^[^ ]* [^ ]* [^ ]* [^ ]* [^ ]* //;p;}' "$MOCK_CRONTAB")
if sh -c "$cron_status"; then echo "Expected cron pipeline to fail" >&2; exit 1; fi
grep -q 'disk alert webhook delivery failed' "$MOCK_LOGGER_LOG"
[[ $(cat "$TASKLY_DISK_STATE_DIR/disk-alert-level") == critical ]]

echo "disk-maintenance tests passed"
