# Local development

Run commands from the repository root unless a different directory is shown.
Use the tool versions listed in [README](../README.md). Production deployment
uses [GitHub Actions](initial-settings.md); these steps are for a development DB.

## Environment and authentication

Copy `.env.example` to `.env` and edit it before starting services. The API rejects
partial OAuth configuration: `AUTH_ISSUER`, `AUTH_CLIENT_ID`,
`AUTH_CLIENT_SECRET`, and `AUTH_REDIRECT_URI` must all be populated or all empty.
The example currently contains three of these four values.

Choose the configuration for the work you are doing:

- For web login, run a separate taskly-auth development environment and register
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

## Application stack

Apply the schema before starting the API:

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
