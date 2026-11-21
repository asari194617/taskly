# Taskly

Taskly is a class and task management service.

## Repository structure

| Path | Role |
| --- | --- |
| [`apps/web/`](apps/web/) | Astro and React web client deployed to Cloudflare Pages |
| [`apps/server/`](apps/server/) | Go and Fiber API with GraphQL and REST, backed by MySQL and Redis |
| [`apps/mcp/`](apps/mcp/) and [`apps/mcp-client-files/`](apps/mcp-client-files/) | TypeScript MCP HTTP service, stdio bridge, and client distribution files |
| [`apps/email-compose/`](apps/email-compose/) | Node.js gRPC service that renders transactional email |
| [`apps/docs/openapi/`](apps/docs/openapi/) | OpenAPI definition and API documentation page |
| [`batches/`](batches/) | Go jobs for notifications, day-shift rotations, moderation, and database backups |
| [`infra/`](infra/) | Docker Compose stacks, Airflow DAGs and runtime, monitoring, CI runners, and operations scripts |
| [`cloudflare-email-workers/`](cloudflare-email-workers/) | Inbound email forwarding Worker configured by [`wrangler.toml`](wrangler.toml) |
| [`docs/`](docs/) | Development guides, architecture, and operations runbooks |
| [`.github/workflows/`](.github/workflows/) | CI, release, deployment, and scheduled workflows |
| [`scripts/`](scripts/) | Repository tooling, including Go linting |
| [`samples/`](samples/) and [`login/`](login/) | Standalone Airflow Compose example and login screenshot |

### Related repositories

