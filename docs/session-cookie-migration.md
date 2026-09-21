# Web session cookie migration (Issue #237)

The web client authenticates to the API with a host-only `taskly_session`
cookie set by `api.taskly.iy0306.com`. It is `HttpOnly`, `SameSite=Lax`, and
`Secure` whenever `NODE_ENV=production`. The API stores the opaque token in
Redis for 30 days and its hash in `sessions` for fallback validation. Native
clients continue to use `Authorization: Bearer` and receive a token in the
login response. API keys and MCP tokens are unchanged.

Production uses `https://taskly.iy0306.com` for the web app and
`https://api.taskly.iy0306.com` for the API. These are cross-origin but
same-site, so the web client uses `credentials: "include"`, the API permits
credentialed requests only from the explicit `API_CORS_ORIGINS` list, and the
cookie remains host-only to the API. Do not configure a wildcard origin or
`SameSite=None` without a separate security review.

For any request carrying the session cookie, unsafe methods require both an
allowed `Origin` and `X-CSRF-Protection: 1`. This includes GraphQL requests
(queries use POST) and REST updates. Requests authenticated only with Bearer
or `X-API-Key` do not need that header. Browser login requests must have an
allowed Origin in production; a browser cannot use `X-Client-Platform` to
obtain a native bearer response.

## Release checklist

1. Verify production `NODE_ENV=production`, `API_CORS_ORIGINS` includes the
   exact web origin (`https://taskly.iy0306.com`), and
   `CORS_ALLOW_CREDENTIALS=true`. Confirm the web build's `PUBLIC_API_URL`
   points to the API HTTPS origin. Never disable Secure in production.
2. Coordinate API and static web deployment. They change the login response
   contract simultaneously (`token` to `authenticated`). During the window
   between deployments, logins on the mismatched versions may fail. Announce
   that short window or temporarily pause sign-in; verify both deployments
   before declaring the release complete. Existing pages should be reloaded.
3. Existing web sessions must sign in again. The new web bundle removes the
   legacy `localStorage.api_token` as soon as it makes a request; no browser
   bearer fallback or token-exchange endpoint is retained. Native sessions
   remain valid.
4. Smoke-test password, email-code, TOTP/recovery, and passkey logins; page
   reload, GraphQL mutation, notification settings, logout, and Safari/PWA.
   Check the login JSON never contains a token and the response Cookie has
   `HttpOnly; Secure; SameSite=Lax`. Confirm invalid Origin / missing CSRF
   header is rejected.
5. Rollback both web and API to the preceding release together if needed.
   A cookie-authenticated browser must re-login after rollback because the old
   client cannot read the HttpOnly cookie. Do not copy it into localStorage.

When changing an origin or proxy, review Cookie Domain/Path, TLS termination,
CORS, `Origin` validation, and the WebAuthn relying-party origin together.
