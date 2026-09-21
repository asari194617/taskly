# OpenTelemetry traces

The API exports HTTP server spans using OTLP/HTTP when `OTEL_EXPORTER_OTLP_ENDPOINT`
or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is set. Without an endpoint, the exporter is
disabled. Existing Datadog tracing, Prometheus metrics, and Sentry error reporting
continue to run. During rollout, an enabled OTLP exporter and Datadog tracer both
record API requests, so account for duplicate ingestion.

Configure an OTLP/HTTP receiver reachable from the API container:

```env
OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4318
OTEL_TRACES_SAMPLE_RATE=0.1
```

The generic endpoint appends `/v1/traces`; the traces-specific endpoint must include
that path. Standard OpenTelemetry exporter variables such as
`OTEL_EXPORTER_OTLP_HEADERS` and `OTEL_EXPORTER_OTLP_TRACES_HEADERS` are supported
by the SDK for authenticated receivers. Keep credentials in the deployment secret
store, never in source control. `OTEL_TRACES_SAMPLE_RATE` must be from 0 to 1 and
defaults to 0.1. Parent sampling decisions are respected. The service name,
environment, and version are taken from the existing `DD_SERVICE`, `DD_ENV`, and
`DD_VERSION` configuration.

Spans use route templates, HTTP method, and response status. The middleware skips
`/healthz`; it does not attach raw URLs, query strings, request bodies, headers,
or user identifiers. Incoming W3C trace context is propagated through Fiber's
request context for future child spans. The provider flushes on clean shutdown.

This first integration covers API HTTP requests. The existing Datadog GORM/MySQL
integration continues to provide database traces; these are not OTLP child spans.
