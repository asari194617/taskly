# Taskly MCP service

This package contains two processes:

- a Fastify HTTP API backed by the Taskly MySQL database
- an MCP stdio bridge that exposes the HTTP operations as MCP tools

## HTTP API

The Compose service is named `mcp` and is exposed at
`http://localhost:3066` for local development.

| Method | Path | Authentication |
| --- | --- | --- |
| GET | `/healthz` | none |
| GET, POST | `/tasks` | `X-MCP-Token` |
| PATCH, DELETE | `/tasks/:id` | `X-MCP-Token` |
| GET, PATCH | `/users/:id` | `X-MCP-Token` |

The server SHA-256 hashes the supplied token and compares it with
`mcps.hashed_token`. Tokens are issued from **Settings > MCP** in the web UI.

```bash
curl -fsS http://localhost:3066/healthz
curl -fsS -H "X-MCP-Token: $TASKLY_MCP_TOKEN" \
  http://localhost:3066/tasks
```

## Local development

```bash
npm ci --legacy-peer-deps --no-audit --no-fund
npm run build
npm test
npm start
```

`npm run dev` watches the compiled `dist/index.js`; run the build first and
rebuild after TypeScript changes.

HTTP server environment variables:

- `MCP_HOST` (default `0.0.0.0`)
- `MCP_PORT` (default `3066`)
- `DB_HOST`, `DB_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DATABASE`

When running on the host against the repository's Compose database, use
`DB_HOST=127.0.0.1` and `DB_PORT=3060`. Inside Compose, use `db:3306`.

## stdio bridge

```bash
npm run build
npm run start:stdio
```

Bridge environment variables:

- `TASKLY_MCP_BASE_URL` (default `http://localhost:3066`)
- `TASKLY_MCP_TOKEN` (preferred) or `MCP_TOKEN`

Example client configuration is available in `gemini.settings.example.json`.
For prebuilt client installation and production URL guidance, see
`../../docs/mcp-docs.md`.
