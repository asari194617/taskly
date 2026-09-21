import Fastify from "fastify";
import fp from "fastify-plugin";
import crypto from "crypto";
export type DBPool = {
  query: (...args: any[]) => Promise<any>;
  execute: (...args: any[]) => Promise<any>;
};

function sha256Hex(s: string): string {
  return crypto.createHash("sha256").update(s).digest("hex");
}

// Keep these defaults aligned with apps/server/internal/utils/support_permissions.go.
// MCP remains owner-scoped even when the main API grants broader roles.
async function canUseTasks(
  pool: DBPool,
  userId: number,
  action: "read" | "create" | "update" | "delete",
  ownerId?: number,
): Promise<boolean> {
  const [roles] = await pool.query(
    "SELECT role FROM role_users WHERE user_id = ? AND role IN ('admin', 'manager') LIMIT 1",
    [userId],
  );
  if (roles.length > 0) return true;

  const [permissions] = await pool.query(
    "SELECT value FROM permissions WHERE user_id = ? AND target = 'tasks' AND method = ? ORDER BY id DESC LIMIT 1",
    [userId, action],
  );
  if (permissions.length > 0) {
    const value = permissions[0].value?.toLowerCase();
    if (!value || value === "all") return true;
    return value === "own" && ownerId === userId;
  }
  return action === "read" || action === "create" || (action === "update" && ownerId === userId);
}

export async function createApp(pool: DBPool) {
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

  // Tasks CRUD (subset)
  app.get("/tasks", async (req, reply) => {
    const userId = (req as any).userId as number;
    if (!(await canUseTasks(pool, userId, "read"))) {
      return reply.code(403).send({ error: "forbidden" });
    }
    const q = (req.query as any)?.q as string | undefined;
    const limit = Math.min(
      Math.max(Number((req.query as any)?.limit ?? 200), 1),
      500,
    );
    const offset = Math.max(Number((req.query as any)?.offset ?? 0), 0);
    let sql =
      "SELECT id, added_user_id, title, subject_id, description, type, is_require_hand_in, is_self_check_required, deadline, deadline_since, deadline_until, created_at, updated_at FROM tasks WHERE added_user_id = ? AND is_hidden = 0";
    const params: any[] = [userId];
    if (q) {
      sql += " AND (title LIKE ? OR description LIKE ?)";
      params.push(`%${q}%`, `%${q}%`);
    }
    sql += " ORDER BY id DESC LIMIT ? OFFSET ?";
    params.push(limit, offset);
    const [rows] = await pool.query(sql, params);
    reply.send(rows);
  });

  app.post("/tasks", async (req, reply) => {
    const body = req.body as any;
    const userId = (req as any).userId as number;
    if (!(await canUseTasks(pool, userId, "create"))) {
      return reply.code(403).send({ error: "forbidden" });
    }
    if (!body || typeof body.title !== "string" || !body.title.trim() ||
        !Number.isSafeInteger(body.subject_id) || body.subject_id <= 0 ||
        typeof body.deadline !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(body.deadline)) {
      return reply.code(400).send({ error: "invalid body" });
    }
    const [res] = await pool.execute(
      `INSERT INTO tasks (added_user_id, title, subject_id, description, type, is_require_hand_in, is_self_check_required, deadline, deadline_since, deadline_until)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [
        userId,
        body.title,
        body.subject_id,
        body.description ?? null,
        body.type ?? "OTHER",
        body.is_require_hand_in ? 1 : 0,
        body.is_self_check_required ? 1 : 0,
        body.deadline,
        body.deadline_since ?? "NONE",
        body.deadline_until ?? "NONE",
      ],
    );
    reply.code(201).send({ id: (res as any).insertId });
  });

  app.patch("/tasks/:id", async (req, reply) => {
    const id = Number((req.params as any).id);
    const userId = (req as any).userId as number;
    if (!Number.isSafeInteger(id) || id <= 0) return reply.code(400).send({ error: "invalid id" });
    if (!(await canUseTasks(pool, userId, "update", userId))) {
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
    maybe("title");
    maybe("subject_id");
    maybe("description");
    maybe("type");
    if (body.is_require_hand_in !== undefined)
      up("is_require_hand_in", body.is_require_hand_in ? 1 : 0);
    if (body.is_self_check_required !== undefined)
      up("is_self_check_required", body.is_self_check_required ? 1 : 0);
    maybe("deadline");
    maybe("deadline_since");
    maybe("deadline_until");
    if (fields.length === 0) return reply.send({ ok: true });
    const sql = `UPDATE tasks SET ${fields.join(", ")} WHERE id = ? AND added_user_id = ? AND is_hidden = 0`;
    values.push(id, userId);
    await pool.execute(sql, values);
    reply.send({ ok: true });
  });

  app.delete("/tasks/:id", async (req, reply) => {
    const id = Number((req.params as any).id);
    const userId = (req as any).userId as number;
    if (!Number.isSafeInteger(id) || id <= 0) return reply.code(400).send({ error: "invalid id" });
    if (!(await canUseTasks(pool, userId, "delete", userId))) {
      return reply.code(403).send({ error: "forbidden" });
    }
    await pool.execute("DELETE FROM tasks WHERE id = ? AND added_user_id = ? AND is_hidden = 0", [
      id,
      userId,
    ]);
    reply.send({ ok: true });
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
