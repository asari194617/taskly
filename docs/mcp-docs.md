# Taskly MCP サーバー利用ガイド

Taskly MCP は、HTTP API と、その API を MCP ツールとして公開する stdio
ブリッジで構成されています。認証には Web UI の **Settings > MCP** で発行した
平文トークンを使用します。

## 提供ツール

- `tasks_list`: `{ limit?, offset?, search? }`
- `tasks_create`: `{ title, subject_id, deadline, description?, type?, is_require_hand_in?, is_self_check_required?, deadline_since?, deadline_until? }`
- `tasks_update`: `{ id, patch }`
- `tasks_delete`: `{ id }`
- `user_get`: `{ id }`
- `user_update`: `{ id, patch }`

HTTP endpoint の詳細は `apps/mcp/README.md` を参照してください。

## ローカルで使う

リポジトリのルートで次を実行します。

```bash
cp .env.example .env
docker compose -f infra/docker-compose.yml up -d --build db mcp
curl -fsS http://localhost:3066/healthz
```

stdio ブリッジをソースから実行する場合:

```bash
npm --prefix apps/mcp ci --legacy-peer-deps --no-audit --no-fund
npm --prefix apps/mcp run build
```

MCP クライアントの設定例:

```json
{
  "mcpServers": {
    "Taskly-server": {
      "command": "node",
      "args": ["/absolute/path/to/taskly/apps/mcp/dist/stdio.js"],
      "env": {
        "TASKLY_MCP_BASE_URL": "http://localhost:3066",
        "TASKLY_MCP_TOKEN": "<YOUR_PLAIN_TOKEN>"
      }
    }
  }
}
```

クライアントを再起動または再読込し、ツール一覧が表示されることを確認します。

## 配布済みクライアントを使う

macOS/Linux:

```bash
curl -fsSL https://cdn.taskly.iy0306.com/docs/mcp/clients/install.sh | bash
```

Windows の手順、特定バージョン、インストール先の変更方法は
`apps/mcp-client-files/README.md` を参照してください。インストール後は設定の
`command` を `taskly-mcp` とし、`TASKLY_MCP_BASE_URL` と
`TASKLY_MCP_TOKEN` を同様に指定します。

## 本番公開

`infra/docker-compose.yml` は MCP をホストの TCP `3066` に公開しますが、
TLS 終端やインターネット向けリバースプロキシは含みません。本番では次を別途構成します。

1. DNS を本番入口へ向ける。
2. 外部リバースプロキシまたはロードバランサーで HTTPS を終端する。
3. MCP の `3066` は信頼できる入口からのみ到達可能にする。
4. 公開 URL の `GET /healthz` が `{"ok":true}` を返すことを確認する。

クライアント側の `TASKLY_MCP_BASE_URL` には、その HTTPS URL を設定します。

## トラブルシュート

`getaddrinfo ENOTFOUND db`:

- MCP をホスト上で直接実行する場合は `DB_HOST=127.0.0.1` と `DB_PORT=3060` を使用します。
- Compose 内では `DB_HOST=db` と `DB_PORT=3306` を使用します。
- `mcp` と `db` が同じ Compose project の `default` network にあることを確認します。

`dist/stdio.js` が見つからない:

- `npm --prefix apps/mcp ci --legacy-peer-deps --no-audit --no-fund && npm --prefix apps/mcp run build` を実行します。

クライアントが disconnected のまま:

- `node` または `taskly-mcp` が PATH から実行できるか確認します。
- `TASKLY_MCP_BASE_URL` に `/healthz` を付けた URL が到達可能か確認します。
- Web UI でトークンを再発行し、`TASKLY_MCP_TOKEN` を更新します。

## セキュリティ

- トークンを設定ファイルへ平文保存する場合は、ファイル権限を所有者のみに制限します。
- トークンをログ、Issue、PR、チャットへ貼り付けないでください。
- `tasks_delete` や一括更新などの破壊的操作は、クライアント側でも実行前確認を設定してください。
