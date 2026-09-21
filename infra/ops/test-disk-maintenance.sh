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
exit 0
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

export MOCK_CRONTAB="$tmp/crontab"
PROJECT_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
"$SCRIPT_DIR/install-user-cron.sh" "$PROJECT_ROOT" >/dev/null
"$SCRIPT_DIR/install-user-cron.sh" "$PROJECT_ROOT" >/dev/null
[[ $(grep -c '^# BEGIN taskly disk maintenance$' "$MOCK_CRONTAB") == 1 ]]
[[ $(grep -c 'disk-maintenance.sh status' "$MOCK_CRONTAB") == 1 ]]
[[ $(grep -c 'disk-maintenance.sh cleanup' "$MOCK_CRONTAB") == 1 ]]

echo "disk-maintenance tests passed"
