# 通知の配信対象（notifications DAG）

このドキュメントは、Airflow の `notifications` DAG が「誰に」「どの通知を」配信するかの判定ロジックを日本語でまとめたものです。実装は下記にあります。

- キュー投入: `batches/notifications/que-manager/main.go`
- メール配信: `batches/notifications/delivery-channels/email/main.go`
- Web Push 配信: `batches/notifications/delivery-channels/web-push/main.go`（現状 DAG では未実行）
- DAG 本体: `infra/airflow/dags/notifications.py`

## 全体像

1. Queue Manager が通知候補を抽出して `notifications_queue` テーブルにキューイングします（email/webpush の両チャネル分を投入）。
2. メール配信ジョブが `notifications_queue` から未処理（processed_at IS NULL）の項目を取り出し、ユーザーごとにまとめて 1 通で配信します。
3. Web Push 配信プログラムは実装済みですが、DAG では未呼び出しです（同テーブルの channel='webpush' を処理）。

## 通知キューに入るアイテム（Queue Manager）

Queue Manager は現在、次の 2 種類の通知を生成します。

- 課題の期限が近い通知（kind = "task"）

  - 「実効的な締切日時」が「今」から「あさって 23:59:59」までの課題を対象とします。
  - 実効的な締切日時は、`deadline` と `deadline_until` の組み合わせで決まります。
    - 例: MORNING -> 07:59:59, P1 -> 10:30:00, P2 -> 12:10:00, ...、指定なしは 23:59:59。
  - 宛先ユーザーは課題の `added_user_id`（課題を追加したユーザー）です。
  - 次の条件に該当する場合はスキップします:
    - ユーザーが削除済み（`users.is_deleted = 1`）
    - ユーザーがブロック中（`blocks` に有効期限内のレコードがある）

- 日直リマインド（kind = "dayshift"）
  - 「前営業日」と「当日」の `day_shift_assignments` に対象日が存在する場合に 1 件キューに入れます。
  - 現実装では `user_id = 0` をキューに入れ、配信チャネル側で担当者展開する想定ですが、メール/プッシュの実装は `user_id = 0` をスキップしているため、現状は配信されません（後述「既知の制限」）。

Queue Manager は、同一内容を email と webpush の 2 チャネルでそれぞれ `notifications_queue` に投入します。

### when_label と本文の例

- when_label: 今日 / あす / あさって / それ以外は日付表記（同年は MM 月 DD 日、別年は YYYY 年 MM 月 DD 日）
- task の本文: `"<日付/時限> まで"`（例: `10/16 5限まで`、`10/16 朝まで`）

## 配信チャネル別の宛先決定

### メール（email）

- 取得対象: `notifications_queue` から `processed_at IS NULL` かつ `channel IS NULL または channel='email'` の行。最大 200 件/回。
- ユーザー単位でグルーピングし、1 ユーザーにつき 1 通のダイジェストメールを送ります。
- 宛先解決: `users` テーブルの `email` を参照します。
  - `user_id` が NULL/0 の項目（例: 現状の dayshift）はスキップ対象です。
  - email 未登録の場合は送信できず、各アイテムは `status='error'` としてマークされます。
- メール本文の組み立ては email-compose の gRPC (`EMAIL_COMPOSE_GRPC_ADDR`、既定: `email-compose:50051`) 経由で行い、件名/テキスト/HTML を受け取って送信します。HTTP は使用しません。
- 送信結果は `notifications_queue.status` に `sent`/`error` で反映され、`processed_at` が更新されます。

必要な環境変数（抜粋）:

- DB 接続: `DB_HOST`, `DB_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DATABASE`
- SMTP: `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`, `SMTP_PASSWORD`
- その他: `EMAIL_COMPOSE_GRPC_ADDR`, `APP_NAME`, `LOCALE`

### Web Push（webpush）

- 取得対象: `notifications_queue` から `processed_at IS NULL AND channel='webpush'` の行。最大 200 件/回。
- 宛先解決: `web_push_subscriptions` から `user_id` に紐づく有効なサブスクリプションを全件取得して送信。
  - `user_id` が NULL/0 の項目はスキップされます（dayshift は現状通知されません）。
- 送信には VAPID 鍵が必要です（`VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`）。
- 送信結果は `sent`/`skipped`/`error` で管理され、`processed_at` が更新されます。

注: 現状の Airflow DAG では Web Push のジョブは実行していません。必要なら `infra/airflow/dags/notifications.py` に追加してください。

## スケジュール

- `infra/airflow/dags/notifications.py` では 1 日 2 回実行されます。
  - `schedule_interval = "0 22,8 * * *"`（UTC）
  - JST では 07:00 と 17:00 に相当します。
- Queue Manager の `now` は `TIMEZONE`（既定: `Asia/Tokyo`）で計算されます。

## 通知キューテーブル（自動作成）

`batches/notifications/que-manager/main.go` は起動時に次のテーブルを存在しなければ作成します。

```sql
CREATE TABLE IF NOT EXISTS notifications_queue (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  kind VARCHAR(32) NOT NULL,
  when_label VARCHAR(32) NOT NULL,
  subject VARCHAR(255) NOT NULL,
  title TEXT,
  body TEXT,
  user_id BIGINT NULL,
  channel VARCHAR(32) NULL, -- 'email' / 'webpush' など
  status VARCHAR(32) NULL,  -- 'sent' / 'error' / 'skipped' など
  error_message TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  processed_at DATETIME NULL,
  INDEX idx_processed_at (processed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 既知の制限・今後の拡張

- 日直（dayshift）の宛先展開は未実装です。
  - 現状: Queue Manager は `user_id = 0` でキュー投入。メール/プッシュは `user_id` が無いものをスキップするため配信されません。
  - 対応案（いずれか）:
    1. Queue Manager 側で `day_shift_assignment_users` から担当者の `user_id` を解決して複数行投入する。
    2. 各配信チャネル側で `kind='dayshift'` かつ `user_id=0` を受け取ったら担当者を展開して送る（その後、元行は `skipped`/`sent` の扱いを検討）。
- 配信のユーザー設定（チャネルごとの ON/OFF、配信時間帯、言語/ロケール等）は今後の拡張余地があります。現状はデフォルト有効で、ユーザーのメールが未登録/無効の場合のみ送れません。
- 1 回の処理で扱う件数は 200 件に制限しています（実装上の上限）。大量通知が想定される場合はスケジュール頻度や並列度の調整を検討してください。

## 参考: 抽出ロジックの要点（課題）

- 対象期間: 「現在」〜「あさって 23:59:59」まで
- 実効締切: `deadline` と `deadline_until` を `CASE` で時刻に変換して合成
- ラベル: `今日` / `あす` / `あさって` / それ以外は日付表記（年またぎ/翌月の考慮あり）
- フィルタ: `users.is_deleted` や `blocks` による除外

不明点や改善要望があれば、このファイルか `notifications.py` の近くに追記してください。
