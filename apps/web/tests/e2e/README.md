# 実 API E2E テスト

PR の `real-api-e2e` ワークフローの `web-api-db` ジョブで MySQL 8.4、Redis、Go API、Astro、Chromium を起動し、`npm run test:e2e` を実行します。`page.route` は使いません。テスト用 Taskly API でセッションを作成し、課題の作成・一覧・編集・完了・削除、通常ユーザーによる管理 GraphQL 操作の拒否を確認します。画面の再読み込みで DB に保存された結果も確認します。OAuth ログインの実経路は taskly-auth の統合テストで検証します。

ローカル実行は専用の `taskly_e2e` MySQL DB と Redis を用意し、CI ワークフローと同じ環境変数を設定してください。リポジトリルートで `infra/ops/apply-schema-ci.sh` を実行して Atlas でスキーマを適用します（Linux Docker の host network を使用するため、macOS では Linux 開発環境または CI で実行します）。続いて `apps/server` で `go build -o e2e-seed ./cmd/e2e-seed`、`go run ./cmd/server` を実行し、`apps/web` で `npm ci && npx playwright install chromium && npm run test:e2e` を実行します。リセット用コマンドは `TASKLY_E2E_RESET=1` と DB 名 `taskly_e2e` の両方がないと動きません。各テスト前に対象 DB と Redis のテストデータを初期化します。共有 DB や Redis を指定しないでください。旧 `cmd/migrate` は削除済みです。環境変数・ランダムポートの受け渡しは [ワークフロー](../../../../.github/workflows/e2e.yml) を正本とします。

通知ワーカーとメールサービスは起動せず、外部通知は送信しません。失敗時は API ログ、スクリーンショット、Playwright トレース、HTML レポートを `real-api-e2e-results` artifact に保存します。MFA、パスキー、外部認証、通知配信、本番環境は対象外です。
