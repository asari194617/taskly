# Cloudflare Email Worker による受信メールの転送

Cloudflare Email Routing で受信したメールを Taskly のサーバー側エンドポイントに転送します。認証用シークレットはリクエストヘッダーに含めます。

<a name="local-development"></a>

## ローカル開発

Node.js 26（Taskly の CI は 26.13.1）を使い、このディレクトリで実行します。

```bash
cp .dev.vars.example .dev.vars
npm ci
npm run dev
```

`npm run dev` は `cf dev` を実行し、Worker を `http://127.0.0.1:8787` で起動します。`.dev.vars` の `DMARC_INBOUND_ENDPOINT` はローカル API の `/integrations/email/inbound` を指し、`INTEGRATION_SECRET` は API 側の `EMAIL_INTEGRATION_SECRET` と揃えます。API のセットアップは [ローカル開発ガイド](../docs/local-development.md#application-stack)を参照してください。

ローカル API と Worker を起動したら、開発用アドレスでテストメールを投入します。

```bash
printf 'From: sender@example.com\r\nTo: inbound@example.com\r\nSubject: Local test\r\nMessage-ID: <local-test@example.com>\r\n\r\nHello\r\n' |
  curl -fsS 'http://127.0.0.1:8787/cdn-cgi/local/email?from=sender%40example.com&to=inbound%40example.com' \
    -H 'Content-Type: message/rfc822' --data-binary @-
```

## デプロイ

- Cloudflare の **Email > Email Routing > Workers** で新しい Email Worker を作成します。
- スクリプトには `worker.js` を指定します。
- 環境変数 `DMARC_INBOUND_ENDPOINT` に転送先（例：`https://your.server/integrations/email/inbound`）を設定します。
- `INTEGRATION_SECRET` には API 側の `EMAIL_INTEGRATION_SECRET` と同じ値を設定します。
- Email Routing のルールで対象のメールをこの Worker に送ります。

## 転送時の処理

- RFC 822 形式のメール全体を Base64 に変換し、`raw` として送ります。DMARC XML や ZIP などの添付ファイルはサーバー側で解析します。
- Worker の処理を軽く保つため、`text` と `html` の本文は抽出しません。必要に応じてサーバーが `raw` から抽出します。
- 転送先のエンドポイントは成功時に HTTP 202 を返し、DMARC、バウンス、その他のイベントを Discord に埋め込みメッセージとして送ります。
