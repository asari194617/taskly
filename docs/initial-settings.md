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

## 本番デプロイ（SSH / Docker Compose）用 GitHub Secrets

ワークフロー: `.github/workflows/deploy-prod.yml`

- PROD_HOST: 本番サーバーのホスト名 or IP（Ubuntu。Docker は事前導入済み）
- PROD_USER: SSH ユーザー名（sudo 不要で Docker を操作できるユーザー推奨）
- PROD_SSH_KEY: 秘密鍵（OpenSSH 形式、改行含む文字列）
- PROD_SSH_PASSPHRASE: 秘密鍵のパスフレーズ（パスフレーズ付き鍵を使う場合のみ）
- PROD_SSH_PORT: SSH ポート（省略可。デフォルト 22）
- PROD_WORKDIR: サーバー側の設置先ディレクトリ（例: /opt/taskly）
- PROD_ENV: `.env` の中身（オプション。未設定ならアップロードしない）
- PROD_COMPOSE_PROFILES: Docker Compose のプロファイル（例: `edge` で Traefik/TLS を有効化。オプション）

動作概要:

- main への push（apps/server, apps/mcp, batches/_, infra/_ 変更時）または手動実行で起動
- リポジトリを rsync でサーバーへ同期（.git は除外）。必要なら `.env` をアップロード
- `infra/docker-compose.yml`（必要に応じて追加 compose ファイル）で `docker compose up -d --build --force-recreate --remove-orphans` を実行
- 既存コンテナは置き換える形で再起動されます

補足:

- パスフレーズ付き鍵は `ssh-agent` + `SSH_ASKPASS` を用いて非対話的に `ssh-add` します（`PROD_SSH_PASSPHRASE` が必要）。
- Prometheus本番運用とDatadog Agentを忘れずに設定する