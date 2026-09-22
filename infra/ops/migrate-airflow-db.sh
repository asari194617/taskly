#!/usr/bin/env bash
# Run from infra/ with COMPOSE_FILE and TASKLY_IMAGE_TAG set. This is an
# explicit maintenance operation, never part of the ordinary deploy.
set -Eeuo pipefail

services=(airflow-webserver airflow-scheduler airflow-dag-processor)
: "${TASKLY_IMAGE_TAG:?set the target release SHA}"

web_id=$(docker compose ps -q airflow-webserver)
previous_image=
previous_tag=$TASKLY_IMAGE_TAG
if [[ -n $web_id ]]; then
  previous_image=$(docker inspect --format '{{.Config.Image}}' "$web_id")
  previous_tag=${previous_image##*:}
  [[ $previous_tag != "$previous_image" ]] || { echo 'Cannot determine previous Airflow image tag' >&2; exit 1; }
fi
for service in "${services[@]:1}"; do
  id=$(docker compose ps -q "$service")
  if [[ -z $web_id ]]; then
    [[ -z $id ]] || { echo 'Airflow is partially installed' >&2; exit 1; }
    continue
  fi
  [[ -n $id ]] || { echo "$service is missing" >&2; exit 1; }
  image=$(docker inspect --format '{{.Config.Image}}' "$id")
  [[ $image == "$previous_image" ]] || { echo 'Airflow services use different images' >&2; exit 1; }
done

target_tag=$TASKLY_IMAGE_TAG
recovery_tag=$previous_tag
stopping=0
recovered=0
recover() {
  local result=$?
  trap - EXIT HUP INT TERM
  if (( stopping && ! recovered )); then
    echo "Restoring Airflow services with image tag $recovery_tag" >&2
    if ! TASKLY_IMAGE_TAG=$recovery_tag docker compose up -d --no-deps --no-build --wait --wait-timeout 600 "${services[@]}"; then
      echo '::error::Airflow recovery failed; check all three services immediately' >&2
      docker compose ps "${services[@]}" >&2 || true
      result=1
    fi
  fi
  exit "$result"
}
trap recover EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

# Set the recovery guard before stop, including partial-stop failures.
stopping=1
if [[ -n $web_id ]]; then
  docker compose stop "${services[@]}"
fi
# prepare-airflow-db.sh may move tables or upgrade metadata before failing.
# Once it starts, the old image is not guaranteed to understand the schema.
recovery_tag=$target_tag
./ops/prepare-airflow-db.sh </dev/null
docker compose up -d --no-deps --no-build --wait --wait-timeout 600 "${services[@]}"
recovered=1
echo 'Airflow metadata migration completed and all three services are healthy'
