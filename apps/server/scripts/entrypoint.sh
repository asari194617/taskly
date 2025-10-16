#!/usr/bin/env bash
set -euo pipefail

# Start cron in background
crond -f -l 8 &
CRON_PID=$!

# Start API in foreground
/app/api &
API_PID=$!

# Wait on both; if API exits, exit container
wait ${API_PID}
# If API exits, kill cron gracefully
kill ${CRON_PID} || true
wait ${CRON_PID} || true
