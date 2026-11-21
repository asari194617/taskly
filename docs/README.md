# ドキュメント一覧

初めてチェックアウトした場合は、[リポジトリの構成](../README.md#repository-structure)と[ローカル開発ガイド](local-development.md)を確認してください。運用資料はリポジトリ内の実装を説明します。本番環境の認証情報、インシデントの証跡、ユーザーデータはアクセスを制限した記録に保管します。

## システム構成と機能

- [Taskly の現行仕様](Specification.md)と[システム構成と設計](architecture.md)
- [リポジトリ概要](../README.md)と [Web クライアントの開発](../apps/web/README.md)
- [API の責務](api-boundaries.md)、[公開 OpenAPI](../apps/docs/openapi/openapi.yaml)、[スキーマの概要](schema.md)
- [Web での課題操作](clients/web/README.md)と[モバイルでの課題入力](clients/web/mobile-task-entry.md)
- [課題の画像](features/task-images.md)と[トークン有効期限の入力](features/token-expiration-input.md)
- [通知設定](notifications.md)、[バッチによる通知配信](batches/notifications/README.md)、[バッチ一覧](../batches/README.md)
- [連携ジョブとトランザクショナルアウトボックス](integration-jobs.md)
- [iCal フィード](ical-feed.md)と [MCP ガイド](mcp-docs.md)
- [メール生成サービス](../apps/email-compose/README.md)と[送信者ロゴ](email-sender-logo.md)
- [セキュリティ関連の破壊的操作の確認](features/security-destructive-action-confirmations.md)、[モバイルの設定画面遷移](features/settings-mobile-navigation.md)、[ヘッダーロゴによる画面遷移](features/header-logo-navigation.md)
- [GraphQL の管理者権限](apps/server/graphql-admin-authorization.md)と[Teams と日直の連携](apps/server/day-shift-skip.md)

## 認証とアカウントデータ

Web のログイン、認証方法の管理、プロフィール画面には [taskly-auth](https://github.com/asari194617/taskly-auth) を使います。認証サービスの起動は [開発環境のセットアップ](https://github.com/asari194617/taskly-auth#開発環境)、認証 DB の移行と切り戻しは [本番認証 DB 移行手順](https://github.com/asari194617/taskly-auth/blob/main/docs/production-migration.md)を参照してください。以下の資料では、Taskly 側でデータの管理主体を切り替える条件と同期仕様を説明します。

- [認証データの書き込み箇所と移行条件](auth-identity-write-inventory.md)
- [ユーザープロジェクション](auth-user-projection-sync.md)と[プロフィールの管理主体](account-profile-ownership.md)
- [アカウントのライフサイクル同期](auth-account-lifecycle-sync.md)
- [セッション Cookie](session-cookie-migration.md)
- [長岡高専企業ガイダンスのセットアップ](nnct-corp-guidance-development.md)（別の taskly-auth クライアント）
- [アカウントの閉鎖、データ消去、開示請求](operations/account-data.md)
- [ポリシーの公開と同意記録](legal/README.md)

## 開発・運用と検証

- [コントリビューションガイド](../CONTRIBUTING.md)と[ローカル開発ガイド](local-development.md)
- [CI/CD と本番デプロイ](initial-settings.md)、[コードのみのリリースの保護策](operations/no-ddl-code-release.md)、[SSH デプロイ中断時の復旧](operations/ssh-deploy-reconciliation.md)
- [リポジトリの公開条件と最終監査](operations/repository-publication.md)
- [GitHub Actions の必須チェック](operations/github-actions-required-checks.md)
- [PR コメントでの Atlas 操作](operations/atlas-pr-comments.md)と[PR のセルフマージ](operations/pr-self-merge.md)
- [準備状態の確認](operations/readiness.md)、[ネットワークポート](operations/network-ports.md)、[監視](operations/metrics-monitoring.md)
- [Redis ACL の運用](redis-acl-operations.md)と[ディスクの保守](operations/disk-maintenance.md)
- [暗号化された MySQL バックアップ](operations/mysql-backup.md)、[復旧とロールバック](operations/recovery-drill.md)、[訓練記録テンプレート](operations/recovery-drill-record-template.md)
- [Airflow メタデータ DB の分離](operations/airflow-db-separation.md)と [Airflow の Python テスト](operations/airflow-python-tests.md)
- [テストカバレッジ](testing/coverage.md)、[モックを使うブラウザテスト](../apps/web/tests/browser/README.md)、[実 API を使う E2E テスト](../apps/web/tests/e2e/README.md)
- [Discord への週次レポート](operations/weekly-discord.md)
- API の [OpenTelemetry](../apps/server/OPENTELEMETRY.md) と [Datadog](../apps/server/DATADOG.md)

説明と実行時の設定が異なる場合、アプリケーションのスキーマは `apps/server/schema.sql` と `atlas.hcl`、デプロイは `infra/docker-compose.yml` と `infra/docker-compose.prod.yml` を優先します。環境設定は `.env.example`、ワークフローは `.github/workflows/`、アプリの設定は各マニフェストを確認してください。`apps/server/migrations` は過去の変更記録で、現在のスキーマ適用は Atlas が行います。OpenAPI ファイルは REST の仕様を記載しています。GraphQL の実行用スキーマは `apps/server/internal/handler/graphql/` にあります。

## 構想と履歴資料

以下は当初の構想、設計案、作業メモの記録です。利用できる機能の確認には[現行仕様](Specification.md)を参照してください。

- [構想と過去の提案](roadmap.md)（ネイティブクライアントなど）
- [過去のバックログ](remain.md)と[セキュリティレビューの記録](improvements.md)
- [日直機能の再設計](new-day-shift.md)、[試験機能の提案](features/0001-exam.md)、[モデレーションの提案](batches/moderate/README.md)
- [メール生成サービスの要件](apps/email-compose/README.md)
- [過去の利用規約改定案](legal/terms-issue-404.md)
