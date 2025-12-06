# Schemas

このドキュメントは、Taskly で使用されるデータスキーマの仕様を説明します。

## 凡例

### `table_name`: 用途

テーブルの説明を入れます。(オプション)

| フィールド名 | 型          | 属性                        | 詳細            |
| ------------ | ----------- | --------------------------- | --------------- |
| `id`         | `INT`       | PK                          |                 |
| `org_id`     | `INT`       | FK(orgs.id), NOT NULL       | 組織を示す id。 |
| `created_at` | `TIMESTAMP` | CURRENT_TIMESTAMP, NOT NULL |                 |

## テーブル一覧

### `users`: ユーザー情報

| フィールド名          | 型             | 属性                                | 詳細                       |
| --------------------- | -------------- | ----------------------------------- | -------------------------- |
| `id`                  | `BIGINT`       | PK, AUTO_INCREMENT                  | ユーザー ID                |
| `email`               | `VARCHAR(255)` | UNIQUE, NOT NULL                    | メールアドレス             |
| `display_name`        | `VARCHAR(255)` |                                     | 表示名                     |
| `hashed_password`     | `VARCHAR(255)` | NULL                                | パスワードハッシュ（任意） |
| `is_deleted`          | `BOOLEAN`      | NOT NULL, DEFAULT FALSE             | 退会/削除フラグ            |
| `is_day_shift_active` | `BOOLEAN`      | NOT NULL, DEFAULT FALSE             | 当番機能の有効化           |
| `display_lang`        | `VARCHAR(5)`   | NOT NULL, DEFAULT 'ja'              | UI 表示言語                |
| `notification_lang`   | `VARCHAR(5)`   | NOT NULL, DEFAULT 'ja'              | 通知の言語                 |
| `email_code_allowed`  | `BOOLEAN`      | NOT NULL, DEFAULT TRUE              | メールコードログインの許可 |
| `is_registered`       | `BOOLEAN`      | NOT NULL, DEFAULT FALSE             | 登録完了フラグ             |
| `is_agreed_policies`  | `BOOLEAN`      | NOT NULL, DEFAULT FALSE             | 規約同意フラグ             |
| `created_at`          | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 作成日時                   |

### `subjects`: 科目

| フィールド名 | 型             | 属性                                | 詳細             |
| ------------ | -------------- | ----------------------------------- | ---------------- |
| `id`         | `BIGINT`       | PK, AUTO_INCREMENT                  | 科目 ID          |
| `name_ja`    | `VARCHAR(255)` | NOT NULL                            | 科目名（日本語） |
| `name_en`    | `VARCHAR(255)` | NULL                                | 科目名（英語）   |
| `expires_at` | `DATETIME`     | NULL                                | 期限（任意）     |
| `created_at` | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 作成日時         |

### `tasks`: 課題・タスク

用途: 科目に紐づく課題。期日や種別、完了状態などを保持。
| フィールド名 | 型 | 属性 | 詳細 |
| ------------- | --- | ---- | ---- |
| `id` | `BIGINT` | PK, AUTO_INCREMENT | タスク ID |
| `added_user_id` | `BIGINT` | FK(users.id), NOT NULL | 追加したユーザー |
| `title` | `VARCHAR(255)` | NOT NULL | タイトル |
| `subject_id` | `BIGINT` | FK(subjects.id), NOT NULL | 科目 ID |
| `description` | `TEXT` | NULL | 詳細説明 |
| `is_require_hand_in` | `BOOLEAN` | NOT NULL, DEFAULT FALSE | 提出必須か |
| `deadline` | `DATE` | NULL | 期日（旧） |
| `deadline_since` | `ENUM('NONE','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10')` | NOT NULL, DEFAULT 'NONE' | 期日の開始コマ（拡張済み） |
| `deadline_until` | `ENUM('NONE','MORNING','LUNCH','AFTER_SCHOOL','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10')` | NOT NULL, DEFAULT 'NONE' | 期日の終了帯/コマ（拡張済み） |
| `type` | `ENUM('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER')` | NOT NULL, DEFAULT 'OTHER' | 課題種別 |
| `is_hidden` | `BOOLEAN` | NOT NULL, DEFAULT 0 | モデレーションにより非表示か |
| `created_at` | `DATETIME` | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 作成日時 |
| `updated_at` | `DATETIME` | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP | 更新日時 |

