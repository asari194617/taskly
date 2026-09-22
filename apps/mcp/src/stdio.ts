import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  CallToolResultSchema,
  ListToolsRequestSchema,
  ListToolsResultSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { fetch, type RequestInit } from "undici";

const BASE_URL = process.env.TASKLY_MCP_BASE_URL || "http://localhost:3066";
const TOKEN = process.env.TASKLY_MCP_TOKEN || process.env.MCP_TOKEN || "";

async function api(path: string, init?: RequestInit) {
  const url = path.startsWith("http") ? path : `${BASE_URL}${path}`;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(init?.headers as Record<string, string> | undefined),
    "X-MCP-Token": TOKEN,
  };
  const res = await fetch(url, { ...init, headers });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`HTTP ${res.status} ${res.statusText}: ${text}`);
  }
  const ct = res.headers.get("content-type") || "";
  if (ct.includes("application/json")) return res.json();
  return res.text();
}

function textContent(data: unknown) {
  const text = typeof data === "string" ? data : JSON.stringify(data);
  return [{ type: "text" as const, text }];
}

type ToolHandler = (args: any) => Promise<unknown>;
type JSONSchemaObject = {
  type: "object";
  properties?: Record<string, unknown>;
} & Record<string, unknown>;

const tools: Record<
  string,
  {
    description?: string;
    inputSchema: JSONSchemaObject;
    handler: ToolHandler;
  }
> = {
  tasks_list: {
    description: "List recent tasks",
    inputSchema: {
      type: "object",
      properties: {
        limit: { type: "integer", minimum: 1, maximum: 500 },
        offset: { type: "integer", minimum: 0 },
        search: { type: "string" },
      },
    },
    handler: async (input: any) => {
      const params = new URLSearchParams();
      if (input?.limit) params.set("limit", String(input.limit));
      if (input?.offset) params.set("offset", String(input.offset));
      if (input?.search) params.set("q", input.search);
      const q = params.toString();
      return api(`/tasks${q ? `?${q}` : ""}`);
    },
  },
  tasks_create: {
    description: "Create a new task",
    inputSchema: {
      type: "object",
      properties: {
        title: { type: "string" },
        subject_id: { type: "integer" },
        description: { type: "string" },
        type: { type: "string" },
        is_require_hand_in: { type: "boolean" },
        is_self_check_required: { type: "boolean" },
        deadline: { type: "string" },
        deadline_since: { type: "string" },
        deadline_until: { type: "string" },
      },
    },
    handler: async (input: any) =>
      api(`/tasks`, { method: "POST", body: JSON.stringify(input) }),
  },
  tasks_update: {
    description: "Patch update a task",
    inputSchema: {
      type: "object",
      properties: {
        id: { type: "integer" },
        patch: {
          type: "object",
          properties: {
            title: { type: "string" },
            subject_id: { type: "integer" },
            description: { type: "string" },
            type: { type: "string" },
            is_require_hand_in: { type: "boolean" },
            is_self_check_required: { type: "boolean" },
            deadline: { type: "string" },
            deadline_since: { type: "string" },
            deadline_until: { type: "string" },
          },
        },
      },
    },
    handler: async (input: any) =>
      api(`/tasks/${input.id}`, {
        method: "PATCH",
        body: JSON.stringify(input.patch || {}),
      }),
  },
  tasks_delete: {
    description: "Delete a task by id",
    inputSchema: {
      type: "object",
      properties: { id: { type: "integer" } },
    },
    handler: async (input: any) =>
      api(`/tasks/${input.id}`, { method: "DELETE" }),
  },
  user_get: {
    description: "Get a user by id",
    inputSchema: {
      type: "object",
      properties: { id: { type: "integer" } },
    },
    handler: async (input: any) => api(`/users/${input.id}`),
  },
  user_update: {
    description: "Update your display name or notification language",
    inputSchema: {
      type: "object",
      properties: {
        id: { type: "integer" },
        patch: {
          type: "object",
          properties: {
            display_name: { type: "string" },
            notification_lang: { type: "string", enum: ["ja", "en"] },
          },
          additionalProperties: false,
        },
      },
    },
    handler: async (input: any) =>
      api(`/users/${input.id}`, {
        method: "PATCH",
        body: JSON.stringify(input.patch || {}),
      }),
  },
};

async function main() {
  const server = new Server(
    { name: "taskly-mcp-stdio", version: "1.0.0" },
    { capabilities: { tools: {} } },
  );

  // tools/list
  server.setRequestHandler(ListToolsRequestSchema, async (_req) => {
    const list = Object.entries(tools).map(([name, t]) => ({
      name,
      description: t.description,
      inputSchema: t.inputSchema,
    }));
    return { tools: list } as unknown as ReturnType<
      (typeof ListToolsResultSchema)["parse"]
    >;
  });

  // tools/call
  server.setRequestHandler(CallToolRequestSchema, async (req) => {
    const name = req.params?.name as string;
    const args = (req.params?.arguments as any) || {};
    const t = tools[name];
    if (!t) {
      return {
        content: [{ type: "text", text: `Unknown tool: ${name}` }],
        isError: true,
      };
    }
    const data = await t.handler(args);
    return { content: textContent(data) } as unknown as ReturnType<
      (typeof CallToolResultSchema)["parse"]
    >;
  });

  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
