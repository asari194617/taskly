#!/usr/bin/env bash
# Apply the declarative schema to a disposable CI database.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
name="taskly-atlas-ci-${GITHUB_RUN_ID:-local}-$$"
plan=$(mktemp)
cleanup() {
  rm -f "$plan"
  docker rm -f "$name" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --name "$name" -p 127.0.0.1::3306 \
  -e MYSQL_ROOT_PASSWORD=atlas_dev_password \
  -e MYSQL_DATABASE=atlas_dev mysql:8.4 >/dev/null
port=$(docker port "$name" 3306/tcp | sed 's/.*://')
for _ in $(seq 1 60); do
  if docker exec "$name" mysql -h 127.0.0.1 -uroot -patlas_dev_password -N -e 'SELECT 1' atlas_dev >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[[ ${ready:-} == 1 ]] || { echo 'Atlas dev database did not become ready' >&2; exit 1; }

atlas=(docker run --rm --network host -v "$root:/workspace:ro" -w /workspace
  -e MYSQL_USER -e MYSQL_PASSWORD -e MYSQL_DATABASE -e DB_HOST -e DB_PORT
  -e "ATLAS_DEV_URL=mysql://root:atlas_dev_password@127.0.0.1:$port/atlas_dev"
  arigaio/atlas:1.3.3-community-alpine)

"${atlas[@]}" schema apply --env taskly --dry-run >"$plan"
cat "$plan"
if grep -Eiq '\b(DROP|MODIFY[[:space:]]+COLUMN|CHANGE[[:space:]]+COLUMN|RENAME)\b' "$plan"; then
  echo 'Atlas unexpectedly planned a destructive change for the disposable CI database.' >&2
  exit 1
fi
"${atlas[@]}" schema apply --env taskly --auto-approve
"${atlas[@]}" schema apply --env taskly --dry-run
