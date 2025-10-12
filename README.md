# Taskly Monorepo

Taskly is a simple, fast class management service. This monorepo contains:

- apps/server: Go (Fiber) Application Server with GraphQL (for app traffic) and REST (for API key integrations), backed by MySQL, Redis, Prometheus metrics
- apps/web: Astro frontend (GraphQL to application server), using shadcn/ui
- apps/mcp: MCP server (TypeScript)
- infra/docker/postfix: Postfix Docker for outbound email
- infra/prometheus: Prometheus scrape config
- docs: OpenAPI spec

Getting started:

1. Copy `.env.example` to `.env` and fill in secrets.
2. Start infrastructure and services with docker-compose (to be added).
3. Run API and Web locally during development.

See `Specification.md` for full requirements.
