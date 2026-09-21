# Redis ACL operations

Production Redis disables the default user. The Taskly API authenticates with the
`REDIS_USERNAME` and `REDIS_PASSWORD` values injected through the existing
encrypted production environment secret. Do not commit either value.

The application user is limited to these commands:

- `PING`, `SELECT`, `GET`, `SET`, `DEL`, `INCR`, `EXPIRE`
- `SADD`, `SMEMBERS`, `SREM`

It can access only Taskly authentication, rate-limit, WebAuthn, TOTP, and session
key prefixes. Administrative commands, scripting, Pub/Sub, arbitrary keys, and
all other command categories remain denied.

## Initial rollout

1. Generate a strong secret outside the repository, for example with
   `openssl rand -base64 48`.
2. Add `REDIS_USERNAME=taskly` and `REDIS_PASSWORD=<generated secret>` to the
   encrypted production environment secret before deploying this change.
3. Deploy Redis and the API together. The Redis health check authenticates before
   reporting healthy, and the API exits during startup if credentials are absent
   or rejected.
4. Verify from inside the Redis container that unauthenticated `PING` returns
   `NOAUTH`, authenticated `PING` returns `PONG`, an allowed temporary key can be
   set and deleted, and `CONFIG GET '*'` plus access to an unrelated key return
   `NOPERM`.

The named `redis-data` volume is retained across container recreation. Existing
sessions therefore remain valid when Redis has persisted them. A session written
after the most recent Redis snapshot can already be lost by a restart under the
existing persistence policy; plan a maintenance window if forced logout is not
acceptable.

## Credential rotation

1. Choose a new username and password together. This configuration supports one
   application credential at a time, so schedule a brief maintenance window.
2. Update the encrypted production environment secret.
3. Recreate Redis and the API together with `docker compose up -d --force-recreate
   redis server`.
4. Run the verification steps above and remove any temporary verification key.

Changing credentials does not delete Redis data or invalidate stored session
tokens. To deliberately invalidate every session, stop the API and delete only
the `session:*` and `user_sessions:*` keys using an explicitly authorized break-
glass procedure.

## Recovery

- If Redis is unhealthy, inspect container status and logs. Startup errors name
  the missing variable but never print its value.
- Restore the matching username/password in the encrypted environment secret and
  recreate Redis and the API. Do not enable the default user or `nopass` as a
  workaround.
- If the ACL file cannot be generated, roll back the application deployment while
  retaining the `redis-data` volume, correct the secret/configuration, and deploy
  again.
- Treat exposure of a credential as an incident: rotate it, review Redis and API
  logs, and decide whether all sessions must be invalidated.
