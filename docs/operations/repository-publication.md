# Repository publication controls and audit

Track the release decision in [#445](https://github.com/asari194617/taskly/issues/445).
The current visibility decision and transition evidence are recorded there.
Publishing source does not change the licensing terms in `LICENSE`.

## Final content audit (2026-11-06)

- A fresh remote mirror contained 159 branch refs, 9 tag refs, 180 pull refs,
  and 1,281 reachable commits; its pack was about 5 MiB. No compiled executable
  signatures or the eight removed incident/extraction/setup document paths were
  found in those reachable objects. This supersedes the older observation that
  advertised pull refs retained large binaries; it does not prove that GitHub's
  cached diffs or unadvertised objects have been purged.
- Gitleaks 8.30.1 scanned all fetched refs with the five previously reviewed,
  exact fingerprints in `.gitleaksignore`: no additional findings. The two
  previously identified legacy commit objects returned HTTP 404 through GitHub's
  Git commit API. This is an availability observation, not proof of server erasure.
- All 676 PRs were inventoried: no cross-repository PRs. GitHub returned 179
  diffs, which were scanned; 497 old diffs returned HTTP 422 after the history
  rewrite and could not be scanned. Issue/PR bodies, comments, review metadata,
  and five asset-free releases were also inspected; metadata secret scans were
  clean. No wiki repository was available to the authenticated owner.
- Of 1,828 completed Actions runs, 1,208 log archives were available and 620
  returned HTTP 410. All 727 artifacts listed at the final snapshot were fetched
  and inspected, along with previously fetched artifacts that expired during the
  audit. Archive entries were extracted without trusting their path names;
  nested ZIP/gzip/tar/zstd payloads were decoded and binary strings inspected.
  Text was materialized with a scanner-supported extension so `.bin` names did
  not silently bypass Gitleaks. There were no remaining extraction errors.
- The expanded diff/log/artifact scan covered about 344 MB and returned 369
  candidates: 211 container image commit tags, 32 Go checksum contexts, 90
  localhost-only CI fixture headers, and 36 occurrences of expired Docker Hub
  image-download signatures. The latter were upstream S3 download URLs with a
  20-minute lifetime ending on November 2, not Taskly credentials. Every candidate
  was classified; no unresolved credential finding remains. No new broad scanner
  exclusions were added to the repository.
- Known production and runner connection identifiers were absent from reachable
  Git objects and the inspected metadata, diffs, logs, and artifacts. Public
  contact/author addresses retain the policy in #439. Raw archives, findings,
  and recovery evidence remain in restricted local storage, outside this repo.
- Cache metadata and workflow cache paths were reviewed: Go/Node dependency
  caches and Docker build caches, with environment files excluded from build
  contexts. Cache payload bytes were not downloaded or scanned. Anonymous access
  to the four application GHCR packages returned HTTP 401; publishing source does
  not authorize changing package visibility. Package administration and the full
  GitHub App installation inventory were unavailable with the audit token's
  scopes. Existing workflow integrations and webhook destinations were reviewed;
  re-audit any future integration or collaborator change.

## Prepared controls

- The main ruleset requires signed commits, PRs, six application checks, and
  `secret-scan`, with no bypass actors. Actions defaults to a read-only token and
  requires full action SHA pins. Production credentials are Environment secrets;
  the repository secret names contain no `PROD_*` or task-image storage secrets.
- The production Environment restricts deployments to protected branches and
  disallows administrator bypass. Reviewer/wait-timer protection is still absent.
  Public fork approval and native security features still require verification
  during the visibility transition.

These are dated observations, not a permanent authorization to publish. The
`secret-scan` workflow checks fetched history on PRs and main pushes using a
checksum-verified Gitleaks binary, read-only token, redacted output, and a
GitHub-hosted runner. It supplements GitHub's native security features. It does
not enumerate GitHub issues, old PR caches, logs, artifacts, or hidden refs.

## PR admission and runner boundary

The publication plan keeps this repository and its production deployment together.
External code contributions are not accepted. GitHub's repository setting must
remain `has_pull_requests=true` and
`pull_request_creation_policy=collaborators_only`: maintainers retain the required
PR/review/merge workflow, while outsiders cannot create PRs. This is an enforced
repository setting, not just the policy in `CONTRIBUTING.md`. See
[GitHub's PR access documentation](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/disabling-pull-requests).

On 2026-11-04 the setting was applied and read back through the REST API. The
collaborator inventory contained only the owner, and the only open PR was from
the owner in this repository. There were no open external PRs to grandfather
into the restricted policy. Recheck both inventories immediately before publishing.

The shared selector's hosted fallback is defense in depth. Runner labels and
container recycling do not isolate malicious code: a job with the host Docker
socket can reach other containers and volumes. Only trusted maintainers may
submit executable code to these runners. Review any collaborator invitation,
GitHub App access change, and imported external code with that trust in mind.
The production guards, dedicated runner, and protected Environment remain in use.

Audit event triggers as well as PR creation. Public issue/comment/review content
must not start privileged jobs or be evaluated as code. The current workflows
have no `issues`, `issue_comment`, `pull_request_review`, or
`pull_request_target` triggers. Existing `workflow_run` consumers require their
own input validation; restricting new PRs does not sanitize old artifacts.

Before enabling external PRs in the future, redesign the untrusted execution
boundary and validate it first. A private deployment repository is an option for
that future contribution model, not a prerequisite for the current publication
plan with external PR creation blocked. GitHub's
[self-hosted runner guidance](https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions#hardening-for-self-hosted-runners)
still applies to every actor allowed to execute workflows.

## Repeatable final audit

Use a new restricted directory, not an existing developer clone: local stale
tags and worktrees can retain history that the remote no longer advertises.

```bash
umask 077
audit_dir=$(mktemp -d)
git clone --mirror https://github.com/asari194617/taskly.git "$audit_dir/repository.git"
git -C "$audit_dir/repository.git" fsck --full
git -C "$audit_dir/repository.git" for-each-ref --format='%(refname)' \
  > "$audit_dir/refs.txt"
git -C "$audit_dir/repository.git" count-objects -vH
# Run from a reviewed checkout containing .gitleaksignore; install Gitleaks 8.30.1.
gitleaks git "$audit_dir/repository.git" --log-opts=--all --redact \
  --ignore-gitleaks-allow --gitleaks-ignore-path=.gitleaksignore \
  --report-format=json --report-path="$audit_dir/secrets.json"
```

Inspect binary signatures and removed document paths across reachable blobs,
not only the main tree. Review production identifiers and personal data separately;
a clean secret scan is not a privacy audit. Keep raw findings, old operations
documents, credentials, and recovery bundles outside the repository. The existing
public contact address and author address follow the decision recorded in #439.

Also audit open/closed issues, PR bodies/diffs/reviews, comments, releases, wiki,
Actions logs/artifacts/caches, and packages. Do not infer safety from retention
settings or a smaller clone. GitHub-owned pull refs cannot be deleted by pushing;
GitHub Support only assists with qualifying sensitive-data removal, not arbitrary
binary cleanup. See [GitHub's removal procedure](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository).

## Visibility transition

1. Complete the content audit. Preserve restricted recovery evidence and record
   the reviewed commit/ref inventory. Verify the collaborators-only PR policy,
   trusted collaborator/App inventory, and absence of existing external PRs.
2. Quiesce Actions for the transition after active production operations finish.
3. Change visibility and immediately read back `collaborators_only`; retain
   `has_pull_requests=true` so maintainer PRs remain usable. Keep read-only default
   tokens, SHA pins, the main ruleset and required checks (including `secret-scan`),
   and production credentials in the protected `production` Environment. Configure
   a one-minute Environment wait timer, retaining protected-branch restrictions
   and disabled administrator bypass. This preserves the owner's automatic
   deployment workflow; a timer is not independent human approval. An all-external-contributors
   run approval policy is additional protection, not a replacement for blocked
   external PR creation.
4. Enable and verify secret scanning, push protection, Dependabot alerts/updates,
   and private vulnerability reporting. Verify the route in `SECURITY.md` works.
5. Restore the saved Actions policy after those controls are verified, then enable
   CodeQL default setup on GitHub-hosted runners for Actions, Go,
   JavaScript/TypeScript, and Python. Wait for its initial analysis and triage
   alerts; CodeQL requires Actions to be enabled to complete this validation.
   Verify that an account without
   collaborator access cannot create a PR, and that maintainer PRs still run the
   required checks. Do not temporarily reopen external PR creation to test runner
   routing. If a separate test account is unavailable, record that validation gap
   explicitly; owner-authenticated readback is not an outsider test.
6. Record results in #437, #439, #441, #443, and #445. Only mark a gate complete
   when its current evidence meets the issue's completion conditions.
