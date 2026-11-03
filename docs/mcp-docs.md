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
- `user_update`: `{ id, patch: { display_name?, notification_lang? } }`（本人のみ。`notification_lang` は `ja` または `en`）

`AUTH_PROFILE_EXTERNAL=1` の環境では表示名は taskly-auth で変更します。
MCP からの表示名更新は HTTP 409 になり、通知言語の更新は利用できます。
HTTP endpoint の詳細は [MCP README](../apps/mcp/README.md) を参照してください。

## ローカルで使う

[ローカル開発手順](local-development.md)に従って `.env` と `infra/.env` を設定し、
Atlas でスキーマを適用してから API と MCP を起動します。MCP は MySQL に加えて
Go API・Redis・email-compose に依存するため、DB と MCP だけでは利用できません。

```bash
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server mcp
curl -fsS http://localhost:3066/readyz
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

`infra/docker-compose.yml` は MCP をホストの loopback TCP `3066` に公開しますが、
TLS 終端やインターネット向けリバースプロキシは含みません。本番では次を別途構成します。

1. DNS を本番入口へ向ける。
2. 外部リバースプロキシまたはロードバランサーで HTTPS を終端する。
3. MCP の `3066` は `127.0.0.1` に限定し、Nginx 経由でのみ到達可能にする。
4. 公開 URL の `GET /readyz` が `{"ok":true}` を返すことを確認する（MySQL と Go API の利用可能性も検査）。

クライアント側の `TASKLY_MCP_BASE_URL` には、その HTTPS URL を設定します。

## トラブルシュート

`getaddrinfo ENOTFOUND db`:

- MCP をホスト上で直接実行する場合も、MySQL をホストへ公開せず Compose 内で実行するか、必要な期間だけ SSH port forwarding を使用します。
- Compose 内では `DB_HOST=db` と `DB_PORT=3306` を使用します。
- `mcp` と `db` が同じ Compose project の `default` network にあることを確認します。

`dist/stdio.js` が見つからない:

- `npm --prefix apps/mcp ci --legacy-peer-deps --no-audit --no-fund && npm --prefix apps/mcp run build` を実行します。

クライアントが disconnected のまま:

- `node` または `taskly-mcp` が PATH から実行できるか確認します。
- `TASKLY_MCP_BASE_URL` に `/readyz` を付けた URL が到達可能か確認します。
- Web UI でトークンを再発行し、`TASKLY_MCP_TOKEN` を更新します。

## セキュリティ

- トークンを設定ファイルへ平文保存する場合は、ファイル権限を所有者のみに制限します。
- トークンをログ、Issue、PR、チャットへ貼り付けないでください。
- `tasks_delete` や一括更新などの破壊的操作は、クライアント側でも実行前確認を設定してください。
