# Taskly

# Overview

A simple, fast class management service.

## Platforms

- Web (PWA対応)
- iOS
- Android
- Mac
- Windows
- API
- iCal (カレンダーとして購読可能にする)
- MCP(Model Context Protocol)サーバー
- Teams (特定のメールアドレスに送信することでPower Automateが実行)
    - タイトルは`[Taskly]tasks`としてください。
    - 本文は下記のようにしてください。タイトルの形式は`教科名: **実験1のレポート提出(○/△ ○○時まで)**`のような形にしてください。descriptionにはtasksテーブルでいう`description`や提出の有無を記載してください。
    
    ```json
    {
      "title": "タイトル",
      "description": "説明"
    }
    ```
    
    - 処理には時間がかかるので、タスク追加時に非同期処理として実装してください。
    - 処理が終了すると、任意のエンドポイントに
    
    ```json
    {
      "is_success": true,
      "task_id": "@{body('Parse_JSON')?['task_id']}",
      "message_id": "@{outputs('Post_message_in_a_chat_or_channel')?['body/id']}",
      "conversation_id": "@{outputs('Post_message_in_a_chat_or_channel')?['body/conversationId']}"
    }
    ```
    
    というbodyを含むPOSTリクエストを送信できます。このエンドポイントのURLも作成してください。
    

## Languages

- Japanese
- English

## Themes

- Dark
- Light (Default)

# Features

- 課題の追加・編集・削除・通知・検索
- 日直の通知
- (ユーザー画面)APIキーの管理
- ブロックリスト
- アカウントシステム
    - ログイン
    - ログアウト
    - パスワードの設定(デフォルトはメールに送られる6桁の番号で認証)
    - 表示名の設定(デフォルトはメールアドレス)
    - アカウント削除 (レコードの削除ではなく`is_deleted`フラグで対応)
    - 2段階認証の設定
        - パスキー
        - TOTP
        - メール(パスワード有効時のみ切り替え可能に)
        - リカバリーコード
    - セッションの管理
    - アカウントのログ閲覧

## Way to notification

- Email
- Web Push
- Notify to Teams (via Email, Build with Power-Automate)
- Push notification from native-app

# Technical Details

## Tech stacks

- Backend Application server and Batch transactions: Go (Fiber)
- Front: Astro (Shadcnを全面利用, CloudFlare Pagesで動作させる)
- iOS/iPad: Swift
- Android: Kotlin
- Mac: Swift
- Windows: WinUI 3
- CDN: CloudFlare R2 (S3のクライアントを使って管理)
- 明日もやるかどうかの判定: Gemini’s API (毎回文面が異なるため)
- CI/CD: GitHub Actions (アプリケーションサーバーのみ)
- DB: MySQL
- メール生成: React Email
- Metrics: Prometheus (アプリケーションサーバー側にローカルからのみアクセスできる`/metrics`エンドポイントを用意してください)
- (APIとiCalとMCPとTeamsを除く)各プラットフォームとアプリケーションサーバーのやりとり: GraphQL
- APIプラットフォーム: REST API

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

## Pages

- 課題一覧(トップ)
- 期限を過ぎた課題(無限スクロールを実装)
- 設定

### Note

- それぞれの課題は期限が近い順に日付ごとにトップページに表示し、過ぎた課題を表示する必要はありません。

## Attention

