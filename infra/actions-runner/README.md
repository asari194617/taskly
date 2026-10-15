# Single-job GitHub Actions runner pool

Trusted same-repository PRs and production image jobs use containerized runners. The primary
pool has the `taskly-ci-primary` label and the existing fallback host has the
`taskly-ci-secondary` label. Up to four CI slots per host are supported. GitHub registrations are persistent, but every runner process accepts
only one job (`run.sh --once`). systemd then starts a new container from the
pinned image, so the workspace and container filesystem are discarded between
jobs. The image includes Playwright browser dependencies, Python/venv, build
tools, and Docker Compose for the CI jobs.

Each slot on the primary host:

- runs in a fresh container with 1.25 CPUs, 7 GiB memory, and a process limit;
- shares a unique temporary runner directory at the same host/container path,
  including runner externals and temporary scripts needed by child containers;
- uses the host network so published service ports are reachable at localhost;
  MySQL/Redis use random loopback ports, and real-API E2E jobs are serialized;
- serializes each workflow's image builds to limit peak disk use and removes the loaded
  CI-only Airflow image after verification;
- keeps only its GitHub runner credentials in a read-only Docker volume;
- mounts the host Docker socket for trusted image build jobs;
- accepts only trusted repository code; external fork PRs stay on GitHub-hosted runners.

The reusable `select-runner` workflow checks fresh heartbeat runs and selects the
primary pool first, the secondary pool second, then `ubuntu-latest`. Its small
selection job runs on GitHub-hosted infrastructure so it cannot be stranded by
either self-hosted pool. A heartbeat is fresh for 15 minutes; a busy or offline
pool naturally becomes ineligible when it cannot complete the scheduled check.
External fork PRs always select `ubuntu-latest`. Set the repository variable
`CI_RUNNER` to `primary`, `secondary`, or `ubuntu-latest` for an explicit override
(hosted billing applies to the last option).
The production deploy runner uses the separate `taskly-prod` label and its own
registration volume. It is recycled after every deploy job in the same way as
the CI slots.

## October 2026 hosted-minutes outage

Set the repository Actions variable `CI_PRIMARY_ONLY=true` while the October
2026 hosted allowance is exhausted. This guard moves the trusted selector job
and both production-watch jobs to `taskly-ci-primary`, and forces all downstream
Linux jobs to primary without consulting heartbeats. It takes precedence over
`CI_RUNNER` and workflow-dispatch overrides, so a busy/offline primary queues
jobs instead of falling back to a billed host. Secondary heartbeats are skipped.
External-fork selectors and jobs remain GitHub-hosted; they may be blocked by
the exhausted allowance and must never be moved to a trusted self-hosted host.

The MCP client release workflow is deferred entirely while the guard is set:
its Windows/macOS executables need native runners, and an incomplete release
must not be published. Re-run the release for its tag after restoring normal mode.

On November 1, clear `CI_PRIMARY_ONLY` (or set it to `false`) to restore the
independent GitHub-hosted selector, heartbeat-based fallback, production watch,
secondary heartbeat, and native client releases. This switch does not expire
by itself; a follow-up in the owning Codex chat is scheduled to clear it.
Do not set `CI_RUNNER=primary` for this temporary mode; leaving it unset preserves
the normal fallback policy after the guard is cleared.

```bash
gh variable set CI_PRIMARY_ONLY --body true --repo asari194617/taskly
# After the billing window resets:
gh variable delete CI_PRIMARY_ONLY --repo asari194617/taskly
```

## Host installation

Run the installer as root on an Ubuntu runner host. It installs Docker when
needed, creates the unprivileged `taskly-runner` service account, builds the
pinned image, and installs the root-owned launchers and systemd unit:

```bash
sudo infra/actions-runner/install-host.sh
```

Job workspaces are created below `/var/lib/taskly-runner/work` by default. This
keeps runner binaries, checkouts, and child-container bind mounts off `/tmp`,
which is a small tmpfs on some Ubuntu installations. The launcher installs its
cleanup trap before populating a workspace, so a failed initialization does not
leak disk space.

The default per-job limits are 3 CPUs and 7 GiB. Override them for the host's
capacity. The 5-vCPU/32-GiB primary host runs four slots with:

```bash
sudo env TASKLY_RUNNER_CPUS=1.25 TASKLY_RUNNER_MEMORY=7g \
  TASKLY_RUNNER_NAME_PREFIX=taskly-ci-primary-slot \
  TASKLY_RUNNER_CI_LABEL=taskly-ci-primary \
  infra/actions-runner/install-host.sh
```

