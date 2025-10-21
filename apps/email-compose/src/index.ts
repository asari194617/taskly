import "./tracing.js";
import { initI18n } from "./i18n.js";
import { startGrpcServer } from "./server.js";
import { initSentry } from "./sentry.js";
import { startHttpServer } from "./http.js";

async function main() {
  // Load env
  const env = process.env;
  const PORT = Number(env.EMAIL_COMPOSE_GRPC_PORT || 50051);
  const HOST = env.EMAIL_COMPOSE_GRPC_HOST || "0.0.0.0";
  const NODE_ENV = env.NODE_ENV || "development";

  // i18n
  await initI18n(process.cwd() + "/dist/locales");

  // Sentry
  initSentry();

  // Start gRPC server
  await startGrpcServer({ host: HOST, port: PORT });
  // Also start HTTP fallback
  const HTTP_PORT = Number(env.EMAIL_COMPOSE_HTTP_PORT || 50052);
  startHttpServer(HTTP_PORT);
  console.log(
    `email-compose gRPC listening on ${HOST}:${PORT} and HTTP on ${HOST}:${HTTP_PORT} (env=${NODE_ENV})`
  );
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
