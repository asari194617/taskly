#!/usr/bin/env bash
set -euo pipefail

network=
cleanup_tool() {
  if [[ -n "$network" ]]; then
    docker rm -f "$network-dev" >/dev/null 2>&1 || true
    docker network rm "$network" >/dev/null 2>&1 || true
  fi
}
# shellcheck source=.github/scripts/atlas-pr-common.sh
source "$(dirname "${BASH_SOURCE[0]}")/atlas-pr-common.sh"

command=${1:?Expected plan or apply}
[[ -n "${ATLAS_DB_USER:-}" && -n "${ATLAS_DB_PASSWORD:-}" ]] ||
  fail 'Configure the Atlas MySQL account in the GitHub Environment.'
port=${ATLAS_DB_PORT:-3306}
[[ "${ATLAS_DB_HOST:-}" =~ ^[A-Za-z0-9][A-Za-z0-9.-]*$ &&
   "$port" =~ ^[1-9][0-9]{0,4}$ && "$port" -le 65535 &&
   "${ATLAS_DB_NAME:-}" =~ ^[A-Za-z0-9_]+$ ]] ||
  fail 'Configure the Atlas MySQL host, port and database in the GitHub Environment.'

schema="$GITHUB_WORKSPACE/proposal/apps/server/schema.sql"
[[ -f "$schema" && "$(realpath "$schema")" == "$(realpath "$GITHUB_WORKSPACE")/proposal/apps/server/schema.sql" ]] ||
  fail 'The PR must provide apps/server/schema.sql as a regular file.'

mkdir -p "$temp/workspace/apps/server"
cp "$GITHUB_WORKSPACE/workflow/atlas.hcl" "$temp/workspace/"
cp "$schema" "$temp/workspace/apps/server/"
if [[ -n "${ATLAS_DB_CA:-}" ]]; then
  printf '%s\n' "$ATLAS_DB_CA" >"$temp/workspace/ca.pem"
else
  cp /etc/ssl/certs/ca-certificates.crt "$temp/workspace/ca.pem"
fi
user=$(jq -nr 'env.ATLAS_DB_USER | @uri')
password=$(jq -nr 'env.ATLAS_DB_PASSWORD | @uri')
target="mysql://$user:$password@$ATLAS_DB_HOST:$port/$ATLAS_DB_NAME?tls=true"

check_revision
if [[ "$command" == 'apply' ]]; then
  gpg_plan --decrypt --output "$temp/approved.sql" "$artifact/plan.sql.gpg" ||
    fail 'Could not decrypt the saved Atlas plan. Check ATLAS_PLAN_KEY and request a new plan.'
  gpg_plan --decrypt --output "$temp/approved-target" "$artifact/target.gpg" ||
    fail 'Could not verify the saved Atlas target. Request a new plan.'
fi

network="taskly-atlas-${GITHUB_RUN_ID:-local}-$$"
docker network create "$network" >"$temp/docker.log" 2>&1 ||
  fail 'Could not create the Atlas Docker network.'
docker run -d --name "$network-dev" --network "$network" --network-alias atlas-dev \
  --tmpfs /var/lib/mysql -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=atlas_dev \
  mysql:8.4 >>"$temp/docker.log" 2>&1 ||
  fail 'Could not start the disposable MySQL database for Atlas.'
ready=false
for ((attempt = 0; attempt < 60; attempt++)); do
  if docker exec "$network-dev" mysql --protocol=TCP -h 127.0.0.1 -uroot -N -e 'SELECT 1' atlas_dev \
    >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
[[ "$ready" == true ]] || fail 'The Atlas dev database did not become ready.'

run_mysql() {
  MYSQL_PWD="$ATLAS_DB_PASSWORD" docker run --rm -i --network "$network" \
    -v "$temp/workspace/ca.pem:/ca.pem:ro" -e MYSQL_PWD mysql:8.4 \
    mysql --protocol=TCP --connect-timeout=10 --ssl-mode=VERIFY_IDENTITY --ssl-ca=/ca.pem \
    --host="$ATLAS_DB_HOST" --port="$port" --user="$ATLAS_DB_USER" --database="$ATLAS_DB_NAME" \
    --binary-mode --batch --skip-column-names --skip-reconnect "$@"
}

run_atlas() {
  docker run --rm --network "$network" -v "$temp/workspace:/workspace:ro" -w /workspace \
    -e SSL_CERT_FILE=/workspace/ca.pem -e ATLAS_DEV_URL=mysql://root@atlas-dev:3306/atlas_dev \
    arigaio/atlas:1.3.3-community-alpine "$@"
}

# Bind the plan to the MySQL instance and schema as well as the PR.
run_mysql -e 'SELECT @@server_uuid, DATABASE()' >"$temp/current-target" 2>"$temp/mysql.log" ||
  fail 'Could not connect to the Atlas target over verified TLS.'
if [[ "$command" == 'apply' ]]; then
  cmp -s "$temp/approved-target" "$temp/current-target" ||
    fail 'The Atlas target differs from the saved plan.'
fi

run_atlas schema apply --env taskly --url "$target" --dry-run \
  --format '{{ range .Changes.Pending }}{{ .Cmd }};{{ "\n" }}{{ end }}' \
  >"$temp/current.sql" 2>"$temp/atlas.log" ||
  fail 'Atlas could not calculate the schema diff. Check the module and database privately.'

if grep -Eiq '\b(DROP|MODIFY[[:space:]]+COLUMN|CHANGE[[:space:]]+COLUMN|RENAME)\b' "$temp/current.sql"; then
  fail 'The Atlas plan contains a destructive change. Use the schema maintenance procedure.'
fi

case "$command" in
  plan)
    if [[ ! -s "$temp/current.sql" ]]; then
      mark_no_changes
      exit 0
    fi
    check_revision
    gpg_plan --symmetric --cipher-algo AES256 --output "$artifact/plan.sql.gpg" "$temp/current.sql" ||
      fail 'Could not encrypt the Atlas plan.'
    gpg_plan --symmetric --cipher-algo AES256 --output "$artifact/target.gpg" "$temp/current-target" ||
      fail 'Could not encrypt the Atlas target.'
    echo 'changes=true' >>"$GITHUB_OUTPUT"
    ;;
  apply)
    cmp -s "$temp/approved.sql" "$temp/current.sql" ||
      fail 'The Atlas schema diff changed. Request and review a new plan.'
    check_revision
    run_mysql <"$temp/approved.sql" >"$temp/apply.log" 2>&1 ||
      fail 'Atlas SQL apply failed. Inspect the database for partial DDL before requesting a new plan.'
    ;;
  *) fail 'Expected plan or apply.' ;;
esac
