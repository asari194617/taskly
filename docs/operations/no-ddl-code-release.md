# Code-only production releases

`deploy-prod` updates application images only when the authoritative schema is
identical to the last successfully deployed revision. It does not invoke Atlas,
start `atlas-dev-db`, or recreate MySQL/Redis because an application environment
variable changed. This keeps the existing shared database running on the
production host with approximately 2 GiB RAM.

## Required proof

Before a deployment marker, rsync, or environment installation, the workflow
streams the reviewed checkout's `infra/ops/deploy-state.sh schema-baseline` over
the existing deployment SSH connection. The installed older helper does not
need to support this mode. This read-only operation requires:

- An owned, non-symlink, mode0600 `.deploy-state` marked `committed` with
  `schema-v1`; both recorded release SHAs must match.
- Healthy existing core applications, MySQL and Redis; the running application
  SHA and normalized live database schema digest must match that committed state.
- Regular `apps/server/schema.sql` and `atlas.hcl` files on the host, with no
  symlink in any source path component.

Only the target/baseline SHAs, database digest and source-file SHA-256 values are
returned. Tokens, connection strings, environment contents, SQL text and rows
are not included. The workflow saves these bounded JSON records as its
`taskly-code-only-schema-<release>` artifact for seven days.

On the GitHub runner, `.github/scripts/verify-no-ddl-release.py` requires the
exact target commit to be checked out, the baseline to be an ancestor, and both
authoritative source files to have identical bytes at the two Git revisions.
The host source hashes must also match that baseline revision. Modified working
copies, missing history, unrelated baselines, first deployments without a known
state, changed SQL/configuration, and malformed attestations stop the release
before any host write. No flag or workflow input bypasses this check.

Git calls disable replacement objects, graft processing, commit-graph shortcuts
and lazy object fetching. Legacy graft files, alternate object stores,
partial-clone configuration and repository/history environment overrides are
rejected rather than repaired. The proof uses the actual immutable objects and
their ancestry in a complete checkout; persistent runner metadata cannot
substitute another tree for a release SHA.

The expected baseline SHA and database digest are passed to `begin` and
`complete`. A change between attestation and deployment stops `begin`; an
unexpected schema change during the code-only release stops `complete` and
retains `in_progress` for operator review. Follow the existing
[recovery procedure](recovery-drill.md) rather than deleting that record.

Application containers use `docker compose up --no-deps`; the existing database
and Redis are not included in that command. The separate, existing Redis ACL
change detection may still explicitly recreate Redis when its policy changes.
Image tags, readiness checks, deploy serialization and the core release-SHA
checks remain in force.

## Schema maintenance

DDL releases and initial provisioning require a separate reviewed maintenance
procedure with backups, compatibility checks and a verified resulting baseline.
`infra/ops/apply-schema.sh` remains an explicit operator tool, but it starts an
Atlas development database and must not be run on this constrained production
host. Compute and validate a schema plan on a suitable isolated runner, then
review the production application and baseline establishment separately. The
automatic code-only workflow fails until a valid unchanged baseline exists;
it does not infer one or silently start a development database.

## Optional Guidance MCP configuration

After the Taskly delegation changes and this deployment path pass review/CI,
set the nonsecret repository or `production` environment Actions variable
`NNCT_GUIDANCE_MCP_PARENT_RESOURCE` to the exact Guidance MCP resource URI. The
workflow passes it as `DEPLOY_NNCT_GUIDANCE_MCP_PARENT_RESOURCE` through a
structured environment entry. `prepare-prod-env.py` validates a canonical HTTPS
URL with the exact `/mcp` path and appends only that key to the existing canonical
`PROD_ENV`. It rejects userinfo, query/fragment, whitespace/control characters,
encoded path aliases and dotenv interpolation. It never prints the value or
existing secrets.

Absent or empty Actions variables preserve the existing environment verbatim,
including an existing parent-resource entry. The two existing required Guidance
event/binding settings remain a separate group. Taskly uses its existing trusted
`API_PUBLIC_URL` and OAuth introspection credentials; it does not receive the new
Auth/Guidance MCP backend secret. Activating the OAuth client also requires its
separate reviewed Auth/Guidance configuration and the actual exact ChatGPT
callback URI; no wildcard or guessed callback is configured here.

## Local verification

```sh
python3 .github/scripts/prepare-prod-env.test.py
python3 .github/scripts/verify-no-ddl-release.test.py
bash .github/scripts/test-prod-env-deploy.sh
bash infra/ops/test-deploy-state.sh
```

These tests use synthetic Git repositories, schema files, and Docker command
stubs. They verify unchanged/changed/unknown baselines, exact target binding,
schema drift, metadata preservation, and rejection before Docker mutation. They
do not access the production database or create a development DB container.
