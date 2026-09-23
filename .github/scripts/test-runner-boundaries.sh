#!/usr/bin/env bash
set -euo pipefail

pr_workflows=(
  .github/workflows/api.yml
  .github/workflows/compose-build.yml
  .github/workflows/e2e.yml
  .github/workflows/email-compose.yml
  .github/workflows/mcp.yml
  .github/workflows/pr-coverage.yml
  .github/workflows/pr-coverage-fork-comment.yml
  .github/workflows/web.yml
)

for workflow in "${pr_workflows[@]}"; do
  if grep -Eq 'self-hosted|runner-fallback|taskly-prod' "$workflow"; then
    echo "PR workflow may not select a self-hosted runner: $workflow" >&2
    exit 1
  fi
done

grep -Fq 'runs-on: [self-hosted, taskly-prod]' .github/workflows/deploy-prod.yml
grep -Fq "'[\"self-hosted\",\"taskly-ci\"]'" .github/workflows/deploy-prod.yml

echo "Runner trust boundaries are valid"
