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

# Task images use a private bucket, independently of the public MCP CDN secrets.
# Inspect the last assignment because that is what dotenv/Compose will load.
nonempty_value_regex="^([^[:space:]\"'#]|\"[^\"[:space:]]|'[^'[:space:]])"
for key in R2_ENDPOINT R2_BUCKET R2_ACCESS_KEY_ID R2_SECRET_ACCESS_KEY; do
  value=$(sed -n "s/^${key}=//p" <<<"$PROD_ENV" | tail -n 1)
  if ! grep -Eq "$nonempty_value_regex" <<<"$value"; then
    echo "PROD_ENV must set $key for task image uploads before deployment." >&2
    exit 1
  fi
done
