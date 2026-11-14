#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
temp=$(mktemp -d)
trap 'rm -rf "$temp"' EXIT
mkdir -p "$temp/bin" "$temp/host/infra" "$temp/host/apps/server"
touch "$temp/host/infra/.env"
old=1111111111111111111111111111111111111111
new=2222222222222222222222222222222222222222
printf '%s\n' "$old" > "$temp/running-sha"
printf '%s\n' "$old" > "$temp/airflow-sha"
printf 'CREATE TABLE users (id bigint AUTO_INCREMENT=42 PRIMARY KEY);\n' > "$temp/schema"
printf '0019_email_suppressions\n0028_notification_delivery.sql\n' > "$temp/migrations"
printf 'CREATE TABLE synthetic_source (id bigint PRIMARY KEY);\n' > "$temp/host/apps/server/schema.sql"
printf 'env "taskly" {}\n' > "$temp/host/atlas.hcl"
cat > "$temp/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MOCK_CALLS"
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
      '{{.State.Status}}')
        if [[ ${MOCK_MISSING_SERVICE:-} == "$4" ]]; then printf 'exited\n'; else printf '%s\n' "${MOCK_STATUS:-running}"; fi ;;
      *) printf '%s\n' "${MOCK_HEALTH:-healthy}" ;;
    esac
    ;;
  *) echo "Unexpected Docker invocation: $*" >&2; exit 1 ;;
esac
MOCK
chmod +x "$temp/bin/docker"
export PATH="$temp/bin:$PATH" MOCK_SHA="$temp/running-sha" MOCK_AIRFLOW_SHA="$temp/airflow-sha" MOCK_SCHEMA="$temp/schema" MOCK_MIGRATIONS="$temp/migrations" MOCK_CALLS="$temp/docker-calls"
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

# Read-only baseline attestation requires a previously committed schema-v1 state.
digest=$(sed -E 's/AUTO_INCREMENT=[0-9]+ //g' "$temp/schema" | sha256sum | cut -d' ' -f1)
printf 'committed\n%s\n%s\n%s\nschema-v1\n' "$new" "$new" "$digest" > "$temp/host/.deploy-state"
chmod 600 "$temp/host/.deploy-state"
cp "$temp/host/.deploy-state" "$temp/expected-state"
bash "$guard" schema-baseline "$temp/host" "$old" > "$temp/baseline.json"
cmp "$temp/host/.deploy-state" "$temp/expected-state"
python3 - "$temp/baseline.json" "$new" "$old" "$digest" "$temp/host" <<'PY'
import hashlib,json,pathlib,sys
baseline=json.loads(pathlib.Path(sys.argv[1]).read_text())
assert baseline['version']==1
assert baseline['baseline_sha']==sys.argv[2]
assert baseline['target_sha']==sys.argv[3]
assert baseline['database_digest']==sys.argv[4]
root=pathlib.Path(sys.argv[5])
assert baseline['sources']=={name:hashlib.sha256((root/name).read_bytes()).hexdigest() for name in ['apps/server/schema.sql','atlas.hcl']}
PY

for issue in missing legacy in-progress bad-mode wrong-target missing-source symlink-source-parent unhealthy schema-drift; do
  cp "$temp/expected-state" "$temp/host/.deploy-state"
  chmod 600 "$temp/host/.deploy-state"
  case "$issue" in
    missing) rm "$temp/host/.deploy-state" ;;
    legacy) sed '$d' "$temp/expected-state" > "$temp/host/.deploy-state" ;;
    in-progress) sed '1s/committed/in_progress/' "$temp/expected-state" > "$temp/host/.deploy-state" ;;
    bad-mode) chmod 644 "$temp/host/.deploy-state" ;;
    wrong-target) sed "3s/$new/$old/" "$temp/expected-state" > "$temp/host/.deploy-state" ;;
    missing-source) mv "$temp/host/atlas.hcl" "$temp/atlas-backup" ;;
    symlink-source-parent) mv "$temp/host/apps" "$temp/outside-apps"; ln -s "$temp/outside-apps" "$temp/host/apps" ;;
    unhealthy) export MOCK_MISSING_SERVICE=db ;;
    schema-drift) printf 'CREATE TABLE unexpected_drift(id bigint);\n' >> "$temp/schema" ;;
  esac
  state_before=missing
  if [[ -f $temp/host/.deploy-state ]]; then state_before=$(sha256sum "$temp/host/.deploy-state" | cut -d' ' -f1); fi
  if bash "$guard" schema-baseline "$temp/host" "$old" > "$temp/output" 2>&1; then
    echo "Untrusted schema baseline accepted: $issue" >&2; exit 1
  fi
  if [[ $state_before == missing ]]; then
    [[ ! -e $temp/host/.deploy-state ]] || { echo 'Baseline probe created a deployment marker' >&2; exit 1; }
  else
    [[ $(sha256sum "$temp/host/.deploy-state" | cut -d' ' -f1) == "$state_before" ]] || { echo 'Baseline probe modified deployment state' >&2; exit 1; }
  fi
  case "$issue" in
    missing-source) mv "$temp/atlas-backup" "$temp/host/atlas.hcl" ;;
    symlink-source-parent) rm "$temp/host/apps"; mv "$temp/outside-apps" "$temp/host/apps" ;;
    unhealthy) unset MOCK_MISSING_SERVICE ;;
    schema-drift) sed '$d' "$temp/schema" > "$temp/schema.tmp"; mv "$temp/schema.tmp" "$temp/schema" ;;
  esac
done

cp "$temp/expected-state" "$temp/host/.deploy-state"
chmod 600 "$temp/host/.deploy-state"
for values in "$old $digest" "$new $(printf '%064d' 0)"; do
  read -r expected_sha expected_digest <<< "$values"
  if bash "$guard" begin "$temp/host" "$old" "$expected_sha" "$expected_digest" > "$temp/output" 2>&1; then
    echo 'A changed baseline between attestation and begin was accepted' >&2; exit 1
  fi
  cmp "$temp/host/.deploy-state" "$temp/expected-state"
done
bash "$guard" begin "$temp/host" "$old" "$new" "$digest"
grep -q '^in_progress$' "$temp/host/.deploy-state"
cp "$temp/host/.deploy-state" "$temp/expected-in-progress"
printf '%s\n' "$old" > "$temp/running-sha"
printf 'CREATE TABLE accidental_schema_change(id bigint);\n' >> "$temp/schema"
if bash "$guard" complete "$temp/host" "$old" "$new" "$digest" > "$temp/output" 2>&1; then
  echo 'A code-only release accepted an unexpected schema change at completion' >&2; exit 1
fi
cmp "$temp/host/.deploy-state" "$temp/expected-in-progress"
sed '$d' "$temp/schema" > "$temp/schema.tmp"; mv "$temp/schema.tmp" "$temp/schema"
bash "$guard" complete "$temp/host" "$old" "$new" "$digest"
grep -q '^committed$' "$temp/host/.deploy-state"
if grep -Eq '^compose (up|run|pull|rm|stop)|atlas' "$temp/docker-calls"; then
  echo 'The state guard invoked a mutating Docker or Atlas command' >&2; exit 1
fi
echo 'Production deployment state guard tests passed'
