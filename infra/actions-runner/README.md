# Ephemeral GitHub Actions runners

Production image jobs use just-in-time GitHub Actions runners. Each runner:

- is registered for one job only with the `taskly-ci` label;
- runs in a fresh container and is removed when the job exits;
- has 3 CPUs, 7 GiB memory, and a process limit;
- is provisioned only by the trusted `deploy-prod` workflow;
- mounts the host Docker socket, so it must never run pull-request code.

Pull-request workflows stay on GitHub-hosted runners. The persistent host runner
uses the separate `taskly-prod` label and is reserved for the production deploy
job.

## Host installation

Build the pinned image and install the root-owned launcher:

```bash
docker build --tag taskly-actions-runner:2.337.0 infra/actions-runner
sudo install -d -m 0755 /opt/taskly-runner
sudo install -o root -g root -m 0755 \
  infra/actions-runner/start-jit-runner.sh \
  /opt/taskly-runner/start-jit-runner.sh
```

The repository must define `RUNNER_POOL_HOST`, `RUNNER_POOL_USER`,
`RUNNER_POOL_PORT`, and `RUNNER_POOL_SSH_KEY`. `RUNNER_FALLBACK_TOKEN` must
have repository Administration write permission so the workflow can request
short-lived JIT configurations.
