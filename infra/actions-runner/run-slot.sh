#!/usr/bin/env bash
set -euo pipefail

slot=${1:?runner slot is required}
image=${TASKLY_RUNNER_IMAGE:-taskly-actions-runner:2.337.0}

if [[ ! "$slot" =~ ^[1-4]$ ]]; then
  echo "Runner slot must be between 1 and 4" >&2
  exit 1
fi

docker_gid=$(stat -c '%g' /var/run/docker.sock)
exec docker run --rm \
  --name "taskly-ci-slot-$slot" \
  --hostname "taskly-ci-slot-$slot" \
  --label taskly.actions-runner=single-job \
  --cpus 3 \
  --memory 7g \
  --pids-limit 2048 \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --group-add "$docker_gid" \
  --mount "type=volume,source=taskly-runner-config-$slot,target=/run/runner-config,readonly" \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  "$image"
