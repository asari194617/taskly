# バッチ処理
各種バッチ処理を実行するためのディレクトリです。

## ディレクトリ構成
- `batches/notifications/que-manager`: 各種通知(課題/日直)を管理するバッチ
- `batches/notifications/delivery-channels/email`: メールで通知を送信するバッチ
- `batches/notifications/delivery-channels/web-push`: Web Pushに通知を送信するバッチ

各配信チャネルに渡す値は共通化します。
ネイティブアプリを開発時に、ネイティブアプリ向けの配信チャネルを追加することを想定しています。

## バッチの実行管理
各処理はApache Airflowで管理します。
Airflowの設定は`infra/airflow`にあります。
AirflowのWeb UIは`http://localhost:3065`でアクセスできます。

