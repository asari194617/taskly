#!/usr/bin/env bash
set -euo pipefail

failed=0

while IFS= read -r -d '' path; do
  mode=$(git ls-files --stage -- "$path" | awk 'NR == 1 { print $1 }')
  if [[ "$mode" != "100755" ]]; then
    continue
  fi

  magic=$(od -An -tx1 -N4 "$path" | tr -d ' \n')
  case "$magic" in
    7f454c46|cafebabe|bebafeca|cefaedfe|cffaedfe|feedface|feedfacf|4d5a*)
      printf 'compiled executable is tracked: %s\n' "$path" >&2
      failed=1
      ;;
  esac
done < <(git ls-files -z)

if ((failed)); then
  printf 'Remove generated executables from Git and add their paths to .gitignore.\n' >&2
  exit 1
fi

echo "No compiled executables are tracked"
