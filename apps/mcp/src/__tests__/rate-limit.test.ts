import assert from "node:assert/strict";
import { test } from "node:test";
import { createApp } from "../app.js";

test("rotating tokens and forwarded IPs cannot bypass the pre-authentication budget", async () => {
  let queries = 0;
  const app = await createApp({
    async query() { queries++; return [[]]; },
    async execute() { throw new Error("unexpected write"); },
  });
  try {
    for (let i = 0; i < 300; i++) {
      const response = await app.inject({ url: "/tasks", headers: {
        "x-mcp-token": `invalid-${i}`, "x-forwarded-for": `192.0.2.${i % 254 + 1}`,
      } });
      assert.equal(response.statusCode, 401);
    }
    const blocked = await app.inject({ url: "/users/1", headers: { "x-mcp-token": "another-token" } });
    assert.equal(blocked.statusCode, 429);
    assert.ok(Number(blocked.headers["retry-after"]) > 0);
    assert.equal(queries, 300);
    // Liveness does no database work and must survive rate limiting.
    assert.equal((await app.inject("/healthz")).statusCode, 200);
    assert.equal((await app.inject("/readyz")).statusCode, 429);
    assert.equal(queries, 300);
    assert.equal((await app.inject({ url: "/tasks", remoteAddress: "192.0.2.1",
      headers: { "x-mcp-token": "invalid" } })).statusCode, 401);
    assert.equal(queries, 301);
  } finally {
    await app.close();
  }
});

test("readiness database probes have a bounded budget too", async () => {
  let queries = 0;
  const app = await createApp({
    async query() { queries++; return [[{ 1: 1 }]]; },
    async execute() { throw new Error("unexpected write"); },
  }, undefined, async () => true);
  try {
    for (let i = 0; i < 300; i++) assert.equal((await app.inject("/readyz")).statusCode, 200);
    assert.equal((await app.inject("/readyz")).statusCode, 429);
    assert.equal(queries, 300);
    assert.equal((await app.inject("/healthz")).statusCode, 200);
  } finally {
    await app.close();
  }
});
