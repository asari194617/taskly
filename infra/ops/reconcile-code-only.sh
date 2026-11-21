#!/usr/bin/env bash
# Streamed by the reviewed manual workflow, executed only as the production SSH user.
set -euo pipefail
workdir=${1:?workdir required}
release=${2:?release required}
baseline=${3:?baseline required}
digest=${4:?database digest required}
fail() { echo 'Reconciliation blocked: a release invariant failed; no completion written' >&2; exit 1; }
[[ $# == 10 && $workdir == /* && $release =~ ^[0-9a-f]{40}$ && $baseline =~ ^[0-9a-f]{40}$ && $digest =~ ^[0-9a-f]{64}$ ]] || fail
hashes=("$5" "$6" "$7" "$8" "$9" "${10}")
for hash in "${hashes[@]}"; do [[ $hash =~ ^[0-9a-f]{64}$ ]] || fail; done
cd "$workdir/infra"
[[ $(pwd -P) == "$workdir/infra" && ! -L ops && ! -L ../apps && ! -L ../apps/server ]] || fail
export COMPOSE_FILE=docker-compose.yml:docker-compose.prod.yml
# Refuse host files that differ from the immutable target checkout selected by
# the owned schema-v2 marker. The workflow passes only bounded SHA-256 values.
relatives=(
  ops/deploy-state.sh
  ops/verify-deploy-health.sh
  ops/disk-maintenance.sh
  ops/monitoring-watchdog.sh
  ../apps/server/schema.sql
  ../atlas.hcl
)
for index in "${!relatives[@]}"; do
  expected=${hashes[$index]}
  relative=${relatives[$index]}
  [[ ! -L $relative && -f $relative && $(sha256sum "$relative" | cut -d' ' -f1) == "$expected" ]] || fail
done
python3 - "$workdir/.deploy-state" "$release" "$baseline" "$digest" <<'PYTHON'
import os, re, stat, sys
from pathlib import Path
p = Path(sys.argv[1])
s = p.lstat()
if not stat.S_ISREG(s.st_mode) or s.st_uid != os.getuid() or stat.S_IMODE(s.st_mode) != 0o600 or s.st_size > 1024:
    sys.exit(1)
lines = p.read_text().splitlines()
if (len(lines) != 6 or lines[:5] != ['in_progress', sys.argv[3], sys.argv[2], sys.argv[4], 'schema-v2']
        or re.fullmatch(r'[0-9a-f]{40}', lines[5]) is None):
    sys.exit(1)
PYTHON
# All commands before complete are observations only; never unpause or reinstall.
./ops/verify-deploy-health.sh >/dev/null 2>&1 || fail
docker compose exec -T airflow-scheduler airflow dags details account_erasure -o plain </dev/null \
  | grep -Eq '^is_paused +False$' || fail
docker compose exec -T server curl --fail --silent --show-error --max-time 10 \
  --unix-socket /var/run/datadog/apm.socket http://localhost/info </dev/null >/dev/null 2>&1 || fail
docker compose exec -T email-compose node -e '
  const socket = require("node:net").connect("/var/run/datadog/apm.socket");
  const timeout = setTimeout(() => process.exit(1), 2000);
  socket.on("connect", () => { clearTimeout(timeout); socket.end(); });
  socket.on("close", () => process.exit(0));
  socket.on("error", () => process.exit(1));
' </dev/null >/dev/null 2>&1 || fail
docker compose exec -T airflow-scheduler sh -ec '
  test -x /opt/airflow/bin/notifications-que-manager
  test -x /opt/airflow/bin/backup-mysql
  test -x /opt/airflow/bin/moderation
  mysqldump --version | grep -q "MySQL Community Server"
' </dev/null >/dev/null 2>&1 || fail
# Compare the exact existing cron block without changing or printing any entry.
crontab -l | python3 -c '
import sys
workdir = sys.argv[1]
lines = sys.stdin.read().splitlines()
begin = "# BEGIN taskly disk maintenance"
end = "# END taskly disk maintenance"
if lines.count(begin) != 1 or lines.count(end) != 1:
    sys.exit(1)
expected = [begin,
    "*/5 * * * * cd " + chr(39) + workdir + "/infra" + chr(39) + " && bash -o pipefail -c " + chr(39) + "./ops/disk-maintenance.sh status 2>&1 | logger -t taskly-disk" + chr(39),
    "* * * * * cd " + chr(39) + workdir + "/infra" + chr(39) + " && ./ops/monitoring-watchdog.sh 2>&1 | logger -t taskly-monitoring",
    "25 3 * * * cd " + chr(39) + workdir + "/infra" + chr(39) + " && bash -o pipefail -c " + chr(39) + "./ops/disk-maintenance.sh cleanup 2>&1 | logger -t taskly-disk" + chr(39), end]
start = lines.index(begin)
if lines[start:start + len(expected)] != expected:
    sys.exit(1)
' "$workdir" || fail
# Only the existing state guard may perform the final atomic state transition.
bash ./ops/deploy-state.sh complete "$workdir" "$release" "$baseline" "$digest"
