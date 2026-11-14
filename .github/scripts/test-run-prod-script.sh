#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
temp=$(mktemp -d)
trap 'rm -rf "$temp"' EXIT
mkdir "$temp/bin"
cat > "$temp/bin/ssh" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
case ${MOCK_SSH_MODE:-normal} in
 normal) exec bash -c "${*: -1}" ;;
 missing) bash -c "${*: -1}" | sed '/^TASKLY_SCRIPT_DONE_/d' ;;
 duplicate) bash -c "${*: -1}" | awk '{print; if ($0 ~ /^TASKLY_SCRIPT_DONE_/) print}' ;;
esac
MOCK
chmod +x "$temp/bin/ssh"
export PATH="$temp/bin:$PATH" PROD_HOST=fixture.test PROD_USER=fixture PROD_SSH_PORT=22
export MOCK_REMOTE_PATH="$temp/remote-path" MOCK_COMPLETE="$temp/complete" MOCK_ARGS="$temp/args"
cat > "$temp/source" <<'SOURCE'
set -euo pipefail
printf '%s\n' "$0" > "$MOCK_REMOTE_PATH"
python3 - "$0" "$MOCK_ARGS" "$@" <<'PYTHON'
import json,sys
from pathlib import Path
assert Path(sys.argv[1]).stat().st_mode & 0o777 == 0o600
Path(sys.argv[2]).write_text(json.dumps(sys.argv[3:]))
PYTHON
cat >/dev/null
printf 'fixture-output\n'
touch "$MOCK_COMPLETE"
SOURCE
argument=$(cat <<'ARGUMENT'
literal $(touch forbidden) ; spaces
ARGUMENT
)
bash "$root/.github/scripts/run-prod-script.sh" 5 "$argument" < "$temp/source" > "$temp/output"
grep -qx fixture-output "$temp/output"
[[ -e $MOCK_COMPLETE && ! -e $(cat "$MOCK_REMOTE_PATH") ]]
python3 - "$temp/args" "$argument" <<'PYTHON'
import json,sys
from pathlib import Path
assert json.loads(Path(sys.argv[1]).read_text()) == [sys.argv[2]]
PYTHON
for mode in missing duplicate; do
 if MOCK_SSH_MODE=$mode bash "$root/.github/scripts/run-prod-script.sh" 5 < "$temp/source" > "$temp/output" 2>&1; then echo "Invalid acknowledgement accepted: $mode" >&2; exit 1; fi
 [[ ! -e $(cat "$MOCK_REMOTE_PATH") ]]
done
for outcome in 'exit 0' 'exit 7' 'sleep 5'; do
 {
  cat <<'PREFIX'
printf '%s\n' "$0" > "$MOCK_REMOTE_PATH"
PREFIX
  printf '%s\n' "$outcome"
 } > "$temp/failure-source"
 if bash "$root/.github/scripts/run-prod-script.sh" 1 < "$temp/failure-source" > "$temp/output" 2>&1; then echo "Incomplete script accepted: $outcome" >&2; exit 1; fi
 [[ ! -e $(cat "$MOCK_REMOTE_PATH") ]]
done
for timeout in 0 1801 invalid; do
 if bash "$root/.github/scripts/run-prod-script.sh" "$timeout" < "$temp/source" > "$temp/output" 2>&1; then echo 'Invalid timeout accepted' >&2; exit 1; fi
done
echo 'Private script transfer, stdin isolation, final acknowledgement and cleanup: PASS'
