import mysql from "mysql2/promise";
import { createApp } from "./app.js";

type Cfg = {
  port: number;
  host: string;
  db: {
    host: string;
    port: number;
    user: string;
    password: string;
    database: string;
  };
};

function env(name: string, def?: string): string {
  const v = process.env[name];
  if (!v && def === undefined) throw new Error(`Missing env: ${name}`);
  return v ?? def!;
}

function cfgFromEnv(): Cfg {
  return {
    port: Number(env("MCP_PORT", "3066")),
    host: env("MCP_HOST", "0.0.0.0"),
    db: {
      host: env("DB_HOST", "127.0.0.1"),
      port: Number(env("DB_PORT", "3306")),
      user: env("MYSQL_USER", "root"),
      password: env("MYSQL_PASSWORD", ""),
      database: env("MYSQL_DATABASE", "taskly"),
    },
  };
}

async function build() {
  const cfg = cfgFromEnv();
  // DB pool
  const pool = mysql.createPool({
    host: cfg.db.host,
    port: cfg.db.port,
    user: cfg.db.user,
    password: cfg.db.password,
    database: cfg.db.database,
    connectionLimit: 10,
    namedPlaceholders: true,
  });
  const app = await createApp(pool);

  const addr = { port: cfg.port, host: cfg.host } as const;
  await app.listen(addr);
  (app.log as any).info?.(
    `MCP server listening on http://${addr.host}:${addr.port}`
  );
}

build().catch((err) => {
  console.error("fatal: ", err);
  process.exit(1);
});
console.log("MCP server scaffold");
