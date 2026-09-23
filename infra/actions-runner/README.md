# Single-job GitHub Actions runner pool

Production image jobs use four containerized runners with the `taskly-ci`
label. GitHub registrations are persistent, but every runner process accepts
only one job (`run.sh --once`). systemd then starts a new container from the
pinned image, so the workspace and container filesystem are discarded between
jobs.

Each slot:

- runs in a fresh container with 3 CPUs, 7 GiB memory, and a process limit;
- keeps only its GitHub runner credentials in a read-only Docker volume;
- mounts the host Docker socket for trusted image build jobs;
- must never run pull-request code.

Pull-request workflows stay on GitHub-hosted runners. The persistent host runner
uses the separate `taskly-prod` label and is reserved for the production deploy
job.

## Host installation

Build the pinned image and install the root-owned launchers and unit:

```bash
docker build --tag taskly-actions-runner:2.337.0 infra/actions-runner
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
why only trusted `main` image jobs may select `taskly-ci`.
