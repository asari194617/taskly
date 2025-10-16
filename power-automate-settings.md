# Power Automate 連携設定ガイド

本書は Taskly と Power Automate の連携（Teams 投稿・リプライ、日直の「ずらし」）を簡単に構成できるよう、入出力の仕様とメール/HTTP のペイロードをまとめたものです。

対象のフロー:

- 課題追加（Taskly → メール → Power Automate → Teams 投稿）
- リプライ（Taskly → メール → Power Automate → Teams 返信）
- 日直の「ずらし」（Power Automate → HTTP → Taskly）

> すべての HTTP 連携は共通でシークレットヘッダーを要求します。
> ヘッダー名: `x-teams-integration-secret`
> 値: `.env` の `TEAMS_INTEGRATION_SECRET`

---

## 1) 課題追加フロー

Taskly が課題作成時に Power Automate 宛てにメールを送信し、フロー側で Teams に新規メッセージを投稿します。

- 送信元: SMTP (`SMTP_FROM`)
- 宛先: `TEAMS_NOTIFY_EMAIL`
- 件名: `[Taskly]tasks:add`
- 本文: JSON（text/plain）

本文(JSON)の例:

```
{"task_id":"123","title":"教科名: タイトル(10/16 5限まで)","description":"説明テキスト(あれば)"}
```

フィールド:

- `task_id`: 数値文字列 (課題 ID)
- `title`: Teams に投稿する見出し用の整形済みタイトル
  - 形式: `科目名: タイトル(期日ラベル)`
- `description`: 説明テキスト（任意、無い場合は空文字または省略可）

Power Automate 側の推奨処理:

1. トリガー: メール受信（件名が `[Taskly]tasks:add` を含む）
2. 本文(JSON)を parse
3. Teams に `title` と `description` でメッセージを投稿
4. 投稿結果の `messageId` と `conversationId` を取得
5. Taskly のコールバックに `task_id` と `message_id` を登録（スレッド返信に必要）

コールバック HTTP リクエスト:

- メソッド: POST
- URL: `${API_PUBLIC_URL}/integrations/teams/callback`
- ヘッダー:
  - `Content-Type: application/json`
  - `x-teams-integration-secret: <TEAMS_INTEGRATION_SECRET>`
- ボディ(JSON):

```
{
  "is_success": true,
  "task_id": 123,
  "message_id": "<TeamsMessageId>",
  "conversation_id": "<ConversationId>"
}
```

- 成功時ステータス: 200 OK

補助エンドポイント（ログ用途）:

- `${API_PUBLIC_URL}/integrations/teams/status` も同スキーマで受け付けます（効果は callback と同じで `tasks_teams_logs` に記録）。

---

## 2) リプライフロー（更新/削除）

Taskly が課題の更新・削除時に Power Automate 宛てにメールを送信し、フロー側で既存メッセージにスレッド返信します。

- 送信元: SMTP (`SMTP_FROM`)
- 宛先: `TEAMS_NOTIFY_EMAIL`
- 件名: `[Taskly]tasks:reply`
- 本文: JSON（text/plain）

本文(JSON)の例（更新時）:

```
{
  "message_id": "<TeamsMessageId>",
  "task_id": "123",
  "message": "課題が更新されました (ID:123)\n・タイトル: '旧' → '新'\n・教科: '数学' → '物理'\n・説明: '（未設定）' → '追加'\n・種別: 'オンライン' → '対面'\n・要提出: いいえ → はい\n・自己チェック: はい → いいえ\n・期限日: 2025-10-15 → 2025-10-20\n・開始限: 指定なし → 2限\n・終了限: 3限まで → 5限まで"
}
```

本文(JSON)の例（削除時）:

```
{
  "message_id": "<TeamsMessageId>",
  "task_id": "123",
  "message": "課題が削除されました: タイトル (ID:123)"
}
```

フィールド:

