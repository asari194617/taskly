#!/usr/bin/env bash
set -euo pipefail

slot=${1:?runner slot is required}
case "$slot" in
  [1-4]) container_name="taskly-ci-slot-$slot" ;;
  prod) container_name=taskly-prod ;;
  *)
    echo "Runner slot must be between 1 and 4, or prod" >&2
    exit 1
    ;;
esac

docker stop --timeout 10 "$container_name"
