# ローカル開発

特に指定がなければ、コマンドはリポジトリのルートで実行します。ツールのバージョンは [README](../README.md) を参照してください。ここでは開発用 DB を使います。

<a name="local-https"></a>

## ローカル HTTPS

開発機上の共通 Caddy プロキシで、各サービスのホスト名を扱います。[Caddy](https://caddyserver.com/docs/install) と [mkcert](https://github.com/FiloSottile/mkcert#installation) をインストールします。macOS では `brew install caddy mkcert` を実行します。各プロセスはループバックか Docker ネットワーク内で動かしてください。

1. `sudoedit /etc/hosts` で `/etc/hosts` を開き、[`dev/https/hosts`](../dev/https/hosts) のエントリを追加します。同じホスト名がすでにあれば、それぞれ `127.0.0.1` を指すように更新します。
2. Taskly のチェックアウトから証明書を生成し、HTTPS プロキシを起動します。

   ```bash
   export TASKLY_DEV_TLS_DIR="$HOME/.local/share/taskly/https"
   dev/https/certificates.sh
   caddy run --config dev/https/Caddyfile --adapter caddyfile
   ```

   `mkcert -install` は開発用の個人 CA をインストールします。ブラウザーが起動中なら再起動してください。CA の秘密鍵は mkcert のディレクトリに置き、TLS ディレクトリはチェックアウトの外に置きます。OS が小さい番号のポートを制限する場合は Caddy の実行ファイルに 443 番ポートを使う権限を付与します。Linux の例は `sudo setcap cap_net_bind_service=+ep "$(command -v caddy)"` です。macOS で権限エラーが出た場合は同じディレクトリから以下で Caddy を起動します。

   ```bash
   sudo env TASKLY_DEV_TLS_DIR="$TASKLY_DEV_TLS_DIR" "$(command -v caddy)" \
     run --config dev/https/Caddyfile --adapter caddyfile
   ```

   443 番ポートが空いている必要があります。
3. 使うサービスを起動します。Caddy は起動したターミナルで動かしたままにします。各 HTTPS オリジンは対応する転送先の起動後に利用できます。

| サービス | ローカル HTTPS の URL | ループバックの転送先 |
| --- | --- | --- |
| Taskly Web | https://local.taskly.iy0306.com | `4321` |
| Taskly API | https://local.api.taskly.iy0306.com | `3063` |
| 認証 UI | https://local.auth.taskly.iy0306.com/login | `3068` |
| 認証 API と OAuth 発行者 | https://local.taskly-auth-api.iy0306.com | `3067` |
| 企業ガイダンス UI | https://local.nnct-corp-guidance-2026.taskly.iy0306.com | `3000` |
| 企業ガイダンス API と任意の MCP エンドポイント | https://local.api.nnct-corp-guidance-2026.taskly.iy0306.com | `3071` |
| ポリシーサイト | https://local.policies.taskly.iy0306.com | `3001` |
| このリポジトリの CDN ファイル | https://local.cdn.taskly.iy0306.com/docs/api/index.html | `dev/https/Caddyfile` がチェックアウト内のファイルを配信 |
| Taskly MCP HTTP API | https://local.mcp.taskly.iy0306.com | `3066` |
| Airflow の管理画面（任意） | https://local.airflow-taskly.iy0306.com | `3065` |

各転送先は[認証](https://github.com/asari194617/taskly-auth#ローカル-https)、[企業ガイダンス](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/development.md)、[ポリシーサイト](https://github.com/asari194617/taskly-policy-web#ローカル-開発)のセットアップ手順で設定します。CDN のローカルホストは、追跡対象の OpenAPI ファイルと MCP クライアントインストーラーのソースを配信します。API リファレンスのサーバー選択でローカル HTTPS API を選んでください。公開済みのバイナリと本番専用アセットは引き続き公開 CDN から取得します。MySQL、Redis、email-compose の gRPC、Airflow のワーカープロセスは開発用ネットワーク内で通信します。

### Taskly のローカル OAuth

認証と Taskly を起動する前に、共用の内部 Docker ネットワークを一度作成します。

```bash
docker network inspect taskly-local-https >/dev/null 2>&1 ||
  docker network create --internal taskly-local-https
```

両サービスに同じ開発用 OAuth クライアントシークレットとコールバックを設定し、taskly-auth を HTTPS 用の Compose 上書き設定で起動します。Taskly は後述のスキーマ適用後に起動します。Taskly のルートにある `.env` の該当項目を次のように置き換え、`infra/.env` に同期します。

```dotenv
WEB_PUBLIC_URL=https://local.taskly.iy0306.com
API_PUBLIC_URL=https://local.api.taskly.iy0306.com
API_CORS_ORIGINS=https://local.taskly.iy0306.com,https://local.cdn.taskly.iy0306.com
AUTH_ISSUER=https://local.taskly-auth-api.iy0306.com
AUTH_CLIENT_ID=taskly-web
AUTH_CLIENT_SECRET=<same development value as taskly-auth>
AUTH_REDIRECT_URI=https://local.api.taskly.iy0306.com/auth/oauth/callback
SESSION_COOKIE_SECURE=true
WEBAUTHN_RP_ID=local.taskly.iy0306.com
WEBAUTHN_ORIGIN=https://local.taskly.iy0306.com
ADMIN_REVIEW_URL=https://local.taskly.iy0306.com/admin/moderation
ICAL_BASE_URL=https://local.api.taskly.iy0306.com/ical
```

`apps/web/.env` では、該当の URL を以下で置き換えます。

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

開発用 Compose 構成を HTTPS 用の上書き設定と合わせて起動します。この設定を使うシェルでは同じ `TASKLY_DEV_TLS_DIR` をエクスポートしてください。

```bash
docker compose -f infra/docker-compose.yml -f dev/https/compose.yaml up -d --build --wait server mcp
curl -fsS https://local.taskly-auth-api.iy0306.com/.well-known/oauth-authorization-server
curl -fsS https://local.api.taskly.iy0306.com/readyz
curl -fsS https://local.mcp.taskly.iy0306.com/readyz
```

開発用 DB が両方とも新規の場合は、[認証用ユーザーを作成](https://github.com/asari194617/taskly-auth#2-開発ユーザーを用意する)し、認証 DB の `users` テーブルで `id` を確認します。`https://local.taskly.iy0306.com/login` を開く前に、同じ ID とメールアドレスを使うユーザーを Taskly の開発 DB に登録します。以下を使う場合は、ID とメールアドレスを開発用認証 DB の値に置き換えてください。

```bash
docker compose -f infra/docker-compose.yml exec -T db sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql -u"$MYSQL_USER" "$MYSQL_DATABASE"' <<'SQL'
INSERT INTO users (id, email, display_name, is_registered)
VALUES (1, 'taskly-local-demo@nagaoka-ct.ac.jp', 'Development User', 1);
SQL
```

上書き設定は、共用ネットワーク上の正確な OAuth 発行者に API コンテナが接続する経路を設け、TLS 検証に使う公開 CA 証明書をマウントします。開発機は同じ発行者を `/etc/hosts` で名前解決します。サービスの再作成や停止にも同じ Compose ファイルを指定してください。証明書を再生成したら Caddy を再起動し、Taskly の `auth-tls` と `server` コンテナを再作成して新しいファイルを読み込ませます。システムの CA ストアを使わない Node.js クライアントでは `NODE_EXTRA_CA_CERTS="$TASKLY_DEV_TLS_DIR/rootCA.pem"` を設定できます。

Airflow がローカル HTTPS のホスト名で URL を生成するようにするには、後述の Airflow 起動コマンドに両方の Compose ファイルを指定します。ローカル HTTPS を廃止するには、Caddy を停止し、`/etc/hosts` から追加したホスト名を削除して、以前のアプリケーション URL に戻します。

## ループバック HTTP と共通設定

### 環境設定と認証

サービスを起動する前に `.env.example` を `.env` にコピーして編集します。OAuth の `AUTH_ISSUER`、`AUTH_CLIENT_ID`、`AUTH_CLIENT_SECRET`、`AUTH_REDIRECT_URI` はすべて設定するか、すべて空にしてください。API は一部のみ設定された状態では起動しません。例には 4 項目のうち 3 項目が設定されています。`WEB_PUBLIC_URL` は例の 2 か所に設定されているため、同じ値にするか、重複を削除してください。

- Web ログインを使う場合は [taskly-auth](https://github.com/asari194617/taskly-auth#開発環境) の開発環境を別途起動し、OAuth クライアントを登録します。4 つの変数をすべて設定し、コールバックは `http://localhost:3063/auth/oauth/callback`、Web オリジンは `WEB_PUBLIC_URL=http://localhost:4321` とします。ブラウザーと API コンテナの両方から `AUTH_ISSUER` に接続できる必要があります。API コンテナ内の `localhost` はそのコンテナ自身を指します。taskly-auth はこのリポジトリの構成には含まれません。
- OAuth プロバイダーなしで API のみを開発する場合は 4 つの変数を空にして、例にある `AUTH_*_EXTERNAL` フラグを `0` のままにします。API は起動できますが、Web のログイン画面は OAuth にリダイレクトします。UI を自動検証する場合は、分離された[実 API の E2E フィクスチャ](../apps/web/tests/e2e/README.md)または[モックを使うブラウザテスト](../apps/web/tests/browser/README.md)を使います。

認証データの管理主体を外部サービスへ移す場合は、[`auth_migration_config.go`](../apps/server/auth_migration_config.go) にある 8 個の管理主体フラグをすべて同時に `1` にします。`AUTH_PROFILE_EXTERNAL=1` には `AUTH_USERS_EXTERNAL=1` も必要です。`AUTH_INTERNAL_TOKEN`（32 文字以上）と 3 つの同期 URL も設定します。管理主体の変更前に、[認証データの移行条件](auth-identity-write-inventory.md)、[ユーザープロジェクション](auth-user-projection-sync.md)、[プロフィールの管理主体](account-profile-ownership.md)、[アカウントのライフサイクル](auth-account-lifecycle-sync.md)を参照してください。データコピーと OAuth クライアント登録は認証システム側で行います。

`JWT_SECRET`、`TOTP_ENC_KEY`、MySQL と Redis のパスワード、連携用シークレットには開発用の値を設定します。開発環境でも `JWT_SECRET` は空にできません。アプリケーション DB と Airflow メタデータ DB では DB 名とアカウントを分けます。`AIRFLOW_META_PASSWORD` に使用できるのは英数字、`_`、`-` だけです。3 つの Airflow サービスすべてに同じ空でない `AIRFLOW__API_AUTH__JWT_SECRET` を明示的に設定してください。スケジューラーにはほかの 2 サービスが使うフォールバックがありません。個別のシークレットは `openssl rand -hex 32` で生成できます。

編集後は、Git の管理対象外である次の 2 ファイルを同期します。

```bash
chmod 600 .env
install -m 600 .env infra/.env
```

サービスは `env_file` からルートの `.env` を読み込みます。運用スクリプトは `infra/` に移動するため、Compose は変数 `${...}` の展開に `infra/.env` を読み込みます。各サービスの `env_file` だけでは、この展開に値が渡りません。

SMTP、R2 画像ストレージ、Web Push の VAPID 鍵、Gemini、Teams、FTPS バックアップを動かすには、それぞれ開発用の認証情報が必要です。Compose の構成には SMTP リレーは含まれません。開発用の送信先が整うまで、外部送信と定期ジョブは無効にしてください。

[長岡高専企業ガイダンス](nnct-corp-guidance-development.md)は専用のリポジトリ、DB、公開 OAuth クライアントを使います。そのアプリを開発する場合は、この Compose 構成とは別にセットアップします。

## アプリケーションの起動

API を起動する前にスキーマを適用します。HTTPS 用の上書き設定を使う場合は、2 行目を前述の HTTPS 用 Compose コマンドに置き換えます。

```bash
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server mcp
curl -fsS http://localhost:3063/readyz
curl -fsS http://localhost:3066/readyz
```

Atlas は Docker 内で動くため、ホストへのインストールは不要です。`schema` プロファイルはスキーマ解析用の一時 MySQL を追加します。スクリプトはアプリ用 DB を起動して計画を表示し、非破壊的な変更を適用してから一時コンテナを削除します。既存 DB には[スキーマ運用手順](initial-settings.md#dbスキーマ管理)を参照してください。`apps/server/migrations` は過去の記録です。`go run ./cmd/migrate` コマンドはありません。

次に別のターミナルで Web クライアントを起動します。

```bash
cp apps/web/.env.example apps/web/.env
npm --prefix apps/web ci
npm --prefix apps/web run dev
```

Web クライアントは `http://localhost:4321` で待ち受けます。ログイン、ログアウト、プロフィール、セキュリティ画面を検証するときは、認証環境に合わせて `PUBLIC_AUTH_*` の URL を設定してください。API と MCP はループバックのポート `3063`、`3066` で待ち受けます。MySQL、Redis、email-compose には Compose のサービスからのみ接続できます。Go API や MCP をホストで直接実行するには、開発用のポートマッピングまたはトンネルと、対応する `DB_HOST`、`DB_PORT`、Redis 設定が必要です。

<a name="optional-airflow"></a>

## Airflow のセットアップ（任意）

**新規の開発 DB** では、アプリケーションの起動後にメタデータを初期化します。ホストに `jq` が必要です。

```bash
docker compose -f infra/docker-compose.yml build airflow-webserver
(cd infra && ./ops/prepare-airflow-db.sh)
docker compose -f infra/docker-compose.yml up -d --no-build --wait --wait-timeout 600 \
  airflow-webserver airflow-scheduler airflow-dag-processor
```

`prepare-airflow-db.sh` はメタデータ専用 DB を作成し、権限を付与して `airflow db migrate` を実行します。既存環境に対しては、先に作業を止めて[メタデータ移行手順](operations/airflow-db-separation.md)を確認してください。このスクリプトは過去の Airflow テーブルの移動や DB 権限の変更も行う場合があります。基本の Compose 構成はサービス起動時にメタデータを移行しません。

管理画面は `http://localhost:3065` です。初回起動時にイメージのエントリーポイントが `_AIRFLOW_WWW_USER_USERNAME` と `_AIRFLOW_WWW_USER_PASSWORD` から FAB ユーザーを作成します（既定値は両方とも `airflow`。ローカル開発専用）。DAG を確認し、データと送信先サービスを用意するまでは一時停止のままにします。スケジュールとジョブの役割は[バッチ一覧](../batches/README.md)を参照してください。

## 停止とトラブルシューティング

```bash
docker compose -f infra/docker-compose.yml ps
docker compose -f infra/docker-compose.yml logs --tail=100 server mcp
docker compose -f infra/docker-compose.yml down
```

`down` は名前付きボリュームを保持します。環境変数を変更したら `up -d` で対象サービスを再作成します。既存の MySQL ボリューム内のアカウントは、`.env` のパスワードを変えても更新されないため、アカウント側を個別に更新してください。API が待ち受けを開始できない場合は、OAuth 設定の不足やテーブルの欠落を確認します。`/healthz` が成功して `/readyz` が失敗する場合は、[依存サービスの準備状態](operations/readiness.md)を確認してください。
