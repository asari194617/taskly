# 通知バッチの配信対象

現行の `notifications` DAG、Queue Manager、配信プログラムの動作をまとめます。

## 実行順序と時刻

`infra/airflow/dags/notifications.py` は毎日 07:00 と 17:00 JST に実行します
（cron は UTC の `0 22,8 * * *`）。Queue Manager の後に email、webpush、discord
の配信ジョブが並列に動きます。実行ファイルは Airflow イメージ内の
`/opt/airflow/bin/` にあります。

## 課題通知

- Queue Manager・メール・Discord は JST の日付で同じ課題を抽出します。朝（07:00）は今日と明日、
  夕方（17:00）は今日の `deadline_until` が `AFTER_SCHOOL`（放課後まで）の課題と、
  明日・明後日の課題が対象です。`deadline_until=NONE`（時間指定なし）の課題は朝夕とも対象外です。
  前日以前と3日後以降も対象外です。
- Queue Manager は対象課題について、対象ユーザーごとに `channel='webpush'` の行を
  `notifications_queue` に追加します。`TASK` と実行時間帯の
  `TASK_MORNING` / `TASK_EVENING` 設定、Web Push の有効状態を参照します。
- Web Push 配信は未処理のキューを最大 200 件読みます。削除済み・ブロック中のユーザー、
  無効な通知設定、購読がないユーザーを除外し、有効な購読へ送信します。VAPID 鍵は
  DB の値を優先し、環境変数を代替として使います。
- メール配信はキューを読みません。実行時に締切が近い課題と対象ユーザーを取得し、
  Email と `TASK`・実行時間帯の設定を確認してユーザーごとにダイジェストを組み立てます。
  `users.notification_lang` に従って email-compose の gRPC で本文を生成し、SMTP で送信します。
- メール配信も上記の朝夕の対象範囲で通知します。SQL の境界値は JST の日付文字列で渡し、
  DB 接続のタイムゾーンによる前日へのずれを防ぎます。
- Queue Manager・メール・Discord は `delivery-channels/common` の `FetchDueTasks` と
  `LoadEligibleUsersAndPrefs` を使います。Web Push はキュー内の単一ユーザーに対して
  同じ判定を行う `LoadEligibleUserAndPrefs` を使います。課題は期限の昇順で取得します。
  ブロック・削除済み判定と端末・通知種別設定の読込はこの module に集約しています。
- 既定値は呼び出し側で指定します。Queue Manager は Email 有効・Web Push 無効、
  `TASK` と朝夕の種別を有効とします。メールは Email/Web Push とも有効、
  通知種別に既定値を置かず、設定がない `TASK`・朝夕は許可します。Discord は
  Email 有効・Web Push 無効、`TASK`/`DAYSHIFT` 有効です。Web Push 配信は
  Web Push 無効を既定値とし、有効な端末設定がある場合だけ送信します。
- Queue Manager は日直候補も作りますが、宛先ユーザーが `0` のままなので現在は
  キューに投入されず、この DAG から日直通知は配信されません。日直向けには別の
  `day_shift_notify_0830` DAG があります。

## 実装

- DAG: `infra/airflow/dags/notifications.py`
- キュー投入: `batches/notifications/que-manager/main.go`
- メール: `batches/notifications/delivery-channels/email/`
- Web Push: `batches/notifications/delivery-channels/web-push/main.go`
- Discord: `batches/notifications/delivery-channels/discord/app.go`

通知設定 API とデータモデルは [`../../notifications.md`](../../notifications.md) を参照してください。
