# Taskly Monorepo

Taskly is a class and task management service. This repository contains:

- `apps/server`: Go/Fiber API server (GraphQL and REST), backed by MySQL and Redis
- `apps/web`: Astro/React web client
- `apps/mcp`: TypeScript HTTP API and stdio MCP bridge
- `apps/email-compose`: Node.js gRPC service for rendering transactional email
- `batches`: Go batch jobs scheduled by Airflow
- `infra`: Docker Compose, Airflow, and production operations scripts
- `docs`: product, API, data-model, and operations documentation

## Local development

Prerequisites:

- Docker with Docker Compose v2
- Node.js 24 and npm (for the web client and local TypeScript development)
- Go 1.25 or newer (only when running Go services outside Docker)

From the repository root:

```bash
cp .env.example .env
# Replace every placeholder secret in .env before using a shared environment.
docker compose -f infra/docker-compose.yml up -d --build
```

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
| API health | `http://localhost:3063/healthz` |
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

## Verification

Run the checks relevant to the component you changed:

```bash
(cd apps/server && go test ./...)
(cd batches && go test ./...)
npm --prefix apps/mcp test
npm --prefix apps/mcp run build
npm --prefix apps/email-compose run build
npm --prefix apps/web run build
docker compose -f infra/docker-compose.yml config --quiet
```

## Documentation

See [`docs/README.md`](docs/README.md) for the documentation index and the
distinction between current references and historical planning material.
Useful entry points include:

- [`docs/Specification.md`](docs/Specification.md): product specification
- [`docs/schema.md`](docs/schema.md): database schema
- [`docs/mcp-docs.md`](docs/mcp-docs.md): MCP client and server guide
- [`docs/initial-settings.md`](docs/initial-settings.md): CI/CD and production operations
- [`docs/operations/task-search.md`](docs/operations/task-search.md): optional OpenSearch task search
- [`apps/docs/openapi/openapi.yaml`](apps/docs/openapi/openapi.yaml): OpenAPI definition

## Production deployment

Merges to `main` that touch deployable services trigger
`.github/workflows/deploy-prod.yml`. The workflow builds immutable images tagged
with the commit SHA, synchronizes the repository to the host, checks disk
capacity, and deploys that exact image tag with Docker Compose. Production
deployments should use this workflow rather than running a local Compose build
on the server.
