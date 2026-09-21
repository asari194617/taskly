# Datadog APM integration

This service is instrumented with Datadog for tracing (HTTP/Fiber, GORM/MySQL) and optional profiling.

Environment variables (read automatically by the tracer):

- DD_TRACE_AGENT_URL: Agent URL. Docker Compose uses `unix:///var/run/datadog/apm.socket`.
- DD_DOGSTATSD_URL: Runtime metrics URL. Docker Compose uses `unix:///var/run/datadog/dsd.socket`.
- DD_AGENT_HOST: Datadog Agent host (default 127.0.0.1)
- DD_TRACE_AGENT_PORT: Agent trace port (default 8126)
- DD_SERVICE: Service name (default taskly-server)
- DD_ENV: Environment (e.g. prod, staging)
- DD_VERSION: Version string (e.g. git sha)
- DD_API_KEY: Not needed by the app directly; required for the Agent if using remote intake
- DD_PROFILING_ENABLED: Set to "true" to enable the in-process CPU/heap profiler
- DD_DB_SERVICE: Optional override for DB service name (default: "${DD_SERVICE}-db")

What’s instrumented:

- Fiber HTTP server: request spans for each route
- GORM (MySQL): query spans via database/sql wrapper + gorm plugin
- Tracer runtime metrics enabled (GC, goroutines, etc.)
- Optional pprof-like Datadog profiler (if DD_PROFILING_ENABLED=true)

How to verify locally:

1. Run a Datadog Agent with APM enabled. Compose expects the standard
   `/var/run/datadog/apm.socket` and `/var/run/datadog/dsd.socket` sockets;
   a server started outside Compose can use `${DD_AGENT_HOST}:${DD_TRACE_AGENT_PORT}`.
2. Start the server and hit a few routes
3. In Datadog APM, look for the service DD_SERVICE and DB child service DD_DB_SERVICE

Notes:

- `DD_TRACE_AGENT_URL` takes precedence over `DD_AGENT_HOST` and `DD_TRACE_AGENT_PORT`.
- Compose mounts only the Agent socket directory and does not publish APM or DogStatsD ports.
- The tracer reads many settings automatically from env vars; no code change is needed per environment.
- To filter noisy routes from traces, adjust the Fiber middleware options.
