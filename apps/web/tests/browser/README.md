# Web ブラウザ結合テスト

`npm ci && npx playwright install chromium && npm run test:browser` で実行します。CI は Chromium と Astro 開発サーバーを起動し、失敗時のスクリーンショットとトレースを `web-browser-results` artifact に保存します。

他の開発サーバーが既定ポートで動いている場合は、`PLAYWRIGHT_PORT=4369 npm run test:browser` のように空いているポートを指定します。

テスト対象はパスワードログイン、課題の作成・編集・完了・検索、完了状態フィルターの Tab・Enter・Space 操作とアクセシブルネーム・選択状態・解除、管理者の教科作成、課題一覧 API の失敗表示と再読み込みです。実際の React 画面、画面遷移、ブラウザの入力を通します。

API は各テストが `page.route` で置き換えます。課題と教科はテストごとに新しいメモリ上の配列を作り、固定のユーザーと教科から始めます。DB、メール、外部認証サービス、本番環境には接続しません。API 実装と DB の結合、MFA・パスキー、モバイル画面、管理者以外の権限拒否は対象外です。
