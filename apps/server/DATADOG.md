# Datadog APM 連携

Datadog で HTTP（Fiber）と DB（GORM / MySQL）のトレースを収集します。プロファイリングも任意で有効にできます。

## トレーサーが自動で読み取る環境変数

- `DD_TRACE_AGENT_URL` はエージェントの URL。Docker Compose では `unix:///var/run/datadog/apm.socket`
- `DD_DOGSTATSD_URL` はランタイムメトリクスの送信先。Docker Compose では `unix:///var/run/datadog/dsd.socket`
- `DD_AGENT_HOST` は Datadog Agent のホスト（既定値 `127.0.0.1`）
- `DD_TRACE_AGENT_PORT` はトレースの送信先ポート（既定値 `8126`）
- `DD_SERVICE` はサービス名（既定値 `taskly-server`）
- `DD_ENV` は環境名（例：`prod`、`staging`）
- `DD_VERSION` はバージョン文字列（例：Git のコミット SHA）
- `DD_API_KEY` はエージェントがリモートの受け入れ先に送信する場合に必要。アプリは直接参照しない
- `DD_PROFILING_ENABLED=true` でプロセス内の CPU・ヒーププロファイラーを有効化
- `DD_DB_SERVICE` で DB サービス名を上書き可能（既定値 `${DD_SERVICE}-db`）

## 計装対象

- Fiber HTTP サーバーのルートごとのリクエストスパン
- `database/sql` ラッパーと GORM プラグインを経由した MySQL のクエリスパン
- トレーサーのランタイムメトリクス（GC、ゴルーチンなど）
- pprof と同様の Datadog プロファイラー（`DD_PROFILING_ENABLED=true` の場合）

## ローカルでの確認

1. APM を有効にした Datadog Agent を起動します。Compose は標準ソケット `/var/run/datadog/apm.socket` と `/var/run/datadog/dsd.socket` を参照します。Compose の外で起動したサーバーは `${DD_AGENT_HOST}:${DD_TRACE_AGENT_PORT}` に接続できます。
2. サーバーを起動し、いくつかのルートにリクエストを送信します。
3. Datadog APM でサービス `DD_SERVICE` と DB の子サービス `DD_DB_SERVICE` を確認します。

## 運用上の補足

- `DD_TRACE_AGENT_URL` は `DD_AGENT_HOST` と `DD_TRACE_AGENT_PORT` より優先されます。
- 本番環境の Compose の上書き設定はエージェントのソケットディレクトリだけをマウントします。APM や DogStatsD のポートは公開しません。
- トレーサーは各種設定を環境変数から自動で読み取ります。環境ごとのコード変更は不要です。
- トレースから不要なルートを除外するには、Fiber ミドルウェアのオプションを調整します。
