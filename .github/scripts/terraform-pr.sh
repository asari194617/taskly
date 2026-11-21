#!/usr/bin/env bash
set -euo pipefail

cleanup_tool() { :; }
# shellcheck source=.github/scripts/infra-pr-common.sh
source "$(dirname "${BASH_SOURCE[0]}")/infra-pr-common.sh"

command=${1:?Expected plan or apply}
root="$GITHUB_WORKSPACE/proposal/terraform"
shopt -s nullglob
configs=("$root"/*.tf "$root"/*.tf.json)
(( ${#configs[@]} > 0 )) || fail 'Add a Terraform root module at terraform/ first.'
[[ ! -L "$root" ]] ||
  fail 'The Terraform root module must be a directory in the PR checkout.'
export TF_DATA_DIR="$temp/terraform"

run_terraform() {
  env -u GITHUB_ENV -u GITHUB_OUTPUT -u GITHUB_PATH -u GITHUB_STEP_SUMMARY -u GITHUB_STATE \
    terraform -chdir="$root" "$@"
}

check_revision
run_terraform init -input=false -no-color -lockfile=readonly >"$temp/init.log" 2>&1 ||
  fail 'Terraform init failed. Check the backend, credentials and provider lock file.'
# Keep state off the ephemeral runner. Explicit and implicit local backends are both rejected.
jq -e '.backend.type | type == "string" and . != "local"' "$TF_DATA_DIR/terraform.tfstate" \
  >/dev/null 2>&1 || fail 'Configure a remote Terraform backend.'

case "$command" in
  plan)
    status=0
    run_terraform plan -input=false -no-color -lock-timeout=60s -detailed-exitcode \
      -out="$temp/plan.tfplan" >"$temp/plan.txt" 2>&1 || status=$?
    case "$status" in
      0) mark_no_changes ;;
      2)
        check_revision
        gpg_plan --symmetric --cipher-algo AES256 --output "$artifact/plan.tfplan.gpg" "$temp/plan.tfplan" ||
          fail 'Could not encrypt the saved plan.'
        gpg_plan --symmetric --cipher-algo AES256 --output "$artifact/plan.txt.gpg" "$temp/plan.txt" ||
          fail 'Could not encrypt the plan output.'
        echo 'changes=true' >>"$GITHUB_OUTPUT"
        ;;
      *) fail 'Terraform plan failed. Check the module and backend in a private terminal.' ;;
    esac
    ;;
  apply)
    gpg_plan --decrypt --output "$temp/plan.tfplan" "$artifact/plan.tfplan.gpg" ||
      fail 'Could not decrypt the saved plan. Check TERRAFORM_PLAN_KEY and request a new plan.'
    check_revision
    run_terraform apply -input=false -no-color -lock-timeout=60s "$temp/plan.tfplan" \
      >"$temp/apply.log" 2>&1 ||
      fail 'Terraform apply failed. Inspect state and request a new plan before retrying.'
    ;;
  *) fail 'Expected plan or apply.' ;;
esac
