#!/usr/bin/env bash
set -euo pipefail

# Child job containers use host bind mounts. Each slot runs at a unique path
# mounted at the same location on the host and in the runner container.
if [[ "$PWD" != /home/runner/actions-runner ]]; then
  cp -a /home/runner/actions-runner/. .
fi

jit_config_file=${RUNNER_JIT_CONFIG_FILE:-/run/secrets/jit-config}
runner_config_dir=${RUNNER_CONFIG_DIR:-/run/runner-config}

if [[ -s "$jit_config_file" ]]; then
  jit_config=$(<"$jit_config_file")
  exec ./run.sh --jitconfig "$jit_config"
fi

if [[ ! -s "$runner_config_dir/.runner" ]]; then
  echo "Persistent runner configuration is missing" >&2
  exit 1
fi

cp -a "$runner_config_dir"/. .
exec ./run.sh --once
