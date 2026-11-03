# 本番ホストのディスク監視・清掃 Runbook

## 保持ポリシー

| 対象 | ポリシー | 保護事項 |
| --- | --- | --- |
| Docker container log | 1ファイル10MB、5世代 | Compose再作成時に適用 |
| 未使用Docker image | 最終参照から7日超で削除 | 稼働中containerのimageはDockerが削除しない |
| Docker build cache | 最終参照から7日超で削除 | volumeは対象外 |
| Airflow task log | 14日超で削除 | `airflow-logs` volume内の通常ファイルのみ |
| ローカルMySQL backup | 14日超で削除 | 古い候補も新しい順に最低3世代を保持 |
| FTPS上のMySQL backup | 既定14日、最低3世代を保持 | `BACKUP_RETENTION_DAYS` で期間を変更。リモートの最低3世代は固定。アプリDBとAirflowメタDBごとに適用 |

`db-data`、`redis-data`、`airflow-logs`、`airflow-backups` を含むDocker volumeに対して、
`docker volume prune` や `docker system prune --volumes` は実行しない。

## 自動運用

デプロイ時に、デプロイユーザーのcrontabへ次のジョブを冪等に設定する。

- 5分ごと: `/` の使用率を確認する
- 毎日03:25 JST: 安全な清掃を実行する

使用率の段階は warning 70%、critical 80%、emergency 90%。段階が変わった時と正常へ
復旧した時だけ、`.env` の `DISK_ALERT_WEBHOOK_URL`（未設定時は`DISCORD_WEBHOOK_URL`）へ通知する。
webhookがない場合はsyslogへ記録する。
Webhookまたはsyslogへの記録が成功した段階だけを保存する。配送に失敗するとコマンドは非0で終了し、
同じ段階が続く限り次の5分ごとの確認で再試行する。成功後は段階が変わるまで重複通知しない。
Webhook未設定時はsyslogへの記録成功を通知成功として扱い、syslog記録失敗時も再試行する。
cronは`pipefail`を有効にしているため、配送失敗時の終了状態をパイプラインでも保持する。
Webhook配送失敗は`journalctl -t taskly-disk`の`disk alert webhook delivery failed`で確認できる。
syslog記録失敗は標準エラーに`disk alert syslog delivery failed`を出す。

デプロイ開始時には安全な清掃を実行し、その後も使用率85%以上、または空き容量10GiB未満なら、
containerを停止する前にデプロイを中止する。閾値は `DISK_*` 環境変数で変更できる。

## 手動確認と復旧

```bash
cd /path/to/taskly/infra
./ops/disk-maintenance.sh status
docker system df
df -h /
```

容量が逼迫している場合は、まず清掃を実行する。

```bash
./ops/disk-maintenance.sh cleanup
docker compose ps
docker volume ls
df -h /
```

清掃後も空きが戻らない場合は `docker system df -v` と `du` で原因を特定する。volumeを削除する場合は、
対象のmount先、最新backup、復元手順を確認し、メンテナンス時間と承認を確保した別作業として行う。
緊急時もDB/Airflow volumeを推測で削除しない。
