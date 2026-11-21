# Local development

Run commands from the repository root unless a different directory is shown.
Use the tool versions listed in [README](../README.md). These steps use development databases.

## Local HTTPS

Use a shared Caddy proxy on your development machine for all of the aliases.
Install [Caddy](https://caddyserver.com/docs/install) and
[mkcert](https://github.com/FiloSottile/mkcert#installation). On macOS, use
`brew install caddy mkcert`. Keep all service processes on loopback or on
Docker networks.

1. Open `/etc/hosts` with `sudoedit /etc/hosts` and add the entries from
   [`dev/https/hosts`](../dev/https/hosts). Update any existing entries for
   these hostnames so that each resolves to `127.0.0.1`.
2. Generate the certificate and start the HTTPS proxy from the Taskly checkout:

   ```bash
   export TASKLY_DEV_TLS_DIR="$HOME/.local/share/taskly/https"
   dev/https/certificates.sh
   caddy run --config dev/https/Caddyfile --adapter caddyfile
   ```

   `mkcert -install` installs your personal development CA. Restart your
   browser if it was already running. Keep the CA private key in the mkcert
   directory and keep the TLS directory outside your checkout. If your OS
   restricts low ports, grant the Caddy executable permission to bind port
   443. On Linux, for example, use
   `sudo setcap cap_net_bind_service=+ep "$(command -v caddy)"`. On macOS,
   if the bind fails with a permission error, run Caddy from the same directory with:

   ```bash
   sudo env TASKLY_DEV_TLS_DIR="$TASKLY_DEV_TLS_DIR" "$(command -v caddy)" \
     run --config dev/https/Caddyfile --adapter caddyfile
   ```

   Port 443 must be free.
3. Start the services you want to use. Leave Caddy running in its terminal.
   Each HTTPS origin works once its corresponding upstream is running.

| Service | Local HTTPS URL | Loopback upstream |
| --- | --- | --- |
| Taskly web | https://local.taskly.iy0306.com | `4321` |
| Taskly API | https://local.api.taskly.iy0306.com | `3063` |
| Auth UI | https://local.auth.taskly.iy0306.com/login | `3068` |
| Auth API and OAuth issuer | https://local.taskly-auth-api.iy0306.com | `3067` |
| Guidance UI | https://local.nnct-corp-guidance-2026.taskly.iy0306.com | `3000` |
| Guidance API and optional MCP endpoint | https://local.api.nnct-corp-guidance-2026.taskly.iy0306.com | `3071` |
| Policy web | https://local.policies.taskly.iy0306.com | `3001` |
| CDN files in this repository | https://local.cdn.taskly.iy0306.com/docs/api/index.html | `dev/https/Caddyfile` serves files from the checkout |
| Taskly MCP HTTP API | https://local.mcp.taskly.iy0306.com | `3066` |
| Airflow UI (optional) | https://local.airflow-taskly.iy0306.com | `3065` |

The [auth setup](https://github.com/asari194617/taskly-auth#ローカル-https),
[guidance setup](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/development.md),
and [policy web setup](https://github.com/asari194617/taskly-policy-web#ローカル-開発)
configure those upstreams. The CDN alias serves checked-in OpenAPI files and MCP
client installer sources. Select the local HTTPS API server in Swagger UI.
Published binaries and production-only assets still
come from the published CDN. MySQL, Redis, email-compose gRPC, and the Airflow
worker processes communicate on their development networks.

### Taskly with local OAuth

Create the shared internal Docker network once before starting auth and Taskly:

```bash
docker network inspect taskly-local-https >/dev/null 2>&1 ||
  docker network create --internal taskly-local-https
```

Configure both services with the same development OAuth client secret and
callback, then start taskly-auth with its HTTPS Compose overlay. Start Taskly
after applying its schema as described below. Replace the matching values in
the Taskly root `.env` and sync it into `infra/.env`:

```dotenv
WEB_PUBLIC_URL=https://local.taskly.iy0306.com
API_PUBLIC_URL=https://local.api.taskly.iy0306.com
API_CORS_ORIGINS=https://local.taskly.iy0306.com,https://local.cdn.taskly.iy0306.com
AUTH_ISSUER=https://local.taskly-auth-api.iy0306.com
AUTH_CLIENT_ID=taskly-web
AUTH_CLIENT_SECRET=<same development value as taskly-auth>
AUTH_REDIRECT_URI=https://local.api.taskly.iy0306.com/auth/oauth/callback
SESSION_COOKIE_SECURE=true
WEBAUTHN_RP_ID=local.taskly.iy0306.com
WEBAUTHN_ORIGIN=https://local.taskly.iy0306.com
ADMIN_REVIEW_URL=https://local.taskly.iy0306.com/admin/moderation
ICAL_BASE_URL=https://local.api.taskly.iy0306.com/ical
```

In `apps/web/.env`, replace the matching URLs with the following:

```dotenv
PUBLIC_API_URL=https://local.api.taskly.iy0306.com
PUBLIC_AUTH_OAUTH_START_URL=https://local.api.taskly.iy0306.com/auth/oauth/start
PUBLIC_AUTH_LOGOUT_URL=https://local.auth.taskly.iy0306.com/logout
PUBLIC_AUTH_PROFILE_URL=https://local.auth.taskly.iy0306.com/profile
PUBLIC_AUTH_SECURITY_URL=https://local.auth.taskly.iy0306.com/security
PUBLIC_AUTH_PASSKEY_URL=https://local.auth.taskly.iy0306.com/passkeys
PUBLIC_AUTH_PASSWORD_URL=https://local.auth.taskly.iy0306.com/security
PUBLIC_AUTH_EMAIL_CODE_URL=https://local.auth.taskly.iy0306.com/security
PUBLIC_AUTH_RECOVERY_URL=https://local.auth.taskly.iy0306.com/security
PUBLIC_AUTH_TOTP_URL=https://local.auth.taskly.iy0306.com/security
```

Run the development Compose stack with the HTTPS overlay. Export the same
`TASKLY_DEV_TLS_DIR` in any shell that runs this overlay.

```bash
docker compose -f infra/docker-compose.yml -f dev/https/compose.yaml up -d --build --wait server mcp
curl -fsS https://local.taskly-auth-api.iy0306.com/.well-known/oauth-authorization-server
curl -fsS https://local.api.taskly.iy0306.com/readyz
curl -fsS https://local.mcp.taskly.iy0306.com/readyz
```

For a fresh pair of development databases, [seed an auth user](https://github.com/asari194617/taskly-auth#2-開発ユーザーを用意する)
and find its `id` in the auth database's `users` table. Create a matching
entry in the Taskly development DB with the same ID and email before
visiting `https://local.taskly.iy0306.com/login`. For example, use the
following command after replacing the ID and email with the values in your
development auth DB:

```bash
docker compose -f infra/docker-compose.yml exec -T db sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql -u"$MYSQL_USER" "$MYSQL_DATABASE"' <<'SQL'
INSERT INTO users (id, email, display_name, is_registered)
VALUES (1, 'taskly-local-demo@nagaoka-ct.ac.jp', 'Development User', 1);
SQL
```

The overlay gives the API container its own route to the exact OAuth issuer on
the shared network and mounts the public CA certificate for TLS verification.
Your workstation resolves that same issuer from `/etc/hosts`. Use the same
overlays when recreating or stopping the services. After regenerating the
certificate, restart Caddy and recreate the Taskly `auth-tls` and `server`
containers so they load the new files. Node clients that do not use your system
CA store can set `NODE_EXTRA_CA_CERTS="$TASKLY_DEV_TLS_DIR/rootCA.pem"`.

The Airflow startup command further down can use both Compose files if you
want Airflow to generate URLs for its local HTTPS hostname. To remove local
HTTPS, stop Caddy, remove these aliases from `/etc/hosts`, and restore the
application URLs used by your earlier setup.

## Loopback HTTP and shared setup

### Environment and authentication

Copy `.env.example` to `.env` and edit it before starting services. The API rejects
partial OAuth configuration: `AUTH_ISSUER`, `AUTH_CLIENT_ID`,
`AUTH_CLIENT_SECRET`, and `AUTH_REDIRECT_URI` must all be populated or all empty.
The example currently contains three of these four values. `WEB_PUBLIC_URL`
is set in two places in the example. Keep both values equal or delete the duplicate.

For the loopback HTTP workflow, choose the configuration for the work you are doing:

- For web login, run a separate [taskly-auth](https://github.com/asari194617/taskly-auth#開発環境)
  development environment and register
  an OAuth client. Set all four variables, with callback
  `http://localhost:3063/auth/oauth/callback` and web origin
  `WEB_PUBLIC_URL=http://localhost:4321`. `AUTH_ISSUER` must be reachable from
  the API container as well as the browser; `localhost` inside the API container
  refers to that container. This repository does not provision taskly-auth.
- For API-only development without an OAuth provider, set all four variables to
  empty and leave the example's `AUTH_*_EXTERNAL` flags at `0`. The API can start,
  but the current web login page still redirects to OAuth. Use the isolated
  [real API E2E fixtures](../apps/web/tests/e2e/README.md) or
  [mocked browser tests](../apps/web/tests/browser/README.md) for automated UI work.

When enabling external auth ownership, all eight ownership flags listed in
[`auth_migration_config.go`](../apps/server/auth_migration_config.go) must be `1`
together; `AUTH_PROFILE_EXTERNAL=1` also requires `AUTH_USERS_EXTERNAL=1`.
Configure `AUTH_INTERNAL_TOKEN` (at least 32 characters) and the three sync URLs
as well. Consult the [identity cutover inventory](auth-identity-write-inventory.md),
[user projection](auth-user-projection-sync.md),
[profile ownership](account-profile-ownership.md), and
[account lifecycle](auth-account-lifecycle-sync.md) before changing ownership.
Copying data and registering the OAuth client happen in the separate auth system.

Replace `JWT_SECRET`, `TOTP_ENC_KEY`, MySQL and Redis passwords, and integration
secrets with development values. `JWT_SECRET` must be nonempty even in development.
Use different names and accounts for the application DB and Airflow metadata DB.
`AIRFLOW_META_PASSWORD` accepts only letters, digits, `_`, and `-`.
Set `AIRFLOW__API_AUTH__JWT_SECRET` explicitly to the same nonempty value for all
three Airflow services; the scheduler does not have the fallback used by the
other two services. `openssl rand -hex 32` can generate individual secrets.

Keep these two ignored files synchronized after edits:

```bash
chmod 600 .env
install -m 600 .env infra/.env
```

Services load the root `.env` through `env_file`. Operations scripts change into
`infra/`, where Compose reads `infra/.env` for `${...}` interpolation. A service's
`env_file` alone does not supply that interpolation.

SMTP, R2 image storage, Web Push VAPID keys, Gemini, Teams, and FTPS backups
require their own development credentials to exercise those features. The
Compose stack does not include an SMTP relay. Leave outbound integrations and
scheduled jobs disabled until their development destinations are configured.

The optional [NNCT company guidance service](nnct-corp-guidance-development.md)
has its own repository, database, and public OAuth client registration. Follow
its setup separately when working on that application; it is not part of this
Compose stack.

## Application stack

Apply the schema before starting the API. Use the HTTPS Compose command above
in place of the second line when using the HTTPS overlay:

```bash
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server mcp
curl -fsS http://localhost:3063/readyz
curl -fsS http://localhost:3066/readyz
```

Atlas runs in Docker and needs no host installation. Its `schema` profile adds a
disposable MySQL instance for schema analysis. The helper starts the application
DB, prints the plan, applies non-destructive changes, and removes the disposable
container afterward. Review [schema operations](initial-settings.md#dbスキーマ管理)
for existing databases. `apps/server/migrations` is historical; there is no
`go run ./cmd/migrate` command.

Then start the web client in a separate terminal:

```bash
cp apps/web/.env.example apps/web/.env
npm --prefix apps/web ci
npm --prefix apps/web run dev
```

The web client listens on `http://localhost:4321`. Set its `PUBLIC_AUTH_*` URLs
for your auth environment when testing login, logout, profile, or security pages.
The API and MCP listen on loopback ports `3063` and `3066`. MySQL, Redis, and
email-compose are reachable by Compose services only. Running Go or MCP directly
on the host requires a development port mapping or tunnel and corresponding
`DB_HOST`, `DB_PORT`, and Redis settings.

## Optional Airflow

For a **new development database**, initialize metadata after the application
stack is running. `jq` must be available on the host:

```bash
docker compose -f infra/docker-compose.yml build airflow-webserver
(cd infra && ./ops/prepare-airflow-db.sh)
docker compose -f infra/docker-compose.yml up -d --no-build --wait --wait-timeout 600 \
  airflow-webserver airflow-scheduler airflow-dag-processor
```

`prepare-airflow-db.sh` creates the separate DB and grants and runs
`airflow db migrate`. With an existing environment, stop and review the
[metadata migration runbook](operations/airflow-db-separation.md) first; the
script can also move legacy Airflow tables and change database privileges.
The base Compose file does not migrate metadata on service startup.

The UI is `http://localhost:3065`; on first startup the image entrypoint creates
the FAB user from `_AIRFLOW_WWW_USER_USERNAME` / `_AIRFLOW_WWW_USER_PASSWORD`
(defaults: `airflow` / `airflow`, for local development only). Confirm the DAGs
and keep them paused until their data and outbound services are ready. Schedules
and job responsibilities are listed in [batches](../batches/README.md).

## Stopping and troubleshooting

```bash
docker compose -f infra/docker-compose.yml ps
docker compose -f infra/docker-compose.yml logs --tail=100 server mcp
docker compose -f infra/docker-compose.yml down
```

`down` retains named volumes. After changing environment variables, use `up -d`
to recreate affected services. Changing MySQL passwords in `.env` does not update
the accounts in an existing MySQL volume; update those accounts deliberately.
An API that fails before listening commonly has partial OAuth configuration or
missing tables. If `/healthz` works but `/readyz` fails, check
[dependency readiness](operations/readiness.md).
