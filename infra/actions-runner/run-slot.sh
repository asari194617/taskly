#!/usr/bin/env bash
set -euo pipefail

slot=${1:?runner slot is required}
image=${TASKLY_RUNNER_IMAGE:-taskly-actions-runner:2.337.0-ci}
runner_cpus=${TASKLY_RUNNER_CPUS:-3}
runner_memory=${TASKLY_RUNNER_MEMORY:-7g}
runner_pids_limit=${TASKLY_RUNNER_PIDS_LIMIT:-2048}

case "$slot" in
  [1-4]) container_name="taskly-ci-slot-$slot" ;;
  prod) container_name=taskly-prod ;;
  *)
    echo "Runner slot must be between 1 and 4, or prod" >&2
    exit 1
    ;;
esac

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

runner_args=(
  --rm
  --name "$container_name"
  --hostname "$container_name"
  --network host
  --workdir "$workspace/runner"
  --volume "$workspace:$workspace"
  --label taskly.actions-runner=single-job
  --cap-drop ALL
  --security-opt no-new-privileges:true
  --group-add "$docker_gid"
  --mount "type=volume,source=taskly-runner-config-$slot,target=/run/runner-config,readonly"
  --volume /var/run/docker.sock:/var/run/docker.sock
)
if [[ -n "$runner_cpus" ]]; then runner_args+=(--cpus "$runner_cpus"); fi
if [[ -n "$runner_memory" ]]; then runner_args+=(--memory "$runner_memory"); fi
if [[ -n "$runner_pids_limit" ]]; then runner_args+=(--pids-limit "$runner_pids_limit"); fi

docker run "${runner_args[@]}" \
  "$image"
