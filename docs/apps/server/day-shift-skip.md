- メソッド: POST
- URL: `${API_PUBLIC_URL}/integrations/dayshift/inbound`
- ヘッダー:
    - `Content-Type: application/json`
    - `x-teams-integration-secret: &lt;TEAMS_INTEGRATION_SECRET&gt;`
- ボディ（JSON）:

```json
{
  "subject": "[メールの件名]",
  "body": "[メールの本文]"
}
```

- 条件: `subject` または `body` に「日直」が含まれない場合、400 を返します。
- 判定: `ShouldRotateByGemini(subject, body)` により「ずらし」判定（環境により Gemini か簡易キーワード）
- 成功時レスポンス: `202 Accepted` + `{ "ok": true, "rotated": true|false }`

## 4) 補足: 認証・セキュリティ

すべての `/integrations/teams/*` と `/integrations/dayshift/inbound` は共通のヘッダー認証が必要です。

- `x-teams-integration-secret: &lt;TEAMS_INTEGRATION_SECRET&gt;`

送信元メールの認証は SMTP 側の設定に依存します。
Taskly から Power Automate へのメール本文はすべて JSON（text/plain）形式です。Power Automate では「本文を JSON として解析」アクションを使用してください。