# メトリクス監視（Issue #338）

## 構成とアクセス

本番の `docker-compose.prod.yml` が Prometheus、Alertmanager、Blackbox Exporter、Grafana を起動する。Prometheus は API と Airflow scheduler 内の exporter を **30 秒間隔**でスクレイプし、ルールも 30 秒間隔で評価する。Airflow exporter は分離された Airflow メタデータ DB を scheduler の既存権限で読み、API にその DB の認証情報を渡さない。保存先は Docker volume `prometheus-data`、**保持期間 15 日**。Alertmanager の状態は `alertmanager-data`、Grafana の設定と状態は `grafana-data`。設定本体は `infra/monitoring/` にあり、デプロイ時に GitHub Actions から同期される。

Grafana はホストの `127.0.0.1:3067`、Prometheus は `127.0.0.1:3068`、Alertmanager は `127.0.0.1:3069` のみで待ち受ける。外部からは SSH トンネル経由で Grafana の **Operations / Taskly Operations** を開く。Grafana はローカルポートに限って匿名 Viewer を許可する。

```sh
ssh -L 3067:127.0.0.1:3067 <user>@<production-host> -i <path-to-private-key>
# 別の端末またはブラウザーで http://127.0.0.1:3067/ を開く
```

通知先は本番 `.env` の `DISCORD_WEBHOOK_URL`。Alertmanager は起動時にその URL を権限 0600 の一時ファイルへ書き、設定ファイルや Git に秘密を残さない。未設定時は起動を失敗させる。解消通知も送る。ホストの cron は `monitoring-watchdog.sh` を毎分実行し、Prometheus・Alertmanager と必須 scrape target の停止を同じ Discord に独立して通知する。ホスト全体の停止は同じホスト上の監視では検知できないため、別経路の外形監視を維持する。

## 指標とアラート

| アラート | 条件 | 一次対応 |
| --- | --- | --- |
| MetricsScrapeDown | API または Airflow exporter のスクレイプ失敗が 2 分 | API は `curl http://127.0.0.1:3064/metrics`、Airflow は scheduler と exporter のログを確認 |
| ServiceDown | API、MCP、Airflow API の HTTP ヘルス失敗が 2 分 | `docker compose ps` と対象ログを確認。Airflow は `/api/v2/monitor/health` の各コンポーネントも確認 |
| APIErrorRate | 5 分間に 20 件超のリクエストがあり、5xx が 5% 超を 5 分継続 | Grafana の 5xx とデプロイ時刻を照合し、API ログと Sentry の直近エラーを確認 |
| APILatency | p95 が 1 秒超を 5 分継続 | 遅いルート、DB/Redis、直近の変更を確認 |
| IntegrationJobsFailed | `failed` が 1 件以上を 2 分 | 管理画面の連携ジョブで失敗理由と試行回数を確認し、原因解消後に手動再試行 |
| IntegrationJobsStalled | `pending`/`processing` の最古が 10 分超を 5 分 | ワーカーのログ、外部連携先、期限切れ lease を確認 |
| NotificationDeliveryFailed | 日直通知の `error` が 1 件以上を 2 分 | Airflow の日直 DAG と配送先、`notification_deliveries` の状態を確認 |
| NotificationBatchFailed | 通知 batch の email・webpush・Discord の最終失敗が最終成功より新しい状態を 2 分 | `notifications` DAG の対象 task log と配送先を確認 |
| AirflowDAGFailed | 対象 DAG の最終失敗が最終成功より新しい状態が 2 分 | Airflow の DAG run と task log を確認し、原因解消後に再実行 |
| AirflowComponentDown | metadatabase・scheduler・dag-processor の health が 2 分間 unhealthy | Airflow の monitor endpoint と対象コンテナログを確認 |
| AirflowDAGStale | 有効かつ一度以上成功した対象 DAG の最終成功から 36 時間を 10 分継続 | scheduler と dag-processor、次回実行時刻を確認 |
| AirflowDAGInactive | 対象 DAG が pause または未登録の状態を 10 分継続 | DAG の import error と pause 状態を確認。意図しない停止なら有効化する |

Airflow の対象は `notifications`、`moderation_daily`、`day_shift_index_increment`、`day_shift_notify_0830`、`account_erasure`。通知 batch の 3 チャネルは Airflow task 単位の成功・失敗時刻を計測する。最終実行時刻が `0` の初回実行前 DAG は stale とみなさず、`taskly_airflow_dag_active` で pause・未登録を別に検知する。`account_erasure` が失敗・停止した場合は[退会データ運用](account-data.md)の期限超過件数を確認する。`backup_mysql` の成功時刻もダッシュボードに表示するが、RPO アラートは [DB バックアップ運用](mysql-backup.md) と #293 で扱う。

`taskly_http_requests_total` と `taskly_http_request_duration_seconds` のラベルは method、Fiber のルートパターン、HTTP status だけ。マッチしない URL は `unmatched` にまとめる。エラー率と遅延の判定では `/healthz` を除外する。連携ジョブの kind はコード中の既知の 4 種と `other` に限定する。ユーザー ID、メールアドレス、トークン、実 URL はラベルに入れない。ジョブ数・最古滞留時間は scrape 時に DB から集計する。DB クエリ失敗時は `/metrics` 自体を失敗させ、収集不能アラートに繋ぐ。

## デプロイ後の確認

`infra/ops/verify-deploy-health.sh` は API と Airflow の `/metrics` の取得と基本指標、Prometheus・Alertmanager・Grafana の readiness を確認する。失敗時はデプロイを失敗にする。次も確認する。

```sh
curl -fsS http://127.0.0.1:3068/api/v1/targets  # taskly-api と taskly-services が up
curl -fsS 'http://127.0.0.1:3068/api/v1/query?query=taskly_integration_jobs'
curl -fsS 'http://127.0.0.1:3068/api/v1/query?query=taskly_airflow_dag_last_run_timestamp_seconds'
```

Grafana の API、ジョブ、Airflow の時系列を確認する。Alertmanager の Discord 通知を確認する際は、`/api/v2/alerts` に `alertname=TasklyMonitoringTest`、`severity=warning`、短い `endsAt` を持つテストアラートを送信し、指定先 Discord の着信と解消通知を確認する。収集停止・API エラー・滞留のルール式は `promtool test rules` のシナリオで再現する。本番の API 停止や偽のジョブ作成は行わない。

## 注意

Grafana と Prometheus はホスト内限定だが、Docker volume には時系列とダッシュボード状態が残る。ホスト容量を監視し、必要なら保持期間を調整する。Alertmanager 自体、Prometheus 自体、ホスト全体の停止は外部監視で補完する。
