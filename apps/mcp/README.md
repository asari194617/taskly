Taskly MCP server (Node + TypeScript)

Overview

- Provides a minimal HTTP MCP-style API to integrate with AI clients or automations.
- Auth via X-MCP-Token header matched against the `mcps.hashed_token` in the DB.
- Endpoints
  - GET /healthz
  - GET /tasks
  - POST /tasks
  - PATCH /tasks/:id
  - DELETE /tasks/:id
  - GET /users/:id
  - PATCH /users/:id

Build & Run (Docker)

- Compose service name: mcp
- Port: 3066 (exposed)
- Environment (in compose)
  - MCP_HOST=0.0.0.0, MCP_PORT=3066
  - DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME

Local development

1. Install deps
   npm i
2. Build
   npm run build
3. Run
   npm start

Stdio bridge (for Gemini CLI or other stdio MCP clients)

- This package also ships a stdio MCP bridge that proxies tools to the HTTP API.
- Build first (npm run build), then run:
  npm run start:stdio

Environment variables:

- TASKLY_MCP_BASE_URL: Base URL of the HTTP MCP (default http://localhost:3066)
- TASKLY_MCP_TOKEN or MCP_TOKEN: Plain token to pass via X-MCP-Token

Gemini CLI settings.json example:
{
"mcpServers": {
"taskly": {
"command": "/usr/bin/env",
"args": [
"node",
"/absolute/path/to/apps/mcp/dist/stdio.js"
],
"env": {
"TASKLY_MCP_BASE_URL": "http://localhost:3066",
"TASKLY_MCP_TOKEN": "<plain-token>"
}
}
}
}

Auth

- Header: X-MCP-Token: <plain_token>
- Server hashes the token with SHA-256 and checks `SELECT user_id FROM mcps WHERE hashed_token = ?`.
- Tokens are issued via the Web UI: Settings > MCP.

Examples

1. Health:
   curl http://localhost:3066/healthz
2. List tasks:
   curl -H "X-MCP-Token: $TOKEN" http://localhost:3066/tasks
3. Create task:
   curl -X POST -H "Content-Type: application/json" -H "X-MCP-Token: $TOKEN" \
    -d '{"title":"New Task","subject_id":1,"deadline":"2025-10-31","type":"OTHER"}' \
    http://localhost:3066/tasks
4. Update user display name:
   curl -X PATCH -H "Content-Type: application/json" -H "X-MCP-Token: $TOKEN" \
    -d '{"display_name":"New Name"}' \
    http://localhost:3066/users/1

Client configuration hints

- Claude Desktop (macOS) config JSON `claude_desktop_config.json` example:
  {
  "mcpServers": {
  "taskly": {
  "command": "/usr/bin/curl",
  "args": ["http://localhost:3066/healthz"],
  "env": { "TASKLY_MCP_URL": "http://localhost:3066", "TASKLY_MCP_TOKEN": "<token>" }
  }
  }
  }
- Most MCP clients can call HTTP endpoints with bearer-like tokens; adapt as needed.
