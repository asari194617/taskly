# バッチ処理

各種バッチ処理を実行するためのディレクトリです。

## ディレクトリ構成

- `batches/notifications/que-manager`: 各種通知(課題/日直)を管理するバッチ
  - `batches/notifications/delivery-channels/email`: メールで通知を送信するバッチ
  - `batches/notifications/delivery-channels/web-push`: Web Pushに通知を送信するバッチ
- `batches/day-shift-rotations`: 日直のローテーションを生成するバッチ
- `batches/backup-mysql`: MySQLのバックアップを行うバッチ

各配信チャネルに渡す値は共通化します。
ネイティブアプリを開発時に、ネイティブアプリ向けの配信チャネルを追加することを想定しています。

## バッチの実行管理

- 各処理はApache Airflowで管理します。
- Airflowの設定は`infra/airflow`にあります。
- AirflowのWeb UIは`http://localhost:3065`でアクセスできます。
- 処理の結果はDiscordのWebhookに通知されます。
- WebhookのURLは`.env`の`DISCORD_WEBHOOK_URL`で設定されています。
- 全てのバッチ処理はGoで実装します。`infra/docker-compose.yml`の`docker compose`でAirflowイメージをビルドし、バッチはイメージ内の事前ビルド済みバイナリとして実行します。

## 各バッチの説明

### `batches/notifications/que-manager`

課題や日直の通知を管理するバッチです。

- 期限が近い課題の通知
- 日直の通知
  を管理します。
  各種通知の条件に合致するものを抽出し、各配信チャネルに渡す形でキューイングします。
- 期限が近い課題の通知は、毎日7:00, 17:00に実行されます。
- 対象の課題は、期限が
  - 当日
  - 翌営業日
  - 翌々営業日
    のものです。
- 表記は
  - 今日
  - あす
  - あさって
  - XX日 (月を跨がない場合)
  - XX月YY日 (月を跨ぐ場合)
  - YYYY年XX月YY日 (年を跨ぐ場合)
    のいずれかになります。
- 日直の通知は、前日と当日に通知します。
- ブロック対象のユーザーには通知しません。
- 通知の内容は、課題のタイトルと期限、日直の担当者名などです。

### `batches/notifications/delivery-channels/email`

メールで通知を送信するバッチです。

- `que-manager`からキューイングされた通知を取得し、メールで送信します。

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
