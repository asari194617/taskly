#!/usr/bin/env bash
set -euo pipefail

slot=${1:?runner slot is required}
image=${TASKLY_RUNNER_IMAGE:-taskly-actions-runner:2.337.0-ci}

if [[ ! "$slot" =~ ^[1-4]$ ]]; then
  echo "Runner slot must be between 1 and 4" >&2
  exit 1
fi

docker_gid=$(stat -c '%g' /var/run/docker.sock)
# Share the complete runner directory (including externals and _temp) with
# sibling containers at an identical, per-job path. Never reuse a workspace.
workspace=$(mktemp -d /tmp/taskly-ci-slot-$slot.XXXXXX)
chmod 0755 "$workspace"
docker run --rm --user root --volume "$workspace:/workspace" --entrypoint sh "$image" \
  -c 'mkdir /workspace/runner; chown 1001:1001 /workspace/runner'
cleanup() {
  # Container jobs may leave root-owned files; remove only this job's directory.
  docker run --rm --user root --volume "$workspace:/cleanup" --entrypoint sh "$image" \
    -c 'find /cleanup -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +'
  rmdir "$workspace"
}
trap cleanup EXIT
trap 'exit 0' TERM INT

docker run --rm \
  --name "taskly-ci-slot-$slot" \
  --hostname "taskly-ci-slot-$slot" \
  --network host \
  --workdir "$workspace/runner" \
  --volume "$workspace:$workspace" \
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
