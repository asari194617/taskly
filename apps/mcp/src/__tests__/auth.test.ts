import { createApp, type DBPool } from "../app";
import crypto from "crypto";
import Fastify from "fastify";

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
    expect(res.statusCode).toBe(401);
  });

  test("rejects invalid token", async () => {
    const app = await createApp(mockPool("good"));
    const res = await app.inject({
      method: "GET",
      url: "/tasks",
      headers: { "x-mcp-token": "bad" },
    });
    expect(res.statusCode).toBe(401);
  });

  test("accepts valid token and returns tasks", async () => {
    const app = await createApp(mockPool("good"));
    const res = await app.inject({
      method: "GET",
      url: "/tasks",
      headers: { "x-mcp-token": "good" },
    });
    expect(res.statusCode).toBe(200);
    // Should return an array (empty mocked)
    expect(JSON.parse(res.body)).toEqual([]);
  });
});
