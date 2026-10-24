#!/usr/bin/env bash
set -euo pipefail

if [[ $(id -u) -ne 0 ]]; then
  echo "install-host.sh must run as root" >&2
  exit 1
fi

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
runner_user=${TASKLY_RUNNER_USER:-taskly-runner}
runner_cpus=${TASKLY_RUNNER_CPUS:-3}
runner_memory=${TASKLY_RUNNER_MEMORY:-7g}
runner_pids_limit=${TASKLY_RUNNER_PIDS_LIMIT:-2048}
image=${TASKLY_RUNNER_IMAGE:-taskly-actions-runner:2.338.1-ci}
workspace_root=${TASKLY_RUNNER_WORK_ROOT:-/var/lib/taskly-runner/work}
runner_name_prefix=${TASKLY_RUNNER_NAME_PREFIX:-taskly-ci-slot}
runner_ci_label=${TASKLY_RUNNER_CI_LABEL:-taskly-ci}

for value in "$runner_name_prefix" "$runner_ci_label"; do
  if [[ ! "$value" =~ ^[A-Za-z0-9._-]+$ ]]; then
    echo "Runner names and labels may contain only letters, digits, dots, underscores, and hyphens" >&2
    exit 1
  fi
done

if ! command -v docker >/dev/null 2>&1; then
  apt-get update
  apt-get install -y docker.io
fi
systemctl enable --now docker.service

if ! id "$runner_user" >/dev/null 2>&1; then
  useradd --system --create-home --shell /usr/sbin/nologin "$runner_user"
fi
usermod --append --groups docker "$runner_user"
install -d -o "$runner_user" -g "$runner_user" -m 0755 "$workspace_root"

docker build --tag "$image" "$root/infra/actions-runner"

install -d -m 0755 /opt/taskly-runner /etc/taskly-runner
install -o root -g root -m 0755 \
  "$root/infra/actions-runner/run-slot.sh" \
  "$root/infra/actions-runner/register-slot.sh" \
  "$root/infra/actions-runner/stop-slot.sh" \
  /opt/taskly-runner/
install -o root -g root -m 0644 \
  "$root/infra/actions-runner/taskly-runner@.service" \
  /etc/systemd/system/taskly-runner@.service

cat > /etc/taskly-runner/runner.env <<EOF
TASKLY_RUNNER_IMAGE=$image
TASKLY_RUNNER_CPUS=$runner_cpus
TASKLY_RUNNER_MEMORY=$runner_memory
TASKLY_RUNNER_PIDS_LIMIT=$runner_pids_limit
TASKLY_RUNNER_WORK_ROOT=$workspace_root
TASKLY_RUNNER_NAME_PREFIX=$runner_name_prefix
TASKLY_RUNNER_CI_LABEL=$runner_ci_label
EOF
chmod 0644 /etc/taskly-runner/runner.env

systemctl daemon-reload

echo "Runner host installed. Register slots before enabling their services."