- 明日も行うことを通知する文面は毎回異なるため、GeminiのAPIに投げて判定してもらってください。
- メールを判定する際、Gemini APIが判定に失敗した場合や、メールが届かなかった場合はDiscordのWebhookにembedsで通知してください。
- 手動でも未履行扱いを行えるようにしてください。
- 正規表現で「日直」が入ったメールをPower Automateで抽出し、APIを叩かせます。このためのエンドポイントも作成してください。
- 通知の送信用にPOSTFIXのメールサーバーも作ってください。
- POSTFIXのメールサーバーはDockerコンテナにしてください。DNS側で設定が必要だと思いますので、必要な項目も教えてください。
- モノレポで構築してください。
- 正常系だけではなく異常系の動作も実装してください。
- 権限管理のために`<Permission>`コンポーネントを作成して利用してください。
- 2段階認証については、以下のような形でテーブルを作ってください(必要があれば適宜変えてもいいです)。パスキーは複数登録できるようにしてください。
    
    ```sql
    -- パスキー (WebAuthn)
    CREATE TABLE passkeys (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        user_id BIGINT NOT NULL,
        credential_id VARCHAR(255) UNIQUE NOT NULL, -- クレデンシャルID
        public_key TEXT NOT NULL,
        sign_count BIGINT NOT NULL,
        device_name VARCHAR(255), -- ユーザーが識別するための名前
        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        FOREIGN KEY (user_id) REFERENCES users(id)
    );
    
    -- TOTP (Authenticator App)
    CREATE TABLE totp_secrets (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        user_id BIGINT NOT NULL UNIQUE, -- 通常ユーザー1人につき1つ
        secret VARCHAR(255) NOT NULL, -- 暗号化して保存
        is_active BOOLEAN NOT NULL DEFAULT FALSE,
        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (user_id) REFERENCES users(id)
    );
    
    -- リカバリーコード
    CREATE TABLE recovery_codes (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        user_id BIGINT NOT NULL,
        hashed_code VARCHAR(255) NOT NULL, -- ハッシュ化して保存
        is_used BOOLEAN NOT NULL DEFAULT FALSE,
        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (user_id) REFERENCES users(id)
    );
    ```
    
- 高速にしたいです。
- それぞれのプラットフォーム、デザインシステムにネイティブでベストな実装を目指してください。(バッチで課題件数を表示するなど)
- 通常、ユーザーは有効なAPIキーを10個まで発行できます(上限に達しても、キーを無効にすれば新しいキーを発行できます)。adminの場合は制限なく利用できます。managerの場合は100個まで利用できます。
- ユーザーのAPIキーの上限は、adminまたはmanagerがpermissionsにtargetを`api_key_limit`を設定された場合にはvalueの値の個数まで作成できます。
- APIドキュメントをOpenAPIのYAML形式で作成してください。
- 日直の通知は毎日8:30に行ってください。
- Redisにセッション情報を記録して、高速化してください。
- 管理画面も作成し(Webのみ)、adminとmanagerがアクセスできるようにしてください。
- 管理画面でのカレンダー管理で、日付範囲を指定したイベントの追加ができるようにしてください。
- 日直のループ終了が近くなったら、バッチ処理でday_shift_assignmentsテーブルに次のサイクルを自動的に追加してください。このバッチ処理は毎日実行し、終了が近いか確認させてください。ただしday_shift_cyclesテーブルにバッチ処理時点で有効期間内のサイクルがない場合は、この処理を実施せず、DiscordのWebhookのembedsでエラーを出せるようにしてください。
- すべてのユーザーは通常、課題を追加・編集できます。adminまたはmanagerは削除ができます。
- 基本的な権限は「admin」「manager」「user」ロールに紐付け、`permissions`テーブルは「特定のユーザーに例外的な権限を与える」ためのオーバーライド用として定義します。
- icalのURLやAPIキーは設定から発行できるようにしてください。
- iCalのURLを発行する場合は、URLクエリにtokenを入れたものを発行してください。
- アプリケーションサーバーとフロントはサブドメインを分けるため、適切なCORSを設定してください。
- それぞれのサービスは最終的にDockerコンテナにしたいです。

## 管理画面

主にこれらの機能を入れて欲しいです。

- ユーザー一覧・検索・編集機能
- ユーザーのブロック/ブロック解除
- ロール（admin/manager）の付与・剥奪
- 権限(permissions)の設定
- カレンダーの編集（日付範囲指定での休日/営業日設定）
- 日直サイクルの管理
- システム全体のログ閲覧

# Notes

## 全体

- ブロックリストに入っているユーザーはすべての機能を利用できません。ログイン時に確認してください。
- GitHub Actionsを使用して、アプリケーションサーバーとMCPサーバー用(これらを一つにまとめても構いません)のCI/CDパイプラインを構築してください。

## 日直について

- 原則、平日に二人ずつ割り当て(ペアは一緒でサイクル)
- 未履行の場合は次の日も継続される
- 未履行である場合、特定の送信者からタイトルまたは本文に「日直」の文字列を含むメールが届く
- 未履行であることを知らせるメールは毎回文面が異なります。
- 臨時休校などのために、日直をずらす操作が発生することがあります。
- 日直のメンバーは一定期間ごとに変更されます。
- 当該ユーザーに対してメールで通知します。