| Repository | Role |
| --- | --- |
| [`taskly-auth`](https://github.com/asari194617/taskly-auth) (private) | Shared login, OAuth, and account settings used by Taskly and NNCT company guidance |
| [`taskly-nnct-corp-guidance`](https://github.com/asari194617/taskly-nnct-corp-guidance) | NNCT company guidance web app, API, and MCP integration with Taskly task completion |
| [`taskly-policy`](https://github.com/asari194617/taskly-policy) | Versioned policy text and release manifests |
| [`taskly-policy-web`](https://github.com/asari194617/taskly-policy-web) | Public policy site that publishes releases from `taskly-policy` |
| [`taskly-environment-ribbon`](https://github.com/asari194617/taskly-environment-ribbon) | Optional Chrome extension that marks production and local development pages |

## Local development

Use the [local HTTPS setup](docs/local-development.md#local-https) to open
Taskly and its companion services on their `local.` domains. The loopback HTTP
commands below also work for isolated development and automated tests.

Prerequisites:

- Docker with Docker Compose v2
- Node.js 26 (CI uses 26.13.1) and npm for the web client and TypeScript checks
- Go 1.27.4 or newer for local Go checks, matching the current CI and Docker builds
- Bash 4+ for operations scripts and `jq` for Airflow database setup

The current web login uses the separate **taskly-auth** service. That service,
its database, and its OAuth client registration are not included in this
repository's Compose stack. Read the [local setup guide](docs/local-development.md)
before starting: `.env.example` needs an auth configuration choice and the
application schema must exist before the API starts.

The optional [NNCT company guidance application](docs/nnct-corp-guidance-development.md)
uses a separate repository, database, and OAuth client. Its setup is independent
of this Compose stack.

From the repository root, create and configure `.env` as described in that guide:

```bash
cp .env.example .env
# Configure auth and replace placeholder secrets before continuing.
chmod 600 .env
install -m 600 .env infra/.env
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server mcp
curl -fsS http://localhost:3063/readyz
curl -fsS http://localhost:3066/readyz
```

This starts MySQL, Redis, email-compose, the API, and MCP. Airflow needs a
separate metadata database initialization; use the
[Airflow setup steps](docs/local-development.md#optional-airflow) before starting
its three services.

Run the web client separately:

```bash
cp apps/web/.env.example apps/web/.env
npm --prefix apps/web ci
npm --prefix apps/web run dev
```

The main local endpoints are:

| Component | URL |
| --- | --- |
| Web client | `http://localhost:4321` |
| API / GraphQL | `http://localhost:3063` / `http://localhost:3063/graphql` |
| API liveness / readiness | `http://localhost:3063/healthz` / `http://localhost:3063/readyz` |
| API metrics | `http://localhost:3064/metrics` |
| MCP HTTP API | `http://localhost:3066` |
| Airflow UI | `http://localhost:3065` |

The API, metrics, MCP, and Airflow ports bind to host loopback only. MySQL,
Redis, and email-compose are available only on the Compose network. See the
[production port inventory](docs/operations/network-ports.md) for details.

To stop the local stack:

```bash
docker compose -f infra/docker-compose.yml down
```

Named volumes contain development data and are retained by this command.

Database schema changes are declared in `apps/server/schema.sql`. Apply the
desired state with the containerized Atlas tool (it starts and waits for MySQL):

```bash
infra/ops/apply-schema.sh
```

The command prints the plan first and refuses destructive changes unless they
were explicitly reviewed and `ATLAS_ALLOW_DESTRUCTIVE=true` is set. Keep
`infra/.env` synchronized with `.env` before running operations scripts.
Historical files in `apps/server/migrations` are kept for audit only.

## Verification

Install each Node package's dependencies first (`npm --prefix apps/<component> ci`;
MCP uses `ci --legacy-peer-deps`). Run the checks relevant to the component you changed:

```bash
(cd apps/server && go test ./...)
(cd batches && go test ./...)
npm --prefix apps/mcp test
npm --prefix apps/mcp run build
npm --prefix apps/email-compose run build
npm --prefix apps/web run check
npm --prefix apps/web run test:unit
npm --prefix apps/web run build
npm --prefix apps/web run test:picomatch-exposure
npm --prefix apps/web run test:canonical
npm --prefix apps/web run test:security-headers
docker compose -f infra/docker-compose.yml config --quiet
```

The web [browser tests](apps/web/tests/browser/README.md),
[real API E2E tests](apps/web/tests/e2e/README.md), and
[Airflow Python tests](docs/operations/airflow-python-tests.md) have additional
setup. Notification delivery channels are separate Go modules: run `go test ./...`
in each of `batches/notifications/delivery-channels/{common,email,discord,web-push}`
when changing them. See [coverage checks](docs/testing/coverage.md) for CI details.

## Documentation

See [`docs/README.md`](docs/README.md) for the documentation index and the
distinction between current references and historical planning material.
Useful entry points include:

- [`docs/Specification.md`](docs/Specification.md): product requirements and planning notes
- [`docs/schema.md`](docs/schema.md): schema overview (the executable source is `apps/server/schema.sql`)
- [`docs/mcp-docs.md`](docs/mcp-docs.md): MCP client and server guide
- [`docs/initial-settings.md`](docs/initial-settings.md): CI/CD and production operations
- [`apps/docs/openapi/openapi.yaml`](apps/docs/openapi/openapi.yaml): OpenAPI definition

## Production deployment

Merges to `main` that match the production paths trigger
[`deploy-prod.yml`](.github/workflows/deploy-prod.yml). It releases commit-SHA
image tags and verifies the application schema against the last committed
production baseline before synchronizing the repository and updating services
with Docker Compose. Application schema changes require separate reviewed
maintenance. Cloudflare Pages deploys the web client separately.

All four service images receive the release SHA tag. Running Airflow containers
are replaced only when their image or Compose configuration changes (or a service
is missing), so their running SHA can differ from the API/MCP/email-compose SHA.
Even a README-only change under `apps/server`, `apps/mcp`, `apps/email-compose`,
`batches`, or `infra` matches the deployment paths. See
[deployment operations](docs/initial-settings.md) and the
[runner pool guide](infra/actions-runner/README.md).

## Source availability and licensing

Taskly is not currently offered under a repository-wide open-source license.
Unless a file or directory contains its own license, all rights are reserved by its copyright holders. In particular,
`apps/mcp-client-files` is separately available under the MIT License contained
in that directory.

See [`LICENSE`](LICENSE) for the scope of these terms and
[`CONTRIBUTING.md`](CONTRIBUTING.md) before proposing a contribution.
