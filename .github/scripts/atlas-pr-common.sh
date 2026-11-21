#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "::error::$1" >&2
  exit 1
}

[[ -n "${PLAN_KEY:-}" ]] || fail 'Set the plan encryption key in the GitHub Environment first.'

artifact="$RUNNER_TEMP/taskly-atlas-plan"
mkdir -p "$artifact"
[[ ! -f "$artifact/no-changes" ]] || fail 'The selected plan had no changes.'

plan_key=$PLAN_KEY
github_token=$GH_TOKEN
unset PLAN_KEY GH_TOKEN
# GPG agent sockets have a filesystem path length limit.
temp=$(mktemp -d /tmp/taskly-atlas.XXXXXXXX)
mkdir -p "$temp/gnupg"
cleanup() {
  cleanup_tool
  gpgconf --homedir "$temp/gnupg" --kill gpg-agent >/dev/null 2>&1 || true
  rm -rf -- "$temp"
}
trap cleanup EXIT

gpg_plan() {
  gpg --no-options --homedir "$temp/gnupg" --batch --yes --pinentry-mode loopback \
    --passphrase-fd 3 "$@" 3<<<"$plan_key" >"$temp/gpg.log" 2>&1
}

check_revision() {
  GH_TOKEN="$github_token" gh api "repos/$GITHUB_REPOSITORY/collaborators/$COMMENTER_LOGIN/permission" |
    jq -e --arg id "$COMMENTER_ID" '
      (.user.id | tostring) == $id and (.permission == "admin" or .permission == "write")
    ' >/dev/null ||
    fail 'The commenter no longer has repository write access.'

  GH_TOKEN="$github_token" gh api "repos/$GITHUB_REPOSITORY/pulls/$PR_NUMBER" |
    jq -e --arg repo "$GITHUB_REPOSITORY" --arg head "$HEAD_SHA" --arg base "$BASE_SHA" \
      --arg branch "$BASE_BRANCH" '
        .state == "open" and .draft == false and .head.repo.full_name == $repo and
        .base.ref == $branch and .head.sha == $head and .base.sha == $base
      ' >/dev/null ||
    fail 'The PR or default branch changed. Update the PR and request a new plan.'
}

mark_no_changes() {
  touch "$artifact/no-changes"
  echo 'changes=false' >>"$GITHUB_OUTPUT"
}
