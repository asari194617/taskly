# Taskly Web アプリ

Astro / React による Web クライアントです。静的ビルドを Cloudflare Pages に配信します。
API と認証サービスは別プロセスです。[リポジトリのローカル開発手順](../../docs/local-development.md)
で API と taskly-auth の設定を確認してください。

## ローカル開発

Node.js 26 を使用します（CI は 26.13.1）。このディレクトリで:

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

プレビュービルドの正規オリジンは既定で `https://taskly.iy0306.com` です。プレビュー専用の正規オリジンを公開する場合は、次のようにビルド時の環境変数 `SITE_URL` を指定します。

```bash
SITE_URL=https://preview.example.com npm run build
```

## Web クライアントの検証

`apps/web` で Node.js 26 を使い、以下を実行します。

```bash
npm ci
npm run check
npm run test:unit
npm run build
npm run test:picomatch-exposure
npm run test:canonical
npm run test:security-headers
npx playwright install chromium
npm run test:browser
```

`check` はページと React コードに対し、Astro と TypeScript の診断を実行します。必須チェックの `web-build-and-test` に含まれるため、Web の CI はすべてのプルリクエストでこれらの検証を行います。

単体テストでは API クライアント、課題の状態遷移、セッションとリダイレクト、通知設定、関連する UI ロジックを確認します。[モックを使うブラウザテスト](tests/browser/README.md)は、ログイン時のリダイレクト、課題の作成・編集・完了・検索、Web Push 設定、モバイルでの画面遷移、管理者による科目作成を確認します。[実 API を使う E2E テスト](tests/e2e/README.md)は、分離した MySQL、Redis、API の構成で、課題の変更が永続化されることと管理者権限のない操作が拒否されることを確認します。フィクスチャと制約は各ガイドを参照してください。

OAuth 認証情報、多要素認証、パスキーは taskly-auth で検証します。OS の通知ダイアログと実際のプッシュ配信はブラウザーで手動確認します。
