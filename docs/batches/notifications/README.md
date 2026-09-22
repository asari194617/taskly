# 通知バッチの配信対象

現行の `notifications` DAG、Queue Manager、配信プログラムの動作をまとめます。

## 実行順序と時刻

`infra/airflow/dags/notifications.py` は毎日 07:00 と 17:00 JST に実行します
（cron は UTC の `0 22,8 * * *`）。Queue Manager の後に email、webpush、discord
の配信ジョブが並列に動きます。実行ファイルは Airflow イメージ内の
`/opt/airflow/bin/` にあります。

## 課題通知

- Queue Manager は JST の当日 00:00 から翌々日 00:00 まで（両端を含む）の締切日時を持つ課題を読み込み、対象ユーザーごとに
  `channel='webpush'` の行を `notifications_queue` に追加します。`TASK` と実行時間帯の
  `TASK_MORNING` / `TASK_EVENING` 設定、Web Push の有効状態を参照します。
- Web Push 配信は未処理のキューを最大 200 件読みます。削除済み・ブロック中のユーザー、
  無効な通知設定、購読がないユーザーを除外し、有効な購読へ送信します。VAPID 鍵は
  DB の値を優先し、環境変数を代替として使います。
- メール配信はキューを読みません。実行時に締切が近い課題と対象ユーザーを取得し、
  Email と `TASK`・実行時間帯の設定を確認してユーザーごとにダイジェストを組み立てます。
  `users.notification_lang` に従って email-compose の gRPC で本文を生成し、SMTP で送信します。
- メール配信の締切日時の抽出範囲も同じです。翌々日の 00:00 より後の課題は
  現行の SQL では対象外です。
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
