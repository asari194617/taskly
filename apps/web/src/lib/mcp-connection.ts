export const TASKLY_MCP_BASE_URL = "https://mcp.taskly.iy0306.com";
export const TASKLY_MCP_COMMAND = "taskly-mcp";
export const TASKLY_MCP_INSTALL_COMMAND =
  "curl -fsSL https://cdn.taskly.iy0306.com/docs/mcp/clients/install.sh | bash";

export function tasklyMcpClientConfig(token = "<issued token>") {
  return JSON.stringify(
    {
      mcpServers: {
        taskly: {
          command: TASKLY_MCP_COMMAND,
          env: {
            TASKLY_MCP_BASE_URL,
            TASKLY_MCP_TOKEN: token,
          },
        },
      },
    },
    null,
    2,
  );
}