インデックス/制約:

- `FK (added_user_id) REFERENCES users(id)`
- `FK (subject_id) REFERENCES subjects(id)`
- `INDEX idx_tasks_deadline_id (deadline, id)`
- `FULLTEXT ft_tasks_title_desc (title, description)`

### `ical_keys`: iCal キー

| フィールド名  | 型             | 属性                                | 詳細             |
| ------------- | -------------- | ----------------------------------- | ---------------- |
| `id`          | `BIGINT`       | PK, AUTO_INCREMENT                  |                  |
| `user_id`     | `BIGINT`       | FK(users.id), NOT NULL              | ユーザー         |
| `token`       | `VARCHAR(255)` | UNIQUE, NOT NULL                    | アクセストークン |
| `description` | `VARCHAR(255)` | NULL                                | 説明             |
| `created_at`  | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP |                  |

### `passkeys`: WebAuthn パスキー

| フィールド名    | 型             | 属性                                                            | 詳細              |
| --------------- | -------------- | --------------------------------------------------------------- | ----------------- |
| `id`            | `BIGINT`       | PK, AUTO_INCREMENT                                              |                   |
| `user_id`       | `BIGINT`       | FK(users.id), NOT NULL                                          | ユーザー          |
| `credential_id` | `VARCHAR(255)` | UNIQUE, NOT NULL                                                | クレデンシャル ID |
| `public_key`    | `TEXT`         | NOT NULL                                                        | 公開鍵            |
| `sign_count`    | `BIGINT`       | NOT NULL                                                        | 署名回数          |
| `device_name`   | `VARCHAR(255)` | NULL                                                            | 端末名            |
| `created_at`    | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP                             |                   |
| `updated_at`    | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP |                   |

### `totp_secrets`: TOTP 秘密鍵

| フィールド名 | 型             | 属性                                | 詳細                              |
| ------------ | -------------- | ----------------------------------- | --------------------------------- |
| `id`         | `BIGINT`       | PK, AUTO_INCREMENT                  |                                   |
| `user_id`    | `BIGINT`       | UNIQUE, NOT NULL                    | ユーザー（1 ユーザー 1 レコード） |
| `secret`     | `VARCHAR(255)` | NOT NULL                            | 秘密鍵                            |
| `is_active`  | `BOOLEAN`      | NOT NULL, DEFAULT FALSE             | 有効化状態                        |
| `created_at` | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP |                                   |

### `recovery_codes`: リカバリーコード

| フィールド名  | 型             | 属性                                | 詳細                   |
| ------------- | -------------- | ----------------------------------- | ---------------------- |
| `id`          | `BIGINT`       | PK, AUTO_INCREMENT                  |                        |
| `user_id`     | `BIGINT`       | FK(users.id), NOT NULL              | ユーザー               |
| `hashed_code` | `VARCHAR(255)` | NOT NULL                            | ハッシュ化されたコード |
| `is_used`     | `BOOLEAN`      | NOT NULL, DEFAULT FALSE             | 使用済みフラグ         |
| `created_at`  | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP |                        |

### `blocks`: ブロック履歴

| フィールド名 | 型         | 属性                                                            | 詳細         |
| ------------ | ---------- | --------------------------------------------------------------- | ------------ |
| `id`         | `BIGINT`   | PK, AUTO_INCREMENT                                              |              |
| `user_id`    | `BIGINT`   | FK(users.id), NOT NULL                                          | 対象ユーザー |
| `reason`     | `TEXT`     | NULL                                                            | 理由         |
| `expires_at` | `DATETIME` | NULL                                                            | 期限         |
| `created_at` | `DATETIME` | NOT NULL, DEFAULT CURRENT_TIMESTAMP                             |              |
| `updated_at` | `DATETIME` | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP |              |

### `role_users`: 役割付与

| フィールド名 | 型                        | 属性                                | 詳細     |
| ------------ | ------------------------- | ----------------------------------- | -------- |
| `id`         | `BIGINT`                  | PK, AUTO_INCREMENT                  |          |
| `user_id`    | `BIGINT`                  | FK(users.id), NOT NULL              | ユーザー |
| `role`       | `ENUM('admin','manager')` | NOT NULL                            | 役割     |
| `created_at` | `DATETIME`                | NOT NULL, DEFAULT CURRENT_TIMESTAMP |          |

