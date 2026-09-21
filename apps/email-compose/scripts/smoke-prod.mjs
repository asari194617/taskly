import { spawn } from "node:child_process";
import net from "node:net";
import process from "node:process";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

const host = "127.0.0.1";

async function reservePort() {
  const server = net.createServer();
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, host, resolve);
  });
  const address = server.address();
  const port = typeof address === "object" && address ? address.port : 0;
  await new Promise((resolve) => server.close(resolve));
  return port;
}

function unary(client, method, request) {
  return new Promise((resolve, reject) => {
    client[method](request, (error, response) => {
      if (error) reject(error);
      else resolve(response);
    });
  });
}

function assertResponse(method, response) {
  for (const field of ["subject", "html", "text"]) {
    if (typeof response?.[field] !== "string" || response[field].length === 0) {
      throw new Error(`${method} returned an empty ${field}`);
    }
  }
}

const port = await reservePort();
const address = `${host}:${port}`;
const child = spawn(process.execPath, ["dist/index.js"], {
  env: {
    ...process.env,
    DD_TRACE_ENABLED: "false",
    EMAIL_COMPOSE_GRPC_HOST: host,
    EMAIL_COMPOSE_GRPC_PORT: String(port),
    NODE_ENV: "production",
    SENTRY_DSN: "",
  },
  stdio: ["ignore", "pipe", "pipe"],
});

let output = "";
child.stdout.on("data", (chunk) => {
  output += chunk;
});
child.stderr.on("data", (chunk) => {
  output += chunk;
});

try {
  const packageDefinition = protoLoader.loadSync("dist/proto/email.proto", {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  const proto = grpc.loadPackageDefinition(packageDefinition).emailcompose;
  const client = new proto.EmailComposer(
    address,
    grpc.credentials.createInsecure(),
  );

  await new Promise((resolve, reject) => {
    client.waitForReady(Date.now() + 15_000, (error) => {
      if (error) reject(new Error(`server did not become ready: ${error.message}`));
      else resolve();
    });
  });

  const cases = [
    [
      "composeLoginCode",
      { locale: "ja", email: "smoke@example.test", code: "123456" },
    ],
    [
      "composeTaskDigest",
      {
        locale: "ja",
        items: [
          {
            when_label: "今日",
            subject: "数学",
            title: "演習問題",
            body: "問題集を解く",
            meta: "今日まで",
            badges: [{ label: "宿題", variant: "blue" }],
          },
        ],
      },
    ],
    [
      "composeModerationAlert",
      {
        locale: "ja",
        total_flagged: 1,
        auto_hidden: 0,
        review_url: "https://example.test/review",
        highlights: [
          { id: "1", title: "確認対象", category: "text", severity: "low" },
        ],
      },
    ],
    [
      "composeDayShift",
      { locale: "ja", members: ["テスト担当"], events: ["朝会"] },
    ],
    [
      "composeDayShiftSkip",
      {
        locale: "ja",
        today_date: "2026-09-21",
        today_names: "テスト担当",
        next_date: "2026-09-22",
        next_names: "次の担当",
      },
    ],
  ];

  for (const [method, request] of cases) {
    const response = await unary(client, method, request);
    assertResponse(method, response);
  }

  client.close();
  console.log(`production smoke test passed (${cases.length} gRPC methods)`);
} catch (error) {
  if (output) process.stderr.write(output);
  throw error;
} finally {
  child.kill("SIGTERM");
  await Promise.race([
    new Promise((resolve) => child.once("exit", resolve)),
    new Promise((resolve) => setTimeout(resolve, 2_000)),
  ]);
  if (child.exitCode === null) child.kill("SIGKILL");
}
