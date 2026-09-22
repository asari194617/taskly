#!/usr/bin/env bash
# Final failure diagnostic, separate from the deploy SSH session.
set -euo pipefail

if [[ -z ${PROD_HOST:-} || -z ${PROD_USER:-} || -z ${PROD_WORKDIR:-} ]]; then
  echo '::error::Airflow status could not be checked: production SSH settings are missing'
  exit 1
fi

if ssh -o BatchMode=yes -p "${PROD_SSH_PORT:-22}" "$PROD_USER@$PROD_HOST" bash -s -- "$PROD_WORKDIR" <<'REMOTE'
set -euo pipefail
cd "$1/infra"
export COMPOSE_FILE=docker-compose.yml:docker-compose.prod.yml
services=(airflow-webserver airflow-scheduler airflow-dag-processor)
failed=0
for service in "${services[@]}"; do
  id=$(docker compose ps -aq "$service")
  if [[ -z $id ]]; then
    echo "$service: missing"
    failed=1
    continue
  fi
  state=$(docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' "$id")
  echo "$service: $state"
  [[ $state == 'running healthy' ]] || failed=1
done
exit "$failed"
REMOTE
then
  echo 'Airflow services are running and healthy after the failed deploy' >> "${GITHUB_STEP_SUMMARY:-/dev/null}"
else
  echo '::error::Airflow is stopped, unhealthy, or could not be inspected after the failed deploy'
  echo 'Airflow requires immediate inspection after this deploy failure.' >> "${GITHUB_STEP_SUMMARY:-/dev/null}"
  exit 1
fi
