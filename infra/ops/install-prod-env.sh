#!/usr/bin/env bash
set -euo pipefail

workdir=${1:?workdir required}
staged=${2:?staged environment required}
release_sha=${3:?release SHA required}
profiles=${4-}
root_env="$workdir/.env"
infra_env="$workdir/infra/.env"

file_mode() {
  if stat -c %a "$1" >/dev/null 2>&1; then
    stat -c %a "$1"
  else
    stat -f %Lp "$1"
  fi
}
if [[ ! -s "$staged" || ! -O "$staged" || $(file_mode "$staged") != 600 ||
      "$staged" != "$workdir"/.env.upload.* || ! -d "$workdir/infra" ]]; then
  echo 'Invalid staged production environment' >&2
  exit 1
fi

infra_staged=$(mktemp "$workdir/infra/.env.upload.XXXXXXXX")
root_backup=''
infra_backup=''
installed=false

cleanup() {
  status=$?
  trap - EXIT
  set +e
  if [[ $status -ne 0 && $installed == true ]]; then
    if [[ -n "$root_backup" ]]; then mv -f "$root_backup" "$root_env"; else rm -f "$root_env"; fi
    if [[ -n "$infra_backup" ]]; then mv -f "$infra_backup" "$infra_env"; else rm -f "$infra_env"; fi
  fi
  rm -f "$staged" "$infra_staged" "$root_backup" "$infra_backup"
  exit "$status"
}
trap cleanup EXIT

install -m 600 "$staged" "$infra_staged"
if [[ -e "$root_env" ]]; then
  root_backup=$(mktemp "$workdir/.env.backup.XXXXXXXX")
  cp -p "$root_env" "$root_backup"
fi
if [[ -e "$infra_env" ]]; then
  infra_backup=$(mktemp "$workdir/infra/.env.backup.XXXXXXXX")
  cp -p "$infra_env" "$infra_backup"
fi

# Compose resolves substitutions from infra/.env and services load ../.env.
export COMPOSE_FILE=docker-compose.yml:docker-compose.prod.yml
export COMPOSE_PROFILES="$profiles"
export TASKLY_IMAGE_TAG="$release_sha"
cd "$workdir/infra"

docker compose --env-file "$staged" config -q
installed=true
mv -f "$staged" "$root_env"
mv -f "$infra_staged" "$infra_env"
chmod 600 "$root_env" "$infra_env"
docker compose config -q
