# email-compose サービス

React Email でメールを生成する小規模な Node.js gRPC サービスです。HTML とプレーンテキストを Go のメール送信処理に返します。

## 機能

- ログイン、セキュリティ確認、メールアドレス確認のコード、課題のダイジェスト、モデレーションの警告、日直の通知とスキップを扱う gRPC API（日本語・英語対応）
- shadcn/ui に合わせたスタイルの React Email テンプレート
- ファイルシステムをバックエンドに使う i18next による国際化
- Sentry と Datadog の計装フック

## 環境変数

- `EMAIL_COMPOSE_GRPC_HOST`（既定値 `0.0.0.0`）
- `EMAIL_COMPOSE_GRPC_PORT`（既定値 `50051`）
- `NODE_ENV`（`development` または `production`）
- `SENTRY_DSN`（任意）
- `SENTRY_TRACES_SAMPLE_RATE` / `SENTRY_PROFILES_SAMPLE_RATE`（任意）
- Datadog 用の `DD_*` 変数（任意）

## ローカル開発

CI と同じ検証には Node.js 26 を使います。コンテナのランタイムは Node.js 26.13.1 です。このディレクトリで次を実行します。

```bash
npm ci
npm run build
npm start
```

サービスは gRPC のポート `50051` で待ち受けます。メール生成用の HTTP エンドポイントはありません。Compose ではネットワーク内にのみこのポートを公開します。API と通知のメール送信処理は `EMAIL_COMPOSE_GRPC_ADDR=email-compose:50051` を使って接続します。RPC の仕様は [`src/proto/email.proto`](src/proto/email.proto)、周辺サービスの構成は[ローカル開発ガイド](../../docs/local-development.md)を参照してください。

認証コードメールの生成リクエストには `expiry_minutes` を指定できます。既存の `ComposeLoginCode` の呼び出しで値を省略すると、従来どおり有効期限を 5 分と表示します。新しい 2 つの認証コード用 RPC の既定値は 10 分です。呼び出し元はコードの実際の有効期間を明示してください。
