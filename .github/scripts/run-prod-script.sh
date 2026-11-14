#!/usr/bin/env bash
# Transfer the entire trusted script before executing it; children never read source.
set -euo pipefail
: "${PROD_HOST:?}" "${PROD_USER:?}" "${PROD_SSH_PORT:?}"
seconds=${1:?remote timeout required}
shift
[[ $seconds =~ ^[1-9][0-9]{0,3}$ && $seconds -le 1800 ]] || exit 2
marker="TASKLY_SCRIPT_DONE_$(python3 -c 'import secrets; print(secrets.token_hex(16))')"
arguments=''
if (( $# )); then printf -v arguments ' %q' "$@"; fi
# The private file exists with mode 0600 before source (which may include a
# short-lived registry token) reaches disk. The outer shell owns cleanup.
remote=$(cat <<'REMOTE'
set -euo pipefail; umask 077; script=$(mktemp); trap 'rm -f -- "$script"' EXIT; trap 'exit 143' HUP INT TERM; chmod 600 "$script"; cat > "$script";
REMOTE
)
remote+=" timeout --kill-after=5 $seconds bash \"\$script\"$arguments </dev/null"
printf -v command 'bash -c %q' "$remote"
{
  cat
  # This must run as the final command IN the source file, never in its caller.
  printf "\nprintf '\\n%%s\\n' '%s'\n" "$marker"
} | ssh -p "$PROD_SSH_PORT" "$PROD_USER@$PROD_HOST" "$command" | awk -v marker="$marker" '
  $0 == marker { acknowledged++; next }
  { print }
  END {
    if (acknowledged != 1) {
      print "Remote script did not acknowledge complete execution" > "/dev/stderr"
      exit 1
    }
  }
'
