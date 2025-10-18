# mcp-client-files

Taskly MCP クライアント配布用ファイル群（Cloudflare R2 配信前提）。

含まれるもの（CI で生成/配布する想定）

- taskly-mcp-darwin-arm64
- taskly-mcp-darwin-amd64
- taskly-mcp-linux-arm64
- taskly-mcp-linux-amd64
- taskly-mcp-windows-amd64.exe
- checksums.txt（各アーティファクトの SHA256）
- install.sh（UNIX/macOS 向けインストーラ）
- install.ps1（Windows PowerShell 向けインストーラ）
- LICENSE / この README

## インストール（macOS/Linux）

R2/CDN の公開 URL を BASE_URL に設定して配布します（運用 URL: https://cdn.taskly.iy0306.com/docs/mcp/clients）。

最新版:

```bash
curl -fsSL https://cdn.taskly.iy0306.com/docs/mcp/clients/install.sh | bash
```

特定バージョン:

```bash
curl -fsSL https://cdn.taskly.iy0306.com/docs/mcp/clients/install.sh | VERSION=v1.0.0 bash
```

インストール先を変更:

```bash
curl -fsSL https://cdn.taskly.iy0306.com/docs/mcp/clients/install.sh | bash -s -- -b "$HOME/.local/bin"
```

## インストール（Windows）

PowerShell（管理者）で:

```powershell
Invoke-WebRequest https://cdn.taskly.iy0306.com/docs/mcp/clients/install.ps1 -OutFile install.ps1
.\install.ps1 -BaseUrl https://cdn.taskly.iy0306.com/docs/mcp/clients
```

## Gemini 設定例

```json
{
  "mcpServers": {
    "Taskly-server": {
      "command": "taskly-mcp",
      "env": {
        "TASKLY_MCP_BASE_URL": "https://mcp.example.com", // または http://localhost:3066
        "TASKLY_MCP_TOKEN": "<YOUR_PLAIN_TOKEN>"
      }
    }
  }
}
```

## 配信運用メモ

- R2 バケットは公開読み取り（または CDN 経由）に設定
- ルートに `install.sh` / `install.ps1` / `latest.json` を配置
- `<tag>/` 配下に各プラットフォームのバイナリと `checksums.txt`
- CI から自動アップロード（.github/workflows を別途参照）
