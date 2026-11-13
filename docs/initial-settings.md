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
- `PROD_ENV` 内の `AIRFLOW_META_DATABASE`, `AIRFLOW_META_USER`, `AIRFLOW_META_PASSWORD`: Airflow メタDB専用の接続設定。既存DBからの移行手順は [`operations/airflow-db-separation.md`](operations/airflow-db-separation.md) を参照
- `TASK_IMAGES_R2_ENDPOINT`, `TASK_IMAGES_R2_BUCKET`, `TASK_IMAGES_R2_ACCESS_KEY_ID`, `TASK_IMAGES_R2_SECRET_ACCESS_KEY`: 課題画像用の非公開R2バケット設定。`production` environment に4項目をまとめて保存し、デプロイ時に `PROD_ENV` に組み込む。直接 `PROD_ENV` に `R2_*` を含める方法も利用できる。未設定・空値ではデプロイを中止する。公開MCPクライアント配信用のリポジトリ Secrets とは独立して設定する。詳細は [`features/task-images.md`](features/task-images.md) を参照
- `NNCT_GUIDANCE_EVENT_ID`, `NNCT_GUIDANCE_TASK_BINDINGS`: 企業ガイダンス連携を有効にする場合、`production` environment に2項目をまとめて保存する。後者は論理キーと登録済みTaskly課題IDのJSONオブジェクト。デプロイ時に既存の完全な `PROD_ENV` を保持して同名の2設定だけを置き換えるため、`PROD_ENV` 自体の再作成は不要。両方未設定なら既存設定を保持し、片方だけの設定や不正なキー・IDではデプロイを停止する。実際の課題IDや配布資料の内容はGitに含めない。
- `PROD_COMPOSE_PROFILES`: 追加で有効にする Compose profile。通常は空。スキーマ適用スクリプトが Atlas 用の `schema` profile を一時的に有効化するため、通常デプロイでの指定は不要
- `PROD_PUBLIC_API_HEALTH_URL`, `PROD_PUBLIC_MCP_HEALTH_URL`, `PROD_PUBLIC_AIRFLOW_HEALTH_URL`: 公開 Nginx 経路も検証する場合の完全な health URL。API と MCP は `/readyz` を指定します。既存の `/healthz` 設定は検証スクリプトが `/readyz` に置き換えます。設定された経路だけ追加検証します。

`main` への push では次のパスが変更された場合に起動します。

- `apps/server/**`
- `atlas.hcl`
- `apps/mcp/**`
- `apps/email-compose/**`
- `batches/**`
- `infra/**`
- `.dockerignore`
- `.github/scripts/image-matrix.sh`
- `.github/scripts/require-prod-env.sh`
- `.github/scripts/prepare-prod-env.py`
- `.github/scripts/check-prod-airflow.sh`
- `.github/workflows/deploy-prod.yml`

手動実行も可能で、`full_build=true` が既定です。イメージの build/release は
共通の `select-runner` が選ぶ runner を使います。選択ジョブ自体と `plan` は常に GitHub-hosted で実行し、
15分以内に成功した heartbeat がある primary、secondary、`ubuntu-latest` の順に選びます。
手動の `runner_override` またはリポジトリ変数 `CI_RUNNER` で明示できます。
`CI_PRIMARY_ONLY=true` はこれらに優先し、選択対象の重いジョブを primary に固定します。
Lint、MCP・email-compose のチェック、カバレッジ、バッチ単体テスト、実行計画・結果集約、
通知などの軽いジョブは両変数によらず `ubuntu-latest` を使います。
2026-11-13 の確認では `CI_PRIMARY_ONLY=true`、`CI_RUNNER` は未設定です。構成・障害時の切り替えは
[runner pool 手順](../infra/actions-runner/README.md)を参照してください。
本番反映は `taskly-prod` 専用runnerで実行します。

パス条件には拡張子の除外がないため、上記ディレクトリ内の README 変更も本番デプロイを起動します。
Web は Cloudflare Pages で別途配信し、この Compose ワークフローではビルドしません。

### デプロイの流れ

