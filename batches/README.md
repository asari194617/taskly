# バッチ処理

各種バッチ処理を実行するためのディレクトリです。

## ディレクトリ構成

- `batches/notifications/que-manager`: 課題の Web Push キューを管理するバッチ
  - `batches/notifications/delivery-channels/email`: メールで通知を送信するバッチ
  - `batches/notifications/delivery-channels/web-push`: Web Pushに通知を送信するバッチ
  - `batches/notifications/delivery-channels/discord`: Discordに通知を送信するバッチ
- `batches/day-shift-rotations`: 日直のローテーションを生成するバッチ
- `batches/backup-mysql`: MySQLのバックアップを行うバッチ
- `batches/moderation`: モデレーション通知を処理するバッチ

通知バッチの現行動作は [`../docs/batches/notifications/README.md`](../docs/batches/notifications/README.md) を参照してください。

## バッチの実行管理

- 各処理はApache Airflowで管理します。
- Airflowの設定は`infra/airflow`にあります。
- AirflowのWeb UIは`http://localhost:3065`でアクセスできます。
- 処理の結果はDiscordのWebhookに通知されます。
- WebhookのURLは`.env`の`DISCORD_WEBHOOK_URL`で設定されています。
- 全てのバッチ処理はGoで実装します。`infra/docker-compose.yml`の`docker compose`でAirflowイメージをビルドし、バッチはイメージ内の事前ビルド済みバイナリとして実行します。

## 各バッチの説明

### `batches/notifications/que-manager`

課題の Web Push 通知キューを管理するバッチです。

- 期限が近い課題を抽出し、対象ユーザーの Web Push キューへ投入します。
- 期限が近い課題の通知は、毎日7:00, 17:00に実行されます。
- 現行の抽出範囲は、JST の当日 00:00 以上、3日後 00:00 未満（2日後の終日まで）です。
- 表記は
  - 今日
  - あす
  - あさって
  - XX日 (月を跨がない場合)
  - XX月YY日 (月を跨ぐ場合)
  - YYYY年XX月YY日 (年を跨ぐ場合)
    のいずれかになります。
- Queue Manager の日直候補は宛先未確定のため、この DAG からは配信されません。日直通知は別の `day_shift_notify_0830` DAG で処理します。
- ブロック対象のユーザーには通知しません。
- 通知の内容は、課題のタイトルと期限です。

### `batches/notifications/delivery-channels/email`

メールで通知を送信するバッチです。

- 実行時に対象課題を取得し、ユーザーごとのダイジェストを生成してメールで送信します。

### `batches/notifications/delivery-channels/web-push`

Web Pushに通知を送信するバッチです。

- `que-manager`からキューイングされた通知を取得し、Web Pushで送信します。

### `batches/day-shift-rotations`

(使わない)

### `batches/backup-mysql`

MySQLのバックアップを圧縮して age で暗号化し、明示的 FTPS でアップロードします。設定と復元手順は [バックアップ運用手順](../docs/operations/mysql-backup.md) を参照してください。

#### 処理

1. `mysqldump` の出力をストリーミングで gzip 圧縮、age 暗号化
2. XServer Static に FTPS で一時名にアップロードし、再取得して SHA-256 を照合
3. 照合成功後に正式名へ変更し、14日を超える古いファイルを削除（最低3世代は保持）
4. Airflow の結果を Discord に通知

接続情報は `.env` の `BACKUP_AGE_RECIPIENT` と以下の変数で設定します。

- `FTP_HOST`
- `FTP_PORT`
- `FTP_USER`
- `FTP_PASSWORD`
- `FTP_DIR` (オプション、デフォルトは`/backups`)

- 毎日1回、深夜3時に取得します。失敗時は Airflow が10分間隔で2回再試行します。
- ローカルファイルの権限は600です。
- ファイル名は UTC の `backup-YYYYMMDD-HHMMSS.sql.gz.age` です。
