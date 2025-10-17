# Sentry integration

Server-side errors and panics are reported to Sentry.

Configuration via environment variables:

- SENTRY_DSN: Your Sentry DSN (if empty, Sentry is disabled)
- SENTRY_ENVIRONMENT: Environment name (defaults to NODE_ENV or "development")
- SENTRY_RELEASE: Optional release version identifier

Fiber middleware and a centralized ErrorHandler capture:

- Unhandled panics (via Sentry Fiber middleware + recover)
- 5xx responses returned by handlers (ErrorHandler)
- Request context (method, route, URL, query, IP)
- Authenticated user context when available (id, email, display_name)

Noisy client errors (4xx) are not sent to Sentry.

Local testing:

1. Set SENTRY_DSN to your project DSN
2. Run the server
3. Hit a route that triggers a panic or returns a 500 to see the event in Sentry
