#!/usr/bin/env bash
set -euo pipefail

# Query from Docker's bridge, where the isolated BuildKit container resolves
# Docker Hub. The host resolver can succeed while bridge DNS is broken.
for attempt in 1 2 3; do
  if docker run --rm --network bridge --pull never alpine:3.20 \
    sh -ec 'nslookup registry-1.docker.io >/dev/null && nslookup auth.docker.io >/dev/null'; then
    echo "Docker bridge can resolve Docker Hub (attempt $attempt)."
    exit 0
  fi
  if (( attempt < 3 )); then
    echo "Docker Hub DNS failed from Docker bridge; retrying in 15 seconds ($attempt/3)." >&2
    sleep 15
  fi
done

echo 'Docker Hub DNS failed from Docker bridge after three attempts.' >&2
exit 1