### `permissions`: 任意権限ルール

| フィールド名 | 型             | 属性                                                            | 詳細             |
| ------------ | -------------- | --------------------------------------------------------------- | ---------------- |
| `id`         | `BIGINT`       | PK, AUTO_INCREMENT                                              |                  |
| `user_id`    | `BIGINT`       | FK(users.id), NOT NULL                                          | ユーザー         |
| `target`     | `VARCHAR(64)`  | NOT NULL                                                        | 対象リソース     |
| `method`     | `VARCHAR(16)`  | NOT NULL                                                        | 操作/メソッド    |
| `value`      | `VARCHAR(255)` | NULL                                                            | 値（オプション） |
| `created_at` | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP                             |                  |
| `updated_at` | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP |                  |

### `sessions`: セッション

| フィールド名      | 型             | 属性                                                            | 詳細                 |
| ----------------- | -------------- | --------------------------------------------------------------- | -------------------- |
| `id`              | `BIGINT`       | PK, AUTO_INCREMENT                                              |                      |
| `client_platform` | `VARCHAR(64)`  | NOT NULL                                                        | クライアント種別     |
| `hashed_token`    | `VARCHAR(255)` | NOT NULL                                                        | トークン（ハッシュ） |
| `expires_at`      | `DATETIME`     | NULL                                                            | 期限                 |
| `user_id`         | `BIGINT`       | FK(users.id), NULL                                              | 所有ユーザー         |
| `created_at`      | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP                             |                      |
| `updated_at`      | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP |                      |

### `api_keys`: API キー

| フィールド名   | 型             | 属性                                                            | 詳細                 |
| -------------- | -------------- | --------------------------------------------------------------- | -------------------- |
| `id`           | `BIGINT`       | PK, AUTO_INCREMENT                                              |                      |
| `user_id`      | `BIGINT`       | FK(users.id), NOT NULL                                          | ユーザー             |
| `hashed_token` | `VARCHAR(255)` | NOT NULL                                                        | トークン（ハッシュ） |
| `description`  | `VARCHAR(255)` | NULL                                                            | 説明                 |
| `is_valid`     | `BOOLEAN`      | NOT NULL, DEFAULT TRUE                                          | 有効フラグ           |
| `expires_at`   | `DATETIME`     | NULL                                                            | 期限（任意）         |
| `created_at`   | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP                             |                      |
| `updated_at`   | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP |                      |

### `logs`: 操作ログ

| フィールド名 | 型            | 属性                                | 詳細         |
| ------------ | ------------- | ----------------------------------- | ------------ |
| `id`         | `BIGINT`      | PK, AUTO_INCREMENT                  |              |
| `user_id`    | `BIGINT`      | FK(users.id), NULL                  | 実行ユーザー |
| `action`     | `VARCHAR(64)` | NOT NULL                            | アクション   |
| `created_at` | `DATETIME`    | NOT NULL, DEFAULT CURRENT_TIMESTAMP |              |

### `api_action_logs`: API アクションログ

| フィールド名   | 型             | 属性                                | 詳細                 |
| -------------- | -------------- | ----------------------------------- | -------------------- |
| `id`           | `BIGINT`       | PK, AUTO_INCREMENT                  |                      |
| `user_id`      | `BIGINT`       | FK(users.id), NULL                  | 実行ユーザー         |
| `hashed_token` | `VARCHAR(255)` | NOT NULL                            | API キー（ハッシュ） |
| `action`       | `VARCHAR(64)`  | NOT NULL                            | アクション           |
| `ip`           | `VARCHAR(64)`  | NOT NULL                            | 発信 IP              |
| `created_at`   | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP |                      |

### `tasks_teams_logs`: Teams 送信ログ

