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
- `notifications_queue`: Web Push 配信用のキュー。

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
  - 日直候補は宛先未確定のため、現状はキューに投入されない。
  - Email はキュー投入せず、配信時に対象課題とユーザーを取得する。
- Web Push Delivery（`batches/notifications/delivery-channels/web-push`）:
  - `channel='webpush'` を購読に配信し、処理済み行をクリーンアップ。Airflow の `notifications` DAG から実行される。
- Email Delivery（`batches/notifications/delivery-channels/email`）:
  - `notifications` DAG から実行され、ユーザーごとの課題ダイジェストを email-compose の gRPC と SMTP で送信する。現行実装は `TASK_MORNING`/`TASK_EVENING` を参照しない。
- Discord Delivery（`batches/notifications/delivery-channels/discord`）:
  - `notifications` DAG から実行される。

対象ユーザーと実行順序は [`batches/notifications/README.md`](batches/notifications/README.md) を参照してください。

## 既定値

- デバイス: `Email=true`, `WebPush=false`。
- 種類: `TASK`, `TASK_MORNING`, `TASK_EVENING`, `DAYSHIFT` は既定 ON。
- 配信メールの言語: `users.notification_lang` の既定値 `ja`。基本設定から日本語・英語を選択でき、ログインコード、課題ダイジェスト、日直、モデレーション通知に反映されます。

## 運用

- 互換 API や旧設定は段階的に devices/types へ集約。

## 既知の注意点

- `payload` は JSON を推奨（将来の拡張用）。
- Mobile（`IOS`/`ANDROID`）は将来拡張枠。
