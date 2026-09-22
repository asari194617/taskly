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
printf '0019_email_suppressions\n0028_notification_delivery.sql\n' > "$temp/migrations"
cat > "$temp/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
case "$1 $2" in
  'compose ps')
    printf '%s\n' "${*: -1}"
    ;;
  'compose exec')
    cat "$MOCK_MIGRATIONS"
    ;;
  'inspect --format')
    case "$3" in
      '{{.Config.Image}}') printf 'ghcr.io/asari194617/taskly/api:%s\n' "$(cat "$MOCK_SHA")" ;;
      '{{.State.Status}}') printf '%s\n' "${MOCK_STATUS:-running}" ;;
      *) printf 'healthy\n' ;;
    esac
    ;;
  *) echo "Unexpected Docker invocation: $*" >&2; exit 1 ;;
esac
MOCK
chmod +x "$temp/bin/docker"
export PATH="$temp/bin:$PATH" MOCK_SHA="$temp/running-sha" MOCK_MIGRATIONS="$temp/migrations"
guard="$root/infra/ops/deploy-state.sh"

# Historic production records omit .sql; malformed names must still fail.
printf 'invalid migration\n' >> "$temp/migrations"
if bash "$guard" begin "$temp/host" "$new" >"$temp/output" 2>&1; then
  echo 'Malformed migration name was accepted' >&2; exit 1
fi
grep -q 'unexpected format' "$temp/output"
sed '$d' "$temp/migrations" > "$temp/migrations.tmp"
mv "$temp/migrations.tmp" "$temp/migrations"

# Interrupted after stopping Airflow: the marker prevents a fresh deploy.
bash "$guard" begin "$temp/host" "$new"
export MOCK_STATUS=exited
if bash "$guard" begin "$temp/host" "$new" >"$temp/output" 2>&1; then
  echo 'Interrupted deployment was accepted' >&2; exit 1
fi
grep -q 'unfinished' "$temp/output"
grep -q 'current images: unhealthy or inconsistent:' "$temp/output"
grep -q 'current migrations:' "$temp/output"
unset MOCK_STATUS

# A migration was recorded before failure. A later run still cannot proceed.
printf '0029_new_schema.sql\n' >> "$temp/migrations"
if bash "$guard" begin "$temp/host" "$new" >"$temp/output" 2>&1; then
  echo 'Interrupted migration was accepted' >&2; exit 1
fi
grep -q 'unfinished' "$temp/output"
if bash "$guard" complete "$temp/host" "$new" >"$temp/output" 2>&1; then
  echo 'Wrong running image was committed' >&2; exit 1
fi
grep -q 'differs from target' "$temp/output"

# Once all services run the new release, completion records its migration state.
printf '%s\n' "$new" > "$temp/running-sha"
bash "$guard" complete "$temp/host" "$new"
printf '0030_unexpected.sql\n' >> "$temp/migrations"
if bash "$guard" begin "$temp/host" "$old" >"$temp/output" 2>&1; then
  echo 'Migration drift was accepted' >&2; exit 1
fi
grep -q 'differs from the last successful deployment' "$temp/output"
sed '$d' "$temp/migrations" > "$temp/migrations.tmp"
mv "$temp/migrations.tmp" "$temp/migrations"
bash "$guard" begin "$temp/host" "$old"
echo 'Production deployment state guard tests passed'
