# Repository publication gates

Track the release decision in [#445](https://github.com/asari194617/taskly/issues/445).
The repository remains private until every applicable gate below has evidence.
Publishing source does not change the licensing terms in `LICENSE`.

## Verified preparation (2026-11-04)

- A fresh remote mirror contained 157 branch refs, 9 tag refs, 178 pull refs,
  and 1,275 reachable commits; its pack was about 5 MiB. No compiled executable
  signatures or the eight removed incident/extraction/setup document paths were
  found in those reachable objects. This supersedes the older observation that
  advertised pull refs retained large binaries; it does not prove that GitHub's
  cached diffs or unadvertised objects have been purged.
- Gitleaks 8.30.1 scanned all fetched refs. Five candidates were reviewed as
  false positives: four references to a localhost-only CI key and one match
  crossing a DAG description and cron keyword. `.gitleaksignore` records only
  those commit/path/rule/line fingerprints. The repeat scan had no other findings.
- The main ruleset requires signed commits, PRs, six application checks, and
  `secret-scan`, with no bypass actors. Actions defaults to a read-only token and
  requires full action SHA pins. Production credentials are Environment secrets;
  the repository secret names contain no `PROD_*` or task-image storage secrets.
- The production Environment restricts deployments to protected branches and
  disallows administrator bypass. Reviewer/wait-timer protection is still absent.
  Public fork approval and native security features still require verification
  during the visibility transition.
- Issue/PR bodies (752), issue comments (1,522), review comments (none returned),
  and releases (5) were fetched with pagination and scanned with Gitleaks: no
  candidates. A global-IPv4 text search returned only a dependency version, not
  a host address. PR diffs/review summaries, Actions logs/artifacts/caches, wiki,
  and other personal or operational details still need separate review.

These are dated observations, not a permanent authorization to publish. The
`secret-scan` workflow checks fetched history on PRs and main pushes using a
checksum-verified Gitleaks binary, read-only token, redacted output, and a
GitHub-hosted runner. It supplements GitHub's native security features. It does
not enumerate GitHub issues, old PR caches, logs, artifacts, or hidden refs.

## Remaining runner boundary

**Do not change this repository to public while privileged repository runners
remain directly selectable by its PR workflows.** The shared selector sends
ordinary fork PRs to GitHub-hosted runners, but a fork can change the workflow
and request `taskly-prod` or a CI label directly. Approval policies require
review of every untrusted workflow revision; they do not make runner labels an
access-control mechanism. A Docker socket gives jobs host-level access, including
other runner containers/volumes on the same host.

The workflow guards reject non-main manual deployment, image publication, and
runner installation. Deployment and release-drift checks use `production` and
`taskly-prod`. These reduce accidental exposure but do not prevent an untrusted
PR from changing its own YAML or compromising a shared host.

Choose and implement one of these publication layouts before the final gate:

1. **Public source repository, existing private deployment repository.** Publish
   an audited source copy without privileged runner registrations or production
   secrets. Keep production operations in the private repository and define a
   reviewed source synchronization procedure. Validate the new public CI and
   secret inventory before enabling contributions.
2. **Publish this repository after moving privileged execution.** Move deployment,
   image publication, release monitoring, and host maintenance to a private
   repository. Remove privileged runner registrations and credentials from the
   public repository; run public PR checks on GitHub-hosted runners. Validate the
   replacement deploy path before changing visibility.

See GitHub's [self-hosted runner security guidance](https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions#hardening-for-self-hosted-runners).

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

1. Finish the chosen runner/deployment separation and the content audit. Preserve
   restricted recovery evidence and record the reviewed commit/ref inventory.
2. Quiesce Actions for the transition after active production operations finish;
   do not allow external PR execution before policies have been verified.
3. Change visibility, then require approval for **all external contributors**.
   Confirm read-only default tokens, SHA pins, the main ruleset and required
   checks (including `secret-scan`), and secret placement in the private deployment
   repository. Configure its Environment reviewer or timer protection as supported.
4. Enable and verify secret scanning, push protection, code scanning for the
   supported languages, Dependabot alerts/updates, and private vulnerability
   reporting. Triage initial alerts and verify the route in `SECURITY.md` works.
5. Enable public CI only after the above checks. Use a harmless external fork PR
   to verify both the usual selector path and an explicitly requested privileged
   label: no public PR may reach a private production runner or its credentials.
   Close the test PR and cancel any intentionally unschedulable test job.
6. Record results in #437, #439, #441, #443, and #445. Only mark a gate complete
   when its current evidence meets the issue's completion conditions.
