# Taskly のシステム構成と設計

モノレポ内のサービスとデータの責務を示します。機能の挙動は[現行仕様](Specification.md)、配置や起動方法は[ローカル開発ガイド](local-development.md)と[本番デプロイ手順](initial-settings.md)を参照してください。

## コンポーネント

| コンポーネント | 責務 |
| --- | --- |
| [Web](../apps/web/README.md) | Astro と React による Web クライアント。静的ビルドを Cloudflare Pages に配信し、通常の課題操作や設定に GraphQL を使います。 |
| [API](../apps/server/) | Go と Fiber による GraphQL・REST サーバー。アプリケーションの権限、課題処理、外部連携、認証後のセッションを扱います。 |
| [taskly-auth](https://github.com/asari194617/taskly-auth) | 別リポジトリで管理する OAuth、認証方法、プロフィールのサービスです。Taskly は認証結果から自サービスのセッションを作り、ユーザープロジェクションを同期します。 |
| [MCP](../apps/mcp/README.md) | TypeScript の MCP サービス。課題操作をアプリケーション API へ渡し、スコープの判定と課題の検証は API が行います。 |
| [Airflow とバッチ](../batches/README.md) | Python の DAG と Go バッチで、通知、日直、バックアップ、データ消去などの定期処理を実行します。 |
| [メール生成](../apps/email-compose/README.md)と[受信メール Worker](../cloudflare-email-workers/README.md) | メール生成サービスは React Email で本文を組み立て、gRPC で返します。配信側は生成結果を SMTP で送信します。Cloudflare の Worker は受信メールを API へ転送します。 |

## データと非同期処理

- 永続データは MySQL、API のセッションストアには Redis を使用します。アプリケーション DB の宣言的な定義は [schema.sql](../apps/server/schema.sql) にあり、Atlas で適用します。認証データの管理主体と同期は[プロフィールの管理主体](account-profile-ownership.md)と[ユーザープロジェクション](auth-user-projection-sync.md)を参照してください。
- 課題画像は Cloudflare R2 に保存します。アップロードと保存期限は[課題の画像](features/task-images.md)を参照してください。
- 課題変更に伴う外部通知は、課題と同じ DB トランザクションでジョブを記録し、コミット後に配信します。定期通知の対象選定・配信は Airflow の DAG が実行します。[連携ジョブと再試行](integration-jobs.md)、[定期通知バッチ](batches/notifications/README.md)
- 日直の担当は有効なサイクル、登録グループの順序、現在のインデックスで求めます。学校カレンダーは `SCHOOL_CAL_URL` で設定し、対象日に重なる予定の `DAY_SHIFT_OFF` を参照します。受信した日直メールはキーワードで判定し、それで該当せず Gemini が設定済みの場合は Gemini でも判定します。[日直バッチ](../infra/airflow/dags/day_shift_index_increment.py)、[日直メールの判定](../apps/server/internal/utils/support_ai.go)

## インターフェースと運用

- GraphQL・REST・MCP の境界と認証は [API の使い分け](api-boundaries.md)、別オリジンでの Cookie と CORS の設定は[ローカル開発ガイド](local-development.md)を参照してください。公開 REST の契約は [OpenAPI](../apps/docs/openapi/openapi.yaml) に定義します。
- API、MCP、メール生成、Airflow のコンテナを GitHub Actions と Docker Compose で配備します。デプロイと DB スキーマ管理は[本番デプロイ手順](initial-settings.md)、メトリクスや接続範囲は[監視](operations/metrics-monitoring.md)と[ネットワークポート](operations/network-ports.md)を参照してください。
