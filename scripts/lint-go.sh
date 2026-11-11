#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Each nested module needs its own invocation; ./... stops at module boundaries.
while IFS= read -r module; do
  echo "go vet: ${module%/go.mod}"
  (cd "${module%/go.mod}" && go vet ./...)
done < <(git ls-files '**/go.mod')
