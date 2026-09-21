#!/usr/bin/env bash
set -euo pipefail

api=false
email_compose=false
mcp=false
airflow=false
all=false

if [[ "${1:-}" == "--all" ]]; then
  all=true
  shift
fi

for path in "$@"; do
  case "$path" in
    apps/server/*)
      api=true
      ;;
    apps/email-compose/*)
      email_compose=true
      ;;
    apps/mcp/*)
      mcp=true
      ;;
    batches/*|infra/airflow/*|.dockerignore)
      airflow=true
      ;;
    .github/workflows/compose-build.yml|.github/workflows/deploy-prod.yml|.github/scripts/image-matrix.sh|.github/scripts/test-image-matrix.sh)
      all=true
      ;;
  esac
done

if [[ "$all" == true ]]; then
  api=true
  email_compose=true
  mcp=true
  airflow=true
fi

matrix='[]'
if [[ "$api" == true ]]; then
  matrix=$(jq -c '. + [{"service":"api","context":"./apps/server","file":"./apps/server/Dockerfile","cache_scope":"api"}]' <<<"$matrix")
fi
if [[ "$email_compose" == true ]]; then
  matrix=$(jq -c '. + [{"service":"email-compose","context":"./apps/email-compose","file":"./apps/email-compose/Dockerfile","cache_scope":"email-compose"}]' <<<"$matrix")
fi
if [[ "$mcp" == true ]]; then
  matrix=$(jq -c '. + [{"service":"mcp","context":"./apps/mcp","file":"./apps/mcp/Dockerfile","cache_scope":"mcp"}]' <<<"$matrix")
fi
if [[ "$airflow" == true ]]; then
  matrix=$(jq -c '. + [{"service":"airflow","context":".","file":"./infra/airflow/Dockerfile","cache_scope":"airflow"}]' <<<"$matrix")
fi

jq -cn --argjson include "$matrix" '{include: $include}'
