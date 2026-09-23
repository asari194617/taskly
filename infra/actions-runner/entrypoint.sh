#!/usr/bin/env bash
set -euo pipefail

jit_config_file=${RUNNER_JIT_CONFIG_FILE:-/run/secrets/jit-config}
if [[ ! -s "$jit_config_file" ]]; then
  echo "JIT runner configuration is missing" >&2
  exit 1
fi

jit_config=$(<"$jit_config_file")
exec ./run.sh --jitconfig "$jit_config"
