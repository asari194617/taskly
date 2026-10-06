#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$root/infra"

existing_profiles=${COMPOSE_PROFILES:-}
export COMPOSE_PROFILES="${existing_profiles:+$existing_profiles,}schema"

plan=$(mktemp)
cleanup() {
  rm -f "$plan"
  docker compose rm -sf atlas-dev-db >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker compose up -d --wait db atlas-dev-db
docker compose run --rm --no-deps atlas schema apply --env taskly --dry-run >"$plan"
cat "$plan"

if grep -Eiq '\b(DROP|MODIFY[[:space:]]+COLUMN|CHANGE[[:space:]]+COLUMN|RENAME)\b' "$plan"; then
  if [[ ${ATLAS_ALLOW_DESTRUCTIVE:-false} != true ]]; then
    echo 'Atlas planned a destructive schema change. Set ATLAS_ALLOW_DESTRUCTIVE=true only after backup and rollback review.' >&2
    exit 1
  fi
fi

docker compose run --rm --no-deps atlas schema apply --env taskly --auto-approve
