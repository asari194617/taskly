# Taskly Monorepo

Taskly is a class and task management service. This repository contains:

- `apps/server`: Go/Fiber API server (GraphQL and REST), backed by MySQL and Redis
- `apps/web`: Astro/React web client (static build for Cloudflare Pages)
- `apps/mcp`: TypeScript HTTP API and stdio MCP bridge
- `apps/email-compose`: Node.js gRPC service for rendering transactional email
- `batches`: Go batch jobs scheduled by Airflow
- `infra`: Docker Compose, Airflow, and production operations scripts
- `cloudflare-email-workers`: inbound email forwarding worker
- `docs`: product, API, data-model, and operations documentation

## Local development

Prerequisites:

- Docker with Docker Compose v2
- Node.js 24 (CI uses 24.22.1) and npm for the web client and TypeScript checks
- Go 1.26 or newer when running Go code outside Docker (CI uses 1.26.10)
- `jq` for Airflow database setup; Bash 4+ for the operations scripts

The API Docker build currently uses Go 1.27.3; the batch builder uses Go
1.26.10. MCP and email-compose runtime images use Node.js 26.13.0. The manifests,
Dockerfiles, and workflows define the exact versions for each component.

The current web login uses the separate **taskly-auth** service. That service,
its database, and its OAuth client registration are not included in this
repository's Compose stack. Read the [local setup guide](docs/local-development.md)
before starting: `.env.example` needs an auth configuration choice and the
application schema must exist before the API starts.

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

Merges to `main` that touch deployable services trigger
`.github/workflows/deploy-prod.yml`. The workflow builds immutable images tagged
with the commit SHA, synchronizes the repository to the host, checks disk
capacity, and deploys that exact image tag with Docker Compose. Production
deployments use this workflow. The web client is deployed separately through
Cloudflare Pages; it is not part of the production Compose deployment.

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
