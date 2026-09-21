import Fastify from "fastify";
import fp from "fastify-plugin";
import crypto from "crypto";
import { httpTaskGateway, type TaskGateway } from "./task_gateway.js";
export type DBPool = {
  query: (...args: any[]) => Promise<any>;
  execute: (...args: any[]) => Promise<any>;
};

function sha256Hex(s: string): string {
  return crypto.createHash("sha256").update(s).digest("hex");
}

function taskForMCP(row: any) {
  const fields: Record<string, string> = {
    ID: "id", AddedUserID: "added_user_id", Title: "title", SubjectID: "subject_id",
    Description: "description", Type: "type", IsRequireHandIn: "is_require_hand_in",
    IsSelfCheckRequired: "is_self_check_required", Deadline: "deadline",
    DeadlineSince: "deadline_since", DeadlineUntil: "deadline_until",
    CreatedAt: "created_at", UpdatedAt: "updated_at",
  };
  if (!row || typeof row !== "object") return row;
  return Object.fromEntries(Object.entries(fields).map(([from, to]) => [to, row[from] !== undefined ? row[from] : row[to]]));
}

export async function createApp(pool: DBPool, taskGateway: TaskGateway = httpTaskGateway()) {
  const app = Fastify({ logger: false });

  // Auth plugin: X-MCP-Token -> mcps.hashed_token match (with expiry)
  app.register(
    fp(async (instance) => {
      instance.addHook("preHandler", async (req, reply) => {
        if (req.url === "/healthz") return; // public
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
    const body = req.body as any;
    const fields: string[] = [];
    const values: any[] = [];
    const up = (k: string, v: any) => {
      fields.push(`${k} = ?`);
      values.push(v);
    };
    const maybe = (k: string) => {
      if (body[k] !== undefined) up(k, body[k]);
    };
    maybe("display_name");
    if (body.is_deleted !== undefined)
      up("is_deleted", body.is_deleted ? 1 : 0);
    maybe("notification_lang");
    if (body.is_registered !== undefined)
      up("is_registered", body.is_registered ? 1 : 0);
    if (body.is_agreed_policies !== undefined)
      up("is_agreed_policies", body.is_agreed_policies ? 1 : 0);
    // push_daily_enabled removed; use notification_devices WEBPUSH via server APIs instead
    if (fields.length === 0) return reply.send({ ok: true });
    const sql = `UPDATE users SET ${fields.join(", ")} WHERE id = ?`;
    values.push(id);
    await pool.execute(sql, values);
    reply.send({ ok: true });
  });

  return app;
}
