#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
temp=$(mktemp -d)
trap 'rm -rf "$temp"' EXIT
mkdir "$temp/bin" "$temp/home"
cat > "$temp/bin/ssh-keyscan" <<'MOCK'
#!/usr/bin/env bash
[[ $1 == -T && $2 == 15 ]] || exit 1
[[ ${MOCK_FAIL_SCAN:-0} == 0 ]] || exit 1
printf 'fixture-host fixture-key\n'
MOCK
cat > "$temp/bin/ssh-agent" <<'MOCK'
#!/usr/bin/env bash
printf 'SSH_AUTH_SOCK=/fixture/agent; export SSH_AUTH_SOCK; SSH_AGENT_PID=1; export SSH_AGENT_PID;\n'
MOCK
cat > "$temp/bin/ssh-add" <<'MOCK'
#!/usr/bin/env bash
if [[ -n ${PROD_SSH_PASSPHRASE:-} ]]; then
 [[ $SSH_ASKPASS_REQUIRE == force ]]
 [[ $("$SSH_ASKPASS") == "$PROD_SSH_PASSPHRASE" ]]
fi
MOCK
cat > "$temp/bin/setsid" <<'MOCK'
#!/usr/bin/env bash
[[ $1 == -w ]]; shift; exec "$@"
MOCK
chmod +x "$temp/bin/"*
export PATH="$temp/bin:$PATH" TASKLY_PROD_SSH_DIRECTORY="$temp/home/.ssh" PROD_HOST=fixture.test PROD_SSH_PORT=22 PROD_SSH_KEY=synthetic-key GITHUB_ENV="$temp/env"
mkdir -p "$TASKLY_PROD_SSH_DIRECTORY"
printf 'old-fixture' > "$TASKLY_PROD_SSH_DIRECTORY/id_rsa"
chmod 644 "$TASKLY_PROD_SSH_DIRECTORY/id_rsa"
for PROD_SSH_PASSPHRASE in '' synthetic-passphrase; do
 export PROD_SSH_PASSPHRASE
 bash "$root/.github/scripts/prepare-prod-ssh.sh" > "$temp/output" 2>&1
 for setting in 'BatchMode yes' 'ConnectTimeout 15' 'ConnectionAttempts 1' 'ServerAliveInterval 15' 'ServerAliveCountMax 3'; do grep -Fq "$setting" "$TASKLY_PROD_SSH_DIRECTORY/config"; done
 if grep -q 'synthetic-' "$temp/output"; then echo 'SSH setup leaked credential fixture' >&2; exit 1; fi
done
python3 - "$TASKLY_PROD_SSH_DIRECTORY/id_rsa" <<'PYTHON'
from pathlib import Path
import sys
assert Path(sys.argv[1]).stat().st_mode & 0o777 == 0o600
PYTHON
mv "$TASKLY_PROD_SSH_DIRECTORY/id_rsa" "$temp/key-victim"
ln -s "$temp/key-victim" "$TASKLY_PROD_SSH_DIRECTORY/id_rsa"
if bash "$root/.github/scripts/prepare-prod-ssh.sh" >/dev/null 2>&1; then echo 'Symlink key destination accepted' >&2; exit 1; fi
[[ $(cat "$temp/key-victim") == synthetic-key ]]
rm "$TASKLY_PROD_SSH_DIRECTORY/id_rsa"
if MOCK_FAIL_SCAN=1 bash "$root/.github/scripts/prepare-prod-ssh.sh" >/dev/null 2>&1; then echo 'Failed key discovery was accepted' >&2; exit 1; fi
echo 'Bounded production SSH setup: PASS'
