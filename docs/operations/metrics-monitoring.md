# メトリクスと監視

本番環境では基本の Compose ファイルに `infra/docker-compose.prod.yml` を重ね、Prometheus、Alertmanager、Blackbox Exporter、Grafana を起動します。メトリクス定義、スクレイプ対象、アラートのしきい値、ダッシュボードは `infra/monitoring/` にあります。基本の開発構成ではこれらのサービスを起動しません。

## 収集とアクセス

Prometheus は 30 秒ごとにメトリクスを取得します。

| ジョブ | 対象 | 収集内容 |
| --- | --- | --- |
| `taskly-api` | `server:9464/metrics` | HTTP トラフィック、連携ジョブ、アプリケーションの状態 |
| `taskly-airflow` | `airflow-scheduler:9470/metrics` | DAG の実行、各コンポーネントの稼働状態、バッチ配信 |
| `taskly-services` | Blackbox の HTTP プローブ | API の `/healthz`、MCP の `/healthz`、Airflow の `/api/v2/monitor/health` |
| `prometheus`、`alertmanager` | 各サービス内のメトリクスエンドポイント | 監視サービスの可用性 |

本番スケジューラーのコマンドは `airflow db migrate` を実行し、バックグラウンドで `airflow_exporter.py` を起動してからスケジューラーを起動します。エクスポーターは Airflow のメタデータとアプリケーションのメトリクスを取得し、収集に失敗すると HTTP 503 を返します。スケジューラーが正常でもメトリクスが取得できない場合は、エクスポーターを調査してください。

API・MCP の Blackbox プローブは **生存状態** を確認します。Compose とデプロイは依存サービスを含む **準備状態** を確認します。障害時は[準備状態の確認手順](readiness.md)に従い、両方を調べてください。生存確認に成功しても、MySQL や Redis が動作しているとは限りません。

ダッシュボードとクエリのインターフェースはホストのループバックにバインドします。本番ホストに SSH でアクセスできる運用担当者は以下のように接続します。

```bash
ssh -N -L 3067:127.0.0.1:3067 -L 3068:127.0.0.1:3068 \
  -L 3069:127.0.0.1:3069 <production-host>
```

Grafana は `http://localhost:3067`、Prometheus は `http://localhost:3068`、Alertmanager は `http://localhost:3069` で開きます。Grafana には Prometheus データソースと Taskly 運用ダッシュボードを設定済みです。匿名の Viewer アクセスを有効にし、ログインフォームを無効にしているため、ループバックへのバインドを保ち、SSH でアクセスを制御します。Prometheus のデータ保持期間は 15 日です。3 サービスとも名前付きボリュームを使います。[ポート一覧](network-ports.md)を参照してください。

## アラートと対応

現在の式と継続時間の定義は `alerts.yml` にあります。スクレイプ・プローブの失敗、API の 5xx 比率とレイテンシー、連携ジョブの失敗・滞留、通知の失敗、DAG の失敗・最終成功の古さ・未稼働を検出します。直近の実行結果、滞留、コレクター停止のいずれを示すかはアラート式で確認してください。バックアップの復元と鮮度確認には[バックアップ手順](mysql-backup.md)も参照します。

Alertmanager はアラート名、インスタンス、DAG でグループ化し、最初の通知まで 30 秒待機します。未解決のアラートは 4 時間ごとに再通知し、解決時も通知します。起動ラッパーには `DISCORD_WEBHOOK_URL` が必要で、Discord の受信設定用に URL を非公開ファイルへ書き込みます。Webhook の変更時は新しい環境変数でコンテナを再作成してください。

アラートへの対応手順は以下のとおりです。

1. 本番で設定した `infra/` で両方の Compose ファイルを指定し、`docker compose ps` を確認します。影響を受けたサービスのログを確認します。
2. Prometheus の `/targets`、`/alerts`、アラート式を確認します。API・MCP の `/readyz` と Airflow のヘルスエンドポイントも確認します。
3. 連携処理が失敗した場合は、再試行前に管理画面のジョブ状態と下流サービスを確認します。[配信は少なくとも 1 回](../integration-jobs.md)です。
4. 通知や Airflow のジョブが失敗した場合は、タスクログと最後に成功した実行を確認します。介入時は[復旧手順](recovery-drill.md)を参照します。
5. 対象サービスとメトリクス収集の両方が復旧したことを確認します。

`infra/ops/install-user-cron.sh` が設定するホストの crontab は、Prometheus とは独立して毎分 `monitoring-watchdog.sh` を実行します。このスクリプトは Prometheus と Alertmanager の準備状態およびスクレイプのサンプル数を確認します。保存した状態が変化したときだけ、`MONITORING_ALERT_WEBHOOK_URL` またはルートの `.env` の `DISCORD_WEBHOOK_URL` を使って Discord に通知します。個々のアラートのルーティングや実配信は確認しないため、監視サービスがすべて停止した場合はホストの cron と syslog を確認してください。

## 設定の検証

`.env` を設定したうえで、リポジトリのルートで実行します。

```bash
docker compose -f infra/docker-compose.yml -f infra/docker-compose.prod.yml config --quiet
infra/ops/test-port-exposure.sh
docker run --rm --entrypoint promtool -w /etc/prometheus \
  -v "$PWD/infra/monitoring:/etc/prometheus:ro" prom/prometheus:v3.15.1 \
  check rules /etc/prometheus/alerts.yml
docker run --rm --entrypoint promtool -w /etc/prometheus \
  -v "$PWD/infra/monitoring:/etc/prometheus:ro" prom/prometheus:v3.15.1 \
  test rules /etc/prometheus/alerts.test.yml
```

バージョンが変わった場合は、本番 Compose ファイルで固定している Prometheus イメージを使います。本番ホストでは `http://127.0.0.1:3068/-/ready` と `http://127.0.0.1:3069/-/ready` を確認し、その後スクレイプ対象を調べます。テストアラートは実際の Discord 通知を発生させるため、運用チームと調整した検証で実施してください。
