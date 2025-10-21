# Taskly improvements (security-focused)

This document captures recommended improvements identified from a repository-wide review. It is prioritized and grouped by area. Scope spans: apps/server (Go/Fiber), apps/web (Astro/React), apps/mcp (Fastify/Node), infra (Docker Compose, Postfix, Traefik, Airflow), and batches.

Note: This report proposes changes only; no runtime code was modified.

## Priority overview

- Critical (address first)
  - Enforce strong secrets in production and remove insecure defaults
  - Tighten auth/token storage flows and reduce attack surface
  - Lock down email delivery and SMTP/Postfix for prod
  - Add rate limiting and abuse protections for auth and MCP endpoints
  - Harden CORS and security headers for server and web
  - Introduce authorization scoping for MCP API resources
- High
  - CSRF strategy check, GraphQL hardening, and request body limits
  - Logging/observability PII scrubbing and token redaction
  - iCal token lifecycle and scope controls
  - Infrastructure: network exposure, TLS/HSTS, secrets handling
- Medium/Low
  - DX/operational checklists, tests, documentation, and cleanup

---

## Backend (apps/server) – Go/Fiber

Critical

- JWT secret default value

  - Issue: `/auth/login` returns a JWT signed with `JWT_SECRET` defaulting to "dev" when env is missing (`apps/server/routes_auth.go`). This is insecure if JWTs are used.
  - Action: Require `JWT_SECRET` in non-development environments; fail fast on startup if missing. Consider removing JWT issuance entirely if unused by clients, to reduce surface.

- Authentication hardening for email code and password login

  - Issue: Email login codes are 6-digit numeric; cooldown of 60s exists, but there's no explicit per-IP attempt throttling or max verification attempts per code/email.
  - Actions:
    - Add rate limiting on `/auth/request-code` and `/auth/login` per IP and per email (e.g., sliding window, exponential backoff on failures).
    - Track attempts per code/email and lock out after N failed tries until expiration.
    - Shorten code validity if feasible (10 min is okay for email; consider 5–10 min) and invalidate immediately on success (already consumed; keep).

- Session tokens and storage

  - Issue: Opaque session tokens are stored client-side and supplied via `Authorization` header (good), but web app stores them in `localStorage` (see apps/web). Sessions are created with 30-day TTL and platform binding is enforced server-side.
  - Actions:
    - Consider moving to HttpOnly, Secure cookies for browser sessions to mitigate XSS token theft. If staying with Authorization header: add strict CSP, reduce XSS risk, and make CORS stricter (see below).
    - Use sliding expiration and revokeOtherSessions after sensitive events (password change, enabling 2FA, recovery usage). A helper exists (`revokeOtherSessions`); wire it where appropriate.

- CORS policy

  - Issue: `AllowCredentials` is enabled by default and `AllowOrigins` comes from env with common localhost origins. With Authorization header auth, credentials are not required.
  - Actions:
    - Set `AllowCredentials=false` by default when using Authorization header.
    - Provide explicit, minimal origin allowlist in production via `API_CORS_ORIGINS`; avoid wildcards.
    - Ensure preflight caches and headers are correct; avoid allowing arbitrary headers beyond what is necessary.

- Security headers

  - Issue: Server doesn't set headers like `Content-Security-Policy`, `X-Frame-Options`/`frame-ancestors`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`.
  - Actions:
    - Add a middleware to set security headers globally. Sample CSP (tighten paths to your CDN/domains):
      - default-src 'self'; script-src 'self' https://cdn.taskly.iy0306.com 'unsafe-inline' 'unsafe-eval'; style-src 'self' https://cdn.taskly.iy0306.com 'unsafe-inline'; img-src 'self' https://cdn.taskly.iy0306.com data:; connect-src 'self' https://api.taskly.example; frame-ancestors 'none'; base-uri 'self'; form-action 'self'
    - Turn on HSTS at the edge (Traefik) with preload if appropriate.

- SMTP and Postfix in production

  - Issue: SMTP helpers support "smtps" and "starttls" correctly, but infra Postfix image is configured with TLS off and `mynetworks = 0.0.0.0/0` for development (`infra/docker/postfix/main.cf`).
  - Actions:
    - For production, don’t use the dev Postfix. Use a proper email provider or locked-down MTA with enforced TLS, auth, and restricted relay networks. Never keep `mynetworks=0.0.0.0/0`.
    - Ensure certificates are validated (no InsecureSkipVerify anywhere; verify batch email channel too).

- Rate limiting and abuse protection

  - Issue: No explicit rate limiting on server auth endpoints; DoS/brute-force risk.
  - Actions:
    - Add IP-based and user/email-based rate limits for `/auth/*`, `/tasks` mutations, and integration callbacks.
    - Add global request body size limits.

