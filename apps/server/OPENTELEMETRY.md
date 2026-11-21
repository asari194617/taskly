# OpenTelemetry トレース

`OTEL_EXPORTER_OTLP_ENDPOINT` または `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` を設定すると、API は HTTP サーバーのスパンを OTLP/HTTP で送信します。どちらも未設定ならエクスポーターは無効です。既存の Datadog トレース、Prometheus メトリクス、Sentry のエラー報告は引き続き動作します。導入時に OTLP エクスポーターと Datadog トレーサーを同時に有効にすると、双方が API リクエストを記録するため、両方への取り込みを見込んでください。

API コンテナから接続できる OTLP/HTTP レシーバーを設定します。

```env
OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4318
OTEL_TRACES_SAMPLE_RATE=0.1
```

共通エンドポイントには `/v1/traces` が付加されます。トレース専用のエンドポイントには、そのパスも指定してください。認証が必要なレシーバーには、SDK が対応する標準変数 `OTEL_EXPORTER_OTLP_HEADERS` や `OTEL_EXPORTER_OTLP_TRACES_HEADERS` を使えます。認証情報はデプロイ用のシークレットに保管し、ソース管理に含めないでください。`OTEL_TRACES_SAMPLE_RATE` は 0 から 1 の範囲で、既定値は 0.1 です。親スパンのサンプリング結果を引き継ぎます。サービス名、環境名、バージョンには既存の `DD_SERVICE`、`DD_ENV`、`DD_VERSION` を使います。

スパンにはルートのテンプレート、HTTP メソッド、応答ステータスを記録します。ミドルウェアは `/healthz` を除外し、生の URL、クエリ文字列、リクエスト本文、ヘッダー、ユーザー識別子を記録しません。受信した W3C Trace Context は Fiber のリクエストコンテキストを経由して後続の子スパンに伝播します。正常終了時にプロバイダーが未送信のスパンを送ります。

この連携の対象は API の HTTP リクエストです。DB のトレースは引き続き Datadog の GORM / MySQL 連携が収集します。DB のスパンは OTLP の子スパンにはなりません。
