#!/usr/bin/env bash
# Run from the release checkout over SSH before any host write, and after health checks.
set -euo pipefail

action=${1:?begin or complete required}
workdir=${2:?production workdir required}
release=${3:?release SHA required}
[[ $release =~ ^[0-9a-f]{40}$ ]] || { echo 'Invalid release SHA' >&2; exit 1; }
[[ -d $workdir/infra && -f $workdir/infra/.env ]] || {
  echo 'Production Compose directory or environment is missing' >&2; exit 1;
}
state_file=$workdir/.deploy-state
cd "$workdir/infra"
export COMPOSE_FILE=docker-compose.yml:docker-compose.prod.yml

fail() {
  echo "Production deployment guard: $*; inspect the host and follow docs/operations/recovery-drill.md before retrying" >&2
  exit 1
}

# Use the running container image, never the release tag from an environment file.
running_sha() {
  local service=$1 id image sha status health
  id=$(docker compose ps -q "$service") || fail "cannot inspect $service"
  [[ -n $id ]] || fail "$service has no container"
  image=$(docker inspect --format '{{.Config.Image}}' "$id") || fail "cannot inspect $service image"
  [[ $image =~ ^ghcr\.io/asari194617/taskly/[^:]+:([0-9a-f]{40})$ ]] || fail "$service has an unexpected image: $image"
  sha=${BASH_REMATCH[1]}
  status=$(docker inspect --format '{{.State.Status}}' "$id") || fail "cannot inspect $service status"
  health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$id") || fail "cannot inspect $service health"
  [[ $status == running && ( $health == healthy || $health == none ) ]] || fail "$service is $status/$health"
  printf '%s\n' "$sha"
}

application_sha() {
  local expected='' actual service
  for service in server email-compose mcp; do
    actual=$(running_sha "$service") || return 1
    [[ -z $expected || $actual == "$expected" ]] || fail "core application images have mixed release SHAs"
    expected=$actual
  done
  printf '%s\n' "$expected"
}

airflow_sha() {
  local expected='' actual service
  for service in airflow-webserver airflow-scheduler airflow-dag-processor; do
    actual=$(running_sha "$service") || return 1
    [[ -z $expected || $actual == "$expected" ]] || fail "Airflow images have mixed release SHAs"
    expected=$actual
  done
  printf '%s\n' "$expected"
}

deployment_images() {
  local application airflow
  application=$(application_sha) || return 1
  airflow=$(airflow_sha) || return 1
  printf 'application=%s airflow=%s\n' "$application" "$airflow"
}

schema_digest() {
  local schema
  schema=$(docker compose exec -T db sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --no-data --no-tablespaces --skip-comments --skip-dump-date "$MYSQL_DATABASE"' </dev/null) || fail 'cannot inspect application schema'
  [[ -n $schema ]] || fail 'application schema is empty'
  # AUTO_INCREMENT reflects row inserts, not schema changes.
  schema=$(printf '%s\n' "$schema" | sed -E 's/AUTO_INCREMENT=[0-9]+ //g')
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "$schema" | sha256sum | cut -d' ' -f1
  else
    printf '%s\n' "$schema" | shasum -a 256 | cut -d' ' -f1
  fi
}

legacy_migration_digest() {
  local names
  names=$(docker compose exec -T db sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot --batch --skip-column-names "$MYSQL_DATABASE" -e "SELECT name FROM migrations ORDER BY name"' </dev/null) || fail 'cannot read legacy application migrations'
  [[ -n $names ]] || fail 'legacy application migrations are empty'
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "$names" | sha256sum | cut -d' ' -f1
  else
    printf '%s\n' "$names" | shasum -a 256 | cut -d' ' -f1
  fi
}

read_state() {
  [[ -f $state_file && ! -L $state_file ]] || fail 'deployment state is missing or a symlink'
  state=()
  while IFS= read -r line; do state+=("$line"); done < "$state_file"
  [[ ${#state[@]} == 4 || ( ${#state[@]} == 5 && ${state[4]} == schema-v1 ) ]] || fail 'deployment state is malformed'
  [[ ${state[0]} == committed || ${state[0]} == in_progress ]] || fail 'deployment state has an invalid phase'
  [[ ${state[1]} =~ ^[0-9a-f]{40}$ && ${state[2]} =~ ^[0-9a-f]{40}$ && ${state[3]} =~ ^[0-9a-f]{64}$ ]] || fail 'deployment state has invalid values'
  state_version=legacy-migrations
  [[ ${#state[@]} == 4 ]] || state_version=${state[4]}
}

write_state() {
  local temp
  temp=$(mktemp "$workdir/.deploy-state.XXXXXXXX")
  chmod 600 "$temp"
  printf '%s\n%s\n%s\n%s\nschema-v1\n' "$1" "$2" "$3" "$4" > "$temp"
  mv -f "$temp" "$state_file"
}

case "$action" in
  begin)
    if [[ -e $state_file ]]; then
      read_state
      if [[ ${state[0]} == in_progress ]]; then
        # A failed run may have stopped Airflow or changed the schema. Show the
        # observed state, but never infer that an automatic retry is safe.
        observed_sha=$(deployment_images 2>&1) || observed_sha="unhealthy or inconsistent: $observed_sha"
        observed_schema=$(schema_digest 2>&1) || observed_schema="unavailable: $observed_schema"
        fail "previous deployment of ${state[2]} is unfinished (last known SHA ${state[1]}, current images: $observed_sha, current schema: $observed_schema)"
      fi
    fi
    current=$(application_sha)
    airflow=$(airflow_sha)
    digest=$(schema_digest)
    if [[ -e $state_file ]]; then
      recorded_digest=$digest
      if [[ $state_version == legacy-migrations ]]; then
        recorded_digest=$(legacy_migration_digest)
      fi
      [[ $current == "${state[1]}" && $recorded_digest == "${state[3]}" ]] || fail 'running SHA or database schema differs from the last successful deployment'
    fi
    write_state in_progress "$current" "$release" "$digest"
    echo "Deployment started: current=$current target=$release airflow=$airflow schema_digest=$digest"
    ;;
  complete)
    read_state
    [[ ${state[0]} == in_progress && ${state[2]} == "$release" ]] || fail 'deployment marker does not match this release'
    current=$(application_sha)
    [[ $current == "$release" ]] || fail "running SHA $current differs from target $release"
    airflow=$(airflow_sha)
    digest=$(schema_digest)
    write_state committed "$release" "$release" "$digest"
    echo "Deployment complete: sha=$release airflow=$airflow schema_digest=$digest"
    ;;
  *) echo 'Expected begin or complete' >&2; exit 2 ;;
esac
