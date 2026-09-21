#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
matrix_script="$script_dir/image-matrix.sh"

assert_services() {
  local expected=$1
  shift
  local actual
  actual=$("$matrix_script" "$@" | jq -r '[.include[].service] | sort | join(",")')
  if [[ "$actual" != "$expected" ]]; then
    echo "expected services '$expected', got '$actual' for paths: $*" >&2
    exit 1
  fi
}

assert_services "mcp" apps/mcp/package.json
assert_services "airflow" batches/go.mod
assert_services "" infra/docker-compose.yml
assert_services "api,email-compose" apps/server/main.go apps/email-compose/package.json apps/server/Dockerfile
assert_services "airflow,api,email-compose,mcp" .github/workflows/deploy-prod.yml
assert_services "airflow,api,email-compose,mcp" --all

echo "image matrix tests passed"