1. 変更パスから `api`, `email-compose`, `mcp`, `airflow` のビルド対象を選択する。
2. 対象イメージを `linux/amd64` でビルドし、commit SHA タグで GHCR に push する。
3. 変更のないサービスの既存 `latest` イメージにも同じリリース SHA タグを付け、4サービスのリリースタグを揃える。Airflow の稼働コンテナは更新条件に該当しない限り旧 SHA のまま維持する。
4. `PROD_ENV` を確認し、`.env` と `infra/.env` を保護してリポジトリを rsync する。
5. 設定を一時ファイルに転送し、Compose の設定検証後に権限 `600` で両方の `.env` を置き換える。検証に失敗した場合は元の設定に戻す。
6. 本番ホストで不要なイメージ/ビルドキャッシュを清掃し、空き容量の基準を確認する。
7. `TASKLY_IMAGE_TAG=<commit SHA>` を指定してイメージを pull する。Redis ACL の entrypoint が変更された場合は Redis を再作成し、DB と Redis の起動を確認する。
8. Atlasで `apps/server/schema.sql` と実DBの差分を表示し、破壊的変更がないことを確認して適用する。成功後にアプリコンテナを更新する。Airflow のイメージまたは Compose 設定が変わった場合だけ Airflow 3サービスを更新する。独立したメタDB分離スクリプトは通常デプロイでは実行しない。
9. 更新したコンテナの再作成後、最大10分間 healthcheck を待つ。
10. API・MCP・Airflow のホスト側 HTTP 応答、Airflow のメタDB・scheduler・DAG processor、全コンテナの20秒間の安定稼働を検証する。
11. Datadog APM socket への接続と Airflow 内のバッチ実行ファイルを検証し、ディスク監視・清掃用のユーザー crontab を更新する。

同時に複数の push があっても実行中のデプロイはキャンセルせず、後続を待機させます。デプロイ失敗時は別の SSH セッションで Airflow 3サービスの状態を確認し、停止・不健康・確認不能なら Actions にエラーを表示します。

Airflow メタDBの初期化・スキーマ更新・旧アプリDBからの分離が必要な場合は、バックアップと DAG の実行状況を確認してから [`operations/airflow-db-separation.md`](operations/airflow-db-separation.md) の明示的な移行手順を実行してください。ベースの Compose は起動時に `db migrate` を実行しません。ただし本番 override の
`airflow-scheduler.command` は exporter の起動前に `airflow db migrate` を実行します。
これは既存メタDBのスキーマ更新であり、新DB・ユーザーの作成や旧アプリDBからのテーブル移動は行いません。
初期化・分離には明示的な移行手順が必要です。

デプロイ先ではホストの Datadog Agent が `/var/run/datadog/apm.socket` と
`/var/run/datadog/dsd.socket` を提供している必要があります。デプロイは両socketの存在を
確認してから本番専用の `infra/docker-compose.prod.yml` を重ね、各アプリへread-onlyで
mountします。通常のローカルCompose起動ではこのoverrideを使いません。APM用TCPポートを
ホスト外部へ公開する必要はありません。

本番ホストではイメージをビルドしません。`docker compose up --build` や
`latest` の直接指定ではなく、ワークフローが配布する commit SHA タグを使用します。

### 小容量ホストのメモリ管理

Airflow 3 の実行方式は `LocalExecutor`、既定の実行枠は1です。
`SequentialExecutor` は Airflow 3.3 では `LocalExecutor` に読み替えられるため、
実行枠を8にすると待機中にも8個の Python ワーカーが常駐します。
全 DAG・通知チャネル・コールバック・UI は維持し、重なったタスクは順番に実行します。
ホスト増強後に並列数を上げる場合は `PROD_ENV` に `TASKLY_AIRFLOW_PARALLELISM` を設定します。
メタDB接続プールは各プロセス2接続、追加接続なしです。

scheduler / DAG processor のコンテナ healthcheck は `infra/airflow/healthcheck.py` で
同一ホスト名の最新ジョブが `running` かつハートビートが閾値内かをメタDBで確認します。
Airflow CLI や provider の読み込みを30秒ごとに繰り返さず、既定30秒の閾値と
`AIRFLOW__SCHEDULER__SCHEDULER_HEALTH_CHECK_THRESHOLD` /
`AIRFLOW__DAG_PROCESSOR__HEALTH_CHECK_THRESHOLD` に対応します。
DBセッションを UTC に固定し、MySQL の `TIMESTAMP` の時差変換で判定を誤らないようにします。

本番 MySQL はテーブルキャッシュ512、定義キャッシュ400、キャッシュ分割数2、
SQL digest 集計1000件、ログバッファ16MiBに設定します。Performance Schema、
InnoDBデータキャッシュ、接続上限、SQL mode、トランザクションの永続化設定は維持します。
補助キャッシュを小さくするため、テーブルオープン回数・ログ待ち・SQL遅延を反映後に確認します。
Prometheus / Grafana / Alertmanager / Blackbox は既存のコンテナ上限より小さい
`GOMEMLIMIT` を設定し、上限到達前の GC を促します。これは Go ランタイムの
ソフト目標であり、プロセス全体の使用量を保証する上限ではありません。

