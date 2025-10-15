## Schemas

### users

- id: PK
- email
- display_name
- hashed_password (オプション、デフォルトはNULL)
- is_deleted
- is_day_shift_active
- created_at

### blocks

- id: PK
- user_id: FK
- reason (オプション、TEXT)
- expires_at (オプション)
- created_at
- updated_at

### notification_types

- id: PK
- type_key: UNIQUE
- description

### notification_settings

通知設定を管理します。

- id: PK
- user_id
- type_id: FK
- is_enabled
- delivery_method: ENUM
- created_at
- updated_at

### sessions

- id: PK
- client_platform
- hashed_token
- expires_at (オプション、Webのみ設定)
- created_at
- updated_at

### logs

- id: PK
- user_id: FK
- action: VARCHAR(64)
- created_at

### role_users

- id: PK
- user_id: FK
- role: “admin” | “manager”
- created_at

### permissions

- id: PK
- user_id
- target: 対象(task, dayshift, apiなど)
- method: 操作(CRUD)
- value (オプション)
- created_at
- updated_at

### subjects

- id: PK
- title
- expires_at
- created_at

### tasks

- id: PK
- added_user_id: FK
- title
- subject_id: FK
- description
- is_require_hand_in
- deadline
- created_at
- updated_at

### tasks_teams_logs

- id: PK
- task_id: FK
- teams_message_id
- teams_conversation_id
- is_success (BOOLEAN)

### api_keys

- id: PK
- user_id: FK
- hashed_token
- description (オプション)
- is_valid
- created_at
- updated_at

### api_action_logs

- id: PK
- user_id: FK
- hashed_token
- action
- ip
- created_at

### ical_keys

- id: PK
- user_id: FK
- token
- description (オプション)
- created_at

### mcps

MCPサーバーとして接続するための認証情報を管理する

- id: PK
- user_id: FK
- hashed_token
- description (オプション)
- created_at

### day_shift_assignments

- id: PK
- date
- cycle_id: FK
- status: “assigned” | “completed” | “unfulfilled”
- notes
- created_at
- updated_at

### day_shift_assignments_users

- id: PK
- day_shift_assignments_id: FK
- user_id: FK

### day_shift_cycles

- id: PK
- begin_date: サイクル開始日
- finish_date: サイクル終了日
- description: サイクルの名称。例: 2025年前期

### business_calendar

- id: PK
- date
- is_business_day
- notes
- created_at
- updated_at