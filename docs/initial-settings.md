# CI・配信・本番運用

この文書は `.github/workflows/` と `infra/ops/` の現行実装を説明します。

## MCP クライアントの配信

ワークフロー: `.github/workflows/release-mcp-client.yml`

必要な GitHub Secrets:

- `R2_BUCKET`: R2 バケット名
- `R2_ACCOUNT_ID`: Cloudflare アカウント ID
- `R2_ACCESS_KEY_ID`: R2 Access Key ID
- `R2_SECRET_ACCESS_KEY`: R2 Secret Access Key

`v*` タグの push または手動実行で macOS、Linux、Windows 用バイナリを
生成し、次のパスへアップロードします。

- ベース: `https://cdn.taskly.iy0306.com/docs/mcp/clients`
- 最新タグ: `latest.json`
- インストーラー: `install.sh`, `install.ps1`
- バージョン別: `<tag>/taskly-mcp-<os>-<arch>`, `<tag>/checksums.txt`

利用方法は `apps/mcp-client-files/README.md` と `docs/mcp-docs.md` を参照してください。

## 本番デプロイ

ワークフロー: `.github/workflows/deploy-prod.yml`

必要な GitHub Secrets:

- `PROD_HOST`: 本番ホスト名または IP
- `PROD_USER`: Docker とユーザー crontab を操作できる SSH ユーザー
- `PROD_SSH_KEY`: OpenSSH 形式の秘密鍵
- `PROD_SSH_PASSPHRASE`: 鍵にパスフレーズがある場合のみ
- `PROD_SSH_PORT`: 省略時は `22`
- `PROD_WORKDIR`: サーバー上の絶対パス
- `PROD_ENV`: 本番 `.env` の内容。空値では本番ホストへの接続前にデプロイを中止する
- `PROD_COMPOSE_PROFILES`: 将来 Compose profile を追加する場合の予約値（現行構成には profile なし）
- `PROD_PUBLIC_API_HEALTH_URL`, `PROD_PUBLIC_MCP_HEALTH_URL`, `PROD_PUBLIC_AIRFLOW_HEALTH_URL`: 公開 Nginx 経路も検証する場合の完全な health URL。設定された経路だけ追加検証します。

`main` への push では次のパスが変更された場合に起動します。

- `apps/server/**`
- `apps/mcp/**`
- `apps/email-compose/**`
- `batches/**`
- `infra/**`
- `.dockerignore`
- `.github/scripts/image-matrix.sh`
- `.github/scripts/require-prod-env.sh`
- `.github/workflows/deploy-prod.yml`

手動実行も可能で、`full_build=true` が既定です。

### デプロイの流れ

1. 変更パスから `api`, `email-compose`, `mcp`, `airflow` のビルド対象を選択する。
2. 対象イメージを `linux/amd64` でビルドし、commit SHA タグで GHCR に push する。
3. 変更のないサービスにも同じリリース SHA タグを付け、4サービスすべてを同一 SHA で固定する。
4. `PROD_ENV` を確認し、`.env` と `infra/.env` を保護してリポジトリを rsync する。
5. 設定を一時ファイルに転送し、Compose の設定検証後に権限 `600` で両方の `.env` を置き換える。検証に失敗した場合は元の設定に戻す。
6. 本番ホストで不要なイメージ/ビルドキャッシュを清掃し、空き容量の基準を確認する。
7. `TASKLY_IMAGE_TAG=<commit SHA>` を指定してイメージを pull する。Redis ACL の entrypoint が変更された場合は Redis を再作成し、DB と Redis の起動を確認する。
8. 新しい API イメージの `/app/migrate` を一度だけ実行し、成功後にアプリコンテナを更新する。
9. アプリコンテナの再作成後、最大10分間 healthcheck を待つ。
10. API・MCP・Airflow のホスト側 HTTP 応答、Airflow のメタDB・scheduler・DAG processor、全コンテナの20秒間の安定稼働を検証する。
11. Datadog APM socket への接続と Airflow 内のバッチ実行ファイルを検証し、ディスク監視・清掃用のユーザー crontab を更新する。

