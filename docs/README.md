# Documentation index

This index identifies which documents describe the current system and which
ones are retained as historical design material.

## Current references

- [`../README.md`](../README.md): local development and repository overview
- [`Specification.md`](Specification.md): product specification, including planned features
- [`schema.md`](schema.md): database schema
- [`notifications.md`](notifications.md): notification settings and delivery
- [`ical-feed.md`](ical-feed.md): public iCal feed scope, errors, and performance verification
- [`mcp-docs.md`](mcp-docs.md): MCP usage and client configuration
- [`initial-settings.md`](initial-settings.md): CI/CD and production deployment
- [`operations/recovery-drill.md`](operations/recovery-drill.md): recovery targets, database restore, rollback, notification failures, and drills
- [`operations/network-ports.md`](operations/network-ports.md): production port exposure and verification
- [`operations/metrics-monitoring.md`](operations/metrics-monitoring.md): metric collection, dashboard, alerts, and response
- [`redis-acl-operations.md`](redis-acl-operations.md): Redis ACL rollout, rotation, and recovery
- [`legal/privacy-data-inventory.md`](legal/privacy-data-inventory.md): Issue #401 data-flow inventory and publication checks
- [`legal/privacy-policy.md`](legal/privacy-policy.md): text of the published privacy policy
- [`../apps/docs/openapi/openapi.yaml`](../apps/docs/openapi/openapi.yaml): OpenAPI definition used by the documentation app
- [`batches/notifications/README.md`](batches/notifications/README.md): notification batch behavior
- [`../batches/README.md`](../batches/README.md): batch inventory and scheduling

When a document conflicts with executable configuration, the implementation and
the following files are authoritative: `infra/docker-compose.yml`, `.env.example`,
`.github/workflows/`, application manifests, and database migrations.

## Historical or planning material

These documents are useful background, but they are not authoritative current
status:

- `improvements.md`: point-in-time security review and recommendations
- `remain.md`: earlier backlog and requirements notes
- `setup.tmp.md`: migration-specific rollout notes
- `new-day-shift.md` and `features/`: design proposals

Update the current references in the same pull request whenever behavior,
configuration, ports, required environment variables, or deployment steps
change.
