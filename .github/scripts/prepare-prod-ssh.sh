#!/usr/bin/env bash
set -euo pipefail
: "${PROD_HOST:?}" "${PROD_SSH_PORT:?}" "${PROD_SSH_KEY:?}" "${PROD_SSH_HOST_KEY:?}" "${GITHUB_ENV:?}"
if [[ ! "$PROD_SSH_HOST_KEY" =~ ^ssh-ed25519[[:blank:]][A-Za-z0-9+/]+={0,2}$ ]]; then
  echo 'PROD_SSH_HOST_KEY must contain one ssh-ed25519 public key without a comment' >&2
  exit 1
fi

ssh_directory=${TASKLY_PROD_SSH_DIRECTORY:-"$HOME/.ssh"}
install -m 700 -d "$ssh_directory"
umask 077
printf 'taskly-production %s\n' "$PROD_SSH_HOST_KEY" > "$ssh_directory/taskly-production-known-hosts"
if ! ssh-keygen -lf "$ssh_directory/taskly-production-known-hosts" >/dev/null; then
  echo 'Invalid production SSH host key' >&2
  exit 1
fi
cat > "$ssh_directory/config" <<'CONFIG'
Host *
  HostKeyAlias taskly-production
  HostKeyAlgorithms ssh-ed25519
  UserKnownHostsFile ~/.ssh/taskly-production-known-hosts
  GlobalKnownHostsFile /dev/null
  StrictHostKeyChecking yes
  BatchMode yes
  ConnectTimeout 15
  ConnectionAttempts 1
  ServerAliveInterval 15
  ServerAliveCountMax 3
CONFIG
eval "$(ssh-agent -s)" >/dev/null
if [[ -n ${PROD_SSH_PASSPHRASE:-} ]]; then
  askpass=$(mktemp)
  trap 'rm -f "$askpass"' EXIT
  cat > "$askpass" <<'ASKPASS'
#!/usr/bin/env sh
printf '%s\n' "$PROD_SSH_PASSPHRASE"
ASKPASS
  chmod 700 "$askpass"
  SSH_ASKPASS="$askpass" SSH_ASKPASS_REQUIRE=force DISPLAY=:0 setsid -w ssh-add - <<<"$PROD_SSH_KEY"
else
  ssh-add - <<<"$PROD_SSH_KEY"
fi
printf 'SSH_AUTH_SOCK=%s\nSSH_AGENT_PID=%s\n' "$SSH_AUTH_SOCK" "$SSH_AGENT_PID" >> "$GITHUB_ENV"
