#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:-}"
[[ -n "$WORKDIR" && -x "$WORKDIR/infra/ops/disk-maintenance.sh" ]] || {
  echo "Usage: $0 ABSOLUTE_PROJECT_WORKDIR" >&2
  exit 2
}

BEGIN_MARKER="# BEGIN taskly disk maintenance"
END_MARKER="# END taskly disk maintenance"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

{
  (crontab -l 2>/dev/null || true) | awk -v begin="$BEGIN_MARKER" -v end="$END_MARKER" '
    $0 == begin { skip=1; next }
    $0 == end { skip=0; next }
    !skip { print }
  '
  echo "$BEGIN_MARKER"
  echo "*/5 * * * * cd '$WORKDIR/infra' && ./ops/disk-maintenance.sh status 2>&1 | logger -t taskly-disk"
  echo "25 3 * * * cd '$WORKDIR/infra' && ./ops/disk-maintenance.sh cleanup 2>&1 | logger -t taskly-disk"
  echo "$END_MARKER"
} >"$tmp"

crontab "$tmp"
echo "Installed Taskly disk monitoring and cleanup in the current user's crontab"
