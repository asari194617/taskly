#!/usr/bin/env bash
set -euo pipefail
: "${PROD_HOST:?}" "${PROD_SSH_PORT:?}" "${PROD_SSH_KEY:?}" "${GITHUB_ENV:?}"
ssh_directory=${TASKLY_PROD_SSH_DIRECTORY:-"$HOME/.ssh"}
install -m 700 -d "$ssh_directory"
umask 077
if [[ -L "$ssh_directory/id_rsa" || -d "$ssh_directory/id_rsa" ]]; then
  echo "Invalid SSH key destination" >&2
  exit 1
fi
keyfile=$(mktemp "$ssh_directory/.taskly-key.XXXXXXXX")
trap 'rm -f "$keyfile" "${askpass:-}"' EXIT
chmod 600 "$keyfile"
printf '%s' "$PROD_SSH_KEY" > "$keyfile"
mv -f "$keyfile" "$ssh_directory/id_rsa"
cat > "$ssh_directory/config" <<'CONFIG'
Host *
  BatchMode yes
  ConnectTimeout 15
  ConnectionAttempts 1
  ServerAliveInterval 15
  ServerAliveCountMax 3
CONFIG
ssh-keyscan -T 15 -p "$PROD_SSH_PORT" "$PROD_HOST" >> "$ssh_directory/known_hosts" 2>/dev/null
eval "$(ssh-agent -s)" >/dev/null
if [[ -n ${PROD_SSH_PASSPHRASE:-} ]]; then
  askpass=$(mktemp)
  cat > "$askpass" <<'ASKPASS'
#!/usr/bin/env sh
printf '%s\n' "$PROD_SSH_PASSPHRASE"
ASKPASS
  chmod 700 "$askpass"
  SSH_ASKPASS="$askpass" SSH_ASKPASS_REQUIRE=force DISPLAY=:0 setsid -w ssh-add "$ssh_directory/id_rsa" </dev/null
else
  ssh-add "$ssh_directory/id_rsa"
fi
printf 'SSH_AUTH_SOCK=%s\nSSH_AGENT_PID=%s\n' "$SSH_AUTH_SOCK" "$SSH_AGENT_PID" >> "$GITHUB_ENV"
