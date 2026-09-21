# 通知設定・配信

通知機構は「通知種類（types）」と「デバイス（devices）」に分離されています。通知種類の設定は方法（メール/プッシュ）に共通です。

## 通知方法（Methods）

- Web Push: 単一のスイッチで有効/無効を切替。ブラウザ購読の登録/解除を行い、端末（device）行をミラーします。
- Email: ユーザー単位の有効/無効。メールは配信時にオンザフライで合成（キュー投入なし）。

## 通知の種類（Types：方法に共通）

- 期限が近い課題（`TASK`）: ON/OFF。
- 朝の通知（`TASK_MORNING`）: ON/OFF（朝バッチ枠で適用）。
- 夕方の通知（`TASK_EVENING`）: ON/OFF（夕方バッチ枠で適用）。
- 日直のお知らせ（`DAYSHIFT`）: ON/OFF。

備考:

- `TASK_MORNING`/`TASK_EVENING` は時間帯ゲート。未設定時は既定 ON。
- 種類は方法に依存せず共通フラグです（メール/プッシュに同様に適用）。

## データモデル

- `notification_type_prefs`: `id`, `user_id`, `notification_type`, `is_enabled`, `created_at`, `updated_at`
- `notification_devices`: `id`, `user_id`, `platform`（`EMAIL`/`WEBPUSH` など）, `name`, `payload`, `is_enabled`, `created_at`, `updated_at`
- `web_push_subscriptions`: ブラウザ Push 購読（`is_revoked` で無効化管理）。
- `notifications_queue`: Web Push 配信用のキュー。`event_key` で同じ対象・日付・時間帯の再投入を防ぐ。
- `notification_deliveries`: 宛先単位の配信結果。`sending`、`sent`、`error` と試行回数・エラーを保持する。

## サーバー API（REST）

- Methods:
  - `GET /settings/notification-methods` → `{ email, webpush }`
  - `POST /settings/notification-methods` body `{ email?: bool, webpush?: bool }`
- Types:
  - `GET /settings/notification-types` → `{ TASK, TASK_MORNING, TASK_EVENING, DAYSHIFT }`
  - `POST /settings/notification-types` body `{ key: string, value: bool }`
- Devices（Web Push）:
  - `GET /push/devices` → `[ { id, name, is_enabled } ]`
  - `POST /push/devices/:id` body `{ is_enabled: bool }`
- Web Push 購読:
  - `POST /push/subscribe`（購読登録＋デバイス enabled）
  - `POST /push/unsubscribe`（購読解除＋デバイス disabled）
- 旧互換:
  - `GET/POST /settings/push`（内部的に Web Push デバイス enabled にマップ）

## フロントエンド

- `apps/web/src/react/settings/Notifications.tsx`: 通知方法、通知種類、端末設定を表示します。
- 端末名は User-Agent から推定。

サンプル:

```graphql
query GetUserNotifs($id: Int!) {
  adminUserNotificationTypePrefs(user_id: $id) {
    TASK
    DAYSHIFT
  }
  adminUserDevices(user_id: $id) {
    id
    platform
    is_enabled
    updated_at
  }
}
```

```graphql
mutation UpdatePrefs($id: Int!) {
  adminUserSetNotificationTypePref(
    user_id: $id
    notification_type: "TASK"
    enabled: true
  )
  adminUserSetNotificationSettings(
    user_id: $id
    email_enabled: true
    webpush_enabled: false
  )
}
```

## バッチ（Queue/Delivery）

- Queue Manager（`batches/notifications/que-manager`）:
  - タスク（JST の今日 00:00〜あさって 00:00）を Web Push キューに投入。`TASK` と時間帯に応じた `TASK_MORNING`/`TASK_EVENING` を適用。
  - 日直は 08:30 の日直 DAG が担当者を確定してキューに投入する。
  - Email はキュー投入せず、配信時に対象課題とユーザーを取得する。
- Web Push Delivery（`batches/notifications/delivery-channels/web-push`）:
  - `channel='webpush'` を購読ごとに配信する。成功済み購読は再実行時に再送しない。Airflow の `notifications` DAG と日直 DAG から実行される。
- Email Delivery（`batches/notifications/delivery-channels/email`）:
  - `notifications` DAG から実行され、ユーザーごとの課題ダイジェストを email-compose の gRPC と SMTP で送信する。`TASK` と実行時刻に応じた `TASK_MORNING`/`TASK_EVENING` を適用する。
- Discord Delivery（`batches/notifications/delivery-channels/discord`）:
  - `notifications` DAG から実行される。

対象ユーザーと実行順序は [`batches/notifications/README.md`](batches/notifications/README.md) を参照してください。

## 既定値

- デバイス: `Email=true`, `WebPush=false`。
- 種類: `TASK`, `TASK_MORNING`, `TASK_EVENING`, `DAYSHIFT` は既定 ON。
- 配信メールの言語: `users.notification_lang` の既定値 `ja`。基本設定から日本語・英語を選択でき、ログインコード、課題ダイジェスト、日直、モデレーション通知に反映されます。

## 運用

- 互換 API や旧設定は段階的に devices/types へ集約。
- 配信失敗は Airflow タスクを失敗にし、`notification_deliveries.status='error'` と Web Push では `notifications_queue.status='error'` に記録する。Web Push の失敗行は `processed_at` を空のまま保ち、次の実行で失敗した購読だけを再試行する。両 DAG は 5 分間隔で最大 2 回再試行する。
- `sending` が残った場合は、SMTP や Push サービスが受け付けた直後にプロセスが停止した可能性がある。自動再送はせず、送信先で受信状況を確認してから `sent` または `error` に更新する。SMTP には受信側の冪等 API がないため、この状態の自動的な厳密一回配信は保証できない。

## 対象者・設定の判定表

| 配信 | 対象者 | 必要な設定 | ブロック・削除 | 再実行 |
| --- | --- | --- | --- | --- |
| 課題 Email | 期限内の課題があるユーザー | Email、`TASK`、朝は `TASK_MORNING` / 夕方は `TASK_EVENING` | 除外 | 同日・同時間帯は送信済みユーザーを除外 |
| 課題 Web Push | 同上 | Web Push デバイス、購読、`TASK`、朝夕の該当設定 | 除外 | 課題・ユーザー・時間帯ごとに重複投入を防ぎ、購読ごとに送信済みを除外 |
| 日直 Email | 当日の担当グループ | Email、`DAYSHIFT` | 除外 | 同日・ユーザーごとに送信済みを除外 |
| 日直 Web Push | 当日の担当グループ | Web Push デバイス、購読、`DAYSHIFT` | 除外 | 同日・ユーザーごとに重複投入を防ぎ、購読ごとに送信済みを除外 |

未設定時は Email と通知種類が ON、Web Push が OFF。朝夕の設定は課題のみに適用し、日直には適用しない。配信直前にもユーザー・ブロック・種類・Web Push デバイスを再確認する。Web Push は購読の endpoint に対応する有効な端末にだけ送る。

## 既知の注意点

- `payload` は JSON を推奨（将来の拡張用）。
- Mobile（`IOS`/`ANDROID`）は将来拡張枠。
