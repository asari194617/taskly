#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
COMPOSE_FILE="$PROJECT_ROOT/infra/docker-compose.yml"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

docker compose --file "$COMPOSE_FILE" config --format json >"$tmp"

# Datastores and gRPC workers must only be reachable on the Compose network.
jq -e '
  . as $config
  | ["db", "redis", "email-compose"] as $internal
  | all($internal[]; (($config.services[.].ports // []) | length) == 0)
' "$tmp" >/dev/null

# Host Nginx and host-local monitoring use these published ports. They must
# never listen on every interface.
jq -e '
  . as $config
  | {
    server: [["3063", 8080], ["3064", 9464]],
    mcp: [["3066", 3066]],
    "airflow-webserver": [["3065", 3065]]
  } as $expected
  | all($expected | keys[] as $service
      | ($expected[$service]) as $ports
      | (($config.services[$service].ports // []) | length) == ($ports | length)
      and all($ports[] as $port
        | any($config.services[$service].ports[];
            .host_ip == "127.0.0.1"
            and .published == $port[0]
            and .target == $port[1])))
' "$tmp" >/dev/null

echo "Compose port exposure policy passed"
