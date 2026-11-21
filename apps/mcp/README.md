# Taskly MCP サービス

このパッケージには 2 つのプロセスがあります。

- MySQL で MCP トークンを認証し、課題の操作を Go API に渡す Fastify HTTP API
- HTTP の各操作を MCP ツールとして公開する MCP stdio ブリッジ

## HTTP API

Compose のサービス名は `mcp` です。ローカル開発では `http://localhost:3066` で利用できます。

HTTP プロセスは、認証や DB クエリを行う前に、直接接続する IP アドレスごとに毎分 300 リクエストまで受け付けます。上限超過時は `Retry-After` を含む HTTP 429 を返します。DB へアクセスしない `/healthz` を除く全ルートが同じ上限を共有し、`/readyz` も対象です。転送元 IP を示すヘッダーやトークンを変えても上限は変わりません。同じリバースプロキシを使うクライアントは、そのプロキシの上限を共有します。カウンターはプロセスごとに保持し、再起動時にリセットします。

| メソッド | パス | 認証 |
| --- | --- | --- |
| GET | `/healthz` | 不要 |
| GET | `/readyz` | 不要 |
| GET, POST | `/tasks` | `X-MCP-Token` |
| PATCH, DELETE | `/tasks/:id` | `X-MCP-Token` |
| GET, PATCH | `/users/:id` | `X-MCP-Token` |

`PATCH /users/:id` で更新できるのは、トークン所有者の `display_name` と `notification_lang`（`ja` または `en`）です。Go API がリクエストを検証して適用します。`AUTH_PROFILE_EXTERNAL=1` の場合、表示名の更新は HTTP 409 を返すため、taskly-auth で変更してください。通知言語は引き続き変更できます。アカウント削除、登録、ポリシーへの同意は `user_update` では変更できません。

サーバーは受け取ったトークンの SHA-256 ハッシュを計算し、`mcps.hashed_token` と照合します。トークンは Web 画面の **設定 > MCP** から発行できます。課題の操作と通知は Go API が処理します。操作はトークン所有者の範囲に限り、メイン API の課題権限に従います。課題を削除できるのは、削除権限を明示的に付与されたユーザーか、管理者・マネージャーです。非表示の課題はこの API に公開しません。

```bash
curl -fsS http://localhost:3066/healthz
curl -fsS http://localhost:3066/readyz
curl -fsS -H "X-MCP-Token: $TASKLY_MCP_TOKEN" \
  http://localhost:3066/tasks
```

`/healthz` はプロセスが応答することを確認します。`/readyz` は短いタイムアウトで MySQL にクエリを実行し、Go API の `/readyz` にも HTTP で接続します。どちらかに失敗すると HTTP 503 を返します。Compose とデプロイの検証には `/readyz` を使います。依存サービスに障害がある場合はコンテナを異常と判定しますが、生存確認のエンドポイントを使った再起動は行いません。

## ローカル開発

CI と同じ検証には Node.js 26 を使います。コンテナのランタイムは Node.js 26.13.1 です。依存サービス全体の起動とスキーマの初期化は[ローカル開発ガイド](../../docs/local-development.md)に従ってください。以下のコマンドは HTTP プロセスをホストで直接起動します。必要な環境変数を事前にエクスポートしてください。

```bash
npm ci --legacy-peer-deps --no-audit --no-fund
npm run build
npm test
npm start
```

`npm run dev` はビルド済みの `dist/index.js` を監視します。事前にビルドし、TypeScript を変更したら再ビルドしてください。

HTTP サーバーが参照する環境変数は以下のとおりです。

- `MCP_HOST`（既定値 `0.0.0.0`）
- `MCP_PORT`（既定値 `3066`）
- `DB_HOST`、`DB_PORT`、`MYSQL_USER`、`MYSQL_PASSWORD`、`MYSQL_DATABASE`
- `TASKLY_API_BASE_URL`（Compose 内の既定値は `http://server:8080`）

Compose 内では `DB_HOST=db` と `DB_PORT=3306` を使います。標準の Compose 構成は MySQL をホスト側に公開しないため、MCP をホストで起動するにはトンネルかポートマッピングを設定してください。Compose の外で起動する場合、`TASKLY_API_BASE_URL` に Go API の URL（例：`http://localhost:3063`）を指定します。

## stdio ブリッジ

```bash
npm run build
npm run start:stdio
```

ブリッジが参照する環境変数は以下のとおりです。

- `TASKLY_MCP_BASE_URL`（既定値 `http://localhost:3066`）
- `TASKLY_MCP_TOKEN`（推奨）または `MCP_TOKEN`

クライアントの設定例は `gemini.settings.example.json` を参照してください。ビルド済みクライアントのインストール方法と本番 URL は [MCP ガイド](../../docs/mcp-docs.md)を参照してください。
