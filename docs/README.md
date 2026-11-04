# Documentation index

Start with [local development](local-development.md) for a new checkout.
Operational references describe the checked-in implementation; production
credentials, incident evidence, and user data belong in restricted records.

## Architecture and features

- [Repository overview](../README.md) and [web development](../apps/web/README.md)
- [API boundaries](api-boundaries.md), [public OpenAPI](../apps/docs/openapi/openapi.yaml), and [schema overview](schema.md)
- [Web task behavior](clients/web/README.md) and [mobile task entry](clients/web/mobile-task-entry.md)
- [Task images](features/task-images.md) and [token expiry input](features/token-expiration-input.md)
- [Notification settings](notifications.md), [batch delivery](batches/notifications/README.md), and [batch inventory](../batches/README.md)
- [Integration jobs and transactional outbox](integration-jobs.md)
- [iCal feed](ical-feed.md) and [MCP guide](mcp-docs.md)
- [Email composer](../apps/email-compose/README.md) and [sender logo](email-sender-logo.md)
- [Destructive security confirmations](features/security-destructive-action-confirmations.md), [mobile settings navigation](features/settings-mobile-navigation.md), and [header logo navigation](features/header-logo-navigation.md)
- [GraphQL admin authorization](apps/server/graphql-admin-authorization.md) and [day shift skips](apps/server/day-shift-skip.md)

## Authentication and account data

The web login, credentials, and profile UI use the separate taskly-auth service.
These references describe the ownership gates and synchronization contracts;
they do not provision that service:

- [Identity writes and cutover gates](auth-identity-write-inventory.md)
- [User projections](auth-user-projection-sync.md) and [profile ownership](account-profile-ownership.md)
- [Account lifecycle synchronization](auth-account-lifecycle-sync.md)
- [Session cookies](session-cookie-migration.md)
- [Account closure, erasure, and data requests](operations/account-data.md)
- [Policy editions and consent records](legal/README.md) and [privacy policy reference](legal/privacy-policy.md)

## Operations and verification

- [CI/CD and production deployment](initial-settings.md)
- [Repository publication gates and final audit](operations/repository-publication.md)
- [GitHub Actions runner pool](../infra/actions-runner/README.md) and [required checks](operations/github-actions-required-checks.md)
- [Readiness](operations/readiness.md), [network ports](operations/network-ports.md), and [monitoring](operations/metrics-monitoring.md)
- [Redis ACL operations](redis-acl-operations.md) and [disk maintenance](operations/disk-maintenance.md)
- [Encrypted MySQL backups](operations/mysql-backup.md), [recovery and rollback](operations/recovery-drill.md), and [drill record template](operations/recovery-drill-record-template.md)
- [Airflow metadata separation](operations/airflow-db-separation.md) and [Airflow Python tests](operations/airflow-python-tests.md)
- [Test coverage](testing/coverage.md), [mocked browser tests](../apps/web/tests/browser/README.md), and [real API E2E tests](../apps/web/tests/e2e/README.md)
- [Weekly Discord report](operations/weekly-discord.md)
- [API OpenTelemetry](../apps/server/OPENTELEMETRY.md) and [Datadog](../apps/server/DATADOG.md)

When prose conflicts with executable configuration, use
`apps/server/schema.sql` and `atlas.hcl` for the application schema,
`infra/docker-compose.yml` plus `infra/docker-compose.prod.yml` for deployment,
`.env.example`, `.github/workflows/`, and application manifests. Files in
`apps/server/migrations` record earlier changes and are not the current schema
application mechanism. The OpenAPI files document REST; GraphQL's executable
schema is in `apps/server/internal/handler/graphql/`.

## Requirements and historical material

The following are requirements or earlier proposals, not an implementation
status checklist. Their checkmarks and example schemas do not establish what is
currently deployed:

- [Original product requirements](Specification.md), including native client plans
- [Earlier backlog](remain.md) and [security review notes](improvements.md)
- [Day shift redesign](new-day-shift.md), [exam proposal](features/0001-exam.md), and [moderation proposal](batches/moderate/README.md)
- [Email composer requirements](apps/email-compose/README.md)
- [Proposed terms revision](legal/terms-issue-404.md); not yet effective

Feature documents linked in the current sections above describe implemented
behavior. Other files under `features/` may contain proposals; consult the
implementation before treating them as current behavior.

Update the relevant current references whenever behavior, configuration, ports,
required environment variables, or deployment steps change.
