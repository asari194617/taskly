# 通知アーキテクチャ（types × devices）

本ドキュメントは通知機構を「通知種類（types）」と「デバイス（devices）」に分離した新アーキテクチャについてまとめます。旧 `notification_settings` は互換 API のみ残し、内部実装は devices/types へ移行済みです。

## コンセプト

- 通知種類（Notification Types）
  - 例: `TASK`, `DAYSHIFT`
  - ユーザーごとに ON/OFF を保持します（テーブル: `notification_type_prefs`）。
  - 既定値: すべて ON。
- デバイス（Notification Devices）
  - 例: `EMAIL`, `WEBPUSH`, 将来拡張: `IOS`, `ANDROID`
  - ユーザーごとにチャネル（プラットフォーム）単位で ON/OFF と任意の `name`/`payload` を保持します（テーブル: `notification_devices`）。
  - 既定値: `EMAIL`=ON, `WEBPUSH`=OFF。
  - `updated_at` 列により最終更新を保持します。

## データモデル

- `notification_type_prefs`
  - `id`, `user_id`, `notification_type`(ENUM/TEXT), `is_enabled`(BOOL), `created_at`, `updated_at`
- `notification_devices`
  - `id`, `user_id`, `platform`(ENUM/TEXT: `EMAIL`/`WEBPUSH`/`IOS`/`ANDROID` ...), `name`(NULL 可), `payload`(NULL 可, JSON 推奨), `is_enabled`(BOOL), `created_at`, `updated_at`

マイグレーション:

- `0007_drop_push_daily_enabled.sql`: 旧 `users.push_daily_enabled` を削除
- `0008_notification_type_prefs.sql`: 種類プリファレンス作成
- `0009_notification_devices.sql`: デバイス登録作成（`updated_at` 含む）

## 互換 API（NotificationSettings の扱い）

- 旧 `NotificationSettings` は廃止。GraphQL 互換フィールド `adminUserNotificationSettings` は内部で devices の `EMAIL`/`WEBPUSH` を参照して返します。
- `updated_at` 相当は、該当デバイス（EMAIL/WEBPUSH）の `updated_at` のうち最新を返す方針です。

## GraphQL API

クエリ:

- `adminUserNotificationTypePrefs(user_id: Int!): NotificationTypePrefs`
  - 例: `{ TASK, DAYSHIFT }`
- `adminUserDevices(user_id: Int!): [NotificationDevice!]!`
  - 例: `{ id user_id platform name payload is_enabled created_at updated_at }`
- `adminUserNotificationSettings(user_id: Int!)`
  - 互換: `{ email_enabled, webpush_enabled, updated_at }`（内部は devices 参照）

ミューテーション:

- `adminUserSetNotificationTypePref(user_id: Int!, notification_type: String!, enabled: Boolean!): Boolean!`
- `adminUserCreateDevice(user_id: Int!, platform: String!, name: String, payload: String, is_enabled: Boolean!): ID!`
- `adminUserUpdateDevice(device_id: Int!, platform: String, name: String, payload: String, is_enabled: Boolean): Boolean!`
- `adminUserDeleteDevice(device_id: Int!): Boolean!`
- 互換: `adminUserSetNotificationSettings(user_id: Int!, email_enabled: Boolean!, webpush_enabled: Boolean!): Boolean!`

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

## キュー処理（配信ロジック）

- キュー投入側（`batches/notifications/que-manager`）は以下の条件で配信対象を決定します:
  1. 対象ユーザーの該当「通知種類」が ON であること（例: タスク通知なら `TASK`）。
  2. 対象ユーザーに有効（`is_enabled=1`）なデバイスが存在するチャネルに対してメッセージを投入（`EMAIL`, `WEBPUSH` 等）。
- これにより「種類 × デバイス」の掛け合わせで配信可否を制御できます。

## 既定値の適用

- 新規ユーザー: type はすべて ON。`EMAIL` デバイスは ON、`WEBPUSH` は OFF（作成は必要に応じて）。
- UI のトグル（管理画面）も同既定を前提に表示します。

## 管理 UI

- 画面: 管理 > Users（`apps/web/src/react/admin/Users.tsx`）
  - 機能:
    - 種類トグル（TASK/DAYSHIFT）
    - デバイス一覧（ON/OFF 切替・削除・追加、`updated_at` 表示）
    - 互換トグル（EMAIL/WEBPUSH）→ devices に反映

## 既知の注意点・今後

- `payload` は JSON 文字列推奨。UI 側でバリデーションを強化予定。
- Mobile（`IOS`/`ANDROID`）は将来拡張用。現時点では配信実装は未接続。
- 旧 `notification_settings` テーブルは不要。GraphQL 互換のみ存置。
