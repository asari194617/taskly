# Single-job GitHub Actions runner pool

Heavy jobs in trusted same-repository PRs and production image jobs use containerized runners. The primary
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
of jobs using the selector.
Production deployment and release-drift checks use the separate `taskly-prod`
label and its own registration volume. It is recycled after every job in the
same way as the CI slots. A separate label or container does not isolate it
from a CI slot with access to the same host Docker socket.

## Lightweight jobs on GitHub-hosted runners

The following jobs run directly on `ubuntu-latest`, independently of `CI_RUNNER`
and `CI_PRIMARY_ONLY`, so they do not compete for self-hosted build slots:

- Biome, Go vet, and Ruff lint checks;
- MCP and email-compose builds and tests;
- batch unit tests, Compose validation/image planning, and final result aggregation;
- PR coverage tests, metadata, and coverage comments, including fork comments;
- weekly Discord reports and failed-deploy notifications;
- the runner selector itself and production image planning.

Production planning still depends on the protected-main selector gate and forwards
its selected runner to image build and release jobs. API tests (including the
database suite), browser tests, E2E, and Docker image builds continue to use the
shared selector. Production SSH jobs keep the dedicated `taskly-prod` runner.

Standard GitHub-hosted runners are free for this public repository. Artifact and
cache storage have separate allowances; see [GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions).

## Primary-only guard for heavy jobs

Normal operation uses the independent hosted selector and heartbeat fallback
above. Check current repository variables, runner registrations, and recent
heartbeat/job results before changing the pool. Registration status alone does
not establish that a pool can run work.

Set `CI_PRIMARY_ONLY=true` to force heavy selected jobs to primary without
consulting heartbeats. It takes
precedence over `CI_RUNNER` and dispatch overrides, so a busy/offline primary
queues heavy jobs. Secondary heartbeats are skipped. External-fork jobs remain
GitHub-hosted. Lightweight jobs and the selector always stay GitHub-hosted, so this
guard no longer permits CI to operate through a complete hosted-runner outage.
Production release-drift checks always use `taskly-prod`. The secret scan also
always uses GitHub-hosted runners, independent of this guard.

The MCP client release workflow is deferred entirely while the guard is set:
its Windows/macOS executables need native runners. After the allowance is
restored, clear the guard and re-run any deferred release for its tag. The guard
does not expire automatically. Do not leave `CI_RUNNER=primary` set when restoring
normal fallback.

```bash
gh variable set CI_PRIMARY_ONLY --body true --repo asari194617/taskly
# Once the allowance is restored:
gh variable delete CI_PRIMARY_ONLY --repo asari194617/taskly
gh variable list --repo asari194617/taskly
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
capacity. For example, configure four primary slots only when the host has
sufficient CPU and memory:

```bash
sudo env TASKLY_RUNNER_CPUS=1.25 TASKLY_RUNNER_MEMORY=7g \
  TASKLY_RUNNER_NAME_PREFIX=taskly-ci-primary-slot \
  TASKLY_RUNNER_CI_LABEL=taskly-ci-primary \
  infra/actions-runner/install-host.sh
```

Example configuration for a smaller secondary host with one CI slot:

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

Runner selection, lightweight jobs, the final fallback, and native client builds
use standard GitHub-hosted runners. Public-repository compute is free; private
repositories consume their included allowance before additional billing applies.
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

## Investigating suspected hardware corruption

Unrelated programs crashing, invalid bytes in freshly downloaded modules, or
repeated Git pack/inflate failures can indicate host hardware problems. Stop
assigning CI and production jobs to the affected host while investigating.
Rebuilding runner containers or deleting caches does not repair failing memory.

Use an operator-approved maintenance window for memory diagnostics. Validate
substantial RAM over multiple passes, dependency integrity, application tests,
and fresh image builds before restoring slots. Keep diagnostic logs, failure
counts, host identifiers, repair evidence, and incident timelines in restricted
operations records. Online runner registrations do not establish hardware health.

## Public repository boundary

External PR creation is blocked by the repository's `collaborators_only` PR
policy. Maintainers retain their PR/review/merge workflow; repository separation
is not required for this contribution model. Keep collaborator and GitHub App
access limited to trusted actors, and do not import unreviewed external code.

The runner selector is routing logic, not an access-control boundary. If external
PRs are enabled in the future, they could edit workflow labels directly. Before
changing that policy, redesign the untrusted execution boundary. Container
recycling and distinct labels do not contain code with host Docker access.

Follow the [publication gates](../../docs/operations/repository-publication.md)
for policy readback, event-trigger review, and verification before public release.
