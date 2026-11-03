# Schemas

この文書は主要テーブルの用途を説明する概要です。全カラム・制約・インデックスの正本は
[`apps/server/schema.sql`](../apps/server/schema.sql)、適用設定は
[`atlas.hcl`](../atlas.hcl)です。差分は [Atlas の手順](initial-settings.md#dbスキーマ管理)
で適用し、過去の `migrations/` の SQL を順に再実行しません。

Airflow メタDBはアプリDBと分離し、Airflow 自身の `db migrate` で管理します。
認証を分離した環境では、このDBのユーザーは taskly-auth の投影です。
[ユーザー同期](auth-user-projection-sync.md)と[プロフィール所有権](account-profile-ownership.md)
を参照してください。

この一覧は 2026-11-03 に `schema.sql` の全テーブル・カラム・制約と照合しました。
SQL の型・NULL・既定値・生成列・キー定義を記載します。DB変更時はこの一覧も更新してください。
`unsigned`、日時の精度、collation も含む実行可能なDDLは `schema.sql` を参照してください。

## テーブル一覧

### `api_action_logs`: API アクションログ

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `DEFAULT NULL` | 実行ユーザー |
| `hashed_token` | `varchar(255)` | `NOT NULL` | API キー（ハッシュ） |
| `action` | `varchar(64)` | `NOT NULL` | アクション |
| `ip` | `varchar(64)` | `NOT NULL` | 発信 IP |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_api_action_logs_user_id` (`user_id`)
```

### `api_keys`: API キー

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `hashed_token` | `varchar(255)` | `NOT NULL` | トークン（ハッシュ） |
| `description` | `varchar(255)` | `DEFAULT NULL` | 説明 |
| `is_valid` | `tinyint(1)` | `DEFAULT '1'` | 有効フラグ |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |
| `expires_at` | `datetime(3)` | `DEFAULT NULL` | 期限（任意） |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_api_keys_user_id` (`user_id`)
```

### `blocks`: ブロック履歴

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | 対象ユーザー |
| `reason` | `longtext` | — | 理由 |
| `expires_at` | `datetime(3)` | `DEFAULT NULL` | 期限 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_blocks_user_id` (`user_id`),
KEY `idx_blocks_user_id_expires_at` (`user_id`,`expires_at`)
```

### `configs`: 構成設定（KV）

課題タイプごとの完了操作の初期値など、アプリ設定を保存する。`system_settings` テーブルは現行スキーマに存在しない。

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `key` | `varchar(128)` | `NOT NULL` | キー |
| `value` | `text` | — | 値 |
| `updated_at` | `timestamp` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` | 更新日時 |

キー・制約:

```sql
PRIMARY KEY (`key`)
```

### `day_shift_assignment_users`: 当番担当者

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `day_shift_assignments_id` | `bigint unsigned` | `NOT NULL` | 割当 |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_asg_id` (`day_shift_assignments_id`),
KEY `idx_user_id` (`user_id`),
CONSTRAINT `fk_asg_users_asg` FOREIGN KEY (`day_shift_assignments_id`) REFERENCES `day_shift_assignments` (`id`),
CONSTRAINT `fk_asg_users_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
```

### `day_shift_assignments`: 当番割当

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `date` | `datetime(3)` | `DEFAULT NULL` | 日付 |
| `cycle_id` | `bigint unsigned` | `DEFAULT NULL` | サイクル |
| `status` | `varchar(16)` | `DEFAULT NULL` | 状態 |
| `notes` | `longtext` | — | メモ |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`)
```

### `day_shift_cycle_group_members`: 当番サイクルグループメンバー

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `group_id` | `bigint unsigned` | `NOT NULL` | グループ |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `position` | `bigint` | `DEFAULT '0'` | 位置 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_day_shift_cycle_group_members_group_id` (`group_id`)
```

### `day_shift_cycle_groups`: 当番サイクルグループ

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `cycle_id` | `bigint unsigned` | `NOT NULL` | サイクル |
| `position` | `bigint` | `DEFAULT '0'` | 位置 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_day_shift_cycle_groups_cycle_id` (`cycle_id`)
```

### `day_shift_cycles`: 当番サイクル

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `begin_date` | `datetime(3)` | `DEFAULT NULL` | 開始日 |
| `finish_date` | `datetime(3)` | `DEFAULT NULL` | 終了日 |
| `description` | `varchar(255)` | `DEFAULT NULL` | 説明 |

キー・制約:

```sql
PRIMARY KEY (`id`)
```

### `email_suppressions`: メール一時停止

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `email` | `varchar(255)` | `NOT NULL` | メールアドレス |
| `reason` | `varchar(255)` | `DEFAULT NULL` | 理由 |
| `suspended_until` | `datetime(3)` | `NOT NULL` | 停止期限 |
| `created_at` | `datetime(3)` | `DEFAULT NULL` |  |
| `updated_at` | `datetime(3)` | `DEFAULT NULL` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_email_suppressions_email` (`email`),
KEY `idx_email_suppressions_until` (`suspended_until`)
```

### `flagged_contents`: モデレーション検知

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `task_id` | `bigint unsigned` | `NOT NULL` | 対象タスク |
| `category` | `varchar(64)` | `NOT NULL` | カテゴリ |
| `severity` | `varchar(16)` | `NOT NULL` | 深刻度 |
| `score` | `double` | `DEFAULT NULL` | スコア |
| `threat` | `tinyint(1)` | `NOT NULL DEFAULT '0'` | 脅威判定 |
| `reason` | `text` | — | 理由 |
| `model` | `varchar(64)` | `DEFAULT NULL` | モデル名 |
| `metadata` | `json` | `DEFAULT NULL` | 付帯情報 |
| `snapshot` | `json` | `DEFAULT NULL` | 対象スナップショット |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_flagged_contents_task_id` (`task_id`)
```

### `ical_keys`: iCal キー

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `token` | `varchar(255)` | `NOT NULL` | アクセストークン |
| `description` | `varchar(255)` | `DEFAULT NULL` | 説明 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `expires_at` | `datetime(3)` | `DEFAULT NULL` | 期限 |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `idx_ical_keys_token` (`token`),
KEY `idx_ical_keys_user_id` (`user_id`)
```

### `integration_jobs`: 外部連携の永続アウトボックス

ジョブの種類、lease、再試行、重複配送は [連携ジョブ](integration-jobs.md)を参照。

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `kind` | `varchar(32)` | `NOT NULL` |  |
| `payload` | `json` | `NOT NULL` |  |
| `status` | `varchar(16)` | `NOT NULL DEFAULT 'pending'` |  |
| `attempts` | `int` | `NOT NULL DEFAULT '0'` |  |
| `available_at` | `datetime(6)` | `NOT NULL DEFAULT CURRENT_TIMESTAMP(6)` |  |
| `lease_until` | `datetime(6)` | `DEFAULT NULL` |  |
| `dedup_key` | `varchar(255)` | `DEFAULT NULL` |  |
| `last_error` | `text` | — |  |
| `created_at` | `datetime(6)` | `NOT NULL DEFAULT CURRENT_TIMESTAMP(6)` |  |
| `updated_at` | `datetime(6)` | `NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `uniq_integration_dedup` (`dedup_key`),
KEY `idx_integration_claim` (`status`,`available_at`,`lease_until`),
KEY `idx_integration_kind_claim` (`kind`,`status`,`available_at`,`lease_until`)
```

### `login_codes`: ログインコード（メール）

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `email` | `varchar(255)` | `NOT NULL` | メールアドレス |
| `hashed_code` | `varchar(255)` | `NOT NULL` | コード（ハッシュ） |
| `expires_at` | `datetime(3)` | `NOT NULL` | 期限 |
| `consumed_at` | `datetime(3)` | `DEFAULT NULL` | 消費日時 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `failed_attempts` | `bigint` | `NOT NULL DEFAULT '0'` | 失敗回数 |
| `locked_until` | `datetime(3)` | `DEFAULT NULL` | ロック期限 |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_login_codes_email` (`email`),
KEY `idx_login_codes_expires_at` (`expires_at`)
```

### `logs`: 操作ログ

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `DEFAULT NULL` | 実行ユーザー |
| `action` | `varchar(64)` | `NOT NULL` | アクション |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_logs_user_id` (`user_id`)
```

### `mcps`: MCP 統合トークン

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `DEFAULT NULL` | ユーザー |
| `hashed_token` | `varchar(255)` | `DEFAULT NULL` | トークン |
| `description` | `longtext` | — | 説明 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `expires_at` | `datetime(3)` | `DEFAULT NULL` | 期限 |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_mcps_user_id` (`user_id`)
```

### `migrations`: 旧マイグレーション履歴

Atlas 導入前の適用履歴を監査用に保持する。新規変更の適用状態には使用しない。

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `name` | `varchar(255)` | `NOT NULL` | 名前 |
| `applied_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` | 適用日時 |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `idx_migrations_name` (`name`)
```

### `notification_deliveries`: 日直通知の配信状態

配信状態と再送の扱いは [通知仕様](notifications.md)を参照。

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `delivery_key` | `varchar(191)` | `NOT NULL` |  |
| `channel` | `varchar(32)` | `NOT NULL` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` |  |
| `status` | `enum('sending','sent','error')` | `NOT NULL` |  |
| `attempts` | `int unsigned` | `NOT NULL DEFAULT '0'` |  |
| `error_message` | `text` | — |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |
| `sent_at` | `datetime` | `DEFAULT NULL` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `uniq_delivery_key` (`delivery_key`),
KEY `idx_delivery_status` (`status`,`updated_at`)
```

### `notification_devices`: 通知デバイス

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `name` | `varchar(255)` | `DEFAULT NULL` | 名称 |
| `platform` | `enum('EMAIL','WEBPUSH','IOS','ANDROID')` | `CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL` | プラットフォーム |
| `payload` | `json` | `DEFAULT NULL` | 端末情報等 |
| `is_enabled` | `tinyint(1)` | `NOT NULL DEFAULT '1'` | 有効か |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_notification_devices_user_id` (`user_id`)
```

### `notification_type_prefs`: 通知タイプ設定（ユーザー別）

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `notification_type` | `enum('TASK','TASK_MORNING','TASK_EVENING','DAYSHIFT')` | `NOT NULL` | 通知対象 |
| `is_enabled` | `tinyint(1)` | `NOT NULL DEFAULT '1'` | 有効か |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_notification_type_prefs_user_id` (`user_id`)
```

### `notifications_queue`: 通知キュー

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `kind` | `varchar(32)` | `NOT NULL` | 種別 |
| `when_label` | `varchar(32)` | `NOT NULL` | タイミングラベル |
| `subject` | `varchar(255)` | `NOT NULL` | 件名 |
| `title` | `text` | — | タイトル |
| `body` | `text` | — | 本文 |
| `user_id` | `bigint unsigned` | `DEFAULT NULL` | 対象ユーザー |
| `channel` | `varchar(32)` | `DEFAULT NULL` | 送信チャネル |
| `status` | `varchar(32)` | `DEFAULT NULL` | ステータス |
| `error_message` | `text` | — | エラー内容 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `processed_at` | `datetime` | `DEFAULT NULL` | 処理日時 |
| `event_key` | `varchar(191)` | `DEFAULT NULL` |  |
| `slot` | `varchar(16)` | `DEFAULT NULL` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `uniq_notification_event` (`event_key`),
KEY `idx_processed_at` (`processed_at`),
KEY `idx_notification_pending` (`channel`,`status`,`created_at`)
```

### `passkeys`: WebAuthn パスキー

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `credential_id` | `varchar(255)` | `NOT NULL` | クレデンシャル ID |
| `public_key` | `text` | `NOT NULL` | 公開鍵 |
| `sign_count` | `bigint` | `NOT NULL` | 署名回数 |
| `device_name` | `varchar(255)` | `DEFAULT NULL` | 端末名 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `idx_passkeys_credential_id` (`credential_id`),
KEY `idx_passkeys_user_id` (`user_id`)
```

### `permissions`: 任意権限ルール

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `target` | `varchar(64)` | `NOT NULL` | 対象リソース |
| `method` | `varchar(16)` | `NOT NULL` | 操作/メソッド |
| `value` | `varchar(255)` | `DEFAULT NULL` | 値（オプション） |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_permissions_user_id` (`user_id`),
KEY `idx_permissions_user_target_method` (`user_id`,`target`,`method`)
```

### `recovery_codes`: リカバリーコード

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `hashed_code` | `varchar(255)` | `NOT NULL` | ハッシュ化されたコード |
| `is_used` | `tinyint(1)` | `DEFAULT '0'` | 使用済みフラグ |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_recovery_codes_user_id` (`user_id`)
```

### `role_users`: 役割付与

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `role` | `enum('admin','manager')` | `CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL` | 役割 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_role_users_user_id` (`user_id`),
KEY `idx_role_users_user_id_role` (`user_id`,`role`)
```

### `sessions`: セッション

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | 所有ユーザー |
| `client_platform` | `varchar(64)` | `NOT NULL` | クライアント種別 |
| `hashed_token` | `varchar(255)` | `NOT NULL` | トークン（ハッシュ） |
| `expires_at` | `datetime(3)` | `DEFAULT NULL` | 期限 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_sessions_user_id` (`user_id`),
KEY `idx_sessions_user` (`user_id`),
KEY `idx_sessions_hashed_token` (`hashed_token`)
```

### `subjects`: 科目

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` | 科目 ID |
| `name_ja` | `varchar(255)` | `NOT NULL` | 科目名（日本語） |
| `name_en` | `varchar(255)` | `DEFAULT NULL` | 科目名（英語） |
| `expires_at` | `datetime(3)` | `DEFAULT NULL` | 期限（任意） |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` | 作成日時 |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_subjects_expires_at_id` (`expires_at`,`id`)
```

### `tasks`: 課題・タスク

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` | タスク ID |
| `added_user_id` | `bigint unsigned` | `NOT NULL` | 追加したユーザー |
| `title` | `varchar(255)` | `NOT NULL` | タイトル |
| `subject_id` | `bigint unsigned` | `NOT NULL` | 科目 ID |
| `description` | `text` | — | 詳細説明 |
| `image_key` | `varchar(128)` | `DEFAULT NULL` |  |
| `type` | `enum('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER')` | `NOT NULL DEFAULT 'OTHER'` | 課題種別 |
| `is_require_hand_in` | `tinyint(1)` | `NOT NULL DEFAULT '0'` | 提出必須か |
| `is_self_check_required` | `tinyint(1)` | `NOT NULL DEFAULT '0'` | 自己確認必須か |
| `is_completion_disabled` | `tinyint(1)` | `NOT NULL DEFAULT '0'` | 完了マークを無効にする（課題ごとに保存） |
| `deadline` | `date` | `NOT NULL` | 期日 |
| `deadline_since` | `enum('NONE','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10')` | `NOT NULL DEFAULT 'NONE'` | 期日の開始コマ |
| `deadline_until` | `enum('NONE','MORNING','LUNCH','AFTER_SCHOOL','P1','P2','P3','P4','P5','P6','P7','P8','P9','P10')` | `NOT NULL DEFAULT 'NONE'` | 期日の終了帯/コマ |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` | 作成日時 |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` | 更新日時 |
| `is_hidden` | `tinyint(1)` | `NOT NULL DEFAULT '0'` | モデレーションにより非表示か |
| `effective_deadline` | `date` | `` GENERATED ALWAYS AS (`deadline`) STORED `` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `idx_tasks_image_key` (`image_key`),
KEY `idx_tasks_subject_id` (`subject_id`),
KEY `idx_tasks_added_user_id` (`added_user_id`),
KEY `idx_tasks_deadline_id` (`deadline`,`id`),
KEY `idx_tasks_effective_deadline_id` (`effective_deadline`,`id`),
FULLTEXT KEY `ft_tasks_title_desc` (`title`,`description`)
```

### `task_images`: 課題画像と表示順

非公開 R2 上の画像との対応・削除・権限は [課題画像](features/task-images.md)を参照。

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `task_id` | `bigint unsigned` | `NOT NULL` |  |
| `image_key` | `varchar(128)` | `NOT NULL` |  |
| `position` | `smallint unsigned` | `NOT NULL` |  |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `idx_task_images_image_key` (`image_key`),
UNIQUE KEY `idx_task_images_task_position` (`task_id`,`position`),
KEY `idx_task_images_task_id` (`task_id`),
CONSTRAINT `fk_task_images_task` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON DELETE CASCADE
```

### `tasks_teams_logs`: Teams 送信ログ

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `task_id` | `bigint unsigned` | `NOT NULL` | 対象タスク |
| `teams_message_id` | `varchar(255)` | `DEFAULT NULL` | メッセージ ID |
| `teams_conversation_id` | `varchar(255)` | `DEFAULT NULL` | 会話 ID |
| `is_success` | `tinyint(1)` | `NOT NULL DEFAULT '0'` | 成否 |
| `callback_url` | `text` | — | コールバック URL |
| `attempts` | `int` | `NOT NULL DEFAULT '0'` | 試行回数 |
| `last_status` | `varchar(32)` | `DEFAULT NULL` | 最終ステータス |
| `last_error` | `text` | — | 最終エラー |
| `last_response_status` | `int` | `DEFAULT NULL` | 応答コード |
| `last_response_body` | `text` | — | 応答本文 |
| `inbound_job_id` | `bigint unsigned` | `DEFAULT NULL` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `uniq_teams_inbound_job` (`inbound_job_id`),
KEY `idx_tasks_teams_logs_task_id` (`task_id`)
```

### `tasks_users_completions`: ユーザー完了状態

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `task_id` | `bigint` | `NOT NULL` | タスク |
| `completed_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` | 完了日時 |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `uniq_user_task` (`user_id`,`task_id`),
KEY `idx_task_user` (`task_id`,`user_id`)
```

### `totp_secrets`: TOTP 秘密鍵

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー（1 ユーザー 1 レコード） |
| `secret` | `varchar(255)` | `NOT NULL` | 秘密鍵 |
| `is_active` | `tinyint(1)` | `DEFAULT '0'` | 有効化状態 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `idx_totp_secrets_user_id` (`user_id`)
```

### `user_reports`: ユーザー通報

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `reporter_user_id` | `bigint unsigned` | `DEFAULT NULL` | 通報者 |
| `task_id` | `bigint unsigned` | `NOT NULL` | 対象タスク |
| `reason` | `varchar(255)` | `NOT NULL` | 理由 |
| `comment` | `text` | — | コメント |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_user_reports_reporter_user_id` (`reporter_user_id`),
KEY `idx_user_reports_task_id` (`task_id`)
```

### `users`: ユーザー情報

認証を分離した環境のユーザー同期は [投影の同期](auth-user-projection-sync.md)を参照。

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` | ユーザー ID |
| `email` | `varchar(255)` | `NOT NULL` | メールアドレス |
| `display_name` | `varchar(255)` | `DEFAULT NULL` | 表示名 |
| `hashed_password` | `varchar(255)` | `DEFAULT NULL` | パスワードハッシュ（任意） |
| `email_code_allowed` | `tinyint(1)` | `DEFAULT '1'` | メールコードログインの許可 |
| `is_deleted` | `tinyint(1)` | `DEFAULT '0'` | 退会/削除フラグ |
| `notification_lang` | `varchar(5)` | `NOT NULL DEFAULT 'ja'` | 通知の言語 |
| `is_registered` | `tinyint(1)` | `DEFAULT '0'` | 登録完了フラグ |
| `is_agreed_policies` | `tinyint(1)` | `DEFAULT '0'` | 旧 API 用互換フラグ。現在の同意判定には使用しない |
| `created_at` | `datetime(3)` | `DEFAULT CURRENT_TIMESTAMP(3)` | 作成日時 |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |
| `display_lang` | `varchar(5)` | `CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT 'ja'` | UI 表示言語 |
| `deleted_at` | `datetime` | `DEFAULT NULL` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
UNIQUE KEY `idx_users_email` (`email`),
KEY `idx_users_erasure_due` (`is_deleted`,`deleted_at`)
```

### `web_push_subscriptions`: Web Push サブスクリプション

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `user_id` | `bigint unsigned` | `NOT NULL` | ユーザー |
| `endpoint` | `varchar(1024)` | `NOT NULL` | エンドポイント |
| `p256dh` | `varchar(1024)` | `NOT NULL` | P256dh |
| `auth` | `varchar(255)` | `NOT NULL` | Auth |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |
| `updated_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_web_push_subscriptions_user_id` (`user_id`)
```

### `web_push_vapid`: Web Push VAPID

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` |  |
| `public_key` | `varchar(255)` | `NOT NULL` | 公開鍵 |
| `private_key` | `varchar(255)` | `NOT NULL` | 秘密鍵 |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`id`)
```

### `account_erasure_holds`: アカウント消去の保留

保留理由と再確認期限を記録する。[消去運用](operations/account-data.md)を参照。

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `user_id` | `bigint unsigned` | `NOT NULL` |  |
| `reason` | `varchar(255)` | `NOT NULL` |  |
| `review_at` | `datetime` | `NOT NULL` |  |
| `created_at` | `datetime` | `NOT NULL DEFAULT CURRENT_TIMESTAMP` |  |

キー・制約:

```sql
PRIMARY KEY (`user_id`),
CONSTRAINT `account_erasure_holds_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
```

### `policy_consents`: 文書同意イベント

保存版の文書、同意判定、保持期間は [文書同意の運用](legal/README.md)を参照。

| カラム | SQL 型 | 属性・既定値 | 用途 |
| --- | --- | --- | --- |
| `id` | `bigint unsigned` | `NOT NULL AUTO_INCREMENT` | PK |
| `user_id` | `bigint unsigned` | `NOT NULL` | 利用者 ID、`users.id` 参照 |
| `terms_version` | `varchar(32)` | `NOT NULL` | 利用規約の保存版 |
| `privacy_version` | `varchar(32)` | `NOT NULL` | プライバシーポリシーの保存版 |
| `security_version` | `varchar(32)` | `NOT NULL` | セキュリティポリシーの保存版 |
| `display_language` | `varchar(5)` | `NOT NULL` | 同意画面の表示言語 |
| `accepted_at` | `datetime(6)` | `NOT NULL` | 同意日時（JST。API の MySQL 接続設定に準拠） |

キー・制約:

```sql
PRIMARY KEY (`id`),
KEY `idx_policy_consents_user_time` (`user_id`,`accepted_at`),
CONSTRAINT `fk_policy_consents_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
```
