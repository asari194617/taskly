#!/usr/bin/env bash
set -euo pipefail

runner_count=${RUNNER_COUNT:?RUNNER_COUNT is required}
runner_prefix=${RUNNER_PREFIX:-build}
: "${RUNNER_POOL_HOST:?RUNNER_POOL_HOST is required}"
: "${RUNNER_POOL_USER:?RUNNER_POOL_USER is required}"
: "${RUNNER_POOL_SSH_KEY:?RUNNER_POOL_SSH_KEY is required}"
: "${GH_TOKEN:?GH_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"

if [[ ! "$runner_count" =~ ^[0-4]$ ]]; then
  echo "RUNNER_COUNT must be between 0 and 4" >&2
  exit 1
fi
if [[ ! "$runner_prefix" =~ ^[a-z0-9-]+$ ]]; then
  echo "Invalid runner prefix" >&2
  exit 1
fi
if (( runner_count == 0 )); then
  exit 0
fi

runner_port=${RUNNER_POOL_PORT:-22}
install -m 700 -d "$HOME/.ssh"
printf '%s' "$RUNNER_POOL_SSH_KEY" > "$HOME/.ssh/taskly-runner-pool"
chmod 600 "$HOME/.ssh/taskly-runner-pool"
ssh-keyscan -p "$runner_port" "$RUNNER_POOL_HOST" >> "$HOME/.ssh/known_hosts" 2>/dev/null

for index in $(seq 1 "$runner_count"); do
  runner_name="taskly-ci-${runner_prefix}-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}-${index}"
  payload=$(jq -nc \
    --arg name "$runner_name" \
    '{name:$name,runner_group_id:1,labels:["self-hosted","Linux","X64","taskly-ci"],work_folder:"_work"}')
  jit_config=$(gh api --method POST \
    "repos/$GITHUB_REPOSITORY/actions/runners/generate-jitconfig" \
    --input - <<<"$payload" \
    --jq '.encoded_jit_config')
  printf '%s' "$jit_config" | ssh \
    -i "$HOME/.ssh/taskly-runner-pool" \
    -o BatchMode=yes \
    -o IdentitiesOnly=yes \
    -p "$runner_port" \
    "$RUNNER_POOL_USER@$RUNNER_POOL_HOST" \
    /opt/taskly-runner/start-jit-runner.sh "$runner_name"
done