- GraphQL hardening
  - Issue: GraphQL is exposed by `registerGraphQL(app)`. Risk for deep queries, introspection on prod, and potential heavy operations.
  - Actions:
    - Enforce POST-only. Add max query depth/complexity rules and timeouts. Disable introspection on production. Consider persisted queries for public endpoints.
    - Add rate limiting on GraphQL endpoint and ensure auth enforced for sensitive resolvers.

High

- ICS token security and lifecycle

  - Issue: Public iCal endpoint serves calendar by token (`/ical/:token.ics`). Tokens are random and routes exist to manage tokens, but further controls help.
  - Actions:
    - Support token rotation and multiple active tokens with names; enforce short revocation path in UI. Limit feed time window if needed.
    - Add `X-Robots-Tag: noindex, nofollow` for ICS responses and set `Content-Disposition: attachment` to reduce leaking via previews.

- Permissions and default access policy

  - Issue: `checkPermission` defaults allow read/create and only restricts update/delete unless permission override exists.
  - Actions:
    - Consider stricter defaults, especially for create, and harden update/delete rules. Add tests for edge cases (ownership mismatches, soft-deleted users).

- Observability/PII

  - Issue: Error handler attaches user info to Sentry. Queries and IPs are included; bodies not captured, which is good, but further scrubbing recommended.
  - Actions:
    - Redact tokens (`Authorization`, `X-API-Key`) from logs and tracing. Scrub PII fields in breadcrumbs. Tune Datadog sampling.

- Input validation
  - Issue: Fiber handlers rely on `BodyParser` with minimal validation.
  - Actions:
    - Add structured validation (lengths, enums, email format) with clear errors. Reject unknown fields to avoid mass assignment.

Medium/Low

- WebAuthn config review

  - Action: Verify RP ID/origin lists in `initWebAuthn()` and ensure strict origin matching in production.

- Session management UX

  - Action: Add endpoints/UI to list active sessions and revoke others. Use existing Redis sets to drive it.

- Tests
  - Action: Expand GraphQL and REST tests to cover auth failures, permission edges, rate limit boundaries, and ICS token revocation.

References: `apps/server/main.go`, `apps/server/routes_auth.go`, `apps/server/middlewares.go`, `apps/server/support.go`.

---

## Frontend (apps/web) – Astro/React

Critical

- Token storage in browser

  - Issue: GraphQL client reads token from `localStorage('api_token')` and sends via `Authorization` header (`apps/web/src/lib/graphql.ts`).
  - Actions:
    - Prefer HttpOnly, Secure same-site cookies for browser sessions to protect against XSS token theft, or ensure strong CSP and eliminate inline/eval where possible.
    - If cookies are used, add CSRF defenses (double submit or SameSite=Lax/Strict + CSRF token for state-changing requests).

- Security headers (served by Astro/edge)
  - Actions:
    - Mirror server-side header recommendations at the edge (Traefik) for static frontend: CSP, frame-ancestors 'none', X-Content-Type-Options, Referrer-Policy, Permissions-Policy, HSTS.

High

- CSRF model verification

  - Issue: With Authorization header and no cookies, CSRF is less of a risk, but if future cookies are introduced, CSRF must be enforced.
  - Action: Add a design note in docs and ensure any cookie-based session switches in web update server protections accordingly.

- Service worker
  - Action: Ensure SW update strategy is safe. If caching is added later, restrict to immutable assets and validate scope; currently push handling is minimal.

Medium/Low

- UI affordances
  - Action: Add account security pages (2FA, sessions, active devices) and ICS token management with warnings/rotation.

References: `apps/web/src/lib/graphql.ts`, `apps/web/src/layouts/AppLayout.astro`.

---

## MCP service (apps/mcp) – Fastify/Node

Critical

- Authorization scoping

  - Issue: Auth checks only validate `X-MCP-Token` against `mcps.hashed_token`, then allow CRUD on tasks and user records. There’s no owner/tenant scoping.
  - Actions:
    - Restrict all task queries/mutations to the authenticated MCP user_id (e.g., `WHERE added_user_id = :userId`), unless a broader scope is explicitly granted.
    - For user endpoints, limit fields and require that `:id` equals authenticated user unless privileged scope.
    - Introduce token scopes in DB (e.g., read_tasks, write_tasks, read_user_self, write_user_self) and enforce per-route.

- Rate limiting and input limits
  - Issue: No built-in rate limiting. Traefik has labels for average/burst but rely on server-side too.
  - Actions:
    - Add Fastify rate limit plugin per route, body size limits, and timeouts.
    - Validate inputs (types, enum values) to prevent malformed data.

High

- Least-privilege DB user

  - Action: Use a DB user with only necessary privileges for MCP; avoid root-like permissions.

- Logging hygiene

  - Action: Never log raw tokens; ensure redaction. Standardize error responses and avoid leakage of SQL specifics.

- Transport security
  - Action: Serve behind TLS (Traefik) only; enforce HTTPS and HSTS at edge; disable non-TLS listeners in prod.

Medium/Low

- Observability
  - Action: Add basic health/metrics, and alarms for error spikes or rate limit triggers.

