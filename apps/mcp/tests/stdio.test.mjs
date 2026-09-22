import assert from "node:assert/strict";
import http from "node:http";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const testDir = path.dirname(fileURLToPath(import.meta.url));
const stdioPath = path.resolve(testDir, "../dist/stdio.js");

test("stdio bridge advertises tools and calls the HTTP API", async () => {
  const requests = [];
  const api = http.createServer((req, res) => {
    requests.push({ url: req.url, token: req.headers["x-mcp-token"] });
    res.writeHead(200, { "content-type": "application/json" });
    res.end(JSON.stringify([{ id: 42, title: "MCP integration test" }]));
  });

  await new Promise((resolve) => api.listen(0, "127.0.0.1", resolve));
  const address = api.address();
  assert.equal(typeof address, "object");

  const transport = new StdioClientTransport({
    command: process.execPath,
    args: [stdioPath],
    env: {
      ...process.env,
      TASKLY_MCP_BASE_URL: `http://127.0.0.1:${address.port}`,
      TASKLY_MCP_TOKEN: "integration-test-token",
    },
  });
  const client = new Client({ name: "taskly-test", version: "1.0.0" });

  try {
    await client.connect(transport);

    const listed = await client.listTools();
    assert.deepEqual(
      listed.tools.map((tool) => tool.name).sort(),
      [
        "tasks_create",
        "tasks_delete",
        "tasks_list",
        "tasks_update",
        "user_get",
        "user_update",
      ],
    );
    const profile = listed.tools.find((tool) => tool.name === "user_update");
    assert.deepEqual(Object.keys(profile.inputSchema.properties.patch.properties).sort(),
      ["display_name", "notification_lang"]);
    assert.equal(profile.inputSchema.properties.patch.additionalProperties, false);

    const result = await client.callTool({
      name: "tasks_list",
      arguments: { limit: 1, search: "integration" },
    });
    assert.equal(result.isError, undefined);
    assert.equal(result.content[0].type, "text");
    assert.deepEqual(JSON.parse(result.content[0].text), [
      { id: 42, title: "MCP integration test" },
    ]);
    assert.deepEqual(requests, [
      {
        url: "/tasks?limit=1&q=integration",
        token: "integration-test-token",
      },
    ]);
  } finally {
    await client.close();
    await new Promise((resolve, reject) =>
      api.close((error) => (error ? reject(error) : resolve())),
    );
  }
});
