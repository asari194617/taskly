# Taskly MCP サーバー 利用ガイド

このドキュメントでは、Taskly の MCP（Model Context Protocol）サーバーを利用したいユーザー向けに、ローカル開発・本番公開・クライアント設定（Gemini CLI など）・トラブルシュートまでをまとめています。

## 提供機能（ツール）

MCP stdio ブリッジ経由で以下のツールが利用できます。

- tasks_list: { limit?, offset?, search? }
- tasks_create: { title, subject_id, deadline, description?, type?, is_require_hand_in?, is_self_check_required?, deadline_since?, deadline_until? }
- tasks_update: { id, patch: {...} }
- tasks_delete: { id }
- user_get: { id }
- user_update: { id, patch: {...} }

HTTP API については `apps/mcp/README.md` を参照してください。

---

## 1. ローカルで使う（localhost）

前提: Docker が動作していること。Taskly リポジトリを clone 済みであること。

1. サービス起動

```bash
cd infra
docker compose up -d --build
```

2. MCP HTTP のヘルスチェック

```bash
curl -I http://localhost:3066/healthz
```

3. MCP トークンの準備

- Web UI（Settings > MCP）からトークンを発行してください（平文トークン）。

4. Gemini CLI 設定（stdio ブリッジ利用）

- `~/.gemini/settings.json` に以下を追記（パスとトークンは環境に合わせて変更）

```json
{
  "mcpServers": {
    "Taskly-server": {
      "command": "/usr/bin/env",
      "args": ["-S", "node", "/ABSOLUTE/PATH/to/apps/mcp/dist/stdio.js"],
      "env": {
        "TASKLY_MCP_BASE_URL": "http://localhost:3066",
        "TASKLY_MCP_TOKEN": "<YOUR_PLAIN_TOKEN>"
      }
    }
  }
}
```

5. 動作確認

- Gemini CLI を再起動 or `/reload`
- `/mcp` を実行し、`Taskly-server - Ready` とツール一覧が表示されれば OK

トラブルシュート（ローカル）

- `getaddrinfo ENOTFOUND db` と出る場合:
  - `mcp` コンテナが `db` ネットワークに接続されていない可能性があります。
  - 本リポジトリの `infra/docker-compose.yml` では `mcp` が `default` と `proxy` の両ネットワークに属するよう設定済みです。
  - `docker compose up -d --build` を再実行してください。
- `stdio.js` が見つからない:
  - `cd apps/mcp && npm ci && npm run build` を行い、`apps/mcp/dist/stdio.js` が存在するか確認してください。
- token が無効:
  - `X-MCP-Token` ヘッダに使うプレーンなトークンを Web UI で再発行し、設定し直してください。

---

## 2. 本番公開（ホスト名 / HTTPS）

要件: サーバがグローバルから 80/443 に到達可能、DNS が設定できること。

1. DNS 設定

- `MCP_DOMAIN`（例: `mcp.example.com`）をサーバの IP に向ける

2. 環境変数（`infra/.env`）

```
TRAEFIK_ACME_EMAIL=your@example.com
MCP_DOMAIN=mcp.example.com
```

3. Traefik（リバースプロキシ）を起動（profiles: edge）

```bash
cd infra
docker compose --profile edge up -d --build traefik mcp
```

4. 動作確認

```bash
curl -I https://mcp.example.com/healthz
```

注意点

- CDN（Cloudflare 等）を使う場合、http-01 チャレンジが通らない場合があります。DNS-01 への切替やプロキシ OFF をご検討ください。
- ローカル環境では 80/443 を既存プロセスが使っている場合が多いので、Traefik を通常起動しない（profiles 利用）運用にしています。

---

## 3. クライアント（一般ユーザー）向け案内テンプレート

以下を、そのままユーザー案内としてお使いください。

### A) ローカルで使う場合（開発者向け / AWS MCP 風コマンド）

1.  Docker をインストールしてください。
2.  管理者から提供されたリポジトリを clone し、以下を実行します。
    ```bash
    cd infra
    docker compose up -d --build
    ```
