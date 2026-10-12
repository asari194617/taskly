#!/usr/bin/env bash
set -euo pipefail

slot=${1:?runner slot is required}
image=${TASKLY_RUNNER_IMAGE:-taskly-actions-runner:2.337.0-ci}
repository_url=${TASKLY_REPOSITORY_URL:-https://github.com/asari194617/taskly}

case "$slot" in
  [1-4])
    runner_name="taskly-ci-slot-$slot"
    runner_labels=taskly-ci
    ;;
  prod)
    runner_name=taskly-prod
    runner_labels=taskly-prod
    ;;
  *)
    echo "Runner slot must be between 1 and 4, or prod" >&2
    exit 1
    ;;
esac

secret_dir=$(mktemp -d /tmp/taskly-runner-registration.XXXXXX)
trap 'rm -rf "$secret_dir"' EXIT
chmod 700 "$secret_dir"
cat > "$secret_dir/token"
chmod 444 "$secret_dir/token"

volume="taskly-runner-config-$slot"
docker volume create "$volume" >/dev/null
docker run --rm \
  --user root \
  --env RUNNER_ALLOW_RUNASROOT=1 \
  --env RUNNER_NAME="$runner_name" \
  --env RUNNER_LABELS="$runner_labels" \
  --env REPOSITORY_URL="$repository_url" \
  --mount "type=volume,source=$volume,target=/runner-config" \
  --mount "type=bind,source=$secret_dir/token,target=/run/secrets/registration-token,readonly" \
  --entrypoint bash \
  "$image" \
  -euo pipefail -c '
    token=$(</run/secrets/registration-token)
    ./config.sh \
      --url "$REPOSITORY_URL" \
      --token "$token" \
      --name "$RUNNER_NAME" \
      --labels "$RUNNER_LABELS" \
      --work _work \
      --unattended \
      --replace
    find . -maxdepth 1 -type f -name ".*" -exec cp -a {} /runner-config/ \;
    find /runner-config -maxdepth 1 -type f -name ".*" \
      -exec chown 1001:1001 {} + \
      -exec chmod 400 {} +
  '
