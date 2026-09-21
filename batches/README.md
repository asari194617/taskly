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
- 現行の抽出範囲は、JST の当日 00:00 から翌々日 00:00 まで（両端を含む）です。
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

MySQLのバックアップを取得し、FTPサーバーにアップロードします。

#### 処理

1. `dump`を実行
2. FTP経由でXServer Staticに接続
3. XServer Staticに `dump`したデータをアップロード
4. XServer Staticのデータのうち7日以上経過したデータを削除
5. 結果をDiscordに通知

FTPサーバーの接続情報は、`.env`の以下の変数で設定します。

- `FTP_HOST`
- `FTP_PORT`
- `FTP_USER`
- `FTP_PASSWORD`
- `FTP_DIR` (オプション、デフォルトは`/backups`)

- 毎日1回、深夜3時に取得します。
- 権限は600に設定します。
- ファイル名は`backup-YYYYMMDD-HHMMSS.sql.gz`のように命名されます。
- バックアップは mysqldump で取得します。
- バックアップは圧縮してからアップロードします。
