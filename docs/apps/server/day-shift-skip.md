# Teams と日直の連携

Taskly と Power Automate は、課題の投稿・取り込みと日直メールの転送を行います。以下の API パスは `API_PUBLIC_URL` を基準とします。Taskly が受ける各 POST リクエストは `Content-Type: application/json` と `X-Teams-Integration-Secret`（環境変数 `TEAMS_INTEGRATION_SECRET` の値）を使用します。

## Taskly の課題を Teams に投稿する

1. Taskly は `TEAMS_NOTIFY_EMAIL` に、件名 `[Taskly]tasks:add` のメールを送信します。テキスト本文は JSON で、`task_id`（文字列）、`title`、`description`、`sent_at` を含みます。
2. Power Automate は本文を JSON として読み取り、チャットまたはチャネルに投稿します。
3. 投稿結果を `POST /integrations/teams/callback` に返します。`task_id` は課題 ID、`message_id` と `conversation_id` は Teams の投稿結果です。

   ```json
   {
     "is_success": true,
     "task_id": "123",
     "message_id": "teams-message-id",
     "conversation_id": "teams-conversation-id"
   }
   ```

Taskly は `task_id` に対応する結果を記録し、成功時は HTTP 200、DB への保存失敗時は HTTP 503 を返します。課題を編集・削除したときの返信メールは、件名が `[Taskly]tasks:reply` で、本文に `message_id`、`task_id`、`message` を含みます。元の投稿 ID を受け取るまでの待機と再試行は[連携ジョブ](../../integration-jobs.md)を参照してください。

## Teams から課題を取り込む

`POST /integrations/teams/inbound` は `email`（登録済みユーザーのメールアドレス）、`title`、`subject_id`（教科 ID）を必須とします。`description`、`deadline`（`YYYY-MM-DD`、省略時は受信日）、`status_callback` は任意です。任意の `Idempotency-Key` ヘッダーで再送時の重複を防げます。

Taskly はジョブを保存してから HTTP 202 と `job_id`、`status` を返し、非同期で課題を作成します。許可された `status_callback` が指定されていれば、作成後に Taskly からその URL へ `is_success` と `task_id` を含む JSON を POST します。許可する HTTPS オリジンは `TEAMS_CALLBACK_ALLOWED_ORIGINS` に設定します。

## 日直メールの受信

Power Automate は対象メールの件名と本文を `POST /integrations/dayshift/inbound` に転送します。

```json
{
  "subject": "日直について",
  "body": "明日の日直を延期します。"
}
```

件名と本文のいずれにも「日直」が含まれない場合は HTTP 400 を返します。Taskly はキーワードで延期を判定し、該当しない場合は設定済みの Gemini でも判定します。

受理時は HTTP 202 と `ok: true`、`rotated` を返します。延期と判定した場合は `rotated: true` と `mode` を返します。`tomorrow` では次の日直実施日の担当順更新をスキップするフラグを設定し、`today` では現在の担当インデックスを 1 つ戻して（下限は 0）、関係するグループへメール通知を試みます。