The existing 2-vCPU secondary host runs one slot with:

```bash
sudo env TASKLY_RUNNER_CPUS=2 TASKLY_RUNNER_MEMORY=2300m \
  TASKLY_RUNNER_NAME_PREFIX=taskly-ci-secondary-slot \
  TASKLY_RUNNER_CI_LABEL=taskly-ci-secondary \
  infra/actions-runner/install-host.sh
```

Do not enable more concurrent slots than the host can support. Create a
repository runner registration token, then register the required CI slots and
the production slot. Registration tokens are short-lived and are not stored on
the host.

```bash
for slot in 1 prod; do
  printf '%s' "$REGISTRATION_TOKEN" | \
    sudo -u taskly-runner /opt/taskly-runner/register-slot.sh "$slot"
  sudo systemctl enable --now "taskly-runner@$slot.service"
done
```

CI slot names and labels come from `TASKLY_RUNNER_NAME_PREFIX` and
`TASKLY_RUNNER_CI_LABEL`. The production slot always registers as `taskly-prod`
with the `taskly-prod` label. Check registrations and services after installation:

```bash
systemctl is-active taskly-runner@1.service taskly-runner@prod.service
docker ps --filter label=taskly.actions-runner=single-job
```

The credential volumes are copied into each fresh container rather than mounted
writable. A job therefore cannot persist changes to its runner registration or
workspace. The Docker socket remains equivalent to host-level access, which is
why only trusted same-repository code may select either CI pool. Container recycling
is not a security boundary against malicious code with Docker access. External
fork code must not be copied into a trusted branch without review.

## Updating existing slots

Build the new image before installing `run-slot.sh`. Existing slots finish their
current job with the old launcher; systemd picks up the new launcher on their
next start. Allow active jobs to complete before restarting an idle slot.
Keep the previous image and launcher until CI is verified for rollback.

The host bind directory is created under the configured
`TASKLY_RUNNER_WORK_ROOT`, with a runner-owned subdirectory. The launcher
removes it when the job finishes,
including files left by root in job containers. After a forced host shutdown,
remove stale directories only after checking no runner/container uses them.

Runner selection and the final fallback consume GitHub-hosted minutes. The macOS
client build also requires a GitHub-hosted macOS runner and is billed normally.
Review workflow changes before approving any external fork run;
a PR can propose changes to its own workflow definitions.


## Stop a slot safely

Wait for the slot to become idle before stopping its systemd service. The unit's
`ExecStop` explicitly stops `taskly-ci-slot-N`; stopping only the Docker client
can leave the container online and accepting jobs. `KillMode=mixed` and the
launcher's signal handler allow its EXIT cleanup to remove the temporary
workspace. A missing container is harmless during a normal single-job exit.

After updating the launcher and unit, run `sudo systemctl daemon-reload`.
Check both `systemctl is-active taskly-runner@N.service` and
`docker ps --filter name=taskly-ci-slot-N` after stopping an idle slot.
For the production slot, use `taskly-runner@prod.service` and container name
`taskly-prod`.

## Memory corruption incident (2026-10-02)

The runner host produced illegal UTF-8 in freshly downloaded Go modules, Git
pack/inflate errors, and crashes in unrelated host programs. With all four CI
containers stopped, `sudo memtester 2G 1` tested 2 GiB of locked memory and
finished with exit code 6 and 1,391 `FAILURE:` reports, including a stuck-address
failure. The diagnostic log is `/tmp/taskly-memtest-initial.log` on the host;
copy it before rebooting because `/tmp` is tmpfs.

This is evidence of memory-subsystem corruption, not proof of which DIMM,
slot, CPU memory controller, or firmware setting is responsible. Rebuilding
runner containers or deleting Go caches does not repair it. The four CI services
were stopped and disabled with owner approval. The separate production-deploy
runner service was stopped too; its boot-time enablement was left unchanged.

Keep the host out of CI until physical/firmware diagnosis and memory testing
pass. After repair, test memory across a substantial portion of available RAM
with multiple passes, then run dependency integrity checks, API build/tests,
and image builds in a fresh runner environment before restoring all slots.
Restore CI startup only after these checks with
`sudo systemctl enable --now taskly-runner@{1..4}.service`. Start the separate
production-deploy runner only after host verification succeeds with
`sudo systemctl enable --now taskly-runner@prod.service`.