| フィールド名            | 型             | 属性                    | 詳細             |
| ----------------------- | -------------- | ----------------------- | ---------------- |
| `id`                    | `BIGINT`       | PK, AUTO_INCREMENT      |                  |
| `task_id`               | `BIGINT`       | FK(tasks.id), NOT NULL  | 対象タスク       |
| `teams_message_id`      | `VARCHAR(255)` | NULL                    | メッセージ ID    |
| `teams_conversation_id` | `VARCHAR(255)` | NULL                    | 会話 ID          |
| `is_success`            | `BOOLEAN`      | NOT NULL, DEFAULT FALSE | 成否             |
| `callback_url`          | `TEXT`         | NULL                    | コールバック URL |
| `attempts`              | `INT`          | NOT NULL, DEFAULT 0     | 試行回数         |
| `last_status`           | `VARCHAR(32)`  | NULL                    | 最終ステータス   |
| `last_error`            | `TEXT`         | NULL                    | 最終エラー       |
| `last_response_status`  | `INT`          | NULL                    | 応答コード       |
| `last_response_body`    | `TEXT`         | NULL                    | 応答本文         |

### `tasks_users_completions`: ユーザー完了状態

| フィールド名   | 型         | 属性                                | 詳細     |
| -------------- | ---------- | ----------------------------------- | -------- |
| `id`           | `BIGINT`   | PK, AUTO_INCREMENT                  |          |
| `user_id`      | `BIGINT`   | FK(users.id), NOT NULL              | ユーザー |
| `task_id`      | `BIGINT`   | FK(tasks.id), NOT NULL              | タスク   |
| `completed_at` | `DATETIME` | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 完了日時 |

ユニーク・インデックス:

- `UNIQUE (user_id, task_id)`
- `INDEX idx_task_user (task_id, user_id)`

### `notifications_queue`: 通知キュー

| フィールド名    | 型             | 属性                                    | 詳細                      |
| --------------- | -------------- | --------------------------------------- | ------------------------- |
| `id`            | `BIGINT`       | PK, AUTO_INCREMENT                      |                           |
| `kind`          | `VARCHAR(32)`  | NOT NULL                                | 種別                      |
| `when_label`    | `VARCHAR(32)`  | NOT NULL                                | タイミングラベル          |
| `subject`       | `VARCHAR(255)` | NOT NULL                                | 件名                      |
| `title`         | `TEXT`         | NULL                                    | タイトル                  |
| `body`          | `TEXT`         | NULL                                    | 本文                      |
| `meta`          | `TEXT`         | NULL                                    | メタ情報                  |
| `badges_json`   | `TEXT`         | NULL                                    | バッジ情報（JSON 文字列） |
| `user_id`       | `BIGINT`       | NULL                                    | 対象ユーザー              |
| `channel`       | `VARCHAR(32)`  | NULL                                    | 送信チャネル              |
| `status`        | `VARCHAR(32)`  | NULL                                    | ステータス                |
| `error_message` | `TEXT`         | NULL                                    | エラー内容                |
| `created_at`    | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP     |                           |
| `processed_at`  | `DATETIME`     | NULL                                    | 処理日時                  |
| インデックス    |                | `INDEX idx_processed_at (processed_at)` |                           |

### `day_shift_cycles`: 当番サイクル

| フィールド名  | 型             | 属性               | 詳細   |
| ------------- | -------------- | ------------------ | ------ |
| `id`          | `BIGINT`       | PK, AUTO_INCREMENT |        |
| `begin_date`  | `DATE`         | NOT NULL           | 開始日 |
| `finish_date` | `DATE`         | NOT NULL           | 終了日 |
| `description` | `VARCHAR(255)` | NOT NULL           | 説明   |

### `day_shift_assignments`: 当番割当

| フィールド名 | 型                                           | 属性                                                            | 詳細     |
| ------------ | -------------------------------------------- | --------------------------------------------------------------- | -------- |
| `id`         | `BIGINT`                                     | PK, AUTO_INCREMENT                                              |          |
| `date`       | `DATE`                                       | NOT NULL                                                        | 日付     |
| `cycle_id`   | `BIGINT`                                     | FK(day_shift_cycles.id), NOT NULL                               | サイクル |
| `status`     | `ENUM('assigned','completed','unfulfilled')` | NOT NULL, DEFAULT 'assigned'                                    | 状態     |
| `notes`      | `TEXT`                                       | NULL                                                            | メモ     |
| `created_at` | `DATETIME`                                   | NOT NULL, DEFAULT CURRENT_TIMESTAMP                             |          |
| `updated_at` | `DATETIME`                                   | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP |          |

