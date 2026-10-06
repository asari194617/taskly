#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
temp=$(mktemp -d)
trap 'rm -rf "$temp"' EXIT
mkdir -p "$temp/bin" "$temp/host/infra"
touch "$temp/host/infra/.env"
old=1111111111111111111111111111111111111111
new=2222222222222222222222222222222222222222
printf '%s\n' "$old" > "$temp/running-sha"
printf '%s\n' "$old" > "$temp/airflow-sha"
printf 'CREATE TABLE users (id bigint AUTO_INCREMENT=42 PRIMARY KEY);\n' > "$temp/schema"
printf '0019_email_suppressions\n0028_notification_delivery.sql\n' > "$temp/migrations"
cat > "$temp/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
case "$1 $2" in
  'compose ps')
    printf '%s\n' "${*: -1}"
    ;;
  'compose exec')
    if [[ $* == *mysqldump* ]]; then cat "$MOCK_SCHEMA"; else cat "$MOCK_MIGRATIONS"; fi
    ;;
  'inspect --format')
    case "$3" in
      '{{.Config.Image}}')
        if [[ $4 == airflow-dag-processor && -n ${MOCK_AIRFLOW_DAG_SHA:-} ]]; then
          printf 'ghcr.io/asari194617/taskly/airflow:%s\n' "$MOCK_AIRFLOW_DAG_SHA"
        elif [[ $4 == airflow-* ]]; then
          printf 'ghcr.io/asari194617/taskly/airflow:%s\n' "$(cat "$MOCK_AIRFLOW_SHA")"
        else
          printf 'ghcr.io/asari194617/taskly/api:%s\n' "$(cat "$MOCK_SHA")"
        fi
        ;;
      '{{.State.Status}}') printf '%s\n' "${MOCK_STATUS:-running}" ;;
      *) printf 'healthy\n' ;;
    esac
    ;;
  *) echo "Unexpected Docker invocation: $*" >&2; exit 1 ;;
esac
MOCK
chmod +x "$temp/bin/docker"
export PATH="$temp/bin:$PATH" MOCK_SHA="$temp/running-sha" MOCK_AIRFLOW_SHA="$temp/airflow-sha" MOCK_SCHEMA="$temp/schema" MOCK_MIGRATIONS="$temp/migrations"
guard="$root/infra/ops/deploy-state.sh"

# Upgrade the four-line migration-history state used before Atlas.
legacy_digest=$(sha256sum "$temp/migrations" | cut -d' ' -f1)
printf 'committed\n%s\n%s\n%s\n' "$old" "$old" "$legacy_digest" > "$temp/host/.deploy-state"

# Interrupted after stopping Airflow: the marker prevents a fresh deploy.
bash "$guard" begin "$temp/host" "$new"
export MOCK_STATUS=exited
if bash "$guard" begin "$temp/host" "$new" >"$temp/output" 2>&1; then
  echo 'Interrupted deployment was accepted' >&2; exit 1
fi
grep -q 'unfinished' "$temp/output"
grep -q 'current images: unhealthy or inconsistent:' "$temp/output"
grep -q 'current schema:' "$temp/output"
unset MOCK_STATUS

# A schema change before failure keeps a later run from proceeding.
printf 'CREATE TABLE subjects (id bigint PRIMARY KEY);\n' >> "$temp/schema"
if bash "$guard" begin "$temp/host" "$new" >"$temp/output" 2>&1; then
  echo 'Interrupted migration was accepted' >&2; exit 1
fi
grep -q 'unfinished' "$temp/output"
if bash "$guard" complete "$temp/host" "$new" >"$temp/output" 2>&1; then
  echo 'Wrong running image was committed' >&2; exit 1
fi
grep -q 'differs from target' "$temp/output"

# A normal release updates the core applications while keeping healthy Airflow
# containers on their previous release. Completion must record that state.
printf '%s\n' "$new" > "$temp/running-sha"
bash "$guard" complete "$temp/host" "$new"

# The three Airflow services must still be healthy and use one release.
if MOCK_AIRFLOW_DAG_SHA=3333333333333333333333333333333333333333 \
  bash "$guard" begin "$temp/host" "$old" >"$temp/output" 2>&1; then
  echo 'Mixed Airflow images were accepted' >&2; exit 1
fi
grep -q 'Airflow images have mixed release SHAs' "$temp/output"

printf 'CREATE TABLE unexpected (id bigint PRIMARY KEY);\n' >> "$temp/schema"
if bash "$guard" begin "$temp/host" "$old" >"$temp/output" 2>&1; then
  echo 'Schema drift was accepted' >&2; exit 1
fi
grep -q 'differs from the last successful deployment' "$temp/output"
sed '$d' "$temp/schema" > "$temp/schema.tmp"
mv "$temp/schema.tmp" "$temp/schema"
bash "$guard" begin "$temp/host" "$old"
echo 'Production deployment state guard tests passed'
