# 初期設定メモ（CI/配信）

## Cloudflare R2 / CDN 配信に必要な GitHub Secrets

- R2_BUCKET: R2 バケット名（例: taskly-cdn）
- R2_ACCOUNT_ID: Cloudflare アカウント ID
- R2_ACCESS_KEY_ID: R2 用 Access Key ID
- R2_SECRET_ACCESS_KEY: R2 用 Secret Access Key

ワークフロー: `.github/workflows/release-mcp-client.yml`

- タグ（v1.2.3 等）を push すると、各プラットフォームのバイナリを生成し、
  `docs/mcp/clients/<tag>/` 配下へアップロード、ルートに `latest.json` / `install.*` を更新します。

## CDN 側パス

- ベース: `https://cdn.taskly.iy0306.com/docs/mcp/clients`
- 最新: `.../latest.json`, `.../install.sh`, `.../install.ps1`
- バージョン別: `.../<tag>/taskly-mcp-<os>-<arch>`, `.../<tag>/checksums.txt`

## Agent モード運用（方針）

- 本リポジトリは Gemini の Agent モード利用を前提（推奨）とします。
- リポジトリ直下の `GEMINI.md` に、エージェントの基本方針（Taskly MCP ツール優先・破壊的操作は確認）を記載済み。
- ユーザー案内は `mcp-docs.md` を参照（Agent モードの有効化手順を含む）。
- CI/配信側に Agent モード固有の設定は不要です（クライアント側の設定のみ）。
