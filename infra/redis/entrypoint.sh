#!/bin/sh
set -eu

: "${REDIS_USERNAME:?REDIS_USERNAME is required}"
: "${REDIS_PASSWORD:?REDIS_PASSWORD is required}"

case "$REDIS_USERNAME" in
  *[!A-Za-z0-9._-]*|'')
    echo "redis: REDIS_USERNAME contains unsupported characters" >&2
    exit 1
    ;;
esac

password_hash="$(printf '%s' "$REDIS_PASSWORD" | sha256sum | cut -d ' ' -f 1)"
acl_file=/run/redis/users.acl
umask 077
mkdir -p /run/redis
chown redis:redis /run/redis
chmod 750 /run/redis

{
  printf '%s\n' 'user default off'
  printf 'user %s reset on #%s resetkeys resetchannels -@all' "$REDIS_USERNAME" "$password_hash"
  printf ' +ping +select +get +set +del +incr +expire +sadd +smembers +srem'
  printf ' ~session:* ~user_sessions:* ~totp-setup:* ~webauthn:*'
  printf ' ~login_code_req_ip:* ~login_code_req_email:*'
  printf ' ~login_attempt_ip:* ~login_attempt_email:* ~graphql_rate:*\n'
} > "$acl_file"
chown redis:redis "$acl_file"

unset REDIS_PASSWORD password_hash
exec /usr/local/bin/docker-entrypoint.sh redis-server --aclfile "$acl_file"
