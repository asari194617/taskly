#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
script="$root/infra/ops/nginx-security-maintenance.sh"

for version in nginx/1.30.7 nginx/1.30.8 nginx/1.31.9 nginx/1.32.0 2.0.0; do
  bash "$script" check-version "$version"
done

for version in nginx/1.30.6 nginx/1.31.8 nginx/1.29.99 nginx/1.24.0; do
  if bash "$script" check-version "$version"; then
    echo "Vulnerable version was accepted: $version" >&2
    exit 1
  fi
done

if bash "$script" check-version invalid >/dev/null 2>&1; then
  echo 'Invalid version was accepted' >&2
  exit 1
fi

echo 'Nginx security version policy: PASS'
