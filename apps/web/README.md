# Taskly (Web app)

Webクライアント用のフォルダ

Preview builds use `https://taskly.iy0306.com` as their canonical origin by
default. To publish a preview-specific canonical origin, provide `SITE_URL` in
the build environment, for example:

```bash
SITE_URL=https://preview.example.com npm run build
```

## Web checks

Run with Node 24 from `apps/web`:

```bash
npm ci
npm run check
npm run test:unit
npm run build
npm run test:canonical
npm run test:security-headers
```

`check` runs Astro and TypeScript diagnostics across pages and React code. The
web CI workflow runs these commands for pull requests that change the web app.

The first flow tests cover task search (including the older API schema and
failure), task update rollback, completion delay/undo/API failure, session
checks and redirect safety, and notification method/type saves with rollback
for HTTP or network failures. API calls are mocked at the GraphQL or fetch
boundary; state tests exercise the helpers used by the screens.

This set does not yet cover task creation/deletion, the full login and MFA
forms, admin CRUD, browser PushManager permissions/subscriptions, or visual
layout. Those need separate browser-level coverage and fixtures.
