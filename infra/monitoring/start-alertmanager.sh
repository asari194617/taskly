#!/bin/sh
set -eu
umask 077
: "${DISCORD_WEBHOOK_URL:?DISCORD_WEBHOOK_URL is required for monitoring alerts}"
printf '%s' "$DISCORD_WEBHOOK_URL" > /tmp/discord-webhook-url
exec /bin/alertmanager --config.file=/etc/alertmanager/alertmanager.yml --storage.path=/alertmanager
