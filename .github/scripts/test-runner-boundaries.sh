#!/usr/bin/env bash
set -euo pipefail

pr_workflows=(api web mcp email-compose e2e compose-build pr-coverage)
for name in "${pr_workflows[@]}"; do
  workflow=".github/workflows/$name.yml"
  grep -Fq 'uses: ./.github/workflows/select-runner.yml' "$workflow"
  if grep -Eq 'runs-on:.*(self-hosted|taskly-prod|ubuntu-latest)|runner-fallback' "$workflow"; then
    echo "PR jobs must use the shared runner selector: $workflow" >&2
    exit 1
  fi
  grep -Fq 'runs-on: ${{ fromJSON(needs.select-runner.outputs.label) }}' "$workflow"
done

selector=.github/workflows/select-runner.yml
grep -Fq "github.event.pull_request.head.repo.full_name != github.repository" "$selector"
grep -Fq "github.event_name == 'pull_request'" "$selector"
grep -Fq "primary: ['self-hosted', 'taskly-ci-primary']" "$selector"
grep -Fq "secondary: ['self-hosted', 'taskly-ci-secondary']" "$selector"
grep -Fq "'ubuntu-latest': ['ubuntu-latest']" "$selector"
grep -Fq "await isHealthy('runner-health-primary.yml')" "$selector"
grep -Fq "await isHealthy('runner-health-secondary.yml')" "$selector"
if grep -Eq 'actions/checkout|secrets\.|taskly-prod|runner-fallback' "$selector"; then
  echo 'Runner selection must not check out code, access secrets, or use production runners' >&2
  exit 1
fi

for tier in primary secondary; do
  health=".github/workflows/runner-health-$tier.yml"
  grep -Fq "runs-on: [self-hosted, taskly-ci-$tier]" "$health"
  grep -Fq 'cancel-in-progress: true' "$health"
done

grep -Fq 'runs-on: [self-hosted, taskly-prod]' .github/workflows/deploy-prod.yml
grep -Fq 'runner: ${{ needs.select-runner.outputs.label }}' .github/workflows/deploy-prod.yml

grep -Fq 'uses: ./.github/workflows/select-runner.yml' .github/workflows/weekly-discord.yml
grep -Fq 'runs-on: ${{ fromJSON(needs.select-runner.outputs.label) }}' .github/workflows/weekly-discord.yml

echo "Runner trust boundaries are valid"
