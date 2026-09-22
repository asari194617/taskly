#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/ops"
cp "$root/infra/ops/migrate-airflow-db.sh" "$tmp/ops/"

cat > "$tmp/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [[ $1 == compose && $2 == ps ]]; then
  if [[ ${TEST_MODE:-} != fresh ]]; then echo "id-$4"; fi
elif [[ $1 == inspect ]]; then
  echo 'ghcr.io/asari194617/taskly/airflow:old-sha'
elif [[ $1 == compose && $2 == stop ]]; then
  echo stop >> "$TEST_LOG"
  [[ ${TEST_MODE:-} != stop-fail ]]
elif [[ $1 == compose && $2 == up ]]; then
  echo "up:${TASKLY_IMAGE_TAG}" >> "$TEST_LOG"
  [[ ${TEST_MODE:-} != up-fail || ${TASKLY_IMAGE_TAG} == old-sha ]]
else
  echo "Unexpected docker command: $*" >&2
  exit 1
fi
MOCK
cat > "$tmp/ops/prepare-airflow-db.sh" <<'MOCK'
#!/usr/bin/env bash
echo migrate >> "$TEST_LOG"
case ${TEST_MODE:-} in
  migrate-fail) exit 1 ;;
  cancelled) kill -TERM "$PPID"; sleep 1 ;;
esac
MOCK
chmod +x "$tmp/bin/docker" "$tmp/ops/prepare-airflow-db.sh"
export PATH="$tmp/bin:$PATH" TASKLY_IMAGE_TAG=new-sha TEST_LOG="$tmp/log"

run_case() {
  local mode=$1 expected_status=$2 expected_log=$3 status=0
  : > "$TEST_LOG"
  (cd "$tmp" && TEST_MODE=$mode ./ops/migrate-airflow-db.sh >/dev/null 2>&1) || status=$?
  if [[ $expected_status == success ]]; then
    [[ $status == 0 ]] || { echo "$mode unexpectedly failed: $status" >&2; exit 1; }
  else
    [[ $status != 0 ]] || { echo "$mode unexpectedly succeeded" >&2; exit 1; }
  fi
  diff -u <(printf '%s\n' "$expected_log") "$TEST_LOG"
}

run_case success success $'stop\nmigrate\nup:new-sha'
run_case fresh success $'migrate\nup:new-sha'
run_case stop-fail failure $'stop\nup:old-sha'
run_case migrate-fail failure $'stop\nmigrate\nup:new-sha'
run_case up-fail failure $'stop\nmigrate\nup:new-sha\nup:new-sha'
run_case cancelled failure $'stop\nmigrate\nup:new-sha'

# A second push waits for the first deploy; the normal deploy has no stop or
# metadata migration command. These assertions guard the workflow contract.
grep -q 'cancel-in-progress: false' "$root/.github/workflows/deploy-prod.yml"
! grep -q 'docker compose stop airflow-' "$root/.github/workflows/deploy-prod.yml"
! grep -q 'prepare-airflow-db.sh' "$root/.github/workflows/deploy-prod.yml"
echo 'Airflow deploy recovery tests passed'
