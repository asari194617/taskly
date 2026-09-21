import { createApp, type DBPool } from "../app.js";
import type { TaskGateway } from "../task_gateway.js";
import assert from "node:assert/strict";
import crypto from "crypto";
import { describe, test } from "node:test";

const sha = (s: string) => crypto.createHash("sha256").update(s).digest("hex");

function mockPool(validToken?: string): DBPool {
  return {
    async query(sql: string, params?: any[]) {
      if (sql.includes("FROM mcps m")) {
        return [validToken && params?.[0] === sha(validToken) ? [{ user_id: 123 }] : []];
      }
      return [[]];
    },
    async execute() { return [{}]; },
  };
}

describe("MCP task bridge", () => {
  test("rejects missing and invalid MCP tokens before forwarding", async () => {
    let calls = 0;
    const gateway: TaskGateway = async () => { calls++; return { status: 200, body: { items: [] } }; };
    const app = await createApp(mockPool("good"), gateway);
    assert.equal((await app.inject({ method: "GET", url: "/tasks" })).statusCode, 401);
    assert.equal((await app.inject({ method: "GET", url: "/tasks", headers: { "x-mcp-token": "bad" } })).statusCode, 401);
    assert.equal(calls, 0);
  });

  test("forwards task operations and preserves MCP responses", async () => {
    const calls: Array<{ token: string; method: string; path: string; body: unknown }> = [];
    const gateway: TaskGateway = async (token, method, path, body) => {
      calls.push({ token, method, path, body });
      if (method === "GET") return { status: 200, body: { items: [{ ID: 7, Title: "task" }], next_cursor: null } };
      if (method === "POST") return { status: 201, body: { ID: 8, Title: "new" } };
      if (method === "PATCH") return { status: 200, body: { id: 8 } };
      return { status: 204, body: undefined };
    };
    const app = await createApp(mockPool("good"), gateway);
    const headers = { "x-mcp-token": "good" };
    const list = await app.inject({ method: "GET", url: "/tasks?limit=50&offset=10&q=report", headers });
    assert.equal(list.statusCode, 200);
    assert.deepEqual(list.json(), [{ id: 7, title: "task" }]);
    const create = await app.inject({ method: "POST", url: "/tasks", headers,
      payload: { title: "new", subject_id: 1, deadline: "2026-09-21" } });
    assert.equal(create.statusCode, 201);
    assert.deepEqual(create.json(), { id: 8 });
    assert.deepEqual((await app.inject({ method: "PATCH", url: "/tasks/8", headers, payload: { title: "changed" } })).json(), { ok: true });
    assert.deepEqual((await app.inject({ method: "DELETE", url: "/tasks/8", headers })).json(), { ok: true });
    assert.deepEqual(calls, [
      { token: "good", method: "GET", path: "/tasks?limit=50&offset=10&q=report", body: undefined },
      { token: "good", method: "POST", path: "/tasks", body: { title: "new", subject_id: 1, deadline: "2026-09-21" } },
      { token: "good", method: "PATCH", path: "/tasks/8", body: { title: "changed" } },
      { token: "good", method: "DELETE", path: "/tasks/8", body: undefined },
    ]);
  });

  test("passes through Go API authorization and validation failures", async () => {
    const gateway: TaskGateway = async () => ({ status: 403, body: { error: "forbidden" } });
    const app = await createApp(mockPool("good"), gateway);
    const res = await app.inject({ method: "POST", url: "/tasks", headers: { "x-mcp-token": "good" },
      payload: { title: "task", subject_id: 1, deadline: "2026-09-21" } });
    assert.equal(res.statusCode, 403);
    assert.deepEqual(res.json(), { error: "forbidden" });
  });

  test("reports task API outages without writing to the database", async () => {
    const gateway: TaskGateway = async () => { throw new Error("connection refused"); };
    const app = await createApp(mockPool("good"), gateway);
    const res = await app.inject({ method: "GET", url: "/tasks", headers: { "x-mcp-token": "good" } });
    assert.equal(res.statusCode, 502);
  });

  test("authenticates only active, unblocked users", async () => {
    const pool: DBPool = {
      async query(sql: string) {
        assert.match(sql, /u\.is_deleted = 0/);
        assert.match(sql, /NOT EXISTS \(SELECT 1 FROM blocks/);
        return [[]];
      },
      async execute() { throw new Error("must not mutate"); },
    };
    const app = await createApp(pool, async () => { throw new Error("must not forward"); });
    const res = await app.inject({ method: "GET", url: "/tasks", headers: { "x-mcp-token": "good" } });
    assert.equal(res.statusCode, 401);
  });

  test("forbids reading another user", async () => {
    const app = await createApp(mockPool("good"), async () => { throw new Error("must not forward"); });
    const res = await app.inject({ method: "GET", url: "/users/456", headers: { "x-mcp-token": "good" } });
    assert.equal(res.statusCode, 403);
  });
});
