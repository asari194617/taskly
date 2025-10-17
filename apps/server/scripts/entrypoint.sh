#!/usr/bin/env sh
set -euo pipefail

# Start cron daemon (busybox crond backgrounds itself without -f)
crond -l 8

# Start the API server in the foreground
exec /app/api
