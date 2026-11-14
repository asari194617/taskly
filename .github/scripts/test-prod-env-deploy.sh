#!/usr/bin/env bash
set -euo pipefail

repo=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
assert() { "$@" || exit 1; }
python3 "$repo/.github/scripts/prepare-prod-env.test.py"
python3 "$repo/.github/scripts/verify-no-ddl-release.test.py"
python3 "$repo/.github/scripts/verify-reconcile-attestation.test.py"
bash "$repo/infra/ops/test-reconcile-code-only.sh"
bash "$repo/.github/scripts/test-prod-ssh.sh"
bash "$repo/.github/scripts/test-run-prod-script.sh"

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
if PROD_ENV='KEY=value' "$repo/.github/scripts/require-prod-env.sh" >/dev/null 2>&1; then
  echo 'Missing Airflow metadata settings were accepted' >&2
  exit 1
fi
airflow_env=$'AIRFLOW_META_DATABASE=taskly_airflow\nAIRFLOW_META_USER=taskly_airflow\nAIRFLOW_META_PASSWORD=example'
r2_env=$'R2_ENDPOINT=https://account.r2.cloudflarestorage.com\nR2_BUCKET=taskly-task-images\nR2_ACCESS_KEY_ID=example-access\nR2_SECRET_ACCESS_KEY=example-secret'
valid_env="$airflow_env"$'\n'"$r2_env"
PROD_ENV="$valid_env" "$repo/.github/scripts/require-prod-env.sh"
if PROD_ENV="$airflow_env" "$repo/.github/scripts/require-prod-env.sh" >/dev/null 2>&1; then
  echo 'Missing task image storage settings were accepted' >&2
  exit 1
fi
for key in R2_ENDPOINT R2_BUCKET R2_ACCESS_KEY_ID R2_SECRET_ACCESS_KEY; do
  missing_env=$(sed "/^${key}=/d" <<<"$valid_env")
  if PROD_ENV="$missing_env" "$repo/.github/scripts/require-prod-env.sh" >/dev/null 2>&1; then
    echo "Missing $key was accepted" >&2
    exit 1
  fi
  for value in '' '   ' '""' "''" '"   "' "'   '" '# unset'; do
    # A later empty assignment must not be hidden by an earlier valid value.
    if PROD_ENV="$valid_env"$'\n'"$key=$value" "$repo/.github/scripts/require-prod-env.sh" >"$tmp/error" 2>&1; then
      echo "Empty $key was accepted" >&2
      exit 1
    fi
    assert grep -q "PROD_ENV must set $key" "$tmp/error"
    if grep -q 'example-secret' "$tmp/error"; then
      echo 'Storage credential leaked in validation error' >&2
      exit 1
    fi
  done
  for value in '"example"' "'example'"; do
    PROD_ENV="$valid_env"$'\n'"$key=$value" "$repo/.github/scripts/require-prod-env.sh"
  done
done

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