- `message_id`: Teams の元メッセージ ID（課題追加時の callback で Taskly に登録が必要）
- `task_id`: 数値文字列 (課題 ID)
- `message`: 日本語の本文。更新時は変更点をすべて列挙。削除時は削除を通知。

Power Automate 側の推奨処理:

1. トリガー: メール受信（件名が `[Taskly]tasks:reply` を含む）
2. 本文(JSON)を parse
3. `message_id` のメッセージに対して `message` をスレッド返信

---

## 3) 日直の「ずらし」フロー

Power Automate が Taskly の HTTP エンドポイントへ POST して、日直を「未充足」にし、翌営業日にローテーションを実施します。

HTTP リクエスト:

- メソッド: POST
- URL: `${API_PUBLIC_URL}/integrations/dayshift/inbound`
- ヘッダー:
  - `Content-Type: application/json`
  - `x-teams-integration-secret: <TEAMS_INTEGRATION_SECRET>`
- ボディ(JSON):

```
{
  "subject": "[メールの件名]",
  "body": "[メールの本文]"
}
```

- 条件: `subject` または `body` に「日直」が含まれない場合、400 を返します。
- 判定: `shouldRotateByGemini(subject, body)` により「ずらし」判定（環境により Gemini か簡易キーワード）
- 成功時レスポンス:
  - `202 Accepted` + `{ "ok": true, "rotated": true|false }`

内部動作（成功時）:

- 当日の日直 `status` を `unfulfilled` に設定（無い場合は当日分を作成）
- 翌営業日に向けて担当者をシフト（`performDayShiftRotation`）
- Discord Webhook（`DISCORD_WEBHOOK_URL`）が設定されていれば通知

---

## 4) 補足: 認証・セキュリティ

- すべての `/integrations/teams/*` と `/integrations/dayshift/inbound` は共通のヘッダー認証を要求します。
  - `x-teams-integration-secret: <TEAMS_INTEGRATION_SECRET>`
- 送信元メールの認証は SMTP 側の設定に依存します。
- Taskly → Power Automate のメール本文はすべて JSON（text/plain）です。Power Automate では「本文を JSON として解析」アクションを使用してください。

---

## 5) 変数一覧（.env）

- `API_PUBLIC_URL`: 例 `http://localhost:3063`
- `TEAMS_INTEGRATION_SECRET`: 共有シークレット（必須）
- `TEAMS_NOTIFY_EMAIL`: Taskly から送るメールの宛先（Power Automate 側で受信するメールアドレス）
- `SMTP_*`: 送信メール設定（ホスト/ポート/ユーザー/パスワード/TLS/タイムアウト）
- `DISCORD_WEBHOOK_URL`: 日直ローテーションの通知用（任意）

---

## 6) テスト用サンプル

- コールバック登録（curl）:

```
curl -X POST "$API_PUBLIC_URL/integrations/teams/callback" \
  -H "Content-Type: application/json" \
  -H "x-teams-integration-secret: $TEAMS_INTEGRATION_SECRET" \
  -d '{"task_id":123,"message_id":"<TeamsMessageId>","conversation_id":"<ConversationId>","is_success":true}'
```

- 日直ずらし（curl）:

```
curl -X POST "$API_PUBLIC_URL/integrations/dayshift/inbound" \
  -H "Content-Type: application/json" \
  -H "x-teams-integration-secret: $TEAMS_INTEGRATION_SECRET" \
  -d '{"subject":"日直について","body":"本日の日直を延期したいです"}'
```

---

## 7) よくある注意点

- スレッド返信のために、課題作成時の Teams 投稿で得た `message_id` を必ず Taskly の `/integrations/teams/callback` に登録してください。
- 課題の更新/削除で送られるメッセージ本文は日本語固定です。Power Automate 側ではそのまま本文として返信してください。
- `tasks_teams_logs` テーブルに処理ログが保存されます。トラブル時はここを参照してください。
