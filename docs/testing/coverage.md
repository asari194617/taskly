# PR coverage policy

Pull requests enforce a 75% minimum for deterministic core logic. The coverage
comment names the measured scope so the percentage is not confused with a
whole-repository figure.

| Group | Included scope | Excluded from the percentage |
| --- | --- | --- |
| API core | GraphQL request validation/types and task-list policy | process bootstrap, database-bound resolver wiring, external service setup |
| Notification core | shared recipient eligibility, deadlines, badges, and DSN helpers | batch process bootstrap and vendor transports |
| Email core | deterministic digest payload and localization (`coverage_core.go`) | SMTP/gRPC transport |
| Web push core | recipient policy and idempotent delivery state (`coverage_core.go`) | network transport and process bootstrap |
| Web logic | API clients, task-list model, auth/session and login-flow modules listed in `vitest.config.ts` | React rendering, vendored UI primitives, Astro pages |
| MCP runtime | compiled application/runtime modules exercised by unit and stdio tests | CLI bootstrap |

Excluded code is still compiled and its existing unit, integration, browser,
and workflow tests continue to run. New deterministic business logic should be
placed in the measured scope; exclusions must not be expanded merely to satisfy
the threshold.
