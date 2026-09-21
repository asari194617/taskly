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
transaction_key=auth_transaction:acl-test
lock_key="$transaction_key:lock"
test "$(auth_redis_cli set "$transaction_key" pending EX 60)" = "OK"
test "$(auth_redis_cli get "$transaction_key")" = "pending"
test "$(auth_redis_cli set "$lock_key" lock-token NX EX 10)" = "OK"
compare_and_delete='if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end'
test "$(auth_redis_cli eval "$compare_and_delete" 1 "$lock_key" wrong-token)" = "0"
test "$(auth_redis_cli get "$lock_key")" = "lock-token"
test "$(auth_redis_cli eval "$compare_and_delete" 1 "$lock_key" lock-token)" = "1"
test "$(auth_redis_cli del "$transaction_key")" = "1"

expect_noperm set unrelated:key denied
expect_noperm eval "$compare_and_delete" 1 unrelated:key lock-token
auth_redis_cli eval 'return redis.call("FLUSHALL")' 0 | grep -q 'ACL failure in script'
expect_noperm config get dir
expect_noperm flushall

echo "Redis ACL integration checks passed"
