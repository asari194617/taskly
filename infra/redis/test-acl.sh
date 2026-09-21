#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
container="taskly-redis-acl-test-$$"
username=taskly
password=test-only-secret

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run --rm -d \
  --name "$container" \
  -e REDIS_USERNAME="$username" \
  -e REDIS_PASSWORD="$password" \
  -v "$script_dir/entrypoint.sh:/usr/local/bin/taskly-redis-entrypoint:ro" \
  --entrypoint /usr/local/bin/taskly-redis-entrypoint \
  redis:7-alpine >/dev/null

for _ in $(seq 1 30); do
  if docker exec -e REDISCLI_AUTH="$password" "$container" \
    redis-cli --user "$username" ping 2>/dev/null | grep -qx PONG; then
    break
  fi
  sleep 1
done

auth_redis_cli() {
  docker exec -e REDISCLI_AUTH="$password" "$container" \
    redis-cli --user "$username" "$@"
}

expect_noperm() {
  local output
  output=$(auth_redis_cli "$@" 2>&1)
  grep -q '^NOPERM ' <<<"$output"
}

test "$(docker exec "$container" redis-cli ping 2>&1)" = "NOAUTH Authentication required."
test "$(auth_redis_cli ping)" = "PONG"
test "$(auth_redis_cli set session:acl-test ok EX 60)" = "OK"
test "$(auth_redis_cli get session:acl-test)" = "ok"
test "$(auth_redis_cli del session:acl-test)" = "1"

expect_noperm set unrelated:key denied
expect_noperm config get dir
expect_noperm flushall

echo "Redis ACL integration checks passed"
