# DMARC 連携設定手順

このドキュメントでは、Cloudflare Email Routing（Email Workers）から Taskly の DMARC Processor（`apps/dmarc-processor`）へメールを転送し、
DMARC 集計レポートの要約やバウンス検知・一時停止（サプレッション）を行うまでの設定手順を説明します。

## 全体像

1. Cloudflare Email Routing で対象メール（postmaster@ / rua= / ruf= 等）を Email Worker に転送
2. Email Worker（`cloudflare-email-workers/worker.js`）が Taskly の `dmarc-processor` エンドポイントへ JSON でフォワード
3. `dmarc-processor` が認証ヘッダーを検証し、DMARC XML（zip/gzip 含む）を解析、または DSN っぽい本文からバウンス宛先を抽出
4. Discord に埋め込み（embeds）で通知、必要ならメールアドレスを一時停止（EmailSuppressions）に登録

## 1) 環境変数の準備

レポジトリ直下の `.env`（本番は `.env.production`）に以下が設定されていることを確認してください。

- `EMAIL_INTEGRATION_SECRET`: Email Worker → dmarc-processor で用いる共有シークレット
- `DISCORD_WEBHOOK_URL`: 通知先 Discord Webhook
- DB 接続（`DB_HOST`, `DB_PORT`, `MYSQL_DATABASE`, `MYSQL_USER`, `MYSQL_PASSWORD`）

## 2) サービスの起動（Docker Compose）

`infra/docker-compose.yml` に `dmarc-processor` サービスが追加されています。開発では以下のコマンドで起動できます。

```sh
cd infra
docker compose up -d --build dmarc-processor
```

デフォルトではホストの `http://localhost:3069`（コンテナ内 8080）で待ち受けます。

## 3) Cloudflare Email Worker の設定

`cloudflare-email-workers/worker.js` を Cloudflare ダッシュボードにコピペしてデプロイしてください。

ファイル冒頭にある定数を編集します。

```js
const ENDPOINT = "https://api.example.com/dmarc/inbound";
const INTEGRATION_SECRET = "change_me_email_inbound_secret";
```

- ENDPOINT には `dmarc-processor` の公開 URL（例: `https://api.taskly.example.com/dmarc/inbound`）を指定してください。
- INTEGRATION_SECRET は `.env` の `EMAIL_INTEGRATION_SECRET` と一致させます。

Email Routing のルールで対象メール（DMARC 集計、バウンスなど）をこの Worker に渡すよう設定します。

### もっと良い運用方法（推奨）

手動コピペの代わりに、Cloudflare Wrangler を使うとデプロイが再現可能になります。

- `wrangler.toml` に `vars` として `EMAIL_INTEGRATION_SECRET` と `EMAIL_INBOUND_ENDPOINT` を定義
- `wrangler deploy` でバージョン管理された Worker を安全に更新

ダッシュボード上の「Worker の変数・シークレット」を使うのも安全で、コードに生の値をベタ書きしなくて済みます。

## 4) エンドポイント（OpenAPI）

`apps/dmarc-processor/openapi.yaml` に `POST /inbound` が定義されています。

- 認証: HTTP ヘッダー `X-Email-Integration-Secret` に共有シークレット
- 形式: JSON（`raw` に RFC822 を Base64 で渡すことも可能）
- 返り値: 202 Accepted（非同期処理）

## 5) Discord 通知と自動サプレッション

- DMARC XML（.xml / .zip / .gz）を検出した場合、ドメイン・期間・合計/成功/失敗・失敗率を Discord の埋め込みとして送信します。
- DMARC でない場合は、本文テキスト/HTML から DSN 風の「Final-Recipient/Original-Recipient」をスキャンしてバウンス候補を抽出し、
  7 日間の一時停止として EmailSuppressions に登録します（DB）。
- 抑止が発生した場合も Discord に通知します。

## 6) 運用上のヒント

- 失敗率が高いドメインは DNS/SPF/DKIM/DMARC の整備や From/Return-Path の見直しをご検討ください。
- `raw` を活用し、将来的に MIME 解析で本文や添付をより高精度に抽出することも可能です。
- 本番では Sentry の DSN を設定し、例外を追跡してください。

---

トラブルシューティング:

- 401 Unauthorized: `EMAIL_INTEGRATION_SECRET` の不一致がないか確認
- 202 が返るが通知が来ない: `DISCORD_WEBHOOK_URL` が空でないか、Firewall でブロックされていないか
- サプレッションが登録されない: DB 接続設定とテーブル作成（マイグレーション/AutoMigrate）を確認