本番での比較は `docker stats` だけでなく cgroup v2 の `memory.current` と
`memory.swap.current` の合計、`vmstat 1 5` の swap in/out、HTTP応答時間、
全 DAG の直近成功・実行時間で行います。スワップに退避したメモリは `docker stats` の
表示だけでは把握できません。ホストには Taskly 外のアプリ・監視Agentも常駐するため、
それらを含む `free -m` / プロセス別 `VmSwap` も確認します。

反映は通常のデプロイで行います。MySQL の起動引数を変更したリリースでは DB が再作成され、
接続が一時的に切断されます。切り戻しはこの設定変更を revert したリリースをデプロイします。
旧 Airflow イメージに戻す場合は healthcheck の Compose 設定も同時に戻してください。
この変更ではアプリDB・AirflowメタDBのスキーマ移行は不要です。

参考: [Airflow LocalExecutor](https://airflow.apache.org/docs/apache-airflow/3.3.0/core-concepts/executor/local.html)、
[MySQL Performance Schemaの自動設定](https://dev.mysql.com/doc/refman/8.4/en/performance-schema-startup-configuration.html)、
[Goのメモリ目標](https://go.dev/doc/gc-guide#Memory_limit)。

### DBスキーマ管理

MySQL と Atlas 用の一時 MySQL の healthcheck は `--protocol=TCP -h 127.0.0.1` で
最終サーバーの待受開始を確認します。公式イメージは初期化中に `--skip-networking` の
一時サーバーを起動するため、`localhost` の socket 接続では初期化完了前に healthy になり、
後続の Atlas が TCP 接続拒否で失敗する場合があります。

`apps/server/schema.sql` がTasklyアプリDBの宣言的な正本です。`atlas.hcl` は接続情報を
環境変数から組み立て、SQL定義の解析には使い捨てのMySQL 8.4を使います。過去の
`apps/server/migrations` は監査資料として残しますが、新しい変更は追加しません。
旧GORM名の `web_push_v_api_ds` は既存Web Push鍵の互換読取に必要なため、Atlasの
管理対象から除外して保持します。正本テーブルは `web_push_vapid` です。

ローカルでは `.env` を設定して `infra/ops/apply-schema.sh` を実行します。スクリプトは
Atlasのdry-runを先に表示し、`DROP TABLE`、`DROP COLUMN`、`DROP INDEX`、`MODIFY COLUMN`、
`CHANGE COLUMN` を含む計画を既定で拒否します。互換期間、検証済みbackup、切り戻し手順を
レビューした場合だけ `ATLAS_ALLOW_DESTRUCTIVE=true` を明示します。本番では通常のpushを
破壊的変更に使用せず、手動デプロイの `allow_destructive_schema` を有効にします。本番
デプロイも同じスクリプトを使い、適用後にアプリを更新します。

新規環境と既存環境は同じ手順です。Atlasは実DBをinspectし、`schema.sql`との差分だけを
適用します。CIは空DBへの適用、主要テーブルの読み書き、2回目のdry-runが空になることを
確認します。MySQL DDLは全体をロールバックできないため、途中失敗後は実スキーマを
Atlasの計画と照合してから修正リリースまたはbackup復元を選びます。

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
- 毎分の Prometheus・Alertmanager・scrape 対象確認（[監視手順](operations/metrics-monitoring.md)）
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

DB 復元、migration の互換性判定、通知バッチ停止、定期訓練は [`operations/recovery-drill.md`](operations/recovery-drill.md) を参照してください。スキーマ変更を含む PR には旧イメージへ戻せる条件を記録します。

1. Actions の失敗ログで対象サービスと healthcheck の理由を確認し、`docker compose ps` と `docker compose logs --tail=100 <service>` で再確認します。
2. アプリの新イメージが原因なら、直前に成功したデプロイの commit SHA を `TASKLY_IMAGE_TAG` に設定し、`cd "$PROD_WORKDIR/infra"` で `COMPOSE_FILE=docker-compose.yml:docker-compose.prod.yml docker compose up -d --wait --wait-timeout 600 --no-build server email-compose mcp airflow-webserver airflow-scheduler airflow-dag-processor` を実行します。MySQL・Redis の volume は削除しません。
3. `./ops/verify-deploy-health.sh` を実行します。DB スキーマ変更が含まれる場合は、イメージだけ戻す前にマイグレーションの互換性を確認します。
