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
Task operations remain scoped to the token owner and follow the main API's
task permission rules. In particular, task deletion is denied unless the
user has an explicit delete permission or an admin/manager role. Hidden tasks
are not exposed through this API.

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

Inside Compose, use `DB_HOST=db` and `DB_PORT=3306`. The default Compose
configuration does not publish MySQL on the host; a host-run MCP process needs
an explicitly configured tunnel or port mapping.

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
