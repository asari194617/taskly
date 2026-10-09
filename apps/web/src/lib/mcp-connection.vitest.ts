import { describe, expect, it } from "vitest";
import {
  TASKLY_MCP_BASE_URL,
  TASKLY_MCP_COMMAND,
  tasklyMcpClientConfig,
} from "./mcp-connection";

describe("Taskly MCP client configuration", () => {
  it("contains the production endpoint, helper command, and token variable", () => {
    const config = JSON.parse(tasklyMcpClientConfig("test-token"));

    expect(config.mcpServers.taskly).toEqual({
      command: TASKLY_MCP_COMMAND,
      env: {
        TASKLY_MCP_BASE_URL,
        TASKLY_MCP_TOKEN: "test-token",
      },
    });
  });
});
