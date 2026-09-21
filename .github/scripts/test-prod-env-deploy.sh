#!/usr/bin/env bash
set -euo pipefail

repo=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
assert() { "$@" || exit 1; }
mode() {
  if stat -c %a "$1" >/dev/null 2>&1; then stat -c %a "$1"; else stat -f %Lp "$1"; fi
}

if env -u PROD_ENV "$repo/.github/scripts/require-prod-env.sh" >/dev/null 2>&1; then
  echo 'Missing PROD_ENV was accepted' >&2
  exit 1
fi
if PROD_ENV='' "$repo/.github/scripts/require-prod-env.sh" >/dev/null 2>&1; then
  echo 'Empty PROD_ENV was accepted' >&2
  exit 1
fi
if PROD_ENV='   ' "$repo/.github/scripts/require-prod-env.sh" >/dev/null 2>&1; then
  echo 'Blank PROD_ENV was accepted' >&2
  exit 1
fi
PROD_ENV='KEY=value' "$repo/.github/scripts/require-prod-env.sh"

mkdir -p "$tmp/source/infra" "$tmp/remote/infra" "$tmp/bin"
printf 'old root\n' > "$tmp/remote/.env"
printf 'old infra\n' > "$tmp/remote/infra/.env"
printf 'remove me\n' > "$tmp/remote/stale"
rsync -a --delete --exclude '/.env' --exclude '/infra/.env' \
  "$tmp/source/" "$tmp/remote/"
assert test "$(cat "$tmp/remote/.env")" = 'old root'
assert test "$(cat "$tmp/remote/infra/.env")" = 'old infra'
assert test ! -e "$tmp/remote/stale"

cat > "$tmp/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ $1 == compose ]]
shift
precheck=false
if [[ ${1:-} == --env-file ]]; then
  precheck=true
  file=$2
  shift 2
else
  file=../.env
fi
[[ $1 == config && $2 == -q ]]
if [[ ${FAIL_AFTER_SWAP:-} == 1 && $precheck == false ]]; then exit 1; fi
! grep -q '^INVALID=1$' "$file"
EOF
chmod +x "$tmp/bin/docker"
export PATH="$tmp/bin:$PATH"

stage=$(mktemp "$tmp/remote/.env.upload.XXXXXXXX")
printf 'KEY=new\n' > "$stage"
"$repo/infra/ops/install-prod-env.sh" "$tmp/remote" "$stage" test-sha ''
assert test "$(cat "$tmp/remote/.env")" = 'KEY=new'
assert test "$(cat "$tmp/remote/infra/.env")" = 'KEY=new'
assert test "$(mode "$tmp/remote/.env")" = 600
assert test "$(mode "$tmp/remote/infra/.env")" = 600

stage=$(mktemp "$tmp/remote/.env.upload.XXXXXXXX")
printf 'INVALID=1\n' > "$stage"
if "$repo/infra/ops/install-prod-env.sh" "$tmp/remote" "$stage" test-sha '' >/dev/null 2>&1; then
  echo 'Invalid Compose configuration was accepted' >&2
  exit 1
fi
assert test "$(cat "$tmp/remote/.env")" = 'KEY=new'
assert test "$(cat "$tmp/remote/infra/.env")" = 'KEY=new'

stage=$(mktemp "$tmp/remote/.env.upload.XXXXXXXX")
printf 'KEY=rollback\n' > "$stage"
if FAIL_AFTER_SWAP=1 "$repo/infra/ops/install-prod-env.sh" "$tmp/remote" "$stage" test-sha '' >/dev/null 2>&1; then
  echo 'Post-install Compose failure was accepted' >&2
  exit 1
fi
assert test "$(cat "$tmp/remote/.env")" = 'KEY=new'
assert test "$(cat "$tmp/remote/infra/.env")" = 'KEY=new'

echo 'Production environment deploy checks passed'
