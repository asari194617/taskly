import { createApp, type DBPool } from "../app.js";
import assert from "node:assert/strict";
import crypto from "crypto";
import { describe, test } from "node:test";

describe("MCP auth preHandler", () => {
  const sha = (s: string) =>
    crypto.createHash("sha256").update(s).digest("hex");

  function mockPool(validToken?: string): DBPool {
    return {
      async query(sql: any, params?: any[]) {
        const hashed = params?.[0];
        if (validToken && hashed === sha(validToken)) {
          return [[{ user_id: 123 }]] as any;
        }
        return [[]] as any;
      },
      async execute() {
        return [{} as any];
      },
    } as DBPool;
  }

  test("rejects missing token", async () => {
    const app = await createApp(mockPool());
    const res = await app.inject({ method: "GET", url: "/tasks" });
    assert.equal(res.statusCode, 401);
  });

  test("rejects invalid token", async () => {
    const app = await createApp(mockPool("good"));
    const res = await app.inject({
      method: "GET",
      url: "/tasks",
      headers: { "x-mcp-token": "bad" },
    });
    assert.equal(res.statusCode, 401);
  });

  test("accepts valid token and returns tasks", async () => {
    const app = await createApp(mockPool("good"));
    const res = await app.inject({
      method: "GET",
      url: "/tasks",
      headers: { "x-mcp-token": "good" },
    });
    assert.equal(res.statusCode, 200);
    // Should return an array (empty mocked)
    assert.deepEqual(JSON.parse(res.body), []);
  });

  test("scopes task reads and mutations to the token owner", async () => {
    const queries: Array<{ sql: string; params?: any[] }> = [];
    const executions: Array<{ sql: string; params?: any[] }> = [];
    const pool: DBPool = {
      async query(sql: string, params?: any[]) {
        if (sql.startsWith("SELECT user_id FROM mcps")) {
          return [[{ user_id: 123 }]];
        }
        queries.push({ sql, params });
        return [[]];
      },
      async execute(sql: string, params?: any[]) {
        executions.push({ sql, params });
        return [{}];
      },
    };
    const app = await createApp(pool);
    const headers = { "x-mcp-token": "good" };

    await app.inject({
      method: "GET",
      url: "/tasks?q=owner",
      headers,
    });
    await app.inject({
      method: "PATCH",
      url: "/tasks/45",
      headers,
      payload: { title: "updated" },
    });
    await app.inject({ method: "DELETE", url: "/tasks/46", headers });

    assert.match(queries[0].sql, /WHERE added_user_id = \?/);
    assert.match(
      queries[0].sql,
      /AND \(title LIKE \? OR description LIKE \?\)/,
    );
    assert.deepEqual(queries[0].params, [123, "%owner%", "%owner%", 200, 0]);
    assert.match(executions[0].sql, /WHERE id = \? AND added_user_id = \?$/);
    assert.deepEqual(executions[0].params, ["updated", 45, 123]);
    assert.equal(
      executions[1].sql,
      "DELETE FROM tasks WHERE id = ? AND added_user_id = ?",
    );
    assert.deepEqual(executions[1].params, [46, 123]);
  });

  test("forbids reading another user", async () => {
    const app = await createApp(mockPool("good"));
    const res = await app.inject({
      method: "GET",
      url: "/users/456",
      headers: { "x-mcp-token": "good" },
    });
    assert.equal(res.statusCode, 403);
  });
});
