#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${PROD_ENV:-}" || ! "$PROD_ENV" =~ [^[:space:]] ]]; then
  echo 'PROD_ENV must contain the production .env before deployment.' >&2
  exit 1
fi

for key in AIRFLOW_META_DATABASE AIRFLOW_META_USER AIRFLOW_META_PASSWORD; do
  if ! grep -Eq "^${key}=[^[:space:]]+" <<<"$PROD_ENV"; then
    echo "PROD_ENV must set $key before deployment." >&2
    exit 1
  fi
done
