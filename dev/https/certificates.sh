#!/usr/bin/env bash
set -euo pipefail

config_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${TASKLY_DEV_TLS_DIR:=${HOME}/.local/share/taskly/https}"
umask 077
mkdir -p -- "$TASKLY_DEV_TLS_DIR"

hosts=()
while read -r address host; do
  [[ "$address" == 127.0.0.1 && -n "$host" ]] || continue
  hosts+=("$host")
done < "$config_dir/hosts"

mkcert -install
mkcert -cert-file "$TASKLY_DEV_TLS_DIR/taskly.pem" \
  -key-file "$TASKLY_DEV_TLS_DIR/taskly-key.pem" "${hosts[@]}"
cp -- "$(mkcert -CAROOT)/rootCA.pem" "$TASKLY_DEV_TLS_DIR/rootCA.pem"
chmod 644 "$TASKLY_DEV_TLS_DIR/rootCA.pem"