ユニーク・インデックス:

- `UNIQUE (date, cycle_id)`

### `day_shift_assignment_users`: 当番担当者

| フィールド名               | 型       | 属性                                   | 詳細     |
| -------------------------- | -------- | -------------------------------------- | -------- |
| `id`                       | `BIGINT` | PK, AUTO_INCREMENT                     |          |
| `day_shift_assignments_id` | `BIGINT` | FK(day_shift_assignments.id), NOT NULL | 割当     |
| `user_id`                  | `BIGINT` | FK(users.id), NOT NULL                 | ユーザー |

<!-- Business calendar feature removed. -->

### `notification_type_prefs`: 通知タイプ設定（ユーザー別）

| フィールド名        | 型                        | 属性                                | 詳細     |
| ------------------- | ------------------------- | ----------------------------------- | -------- |
| `id`                | `BIGINT`                  | PK, AUTO_INCREMENT                  |          |
| `user_id`           | `BIGINT`                  | FK(users.id), NOT NULL              | ユーザー |
| `notification_type` | `ENUM('TASK','DAYSHIFT')` | NOT NULL                            | 通知対象 |
| `is_enabled`        | `TINYINT(1)`              | NOT NULL, DEFAULT 1                 | 有効か   |
| `created_at`        | `DATETIME`                | NOT NULL, DEFAULT CURRENT_TIMESTAMP |          |

ユニーク・インデックス/制約:

- `UNIQUE (user_id, notification_type)`
- `FOREIGN KEY fk_ntp_user (user_id) REFERENCES users(id) ON DELETE CASCADE`

### `notification_devices`: 通知デバイス

| フィールド名      | 型                                        | 属性                                                                                                            | 詳細             |
| ----------------- | ----------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ---------------- |
| `id`              | `BIGINT`                                  | PK, AUTO_INCREMENT                                                                                              |                  |
| `user_id`         | `BIGINT`                                  | FK(users.id), NOT NULL                                                                                          | ユーザー         |
| `name`            | `VARCHAR(255)`                            | NULL                                                                                                            | 名称             |
| `platform`        | `ENUM('EMAIL','WEBPUSH','IOS','ANDROID')` | NOT NULL                                                                                                        | プラットフォーム |
| `payload`         | `JSON`                                    | NULL                                                                                                            | 端末情報等       |
| `is_enabled`      | `TINYINT(1)`                              | NOT NULL, DEFAULT 1                                                                                             | 有効か           |
| `created_at`      | `DATETIME`                                | NOT NULL, DEFAULT CURRENT_TIMESTAMP                                                                             |                  |
| `updated_at`      | `DATETIME`                                | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP                                                 |                  |
| インデックス/制約 |                                           | `INDEX idx_user_platform (user_id, platform)`, `FK fk_nd_user (user_id) REFERENCES users(id) ON DELETE CASCADE` |                  |

### `login_codes`: ログインコード（メール）

| フィールド名      | 型             | 属性                                                                                   | 詳細               |
| ----------------- | -------------- | -------------------------------------------------------------------------------------- | ------------------ |
| `id`              | `BIGINT`       | PK, AUTO_INCREMENT                                                                     |                    |
| `email`           | `VARCHAR(255)` | NOT NULL                                                                               | メールアドレス     |
| `hashed_code`     | `VARCHAR(255)` | NOT NULL                                                                               | コード（ハッシュ） |
| `expires_at`      | `DATETIME`     | NOT NULL                                                                               | 期限               |
| `consumed_at`     | `DATETIME`     | NULL                                                                                   | 消費日時           |
| `failed_attempts` | `INT`          | NOT NULL, DEFAULT 0                                                                    | 失敗回数           |
| `locked_until`    | `DATETIME`     | NULL                                                                                   | ロック期限         |
| `created_at`      | `DATETIME`     | NOT NULL, DEFAULT CURRENT_TIMESTAMP                                                    |                    |
| インデックス      |                | `INDEX idx_login_codes_email (email)`, `INDEX idx_login_codes_expires_at (expires_at)` |                    |

### `flagged_contents`: モデレーション検知