3.  管理者から発行された MCP トークンを受け取り、Gemini 設定に反映します。 - `~/.gemini/settings.json` に以下を設定（コマンド名のみ。AWS の MCP サーバーと同じ感覚で使えます）
    ```json
    {
      "mcpServers": {
        "Taskly-server": {
          "command": "taskly-mcp",
          "args": [],
          "env": {
            "TASKLY_MCP_BASE_URL": "http://localhost:3066",
            "TASKLY_MCP_TOKEN": "<YOUR_PLAIN_TOKEN>"
          }
        }
      }
    }
    ```
         - 事前に以下のいずれかでコマンドを導入してください。
        	 - 開発用（ローカルリンク）: `cd apps/mcp && npm run build && npm run link`（`taskly-mcp` がPATHに入ります）
        	 - 配布版（推奨）: Homebrew/ZIP配布で `/usr/local/bin/taskly-mcp` を配置（今後用意予定）
4.  Gemini を再起動し、`/mcp` で Taskly-server が Ready になっていることを確認してください。

### B) ホスト名（HTTPS）で使う場合（一般ユーザー / AWS MCP 風コマンド）

1. 管理者から提供された URL（例: `https://mcp.example.com`）とトークンを受け取ってください。
2. `~/.gemini/settings.json` を以下のように設定します。
   ```json
   {
     "mcpServers": {
       "Taskly-server": {
         "command": "taskly-mcp",
         "args": [],
         "env": {
           "TASKLY_MCP_BASE_URL": "https://mcp.example.com",
           "TASKLY_MCP_TOKEN": "<YOUR_PLAIN_TOKEN>"
         }
       }
     }
   }
   ```
3. Gemini を再起動し、`/mcp` で Taskly-server が Ready になっていることを確認してください。

---

## 4. よくある質問（FAQ）

Q. ツール実行時に `getaddrinfo ENOTFOUND db` と出る

- A. サービス間名前解決ができていない可能性があります。`infra/docker-compose.yml` の `mcp` サービスが `default` ネットワークに参加していること、`docker compose up -d --build` を再実行することを確認してください。

Q. `Taskly-server - Disconnected` のまま

- A. `stdio.js` の絶対パス、`node` コマンドの PATH、`TASKLY_MCP_BASE_URL` の URL が正しいか確認。必要に応じて `command` を node のフルパス（例: `/opt/homebrew/bin/node`）で指定してください。

Q. トークンが通らない

- A. Web UI（Settings > MCP）で新しいトークンを発行し、`TASKLY_MCP_TOKEN` に設定してください。

Q. JSON で結果を受け取りたい

- A. 現在は MCP v0.4 の互換性に合わせてテキスト返却を行っています。必要であれば結果の整形を検討します。

---

## 5. 開発者向け補足

- stdio サーバー: `apps/mcp/src/stdio.ts`
- HTTP サーバー: `apps/mcp/src/index.ts`
- ビルド: `cd apps/mcp && npm ci && npm run build`
- ログ・監視・本番公開: `infra/docker-compose.yml` の Traefik（profiles: edge）を参照

---

## 6. Agent モードの有効化（Gemini）

Gemini CLI で Taskly MCP を最大限活用するには、Agent モードを有効化すると便利です。

手順（例）

1. `~/.gemini/settings.json` に Taskly MCP（stdio）の設定が入っていることを確認
2. Gemini を再起動して `/mcp` で `Taskly-server - Ready` とツール一覧が見えることを確認
3. Agent モードを有効化
   - 画面上の「Agent mode」トグルを ON にする、または `/agent on` を実行（CLI のバージョンにより表記は異なります）
4. 以降、自然言語で依頼しても、必要に応じて MCP ツール（tasks_list/create/update/delete, user_get/update）を自動で呼び出します

運用のコツ

- 破壊的操作（tasks_delete / 大量更新など）は、GEMINI.md に「確認を求める」ルールを記載しておくと安心です
- ツールが未接続の場合は、まず `/mcp` で接続状態を確認し、設定の BASE_URL/TOKEN を見直してください
- ツール実行ログや結果は、なるべくテキストで返し、必要に応じて整形してもらうようエージェントに促せます
