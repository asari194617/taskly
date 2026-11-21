# Cloudflare Email Worker による受信メールの転送

Cloudflare Email Routing で受信したメールを Taskly のサーバー側エンドポイントに転送します。認証用シークレットはリクエストヘッダーに含めます。

## デプロイ

- Cloudflare の **Email > Email Routing > Workers** で新しい Email Worker を作成します。
- スクリプトには `email-worker.js` を指定します。
- 環境変数 `EMAIL_INBOUND_ENDPOINT` に転送先（例：`https://your.server/integrations/email/inbound`）を設定します。
- `EMAIL_INTEGRATION_SECRET` にはサーバー側の同名変数と同じ値を設定します。
- Email Routing のルールで対象のメールをこの Worker に送ります。

## 転送時の処理

- RFC 822 形式のメール全体を Base64 に変換し、`raw` として送ります。DMARC XML や ZIP などの添付ファイルはサーバー側で解析します。
- Worker の処理を軽く保つため、`text` と `html` の本文は抽出しません。必要に応じてサーバーが `raw` から抽出します。
- 転送先のエンドポイントは成功時に HTTP 202 を返し、DMARC、バウンス、その他のイベントを Discord に埋め込みメッセージとして送ります。
