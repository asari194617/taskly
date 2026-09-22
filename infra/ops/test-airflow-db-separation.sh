#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
config=$(docker compose --env-file "$root/.env.example" --file "$root/infra/docker-compose.yml" config --format json)
jq -e '
  . as $c
  | ($c.services.db.environment.MYSQL_DATABASE != $c.services["airflow-webserver"].environment.AIRFLOW_META_DATABASE)
  and ($c.services.db.environment.MYSQL_USER != $c.services["airflow-webserver"].environment.AIRFLOW_META_USER)
  and (["airflow-webserver", "airflow-scheduler", "airflow-dag-processor"] | all(.[]; . as $service |
    ($c.services[$service].environment.AIRFLOW__DATABASE__SQL_ALCHEMY_CONN | endswith("/" + $c.services[$service].environment.AIRFLOW_META_DATABASE))
    and $c.services[$service].environment.MYSQL_DATABASE == $c.services.db.environment.MYSQL_DATABASE
    and $c.services[$service].environment.MYSQL_ROOT_PASSWORD == ""
    and all($c.services[$service].volumes[]; .target != "/opt/airflow/.env" and .target != "/workspace")))
  and (["server", "mcp", "email-compose"] | all(.[]; . as $service |
    $c.services[$service].environment.AIRFLOW_META_PASSWORD == ""
    and $c.services[$service].environment.MYSQL_ROOT_PASSWORD == ""))
' <<<"$config" >/dev/null

echo 'Airflow database separation configuration passed'
