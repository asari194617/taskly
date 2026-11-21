#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
temp=$(mktemp -d)
trap 'rm -rf "$temp"' EXIT
mkdir "$temp/bin" "$temp/home"
cat > "$temp/bin/ssh-agent" <<'MOCK'
#!/usr/bin/env bash
printf 'SSH_AUTH_SOCK=/fixture/agent; export SSH_AUTH_SOCK; SSH_AGENT_PID=1; export SSH_AGENT_PID;\n'
MOCK
cat > "$temp/bin/ssh-add" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ $1 == - ]]
[[ $(cat) == "$PROD_SSH_KEY" ]]
if [[ -n ${PROD_SSH_PASSPHRASE:-} ]]; then
  [[ $SSH_ASKPASS_REQUIRE == force ]]
  [[ $("$SSH_ASKPASS") == "$PROD_SSH_PASSPHRASE" ]]
fi
MOCK
cat > "$temp/bin/setsid" <<'MOCK'
#!/usr/bin/env bash
[[ $1 == -w ]] || exit 1
shift
exec "$@"
MOCK
chmod +x "$temp/bin/"*
ssh-keygen -q -t ed25519 -N '' -f "$temp/host-key"
export PROD_SSH_HOST_KEY=$(awk '{print $1, $2}' "$temp/host-key.pub")
export PATH="$temp/bin:$PATH" TASKLY_PROD_SSH_DIRECTORY="$temp/home/.ssh" PROD_HOST=fixture.test PROD_SSH_PORT=22 PROD_SSH_KEY=synthetic-key GITHUB_ENV="$temp/env"
script="$root/.github/scripts/prepare-prod-ssh.sh"
for PROD_SSH_PASSPHRASE in '' synthetic-passphrase; do
  export PROD_SSH_PASSPHRASE
  bash "$script" > "$temp/output" 2>&1
  grep -Fxq "taskly-production $PROD_SSH_HOST_KEY" "$TASKLY_PROD_SSH_DIRECTORY/taskly-production-known-hosts"
  for setting in 'HostKeyAlias taskly-production' 'HostKeyAlgorithms ssh-ed25519' 'StrictHostKeyChecking yes' 'GlobalKnownHostsFile /dev/null' 'BatchMode yes' 'ConnectTimeout 15' 'ConnectionAttempts 1' 'ServerAliveInterval 15' 'ServerAliveCountMax 3'; do
    grep -Fxq "  $setting" "$TASKLY_PROD_SSH_DIRECTORY/config"
  done
  grep -Fxq 'SSH_AUTH_SOCK=/fixture/agent' "$GITHUB_ENV"
  [[ ! -e "$TASKLY_PROD_SSH_DIRECTORY/id_rsa" ]]
  if grep -Fq 'synthetic-' "$temp/output"; then
    echo 'SSH setup leaked credential fixture' >&2
    exit 1
  fi
done
for invalid_key in '' 'ssh-ed25519 AAAA' "$PROD_SSH_HOST_KEY"$'\nother-host ssh-ed25519 other-key'; do
  if PROD_SSH_HOST_KEY="$invalid_key" bash "$script" >/dev/null 2>&1; then
    echo 'Missing, invalid or unexpected host key accepted' >&2
    exit 1
  fi
done
echo 'Production SSH host key and agent setup: PASS'
