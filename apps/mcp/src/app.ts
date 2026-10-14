import Fastify from "fastify";
import fp from "fastify-plugin";
import crypto from "crypto";
import { httpTaskGateway, type TaskGateway } from "./task_gateway.js";
export type DBPool = {
  query: (...args: any[]) => Promise<any>;
  execute: (...args: any[]) => Promise<any>;
};

const readinessTimeoutMs = 1500;

async function apiReady(): Promise<boolean> {
  const base = process.env.TASKLY_API_BASE_URL || "http://server:8080";
  const response = await fetch(new URL("/readyz", base), {
    signal: AbortSignal.timeout(readinessTimeoutMs),
  });
  return response.ok;
}

function sha256Hex(s: string): string {
  return crypto.createHash("sha256").update(s).digest("hex");
}

function taskForMCP(row: any) {
  const fields: Record<string, string> = {
    ID: "id", AddedUserID: "added_user_id", Title: "title", SubjectID: "subject_id",
    Description: "description", Type: "type", IsRequireHandIn: "is_require_hand_in",
    IsSelfCheckRequired: "is_self_check_required", IsCompletionDisabled: "is_completion_disabled", Deadline: "deadline",
    DeadlineSince: "deadline_since", DeadlineUntil: "deadline_until",
    CreatedAt: "created_at", UpdatedAt: "updated_at",
  };
  if (!row || typeof row !== "object") return row;
  return Object.fromEntries(Object.entries(fields).map(([from, to]) => [to, row[from] !== undefined ? row[from] : row[to]]));
}

export async function createApp(
  pool: DBPool,
  taskGateway: TaskGateway = httpTaskGateway(),
  checkApiReady: () => Promise<boolean> = apiReady,
) {
  const app = Fastify({ logger: false });

  // Auth plugin: X-MCP-Token -> mcps.hashed_token match (with expiry)
  app.register(
    fp(async (instance) => {
      instance.addHook("preHandler", async (req, reply) => {
        if (req.url === "/healthz" || req.url === "/readyz") return; // public
        const token = (req.headers["x-mcp-token"] ||
          req.headers["x-mcp-api-token"]) as string | undefined;
        if (!token) {
          reply.code(401).send({ error: "missing token" });
          return;
        }
        const hashed = sha256Hex(token);
        const [rows] = await pool.query(
          `SELECT m.user_id FROM mcps m
           JOIN users u ON u.id = m.user_id AND u.is_deleted = 0
           WHERE m.hashed_token = ? AND (m.expires_at IS NULL OR m.expires_at > NOW())
           AND NOT EXISTS (SELECT 1 FROM blocks b WHERE b.user_id = u.id
             AND (b.expires_at IS NULL OR b.expires_at > NOW())) LIMIT 1`,
          [hashed],
        );
        if (!rows || rows.length === 0) {
          reply.code(401).send({ error: "invalid token" });
          return;
        }
        (req as any).userId = rows[0].user_id as number;
      });
    }),
  );

  app.get("/healthz", async () => ({ ok: true }));
  app.get("/readyz", async (_req, reply) => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const deadline = new Promise<null>((resolve) => {
      timer = setTimeout(() => resolve(null), 2000);
    });
    const checks = await Promise.race([Promise.allSettled([
      pool.query({ sql: "SELECT 1", timeout: readinessTimeoutMs }),
      checkApiReady(),
    ]), deadline]);
    clearTimeout(timer);
    if (!checks || checks[0].status !== "fulfilled" || checks[1].status !== "fulfilled" || !checks[1].value) {
      return reply.code(503).send({ ok: false });
    }
    return { ok: true };
  });

  // The MCP facade preserves its response shape while the Go API performs
  // task authorization, validation, persistence and notifications.
  const tokenFrom = (req: any) => String(req.headers["x-mcp-token"] || req.headers["x-mcp-api-token"] || "");
  app.get("/tasks", async (req, reply) => {
    const input = req.query as any;
    const limit = Math.min(Math.max(Number(input?.limit ?? 200), 1), 500);
    const offset = Math.max(Number(input?.offset ?? 0), 0);
    if (!Number.isSafeInteger(limit) || !Number.isSafeInteger(offset)) {
      return reply.code(400).send({ error: "invalid pagination" });
    }
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    if (input?.q) params.set("q", String(input.q));
    try {
      const result = await taskGateway(tokenFrom(req), "GET", `/tasks?${params}`);
      if (result.status !== 200) return reply.code(result.status).send(result.body);
      return reply.send((result.body?.items ?? []).map(taskForMCP));
    } catch {
      return reply.code(502).send({ error: "task API unavailable" });
    }
  });

  app.post("/tasks", async (req, reply) => {
    try {
      const result = await taskGateway(tokenFrom(req), "POST", "/tasks", req.body);
      if (result.status !== 201) return reply.code(result.status).send(result.body);
      return reply.code(201).send({ id: result.body?.id ?? result.body?.ID });
    } catch {
      return reply.code(502).send({ error: "task API unavailable" });
    }
  });

  app.patch("/tasks/:id", async (req, reply) => {
    const id = Number((req.params as any).id);
    if (!Number.isSafeInteger(id) || id <= 0) return reply.code(400).send({ error: "invalid id" });
    try {
      const result = await taskGateway(tokenFrom(req), "PATCH", `/tasks/${id}`, req.body);
      if (result.status !== 200) return reply.code(result.status).send(result.body);
      return reply.send({ ok: true });
    } catch {
      return reply.code(502).send({ error: "task API unavailable" });
    }
  });

  app.delete("/tasks/:id", async (req, reply) => {
    const id = Number((req.params as any).id);
    if (!Number.isSafeInteger(id) || id <= 0) return reply.code(400).send({ error: "invalid id" });
    try {
      const result = await taskGateway(tokenFrom(req), "DELETE", `/tasks/${id}`);
      if (result.status !== 204) return reply.code(result.status).send(result.body);
      return reply.send({ ok: true });
    } catch {
      return reply.code(502).send({ error: "task API unavailable" });
    }
  });

  // Users read/update (subset)
  app.get("/users/:id", async (req, reply) => {
    const id = Number((req.params as any).id);
    const userId = (req as any).userId as number;
    if (id !== userId) {
      return reply.code(403).send({ error: "forbidden" });
    }
    const [rows] = await pool.query(
      "SELECT id, email, display_name, is_deleted, notification_lang, is_registered, is_agreed_policies, created_at FROM users WHERE id = ? LIMIT 1",
      [id],
    );
    if (!rows || rows.length === 0)
      return reply.code(404).send({ error: "not found" });
    reply.send(rows[0]);
  });

  app.patch("/users/:id", async (req, reply) => {
    const id = Number((req.params as any).id);
    const userId = (req as any).userId as number;
    if (id !== userId) {
      return reply.code(403).send({ error: "forbidden" });
    }
    const body = req.body;
    if (!body || typeof body !== "object" || Array.isArray(body) ||
      Object.entries(body).some(([key, value]) =>
        !["display_name", "notification_lang"].includes(key) || typeof value !== "string")) {
      return reply.code(400).send({ error: "invalid profile patch" });
    }
    if (Object.keys(body).length === 0) return reply.send({ ok: true });
    try {
      const result = await taskGateway(tokenFrom(req), "PATCH", "/users/me", body);
      if (result.status !== 204) return reply.code(result.status).send(result.body);
      return reply.send({ ok: true });
    } catch {
      return reply.code(502).send({ error: "user API unavailable" });
    }
  });

  return app;
}
