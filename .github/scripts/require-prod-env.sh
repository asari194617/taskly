#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${PROD_ENV:-}" || ! "$PROD_ENV" =~ [^[:space:]] ]]; then
  echo 'PROD_ENV must contain the production .env before deployment.' >&2
  exit 1
fi
