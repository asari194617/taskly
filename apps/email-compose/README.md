# email-compose service

A small Node.js gRPC service that composes transactional emails using React Email. It returns HTML and plain text to the Go mailer, which performs the actual sending.

Features:

- gRPC API for login, security verification, and email address confirmation codes,
  task digests, moderation alerts, day shift notices,
  and day shift skips, with locale (en/ja)
- React Email templates styled similar to shadcn/ui
- i18n via i18next with filesystem backend
- Sentry and Datadog instrumentation hooks

Environment variables:

- EMAIL_COMPOSE_GRPC_HOST (default: 0.0.0.0)
- EMAIL_COMPOSE_GRPC_PORT (default: 50051)
- NODE_ENV (development|production)
- SENTRY_DSN (optional)
- SENTRY_TRACES_SAMPLE_RATE / SENTRY_PROFILES_SAMPLE_RATE (optional)
- DD\_\* variables for Datadog (optional)

## Local development

Use Node.js 24 for the same checks as CI; the container runtime uses Node.js
26.13.1. From this directory:

```bash
npm ci
npm run build
npm start
```

The service listens on gRPC port `50051`. There is no HTTP email-rendering
endpoint. Compose exposes this port only inside its network. The API and
notification mailer connect using `EMAIL_COMPOSE_GRPC_ADDR=email-compose:50051`.
See [`src/proto/email.proto`](src/proto/email.proto) for the RPC contract and the
[local setup guide](../../docs/local-development.md) for the surrounding stack.

Code-composition requests accept `expiry_minutes`. Existing `ComposeLoginCode`
callers that omit it retain the five-minute wording. The two new auth-code RPCs
default to ten minutes. Callers must set the actual code lifetime explicitly.
