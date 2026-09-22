import { createApp, type DBPool } from "../app.js";
import assert from "node:assert/strict";
import { test } from "node:test";
import { createServer } from "node:http";

test("readiness follows MySQL and Go API outages and recovery", async () => {
  let dbReady = true;
  let apiReady = true;
  const pool: DBPool = {
    async query(query: { sql: string; timeout: number }) {
      assert.equal(query.sql, "SELECT 1");
      assert.ok(query.timeout <= 2000);
      if (!dbReady) throw new Error("MySQL unavailable");
      return [[{ 1: 1 }]];
    },
    async execute() { throw new Error("unexpected write"); },
  };
  const app = await createApp(pool, undefined, async () => apiReady);
  try {
    const status = async (path: string) => (await app.inject({ method: "GET", url: path })).statusCode;
    assert.equal(await status("/readyz"), 200);
    dbReady = false;
    assert.equal(await status("/healthz"), 200);
    assert.equal(await status("/readyz"), 503);
    dbReady = true;
    assert.equal(await status("/readyz"), 200);
    apiReady = false;
    assert.equal(await status("/readyz"), 503);
    apiReady = true;
    assert.equal(await status("/readyz"), 200);
  } finally {
    await app.close();
  }
});

test("readiness calls the configured Go API readiness route", async () => {
  let apiReady = true;
  const upstream = createServer((req, res) => {
    assert.equal(req.url, "/readyz");
    res.writeHead(apiReady ? 200 : 503).end();
  });
  await new Promise<void>((resolve) => upstream.listen(0, "127.0.0.1", resolve));
  const address = upstream.address();
  assert.ok(address && typeof address !== "string");
  const previous = process.env.TASKLY_API_BASE_URL;
  process.env.TASKLY_API_BASE_URL = `http://127.0.0.1:${address.port}`;
  const app = await createApp({
    async query() { return [[{ 1: 1 }]]; },
    async execute() { throw new Error("unexpected write"); },
  });
  try {
    assert.equal((await app.inject("/readyz")).statusCode, 200);
    apiReady = false;
    assert.equal((await app.inject("/readyz")).statusCode, 503);
    apiReady = true;
    assert.equal((await app.inject("/readyz")).statusCode, 200);
  } finally {
    await app.close();
    upstream.close();
    if (previous === undefined) delete process.env.TASKLY_API_BASE_URL;
    else process.env.TASKLY_API_BASE_URL = previous;
  }
});
