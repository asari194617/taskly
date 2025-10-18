#!/usr/bin/env bash
set -euo pipefail

APP="taskly-mcp"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"
# Cloudflare R2/配信CDNの公開URL（例: https://downloads.example.com/mcp）
BASE_URL="${BASE_URL:-https://cdn.taskly.iy0306.com/docs/mcp/clients}"

usage() {
  cat <<USAGE
Usage:
  curl -fsSL ${BASE_URL}/install.sh | bash
  curl -fsSL ${BASE_URL}/install.sh | VERSION=v1.0.0 bash
  curl -fsSL ${BASE_URL}/install.sh | bash -s -- -b ~/.local/bin -v v1.0.0
Options:
  -b <dir>   Install directory (default: ${BIN_DIR})
  -v <tag>   Version tag (default: latest)
USAGE
}

while getopts "b:v:h" opt; do
  case "$opt" in
    b) BIN_DIR="$OPTARG" ;;
    v) VERSION="$OPTARG" ;;
    h) usage; exit 0 ;;
  esac
done

uname_s=$(uname -s | tr '[:upper:]' '[:lower:]')
uname_m=$(uname -m)
case "$uname_s" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  *) echo "Unsupported OS: $uname_s"; exit 1 ;;
esac
case "$uname_m" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *) echo "Unsupported ARCH: $uname_m"; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "${BASE_URL}/latest.json" | sed -En 's/.*"tag"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p')"
  [ -n "$VERSION" ] || { echo "Failed to resolve latest version"; exit 1; }
fi

ASSET="${APP}-${OS}-${ARCH}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${ASSET} (${VERSION}) ..."
curl -fL "${BASE_URL}/${VERSION}/${ASSET}" -o "${TMP_DIR}/${APP}"

echo "Verifying checksum ..."
curl -fL "${BASE_URL}/${VERSION}/checksums.txt" -o "${TMP_DIR}/checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  grep " ${ASSET}$" "${TMP_DIR}/checksums.txt" | sha256sum -c -
else
  EXPECTED="$(grep " ${ASSET}$" "${TMP_DIR}/checksums.txt" | awk '{print $1}')"
  ACTUAL="$(shasum -a 256 "${TMP_DIR}/${APP}" | awk '{print $1}')"
  [ "$EXPECTED" = "$ACTUAL" ] || { echo "Checksum mismatch"; exit 1; }
fi

chmod +x "${TMP_DIR}/${APP}"
mkdir -p "${BIN_DIR}"
if [ -w "${BIN_DIR}" ]; then
  mv "${TMP_DIR}/${APP}" "${BIN_DIR}/${APP}"
else
  sudo mv "${TMP_DIR}/${APP}" "${BIN_DIR}/${APP}"
fi

echo "Installed ${APP} to ${BIN_DIR}/${APP}"
echo "Example Gemini settings (~/.gemini/settings.json):"
cat <<JSON
{
  "mcpServers": {
    "Taskly-server": {
      "command": "taskly-mcp",
      "args": [],
      "env": {
        "TASKLY_MCP_BASE_URL": "https://mcp.example.com", // or http://localhost:3066
        "TASKLY_MCP_TOKEN": "<YOUR_PLAIN_TOKEN>"
      }
    }
  }
}
JSON
