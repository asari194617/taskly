# Single-job GitHub Actions runner pool

Trusted same-repository PRs and production image jobs use four containerized runners with the `taskly-ci`
label. GitHub registrations are persistent, but every runner process accepts
only one job (`run.sh --once`). systemd then starts a new container from the
pinned image, so the workspace and container filesystem are discarded between
jobs. The image includes Playwright browser dependencies, Python/venv, build
tools, and Docker Compose for the CI jobs.

Each slot:

- runs in a fresh container with 3 CPUs, 7 GiB memory, and a process limit;
- shares a unique temporary runner directory at the same host/container path,
  including runner externals and temporary scripts needed by child containers;
- uses the host network so published service ports are reachable at localhost;
  MySQL/Redis use random loopback ports, and real-API E2E jobs are serialized;
- keeps only its GitHub runner credentials in a read-only Docker volume;
- mounts the host Docker socket for trusted image build jobs;
- accepts only trusted repository code; external fork PRs stay on GitHub-hosted runners.

The reusable `select-runner` workflow defaults to `self-hosted, taskly-ci`, including
its own selection job, so it does not require GitHub-hosted minutes to start.
External fork PRs select `ubuntu-latest`. Set the repository variable `CI_RUNNER`
to `ubuntu-latest` for an explicit hosted override (normal billing applies).
There is no automatic paid fallback: an offline pool leaves trusted jobs queued.
The persistent host runner
uses the separate `taskly-prod` label and is reserved for the production deploy
job.

## Host installation

Build the pinned image and install the root-owned launchers and unit:

```bash
docker build --tag taskly-actions-runner:2.337.0-ci infra/actions-runner
sudo install -d -m 0755 /opt/taskly-runner
sudo install -o root -g root -m 0755 \
  infra/actions-runner/run-slot.sh \
  infra/actions-runner/register-slot.sh \
  /opt/taskly-runner/
sudo install -o root -g root -m 0644 \
  infra/actions-runner/taskly-runner@.service \
  /etc/systemd/system/taskly-runner@.service
```

Create a repository runner registration token, then register and enable each
slot. Registration tokens are short-lived and are not stored on the host.

```bash
for slot in 1 2 3 4; do
  printf '%s' "$REGISTRATION_TOKEN" | \
    /opt/taskly-runner/register-slot.sh "$slot"
  sudo systemctl enable --now "taskly-runner@$slot.service"
done
```

The credential volumes are copied into each fresh container rather than mounted
writable. A job therefore cannot persist changes to its runner registration or
workspace. The Docker socket remains equivalent to host-level access, which is
why only trusted same-repository code may select `taskly-ci`. Container recycling
is not a security boundary against malicious code with Docker access. External
fork code must not be copied into a trusted branch without review.

## Updating existing slots

Build the new image before installing `run-slot.sh`. Existing slots finish their
current job with the old launcher; systemd picks up the new launcher on their
next start. Allow active jobs to complete before restarting an idle slot.
Keep the previous image and launcher until CI is verified for rollback.

The host bind directory is created under `/tmp/taskly-ci-slot-N.*`, with a
runner-owned subdirectory. The launcher removes it when the job finishes,
including files left by root in job containers. After a forced host shutdown,
remove stale directories only after checking no runner/container uses them.

Runner startup and selection never automatically consume paid hosted minutes.
The macOS client build still requires a GitHub-hosted macOS runner and is billed
normally. Review workflow changes before approving any external fork run;
a PR can propose changes to its own workflow definitions.