デプロイ先ではホストの Datadog Agent が `/var/run/datadog/apm.socket` と
`/var/run/datadog/dsd.socket` を提供している必要があります。デプロイは両socketの存在を
確認してから本番専用の `infra/docker-compose.prod.yml` を重ね、各アプリへread-onlyで
mountします。通常のローカルCompose起動ではこのoverrideを使いません。APM用TCPポートを
ホスト外部へ公開する必要はありません。

本番ホストではイメージをビルドしません。`docker compose up --build` や
`latest` の直接指定ではなく、ワークフローが配布する commit SHA タグを使用します。

### DB マイグレーション

本番の既存スキーマと `migrations` 表は `0025_integration_jobs.sql` まで照合済みです。
`0023`〜`0025` はスキーマ変更が先に存在したため、2026-09-21 に適用履歴を補正しました。
自動実行の対象は `apps/server/migrations/0026_*.sql` 以降です。古い SQL は
重複した変更を含むため、新規DBをこの仕組みだけで作成することはできません。

新しいマイグレーションは連番の `NNNN_name.sql` を追加し、一度適用したファイルは
変更しないでください。デプロイ中に MySQL の名前付きロックを取得し、未適用の
ファイルを名前順に実行して `migrations` 表へ記録します。失敗した場合は
アプリの更新を止めます。MySQL の DDL はファイル全体をロールバックできないため、
失敗後は本番の変更状態を調べてから再実行してください。破壊的な変更はアプリとの
互換期間を設け、別リリースで適用してください。

Teams連携の `status_callback` を使用する場合は、デプロイ前に `PROD_ENV` の
`TEAMS_CALLBACK_ALLOWED_ORIGINS` に送信先の正規HTTPSオリジンをカンマ区切りで設定します
（例: `https://flow.example.com`）。未設定の場合、コールバック付き受信リクエストは
400で拒否されます。許可されたオリジンでも、内部IPへの名前解決とリダイレクトは拒否します。
設定後は実際のPower Automateフローでコールバック到達を確認してください。

## ディスク監視と清掃

`infra/ops/disk-maintenance.sh` は Docker volume を削除しません。既定では、
古い未使用イメージ/ビルドキャッシュ、Airflow ログ、バックアップを保持期間に
従って清掃します。デプロイ前は使用率と空き容量を検査し、基準を満たさない場合は
デプロイを中止します。

`infra/ops/install-user-cron.sh` が次をユーザー crontab に登録します。

- 5分ごとのディスク状態確認
- 毎日 03:25 の清掃

通知先は `DISK_ALERT_WEBHOOK_URL`、未設定時は `DISCORD_WEBHOOK_URL` です。
しきい値と保持期間は `.env.example` に記載された `DISK_*`,
`DOCKER_RETENTION_HOURS`, `AIRFLOW_LOG_RETENTION_DAYS`,
`BACKUP_RETENTION_DAYS`, `BACKUP_MIN_COPIES` で上書きできます。

## 運用上の注意

- 本番 `.env` の秘密値をリポジトリへ commit しないでください。
- Compose は API、metrics、Airflow、MCP を `127.0.0.1` のみに公開します。
  MySQL、Redis、email-compose は Docker 内部ネットワークだけで通信します。
  公開ポート一覧と確認方法は `operations/network-ports.md` を参照してください。
- `infra/docker-compose.yml` 内に Traefik や TLS 終端サービスはありません。
- healthcheck または HTTP 応答が失敗するとデプロイジョブは失敗します。ログにはサービスの状態、healthcheck の直近の出力、コンテナログが出ます。

### デプロイ失敗時の切り戻し

1. Actions の失敗ログで対象サービスと healthcheck の理由を確認し、`docker compose ps` と `docker compose logs --tail=100 <service>` で再確認します。
2. アプリの新イメージが原因なら、直前に成功したデプロイの commit SHA を `TASKLY_IMAGE_TAG` に設定し、`cd "$PROD_WORKDIR/infra"` で `COMPOSE_FILE=docker-compose.yml:docker-compose.prod.yml docker compose up -d --wait --wait-timeout 600 --no-build server email-compose mcp airflow-webserver airflow-scheduler airflow-dag-processor` を実行します。MySQL・Redis の volume は削除しません。
3. `./ops/verify-deploy-health.sh` を実行します。DB スキーマ変更が含まれる場合は、イメージだけ戻す前にマイグレーションの互換性を確認します。
