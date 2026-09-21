import assert from "node:assert/strict";

import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const binary = process.argv[2];
assert.ok(binary, "usage: node tests/verify-binary.mjs <binary>");

const transport = new StdioClientTransport({ command: binary });
const client = new Client({ name: "taskly-release-test", version: "1.0.0" });

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
  console.log(`verified ${listed.tools.length} MCP tools in ${binary}`);
} finally {
  await client.close();
}
