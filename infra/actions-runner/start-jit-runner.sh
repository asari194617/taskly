#!/usr/bin/env bash
set -euo pipefail

image=${TASKLY_RUNNER_IMAGE:-taskly-actions-runner:2.337.0}
runner_name=${1:?runner name is required}

if [[ ! "$runner_name" =~ ^taskly-ci-[a-z0-9-]+$ ]]; then
  echo "Invalid runner name: $runner_name" >&2
  exit 1
fi
if docker container inspect "$runner_name" >/dev/null 2>&1; then
  echo "Runner container already exists: $runner_name" >&2
  exit 1
fi
if ! docker image inspect "$image" >/dev/null 2>&1; then
  echo "Runner image is unavailable: $image" >&2
  exit 1
fi

secret_dir=$(mktemp -d /tmp/taskly-runner-jit.XXXXXX)
trap 'rm -rf "$secret_dir"' EXIT
chmod 700 "$secret_dir"
cat > "$secret_dir/config"
# The container runs as uid 1001. The parent directory stays mode 0700 on the
# host, while the bind-mounted file must be readable by that container user.
chmod 444 "$secret_dir/config"

docker_gid=$(stat -c '%g' /var/run/docker.sock)
docker_lifecycle=(--rm)
if [[ ${TASKLY_RUNNER_KEEP_FAILED:-0} == 1 ]]; then
  docker_lifecycle=()
fi
docker run --detach "${docker_lifecycle[@]}" \
  --name "$runner_name" \
  --hostname "$runner_name" \
  --label taskly.actions-runner=ephemeral \
  --cpus 3 \
  --memory 7g \
  --pids-limit 2048 \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --group-add "$docker_gid" \
  --mount type=bind,source="$secret_dir/config",target=/run/secrets/jit-config,readonly \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  "$image" >/dev/null

for _ in $(seq 1 20); do
  runner_logs=$(docker logs "$runner_name" 2>&1 || true)
  if grep -q 'Listening for Jobs' <<<"$runner_logs"; then
    echo "Started ephemeral runner $runner_name"
    exit 0
  fi
  if ! docker container inspect "$runner_name" >/dev/null 2>&1; then
    echo "Runner container exited during startup: $runner_name" >&2
    printf '%s\n' "$runner_logs" >&2
    exit 1
  fi
  sleep 1
done

echo "Runner did not become ready: $runner_name" >&2
docker logs "$runner_name" >&2 || true
exit 1