References: `apps/mcp/src/index.ts`, Traefik labels in `infra/docker-compose.yml`.

---

## Infrastructure (infra/docker-compose, Traefik, Postfix, Airflow)

Critical

- Postfix config (dev vs prod)

  - Issue: `infra/docker/postfix/main.cf` disables TLS and allows all networks.
  - Actions:
    - Create a separate production postfix or remove it entirely; prefer a managed email provider.
    - If using your own MTA, restrict `mynetworks`, enable TLS, enforce auth, and enable SPF/DKIM/DMARC.

- Network exposure

  - Issue: Compose exposes MySQL, Redis, API, SMTP to host for dev.
  - Actions:
    - In production, bind services to internal networks only and expose only necessary entry points (Traefik/HTTPS, API if needed). Avoid exposing DB/Redis to the internet.

- TLS/HSTS
  - Actions:
    - Ensure Traefik enables HSTS for HTTPS routers and redirect HTTP->HTTPS (already configured). Add preload if domains qualify.

High

- Secrets management

  - Actions:
    - Move all secrets to environment management (Docker secrets, vault, or CI/CD secrets). Do not bake secrets in images.
    - Create a checklist: JWT_SECRET, TOTP_ENC_KEY, SMTP creds, DB creds, Discord/Teams secrets, VAPID keys, API key salts.

- Metrics and logs
  - Actions:
    - Ensure metrics endpoints are not exposed publicly. Restrict `9464` to internal networks only.

Medium/Low

- Airflow
  - Action: Use Postgres for Airflow metastore in non-dev. Lock down UI behind auth.

References: `infra/docker-compose.yml`, `infra/docker/postfix/main.cf`.

---

## Batch jobs (batches/\*)

Critical/High

- Backups

  - Actions:
    - Prefer SFTP/FTPS over plain FTP; encrypt backups at rest (PGP) before upload. Keep short-lived credentials and rotate regularly.
    - Document and test backup restore procedures. Set retention policies and verify pruning jobs.

- Email delivery channel
  - Actions:
    - Verify STARTTLS with proper certificate validation; avoid any InsecureSkipVerify usage. Align with server SMTP helper’s secure modes.

Medium/Low

- Notifications pipeline
  - Actions:
    - Add dead-letter queues and retry limits. Record structured logs for delivery failures (already partially tracked in `tasks_teams_logs`).

---

## Additional cross-cutting improvements

High

- 2FA flows

  - Actions:
    - Enforce 2FA for admin/manager roles. Provide recovery flow UX. Rate limit TOTP/recovery attempts.

- API keys

  - Actions:
    - Add explicit scopes and TTLs to API keys; allow naming/rotation. Ensure route scope middleware fully aligns with intended surface.

- Data privacy
  - Actions:
    - Scrub PII from logs/telemetry. Formalize data retention for logs and queues.

Medium/Low

- Documentation and runbooks
  - Actions:
    - Production checklist for env vars and TLS, incident response, and key rotation. Document CORS/headers and auth model choices.

---

## Quick wins (0–2 days)

- Set strong `JWT_SECRET` and fail fast if missing in non-dev; remove JWT issuance if unused.
- Tighten CORS: disable credentials by default; set strict origin allowlist for prod.
- Add global security headers via middleware; enable HSTS at Traefik.
- Add IP/email rate limits to `/auth/request-code` and `/auth/login` (and `/graphql`).
- Lock dev Postfix to dev only; document production mail provider use.
- MCP: enforce per-user scoping on all task queries/updates; add rate limiting.
- Redact tokens from logs and Sentry; ensure no secrets in traces.

## Near-term (1–2 weeks)

- Decide on browser session storage model (cookies vs Authorization header) and implement CSRF defenses if cookies are used.
- Implement session/device management UI + revoke-other-sessions hooks.
- Add GraphQL depth/complexity limits and disable introspection on prod.
- Introduce API key scopes/TTLs and management UI.
- Harden backup pipeline (SFTP/PGP) and add restore drills.
- Strengthen SMTP/TLS posture for all environments; remove any insecure fallbacks.

## Longer-term (1–2 months)

- Formalize privacy/observability policies, PII scrubbing, and retention.
- Expand automated tests for auth/permissions/rate limits/ICS.
- Migrate Airflow to managed DB and protect UI.
- Introduce centralized secrets management (Vault/Azure Key Vault/GCP Secret Manager).

---

## Acceptance checks

- Security headers visible on responses (server and static); CSP reports clean.
- CORS behaves as expected; only allowed origins succeed; credentials disabled unless required.
- Auth endpoints rate-limited; brute-force attempts are throttled and logged.
- MCP endpoints scoped to token owner; attempts to access other users’ data are denied.
- SMTP in prod enforces TLS and auth; no open relay; email proofs (SPF/DKIM/DMARC) validated.
- Tokens and keys are never logged; Sentry and Datadog show redacted metadata only.
- Backups are encrypted, transferred over secure channels, and successfully restored in test.
