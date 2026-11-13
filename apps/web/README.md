# Taskly (Web app)

Astro / React による Web クライアントです。静的ビルドを Cloudflare Pages に配信します。
API と認証サービスは別プロセスです。[リポジトリのローカル開発手順](../../docs/local-development.md)
で API と taskly-auth の設定を確認してください。

## Local development

Node.js 24 を使用します（CI は 24.22.3）。このディレクトリで:

```bash
cp .env.example .env
npm ci
npm run dev
```

`http://localhost:4321` で起動します。`PUBLIC_API_URL` は API の origin、
`PUBLIC_AUTH_OAUTH_START_URL` は API の `/auth/oauth/start`、その他の
`PUBLIC_AUTH_*` は別途起動する認証サービスの画面 URL に設定します。
ログイン・ログアウト・プロフィール・セキュリティ画面を確認する際は、
`.env.production` の本番 URL をローカル用に上書きしてください。

`npm run build` の生成先は `dist/`、`npm run preview` はビルド結果の確認用です。
`npm run storybook` はコンポーネントの開発画面（port 6006）、
`npm run build-storybook` はその静的ビルドを生成します。

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
npx playwright install chromium
npm run test:browser
```

`check` runs Astro and TypeScript diagnostics across pages and React code. The
web CI workflow runs these checks for every pull request because
`web-build-and-test` is a required check.

Unit tests cover API clients, task state transitions, session and redirect
handling, notification settings, and supporting UI logic. The
[mocked browser suite](tests/browser/README.md) covers login redirects, task
creation/editing/completion/search, Web Push settings, mobile navigation, and
admin subject creation. The [real API E2E suite](tests/e2e/README.md) runs against
an isolated MySQL/Redis/API stack and verifies persisted task changes and admin
authorization failures. See the linked guides for fixtures and limitations.

OAuth credentials, MFA, and passkeys are tested in taskly-auth. OS notification
dialogs and real push delivery still require manual browser verification.
