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

Build: npm run build
Run: node dist/index.js

Code-composition requests accept `expiry_minutes`. Existing `ComposeLoginCode`
callers that omit it retain the five-minute wording. The two new auth-code RPCs
default to ten minutes. Callers must set the actual code lifetime explicitly.
