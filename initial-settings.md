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
