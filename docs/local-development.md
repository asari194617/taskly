# ローカル開発

特に指定がなければ、コマンドはリポジトリのルートで実行します。[README](../README.md) に記載されたツールのバージョンを使い、開発用データベースを用意してください。

<a name="local-https"></a>

## ローカル HTTPS

開発機で Caddy を起動し、各サービスを本番ホスト名の先頭に `local.` を付けた URL で開きます。開発サーバーはループバックまたは Docker の開発ネットワークで待機させます。

[Caddy](https://caddyserver.com/docs/install) と [mkcert](https://github.com/FiloSottile/mkcert#installation) をインストールします。macOS では `brew install caddy mkcert` を実行します。

1. `sudoedit /etc/hosts` で [`dev/https/hosts`](../dev/https/hosts) の内容を追加します。すでに同じホスト名の行がある場合は、その行を更新して `127.0.0.1` に揃えます。
2. Taskly のチェックアウトから開発用証明書を作成し、Caddy を起動します。

   ```bash
   export TASKLY_DEV_TLS_DIR="$HOME/.local/share/taskly/https"
   dev/https/certificates.sh
   caddy run --config dev/https/Caddyfile --adapter caddyfile
   ```

   スクリプト内の `mkcert -install` で開発用CAが登録されます。ブラウザを起動済みの場合は再起動します。CAの秘密鍵は mkcert のディレクトリで保管し、証明書の出力先はチェックアウトの外に置きます。

   ポート `443` を空けておきます。Linux で低いポートへのバインド権限が必要な場合は、たとえば `sudo setcap cap_net_bind_service=+ep "$(command -v caddy)"` を実行します。macOS で権限エラーが出る場合は、同じディレクトリから次のコマンドで Caddy を起動します。

   ```bash
   sudo env TASKLY_DEV_TLS_DIR="$TASKLY_DEV_TLS_DIR" "$(command -v caddy)" \
     run --config dev/https/Caddyfile --adapter caddyfile
   ```

3. Caddy を起動したまま、使うサービスを立ち上げます。各 URL は、対応する開発サーバーが起動すると利用できます。

| サービス | ローカル HTTPS URL | 転送先 |
| --- | --- | --- |
| Taskly の画面 | https://local.taskly.iy0306.com | `4321` |
| Taskly API | https://local.api.taskly.iy0306.com | `3063` |
| 認証画面 | https://local.auth.taskly.iy0306.com/login | `3068` |
| 認証 API・OAuth issuer | https://local.taskly-auth-api.iy0306.com | `3067` |
| 企業ガイダンスの画面 | https://local.nnct-corp-guidance-2026.taskly.iy0306.com | `3000` |
| 企業ガイダンスの API・任意の MCP エンドポイント | https://local.api.nnct-corp-guidance-2026.taskly.iy0306.com | `3071` |
| 規約サイト | https://local.policies.taskly.iy0306.com | `3001` |
| このリポジトリの CDN 配信用ファイル | https://local.cdn.taskly.iy0306.com/docs/api/index.html | `dev/https/Caddyfile` でチェックアウト内のファイルを配信 |
| Taskly MCP HTTP API | https://local.mcp.taskly.iy0306.com | `3066` |
| Airflow の画面（任意） | https://local.airflow-taskly.iy0306.com | `3065` |

サービスごとの起動方法は [認証](https://github.com/asari194617/taskly-auth#ローカル-https)、[企業ガイダンス](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/development.md)、[規約サイト](https://github.com/asari194617/taskly-policy-web#ローカル-開発)を参照してください。

開発用CDNは、チェックアウト内の OpenAPI ファイルと MCP クライアントのインストーラーを配信します。Swagger UI でローカル HTTPS の API サーバーを選択してください。公開済みのバイナリや本番用アセットは公開CDNから取得します。MySQL、Redis、email-compose の gRPC、Airflow のワーカーは開発用ネットワーク内で通信します。

<a name="taskly-with-local-oauth"></a>

### Taskly でローカル OAuth を使う

認証 API と画面は [taskly-auth](https://github.com/asari194617/taskly-auth) のチェックアウトから起動します。Taskly と同じ親ディレクトリにチェックアウトする例です。

```bash
git clone https://github.com/asari194617/taskly-auth.git
```

認証と Taskly を起動する前に、共有 Docker ネットワークを作成します。

```bash
docker network inspect taskly-local-https >/dev/null 2>&1 ||
  docker network create --internal taskly-local-https
```

開発用の OAuth クライアントシークレットとコールバック URL を両サービスで揃えます。認証サービスは [taskly-auth のローカル HTTPS 手順](https://github.com/asari194617/taskly-auth#ローカル-https)に従って起動します。Taskly のルートにある `.env` の該当項目を次の値へ置き換え、`infra/.env` に同期します。Taskly の起動前に、[スキーマを適用](#application-stack)してください。

```dotenv
WEB_PUBLIC_URL=https://local.taskly.iy0306.com
API_PUBLIC_URL=https://local.api.taskly.iy0306.com
API_CORS_ORIGINS=https://local.taskly.iy0306.com,https://local.cdn.taskly.iy0306.com
AUTH_ISSUER=https://local.taskly-auth-api.iy0306.com
AUTH_CLIENT_ID=taskly-web
AUTH_CLIENT_SECRET=<taskly-auth と同じ開発用の値>
AUTH_REDIRECT_URI=https://local.api.taskly.iy0306.com/auth/oauth/callback
SESSION_COOKIE_SECURE=true
WEBAUTHN_RP_ID=local.taskly.iy0306.com
WEBAUTHN_ORIGIN=https://local.taskly.iy0306.com
ADMIN_REVIEW_URL=https://local.taskly.iy0306.com/admin/moderation
ICAL_BASE_URL=https://local.api.taskly.iy0306.com/ical
```

`apps/web/.env` に次の URL を設定します。

```dotenv
PUBLIC_API_URL=https://local.api.taskly.iy0306.com
PUBLIC_AUTH_OAUTH_START_URL=https://local.api.taskly.iy0306.com/auth/oauth/start
PUBLIC_AUTH_LOGOUT_URL=https://local.auth.taskly.iy0306.com/logout
PUBLIC_AUTH_PROFILE_URL=https://local.auth.taskly.iy0306.com/profile
PUBLIC_AUTH_SECURITY_URL=https://local.auth.taskly.iy0306.com/security
PUBLIC_AUTH_PASSKEY_URL=https://local.auth.taskly.iy0306.com/passkeys
PUBLIC_AUTH_PASSWORD_URL=https://local.auth.taskly.iy0306.com/security
PUBLIC_AUTH_EMAIL_CODE_URL=https://local.auth.taskly.iy0306.com/security
PUBLIC_AUTH_RECOVERY_URL=https://local.auth.taskly.iy0306.com/security
PUBLIC_AUTH_TOTP_URL=https://local.auth.taskly.iy0306.com/security
```

HTTPS 用の Compose ファイルを重ねて起動します。この Compose 設定を使うシェルごとに、同じ `TASKLY_DEV_TLS_DIR` をエクスポートします。

```bash
docker compose -f infra/docker-compose.yml -f dev/https/compose.yaml up -d --build --wait server mcp
curl -fsS https://local.taskly-auth-api.iy0306.com/.well-known/oauth-authorization-server
curl -fsS https://local.api.taskly.iy0306.com/readyz
curl -fsS https://local.mcp.taskly.iy0306.com/readyz
```

両方の開発用DBを新規作成した場合は、まず [開発用の認証ユーザーを作成](https://github.com/asari194617/taskly-auth#2-開発ユーザーを用意する)し、認証側の `users` テーブルで `id` を確認します。`https://local.taskly.iy0306.com/login` を開く前に、Taskly の開発用DBにも同じ ID とメールアドレスでユーザーを登録します。次の例では、ID とメールアドレスを確認した値に置き換えてください。

```bash
docker compose -f infra/docker-compose.yml exec -T db sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql -u"$MYSQL_USER" "$MYSQL_DATABASE"' <<'SQL'
INSERT INTO users (id, email, display_name, is_registered)
VALUES (1, 'taskly-local-demo@nagaoka-ct.ac.jp', 'Development User', 1);
SQL
```

API コンテナは共有ネットワークを使い、ブラウザと同じ OAuth issuer に接続します。TLS 検証にはコンテナにマウントした CA の公開証明書を使い、開発機では `/etc/hosts` で issuer の名前を解決します。

コンテナの再作成や停止にも同じ Compose ファイルを指定します。証明書を作り直した場合は Caddy を再起動し、Taskly の `auth-tls` と `server` コンテナを再作成します。システムの CA ストアを参照しない Node.js クライアントでは `NODE_EXTRA_CA_CERTS="$TASKLY_DEV_TLS_DIR/rootCA.pem"` を使えます。

[Airflow の起動手順](#optional-airflow)でも、2つの Compose ファイルを指定すると Airflow が開発用 HTTPS ホスト名で URL を生成します。ローカル HTTPS の利用をやめる場合は、Caddy を停止し、追加した `/etc/hosts` の行を削除して、アプリの URL 設定を元に戻します。

<a name="loopback-http-and-shared-setup"></a>

## ループバック HTTP と共通設定

<a name="environment-and-authentication"></a>

### 環境変数と認証

起動前に `.env.example` を `.env` にコピーして編集します。API は `AUTH_ISSUER`、`AUTH_CLIENT_ID`、`AUTH_CLIENT_SECRET`、`AUTH_REDIRECT_URI` の4つがすべて設定されているか、すべて空の場合に起動します。サンプルにはこのうち3つの値が入っています。`WEB_PUBLIC_URL` は2か所にあるため、両方を同じ値にするか、重複する行を削除してください。

ループバック HTTP で開発する場合は、作業内容に合わせて認証を設定します。

- Web ログインを試す場合は、別途 [taskly-auth の開発環境](https://github.com/asari194617/taskly-auth#開発環境)を起動し、OAuth クライアントを登録します。上の4変数をすべて設定します。コールバックは `http://localhost:3063/auth/oauth/callback`、Web のオリジンは `WEB_PUBLIC_URL=http://localhost:4321` です。`AUTH_ISSUER` はブラウザと API コンテナの両方から到達できるようにしてください。コンテナ内の `localhost` はコンテナ自身を指します。
- OAuth プロバイダーを使わず API だけを開発する場合は、上の4変数を空にし、サンプルの `AUTH_*_EXTERNAL` は `0` のままにします。Web のログイン画面は OAuth に遷移するため、UI の自動テストには [実APIのE2E用データ](../apps/web/tests/e2e/README.md)や [モックを使うブラウザーテスト](../apps/web/tests/browser/README.md)を利用します。

認証情報の書き込みを外部の認証サービスへ移す場合は、[`auth_migration_config.go`](../apps/server/auth_migration_config.go) にある8つの所有権フラグを同時に `1` にします。`AUTH_PROFILE_EXTERNAL=1` には `AUTH_USERS_EXTERNAL=1` も必要です。`AUTH_INTERNAL_TOKEN` を32文字以上にし、3つの同期用 URL も設定します。作業前に [認証情報の書き込み経路](auth-identity-write-inventory.md)、[ユーザー同期](auth-user-projection-sync.md)、[プロフィールの所有権](account-profile-ownership.md)、[退会・消去](auth-account-lifecycle-sync.md)を確認してください。データのコピーと OAuth クライアントの登録は、認証サービス側で行います。

`JWT_SECRET`、`TOTP_ENC_KEY`、MySQL・Redis のパスワード、外部連携の秘密値を開発用の値に置き換えます。開発環境でも `JWT_SECRET` には値が必要です。アプリ用DBと Airflow のメタデータ用DBには別々のデータベース名とアカウントを使います。`AIRFLOW_META_PASSWORD` に使用できるのは英数字、`_`、`-` です。3つの Airflow サービスで共通の `AIRFLOW__API_AUTH__JWT_SECRET` を明示的に設定してください。scheduler ではこの値の既定値が使われません。秘密値は `openssl rand -hex 32` などで生成できます。

編集後は `.env` と `infra/.env` を同期します。

```bash
chmod 600 .env
install -m 600 .env infra/.env
```

サービスは `env_file` からルートの `.env` を読みます。運用スクリプトは `infra/` に移動して実行するため、Compose ファイル内の `${...}` の展開には `infra/.env` が使われます。

SMTP、Web Push の VAPID キー、Gemini、Teams、FTPS バックアップを動かすには、それぞれの開発用認証情報が必要です。課題画像は [ローカル R2](#cloudflare-local-development)を利用できます。SMTP リレーは Compose に含まれません。開発用の送信先を設定するまでは外部連携と定期ジョブを停止しておきます。

[企業ガイダンス](nnct-corp-guidance-development.md)は、独立したリポジトリ、DB、公開 OAuth クライアントを使います。ガイダンスを開発する場合は、別途その起動手順に従ってください。

<a name="application-stack"></a>

## アプリケーションの起動

API を起動する前にスキーマを適用します。HTTPS を使う場合は、スキーマを適用したあと [HTTPS 用の Compose ファイルを重ねて起動](#taskly-with-local-oauth)します。

```bash
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server mcp
curl -fsS http://localhost:3063/readyz
curl -fsS http://localhost:3066/readyz
```

Atlas は Docker 内で動きます。`schema` プロファイルはスキーマ検証用に使い捨ての MySQL を起動します。スクリプトはアプリ用DBを起動し、適用計画を表示して非破壊的な変更を適用したあと、使い捨てのコンテナを削除します。既存の DB については [DBスキーマ管理](initial-settings.md#dbスキーマ管理)を確認してください。`apps/server/migrations` は過去の記録です。

別のターミナルで Web クライアントを起動します。`apps/web/.env` を設定済みの場合は、最初のコピーを省略します。

```bash
cp apps/web/.env.example apps/web/.env
npm --prefix apps/web ci
npm --prefix apps/web run dev
```

`npm run dev` は `cf dev` から Astro を起動します。Web クライアントの待受先は `http://localhost:4321` です。ログイン、ログアウト、プロフィール、セキュリティ画面を試す場合は、使用する認証環境に合わせて `PUBLIC_AUTH_*` の URL を設定します。

API と MCP のループバックポートはそれぞれ `3063` と `3066` です。MySQL、Redis、email-compose には Compose のネットワークから接続します。Go や MCP をホストで直接起動する場合は、開発用のポートマッピングかトンネルを用意し、`DB_HOST`、`DB_PORT`、Redis の接続設定を合わせてください。

<a name="cloudflare-local-development"></a>

## Cloudflare のローカル開発

Web と受信メール Worker は、各パッケージのロックファイルに固定された Cloudflare CLI（`cf`）で起動します。ローカル起動に Cloudflare へのログインは不要です。[Cloudflare CLI の開発ガイド](https://developers.cloudflare.com/cf/projects/)

### 課題画像をローカル R2 で試す

[通常のループバック HTTP 設定](#loopback-http-and-shared-setup)で、[API 起動前にスキーマを適用](#application-stack)してください。画像のアップロード・表示・削除を試す場合は、API と MCP の起動コマンドに R2 用の Compose ファイルを追加します。

```bash
docker compose -f infra/docker-compose.yml -f dev/cloudflare/compose.yaml \
  up -d --build --wait server mcp r2
```

R2 サービスは `cf dev` で開発用バケット `taskly-task-images-local` を起動します。Compose が API にローカルの接続先と開発用の認証情報を渡すため、`.env` の `R2_*` は空欄で構いません。API が返す署名付き URL と API 自身の S3 リクエストは、ともに `http://127.0.0.1:8788/cdn-cgi/local/r2/s3` を使います。

[ローカル S3 API](https://developers.cloudflare.com/r2/local-development/#serve-buckets-over-the-s3-api)は実験機能で、ループバックの `Host` と `Origin` を受け付けます。ブラウザーで `http://localhost:4321` を開いて課題画像を試してください。Airflow のアカウント消去ジョブで R2 連携を検証する場合は、別途、開発用の非公開バケットと認証情報を設定してください。

画像の一覧は `http://127.0.0.1:8788/cdn-cgi/local/explorer` で確認できます。CLI からバケット内の既存のキーを取得するには、次を実行します。

```bash
docker compose -f infra/docker-compose.yml -f dev/cloudflare/compose.yaml \
  exec -T r2 ./node_modules/.bin/cf r2 objects get '<object-key>' \
  --bucket-name taskly-task-images-local --mode r2 --local > downloaded-image
```

停止時も同じ Compose ファイルを指定します。画像は開発用の名前付きボリューム `r2-data` に残ります。

```bash
docker compose -f infra/docker-compose.yml -f dev/cloudflare/compose.yaml down
```

### 受信メール Worker を試す

[受信メール Worker の手順](../cloudflare-email-workers/README.md#local-development)に従って `cf dev` を起動し、ローカルのメール受信イベントを使って転送を確認します。

<a name="optional-airflow"></a>

## Airflow（任意）

**新規の開発用DB**では、アプリケーションの起動後に Airflow のメタデータを初期化します。ホストに `jq` を用意してください。

```bash
docker compose -f infra/docker-compose.yml build airflow-webserver
(cd infra && ./ops/prepare-airflow-db.sh)
docker compose -f infra/docker-compose.yml up -d --no-build --wait --wait-timeout 600 \
  airflow-webserver airflow-scheduler airflow-dag-processor
```

`prepare-airflow-db.sh` はメタデータ用DBと権限を設定し、`airflow db migrate` を実行します。既存環境では、先に [メタデータDBの移行手順](operations/airflow-db-separation.md)を確認してください。このスクリプトは旧メタデータテーブルを移動し、DB の権限を変更することがあります。メタデータのマイグレーションは起動前に明示的に実行します。

画面は `http://localhost:3065` で開きます。HTTPS 用の Compose 設定を重ねた場合は `https://local.airflow-taskly.iy0306.com` から開けます。初回起動時に、イメージのエントリーポイントが `_AIRFLOW_WWW_USER_USERNAME` と `_AIRFLOW_WWW_USER_PASSWORD` の値で FAB ユーザーを作成します。既定値は開発専用の `airflow` / `airflow` です。データと開発用の送信先を用意するまでは DAG を停止しておきます。実行スケジュールと各ジョブの役割は [バッチの説明](../batches/README.md)を参照してください。

<a name="stopping-and-troubleshooting"></a>

## 停止とトラブルシューティング

```bash
docker compose -f infra/docker-compose.yml ps
docker compose -f infra/docker-compose.yml logs --tail=100 server mcp
docker compose -f infra/docker-compose.yml down
```

`down` では名前付きボリュームが保持されます。環境変数を変更したあとは `up -d` で対象サービスを再作成します。`.env` で MySQL のパスワードを変更した場合は、既存ボリューム内の MySQL アカウント側にも変更を反映してください。

API が待受を開始する前に終了する場合は、OAuth 設定が一部だけ入っていないか、必要なテーブルが揃っているかを確認します。`/healthz` に応答し、`/readyz` に失敗する場合は [依存サービスの readiness](operations/readiness.md)を確認してください。