| フィールド名 | 型                | 属性                                                                                                 | 詳細                 |
| ------------ | ----------------- | ---------------------------------------------------------------------------------------------------- | -------------------- |
| `id`         | `BIGINT UNSIGNED` | PK, AUTO_INCREMENT                                                                                   |                      |
| `task_id`    | `BIGINT UNSIGNED` | NOT NULL                                                                                             | 対象タスク           |
| `category`   | `VARCHAR(64)`     | NOT NULL                                                                                             | カテゴリ             |
| `severity`   | `VARCHAR(16)`     | NOT NULL                                                                                             | 深刻度               |
| `score`      | `DECIMAL(5,3)`    | NULL                                                                                                 | スコア               |
| `threat`     | `BOOLEAN`         | NOT NULL, DEFAULT FALSE                                                                              | 脅威判定             |
| `reason`     | `TEXT`            | NULL                                                                                                 | 理由                 |
| `model`      | `VARCHAR(64)`     | NULL                                                                                                 | モデル名             |
| `metadata`   | `JSON`            | NULL                                                                                                 | 付帯情報             |
| `snapshot`   | `JSON`            | NULL                                                                                                 | 対象スナップショット |
| `created_at` | `DATETIME`        | NOT NULL, DEFAULT CURRENT_TIMESTAMP                                                                  |                      |
| インデックス |                   | `INDEX idx_flagged_contents_task_id (task_id)`, `INDEX idx_flagged_contents_created_at (created_at)` |                      |

### `user_reports`: ユーザー通報

| フィールド名       | 型                | 属性                                                                                             | 詳細       |
| ------------------ | ----------------- | ------------------------------------------------------------------------------------------------ | ---------- |
| `id`               | `BIGINT UNSIGNED` | PK, AUTO_INCREMENT                                                                               |            |
| `reporter_user_id` | `BIGINT UNSIGNED` | NULL                                                                                             | 通報者     |
| `task_id`          | `BIGINT UNSIGNED` | NOT NULL                                                                                         | 対象タスク |
| `reason`           | `VARCHAR(255)`    | NOT NULL                                                                                         | 理由       |
| `comment`          | `TEXT`            | NULL                                                                                             | コメント   |
| `created_at`       | `DATETIME`        | NOT NULL, DEFAULT CURRENT_TIMESTAMP                                                              |            |
| インデックス       |                   | `INDEX idx_user_reports_task_id (task_id)`, `INDEX idx_user_reports_reporter (reporter_user_id)` |            |

### `email_suppressions`: メール一時停止

| フィールド名      | 型                | 属性                                                                                                 | 詳細           |
| ----------------- | ----------------- | ---------------------------------------------------------------------------------------------------- | -------------- |
| `id`              | `BIGINT UNSIGNED` | PK, AUTO_INCREMENT                                                                                   |                |
| `email`           | `VARCHAR(255)`    | NOT NULL                                                                                             | メールアドレス |
| `reason`          | `TEXT`            | NULL                                                                                                 | 理由           |
| `suspended_until` | `DATETIME`        | NOT NULL                                                                                             | 停止期限       |
| `created_at`      | `DATETIME`        | NOT NULL, DEFAULT CURRENT_TIMESTAMP                                                                  |                |
| インデックス      |                   | `INDEX idx_email_suppressions_email (email)`, `INDEX idx_email_suppressions_until (suspended_until)` |                |

### `configs`: 構成設定（KV）

| フィールド名 | 型             | 属性                                                            | 詳細     |
| ------------ | -------------- | --------------------------------------------------------------- | -------- |
| `key`        | `VARCHAR(128)` | PK, NOT NULL                                                    | キー     |
| `value`      | `TEXT`         | NULL                                                            | 値       |
| `updated_at` | `TIMESTAMP`    | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP | 更新日時 |

シード（初期値）:

- `day_shift_current_index`: `'0'`
- `day_shift_current_cycle`: `''`

## 備考

- マイグレーションにより `subjects.title` は廃止され、`name_ja`/`name_en` に統合されています。
- 一部テーブル（例: `web_push_subscriptions`, `mcps`）はアプリ側の AutoMigrate や別モジュールで作成される可能性があり、このドキュメントには含まれていません。
