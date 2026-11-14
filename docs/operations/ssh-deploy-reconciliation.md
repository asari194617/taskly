# Interrupted code-only deployment reconciliation

The production SSH client sends a keepalive every 15 seconds and fails after three unanswered probes. Connection setup has a 15-second limit. Every production SSH step and the full deployment job also have finite deadlines. SSH, SCP and rsync share this configuration; a lost transport cannot leave the deployment waiting indefinitely.

A failed transport does not prove the remote application failed. Do not delete `.deploy-state`, run a fresh deployment over an `in_progress` release, or change the marker by hand.

## One reviewed release

`reconcile-code-only-release` is a manual-only production workflow for release `85222d44a5bf082d36d9dd781889eda73f0ff013` from deployment run `43592044959`, attempt 1. It accepts no inputs, arbitrary release SHAs, replacement artifacts or alternative hosts. It runs only from protected main, uses the existing production environment and dedicated SSH identity, and shares the ordinary deployment concurrency lock.

Before dispatch, review the failed deployment and the actual service health. The original run must have ended in failure. Its schema check, deployment marker, repository sync and environment installation must have succeeded, while its completion step must have been skipped. The workflow verifies the exact original artifact ID and SHA-256, bounded ZIP entries and JSON, matching baseline/proof, immutable target checkout, true baseline ancestry, and unchanged SQL/Atlas blobs using the existing code-only verifier. An expired or unavailable artifact blocks recovery; it must not be replaced by a guessed baseline.

On the production host, recovery requires the same source bytes, an owned mode-0600 `in_progress` marker bound to the original baseline/target/database digest, healthy stable services, the unpaused account-erasure DAG, Datadog sockets, expected Airflow executables, and the exact existing maintenance cron block. These are read-only observations: recovery does not pull/recreate images, install environment settings, run Atlas, unpause a DAG, repair cron, send notifications, or create a database. A missing invariant fails closed and needs a separately reviewed repair.

Only after every check passes does the existing deployment guard perform `complete` with the verified old baseline and database digest. It rechecks the target images and unchanged live schema before its atomic state transition. Concurrent changes and schema drift keep the marker unfinished. A second reconciliation fails because the marker is already committed.

Dispatch this workflow only after its source has passed review and CI. Confirm the workflow succeeds and the next normal deployment can read a committed baseline. The workflow is deliberately pinned to this incident; a different interrupted release requires a new reviewed change rather than broader inputs or a skip flag.
