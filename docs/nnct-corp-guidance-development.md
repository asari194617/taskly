# NNCT company guidance service

The [taskly-nnct-corp-guidance repository](https://github.com/asari194617/taskly-nnct-corp-guidance)
provides the company directory, assigned visits, schedule, venue maps, and saved
companies for the NNCT guidance event. It is a separate application using
taskly-auth; it is not required to start the Taskly application stack.

| Component | Development | Production |
| --- | --- | --- |
| Next.js web client | `http://127.0.0.1:3000` | `https://nnct-corp-guidance-2026.taskly.iy0306.com` |
| Go API | `http://127.0.0.1:3071` | `https://api.nnct-corp-guidance-2026.taskly.iy0306.com` |
| MySQL | Dedicated Compose DB, host loopback `3308` | Separate database and DB account in the production MySQL deployment |

The web client is statically exported to a dedicated Cloudflare R2 bucket.
The Go API runs on the application server and stores event information,
user-specific saved companies, and roster-number preferences in MySQL. A
separate **private** R2 bucket stores the PDF documents and map images. After
authentication, the API issues S3 presigned URLs and the browser reads document
bytes directly from R2. The API does not read local document files or mount a
document volume. Source documents, extracted data, and credentials remain
outside Git; the public UI bucket receives only the static web build.

## Register a development OAuth client

Start a separate [taskly-auth development environment](https://github.com/asari194617/taskly-auth#開発環境)
and prepare a development user there. In that environment, append this entry to
`OAUTH_PUBLIC_CLIENTS_JSON`, retaining any existing registrations:

```json
{"client_id":"nnct-corp-guidance-2026","redirect_uri":"http://127.0.0.1:3071/auth/callback"}
```

The setting is an array of these objects. Recreate the development auth service
after changing its configuration. The guidance service uses a public OAuth
client with PKCE S256 and does not need Taskly's confidential client secret or
its internal synchronization token. Do not replace the existing `taskly-web`
client or change Taskly's authentication ownership flags for this setup.
Use the dedicated client ID above in the isolated development auth environment
as well: Taskly's first-party guidance bridge accepts that identity only.
Production uses a separate auth environment and its registered HTTPS callback.

Use `127.0.0.1` consistently for the guidance UI, API, and callback. taskly-auth accepts an
HTTP public-client callback on a loopback IP, but not an HTTP `localhost`
callback. The guidance API also requires the browser Origin to match its
configured UI origin exactly.

The identity provider has its own origin. Keep taskly-auth's default issuer and
authentication UI/API on `localhost`, as described in its setup guide. If that
service already uses a different origin, set the guidance `AUTH_ISSUER` to its
actual issuer; do not change only one side of the provider's browser/API setup.

## Start the database and import private data

For the guidance repository, use Node.js 26.13.1 (`.nvmrc`) and Go 1.27.4
(`go.mod`). Taskly's own checks use its own manifests and workflows. Python 3.12,
the Python data-extraction dependencies (including Pillow), and the `tesseract`
OCR command are needed when importing a supplied guidebook. Install Tesseract
with `brew install tesseract` on macOS or `sudo apt-get install tesseract-ocr`
on Ubuntu. Run the following in a separate checkout:

```bash
git clone https://github.com/asari194617/taskly-nnct-corp-guidance.git
cd taskly-nnct-corp-guidance
docker compose up -d --wait db
python3 -m venv .venv
.venv/bin/python -m pip install -r scripts/requirements-data.txt
```

The database container initializes a new development volume from `schema.sql`.
It is independent of Taskly's database and taskly-auth's database. Existing
volumes require deliberate schema updates; changing the Compose environment
does not change credentials already stored in MySQL.

An existing guidance database needs `migrations/001_preferences.sql` before
running the API with roster-number preferences. A new volume already receives
that table from `schema.sql`. For an existing **development** Compose database,
apply the reviewed additive migration as follows:

```bash
docker compose exec -T db sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql -u"$MYSQL_USER" "$MYSQL_DATABASE"' \
  < migrations/001_preferences.sql
```

Production migrations are a separate administrator operation; the application
DB account has no DDL privileges and routine image deployment does not apply
this migration. Follow the guidance repository's deployment instructions.

Choose an input PDF and a private output directory **outside every Git checkout**.
Replace the example paths before running the extractor:

```bash
export GUIDANCE_PDF='/absolute/path/to/guidebook.pdf'
export DATA_DIR='/absolute/private/path/to/guidance-data'
.venv/bin/python scripts/extract_guidance.py "$GUIDANCE_PDF" "$DATA_DIR"
export MYSQL_DSN='guidance:development-only@tcp(127.0.0.1:3308)/guidance?parseTime=true&charset=utf8mb4&loc=UTC'
go run ./cmd/import-guidance -check -input "$DATA_DIR/guidance.json"
go run ./cmd/import-guidance -input "$DATA_DIR/guidance.json"
```

These DB credentials are for the isolated development Compose environment only.
The extractor validates references and creates the private JSON, map images,
and documents used by the API. Interactive booth bounds come from the actual
PDF rectangles, with booth numbers resolved by OCR. Extraction stops when that
mapping is ambiguous. Update the extracted JSON and images together; existing
data without booth bounds must be re-extracted and imported for the interactive
map. Do not copy its output, database exports, or
credentials into either repository, frontend assets, or public CI artifacts.

The importer still validates the local source files and transactionally writes
the dataset and document allowlist to MySQL. It does not upload document bytes
to R2. `DATA_DIR` in the commands above is the administrator's private working
directory; it is not an API runtime setting or a directory to mount into the
API container.

Before using the application, upload **only** files listed in the private
JSON's `documents` array to the private document bucket, using each
`file_name` as its object key. Keep extracted JSON, standalone student-assignment
exports, and DB exports out of both R2 buckets. Use separate administrator write credentials
for upload, then read the objects back and verify checksums, MIME types,
`Content-Disposition: inline`, and `Cache-Control: no-store`. Confirm that an
unsigned request is denied. The authoritative upload, CORS, permission, and
rollback steps are in the guidance repository's
[private R2 runbook](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/private-r2.md).

## Configure private document storage

For development, use a private development document bucket or an already
running loopback S3-compatible service. The API requires an R2 endpoint,
bucket, and credentials and does not fall back to local files. It signs with
AWS SDK for Go v2, region `auto`, and path-style object URLs. Replace all example
values below with that development storage configuration:

```bash
export DOCUMENT_STORAGE='r2'
export R2_ENDPOINT='https://your-account-id.r2.cloudflarestorage.com'
export R2_BUCKET='your-private-development-documents'
export R2_ACCESS_KEY_ID='replace-with-bucket-scoped-read-only-access-key-id'
export R2_SECRET_ACCESS_KEY='replace-with-bucket-scoped-read-only-secret'
```

Use Object Read only credentials restricted to that exact private bucket.
Do not reuse the public UI deployment key or an administrator upload key.
Pass these settings to the Go process only, never through `NEXT_PUBLIC_*`,
the frontend build, Git, or CI logs. For a loopback object service, an HTTP
`R2_ENDPOINT` is accepted only on a loopback hostname and with an HTTP
development API origin. Configure the service and its bucket to accept the
same signing method and development credentials.

Allow the development UI origin `http://127.0.0.1:3000` for the document
bucket's GET/HEAD CORS policy as described in the private R2 runbook. Keep
the bucket private without a public custom domain or `r2.dev` access. GET
and HEAD use different signatures; a presigned GET URL cannot be reused for
a HEAD request.

## Start the API and web client

In the same terminal as the import, configure the API and start it:

```bash
export ADDR='127.0.0.1:3071'
export UI_ORIGIN='http://127.0.0.1:3000'
export API_ORIGIN='http://127.0.0.1:3071'
export AUTH_ISSUER='http://localhost:3067'
guidance_registration='nnct-corp-guidance-2026'
export AUTH_CLIENT_ID="$guidance_registration"
export SESSION_ENCRYPTION_KEY="$(openssl rand -base64 32)"
go run ./cmd/api
```

Keep the generated encryption key in private development configuration if
sessions must survive restarts. The API reads process environment variables;
copying `.env.example` alone does not load them into `go run`. DSNs contain shell
metacharacters and must remain quoted when exporting them.

In a second terminal, from the guidance checkout:

```bash
npm --prefix apps/web ci
npm --prefix deploy/r2 ci
NEXT_PUBLIC_API_URL='http://127.0.0.1:3071' npm --prefix apps/web run dev
```

Open `http://127.0.0.1:3000` and sign in through the development auth service.
`GET /healthz` checks MySQL connectivity; `GET /readyz` also requires an imported
dataset. An unauthenticated request to `/api/guidance`, `/api/documents/{id}`, or
`/api/bookmarks` must return `401`. Saved companies belong to the signed-in
Taskly account and are stored in the guidance database.

The static pages are `/` (home), `/search/`, `/favorites/`, `/schedule/`,
`/documents/`, and `/map/`. Production Cloudflare rewrites each page to its
own exported `index.html`, preserving its URL and query string. These are
separate pages; the old `?view=` URLs are compatibility links. The map fills
the area between the header and mobile navigation. Online PDF/map requests go
directly to private R2 rather than the Go server's filesystem.

On first login, set the roster number for Ec4 in the setup dialog, or choose to
do it later. The API accepts numbers with an assignment in the imported guide,
and stores the selection per Taskly account. The setting can be changed from
the user menu or favorites page and is shared across devices. It is not an inference
of the user's enrollment and is not stored in the URL.

New logins have an absolute 30-day local session. The API encrypts the upstream
access and rotating refresh tokens and refreshes access automatically without
extending that deadline. It continues checking the upstream identity for
protected requests. Existing short sessions keep their previous deadline
until the next login; roster preferences and saved companies are retained.

Document links are issued after authentication and last at most five minutes,
bounded by the session time remaining at issuance. The PDF or map URL can open
without a viewer Cookie and remains usable until its R2 signature expires.
Logout prevents new URL issuance but does **not** immediately revoke URLs
already issued by R2. Do not include these temporary bearer URLs in public
logs, issue comments, or test artifacts.

## Offline reference at the venue

After an online visit, the same device can read the last successfully fetched
companies, schedule, assigned visits, favorites, home state, and downloaded map
images for up to 24 hours. The snapshot is scoped to the confirmed account and
roster; account changes, logout, `401`, and roster changes clear it. Offline
mode is read-only: completion, favorites, and roster edits are disabled, and
reconnection reloads the server state without replaying queued writes.

The Service Worker caches only the public static shell. Private snapshots and
map image bytes are stored separately in IndexedDB; authentication data,
presigned URLs, and PDFs are not cached. PDF viewing and external forms need
connectivity. The existing private R2 bucket's exact-UI-origin GET/HEAD CORS
policy permits map-byte retrieval without making objects public or replacing
short presigned URLs with permanent links.

The normal guidance build generates `/offline-worker.js` with its asset
manifest. The existing R2 deploy step regenerates it from the final
CSP-protected HTML and uploads assets, HTML, then the worker. Installing
`deploy/r2` dependencies supplies the `parse5` parser used by CSP preparation
and the deployment tests. Normal builds and deployments use the existing
Cloudflare setup without a new Worker service or bucket.

A zone-level browser TTL override can supersede the R2 object's `no-cache`.
Keep the cache rule scoped only to the exact UI host and `/offline-worker.js`,
with edge caching disabled and browser TTL set to `respect_origin`. Deployment
verification checks the public worker's JavaScript MIME, `no-cache`, and byte
identity with the final uploaded worker. Immutable asset policies and the
zone-wide TTL remain unchanged.

Check the public UI URL after deployment; the source of truth for expiry,
cache boundaries, CSP, and worker verification is the guidance
[offline guide](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/offline.md).

## Guidance lint and formatting

Run these commands from the **guidance checkout**, not the Taskly repository.
The npm scripts live in `apps/web` but invoke a runner for the whole guidance
repository: Biome 2.6.7, Ruff 0.17.5, `go vet`, and `gofmt`.

```bash
cd /absolute/path/to/taskly-nnct-corp-guidance
npm --prefix apps/web ci
npm --prefix deploy/r2 ci
python3 -m venv .venv
. .venv/bin/activate
python3 -m pip install -r requirements-lint.txt
npm --prefix apps/web run lint
npm --prefix apps/web run format:check
```

Use `npm --prefix apps/web run format` to apply formatting, then rerun lint
and the formatting check. CI runs both checks for all changes without applying
formatting or committing files. See the guidance
[code-quality guide](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/code-quality.md)
for the measured source types and exclusions. These commands do not change
Taskly's lint or CI configuration.

## Verification and production operations

Use the guidance repository's verification instructions and workflows for its
unit tests, real-MySQL integration tests, browser E2E tests, and Go, frontend,
and Docker builds. PR reports distinguish unit, integration, and browser
coverage. Required checks run for all PR changes, including documentation-only
changes; they must not use path filters that leave a required check unreported.
The measured Go application, frontend, importer, and PDF-extraction tests have
80% minimums. Guidance's real-MySQL run selects `^TestMySQL` and separately
requires 80% statement coverage across `internal/server`. The deployed Service
Worker functions are measured separately in Node with 80% minimums for lines, branches,
and functions. Chromium page E2E coverage is reported separately from unit,
MySQL, and Service Worker coverage.

The browser E2E helper `node scripts/e2e-run.mjs` starts its own loopback
S3-compatible object service alongside the test OAuth service, real Go API,
and disposable MySQL database. It uses synthetic files and independent SigV4,
expiry, Range, and HEAD validation; real R2 credentials and production documents
are not needed. Follow the guidance repository's
[testing guide](https://github.com/asari194617/taskly-nnct-corp-guidance/blob/main/docs/testing.md)
for the disposable DB variables, browser installation, and execution steps.

The guidance repository uses separate `production-ui` and `production-api`
GitHub Actions environments with GitHub-hosted runners. The public UI upload
and private document reads use separate bucket-scoped credentials. API
deployment uses a dedicated SSH key restricted to the
deployment command, with a pinned server host identity. Runtime DB credentials,
the session encryption key, and the private bucket's read-only key stay on the
application server. PDF/PNG objects stay in private R2; source extraction and
import validation happen in an administrator's private working directory.
The runtime Compose configuration does not mount document files or read
`DATA_DIR`.
See the guidance repository's deployment instructions for environment settings,
data import, readiness checks, and rollback. Taskly's Compose files and
`infra/ops/apply-schema.sh` do not deploy or migrate this separate service.

## Taskly completion synchronization

The optional first-party bridge stores no separate completion flags. Guidance
reads and updates Taskly's existing `tasks_users_completions` records; checking
or unchecking either service is reflected by the next read in the other.
Opening the guidance homepage never creates shared class tasks.

Newly checking a designated term's visit survey in Guidance also checks its
assigned visit if that visit was not already complete. The automatic origin
persists on the canonical row across reloads and service restarts. Undoing
either end of this automatically owned pair **in Guidance**, within the same
event and assignment scope, clears both checks. Earlier manual visits and
later explicit manual checks in either service are preserved. Repeating an
already-complete survey does not recreate a manually undone visit.

Taskly's own controls remain single-item manual operations: they release the
affected automatic pair ownership without changing the other item's check.
If the roster/company assignment scope changes, Guidance preserves the other
completion and retires the stale automatic origin.

First provision or identify the intended canonical tasks through the normal
Taskly administration workflow. Keep the real task IDs and imported event data
in private operator configuration, outside Git. Configure the Taskly API with:

- `NNCT_GUIDANCE_EVENT_ID`: the imported event's exact opaque namespace.
- `NNCT_GUIDANCE_TASK_BINDINGS`: a JSON object mapping logical keys to existing
  positive, unique Taskly task IDs. Accepted keys are `assigned-visit-N`,
  `visit-survey-N` (`N` is 1–99), `overall-survey`, and `sheet-submission`.
  Both fixed keys and at least one key from each numbered group are required;
  at most 64 bindings are accepted. The guidance API validates its needed keys
  against its private imported schedule and selected roster assignment.
- The existing confidential `AUTH_ISSUER`, `AUTH_CLIENT_ID`, and
  `AUTH_CLIENT_SECRET`, and the existing `WEB_PUBLIC_URL`. Production requires
  HTTPS. There is no additional service-wide integration secret.

For example, this **synthetic** shape illustrates the binding format; replace
all values with private operator configuration and never use example IDs in
production:

```json
{"assigned-visit-2":101,"visit-survey-2":102,"overall-survey":103,"sheet-submission":104}
```

Restart the Taskly API after changing configuration. Without valid configuration
the bridge returns `503`; existing Taskly routes continue to work. To roll back
the integration, remove the two `NNCT_GUIDANCE_*` values and restart. Existing
tasks and canonical user completion records remain intact.

### Survey/assigned-visit rollout

Reuse the existing event namespace, bindings, tasks, and form links. This update
adds no environment keys, replacement tasks, or form provisioning.

The authoritative Taskly schema is `apps/server/schema.sql`. The canonical
`tasks_users_completions` table gains three nullable provenance columns:
`auto_event_id`, `auto_source_task_id`, and `auto_assignment_scope`. Existing
rows remain manual with NULL provenance. Apply this through Taskly's Atlas
workflow, not a historical `apps/server/migrations` file or Guidance DB migration.

From the **Taskly repository root**, apply the Atlas plan before starting the
new Taskly API, using the environment described in
[local development](local-development.md):

```bash
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server
```

Production's existing Taskly workflow also applies the reviewed Atlas plan
before updating the API. **Deploy the Taskly schema and API before the Guidance
API version that sends linked requests.** Verify Taskly first, coordinate both
services during rollback, and retain the added nullable columns. Task IDs, form
links, event data, and production configuration stay outside Git.

Only the guidance backend calls `GET /integrations/nnct-guidance/tasks` and
`PUT /integrations/nnct-guidance/tasks/{key}`. It forwards the signed-in user's
current Taskly-auth access token as `Authorization: Bearer …` and sends the
exact `X-Guidance-Event-ID`. The browser never receives this token, confidential
client credentials, or a cross-user identifier. Taskly introspects each request
using its own confidential client and accepts only the dedicated first-party
client `nnct-corp-guidance-2026`, `taskly-api` audience, trusted issuer,
`taskly:login` scope, live token, and numeric user subject. Development tests use
the same client identity in an isolated synthetic auth environment; unrelated
OAuth clients cannot write through the bridge.

GET returns `{event_id,tasks:[{key,task_id,completed,completion_disabled,taskly_url}]}`.
An ordinary PUT sends `{completed:boolean}` and returns the updated task object.
Linked Guidance operations add `link_assigned_visit:true` and a validated,
opaque `assignment_scope`; responses include the counterpart in `related_tasks`.
This mode checks/undoes a survey or undoes its assigned visit; explicitly
checking a visit remains a single-item manual operation.
These are backend protocol fields, not operator settings or browser-supplied
task IDs. Both tasks are validated and updated in one transaction; explicit
manual checks retain the single-item operation.
Both modes support checking and undoing a check for the authenticated subject.
Both responses use `Cache-Control: no-store`. Deleted/blocked accounts, current
policy consent, task moderation, task read permissions, and disabled completion
are enforced. A disabled task can still be unchecked, matching Taskly's normal
completion behavior. Event mismatch or disabled checking returns `409`;
unlisted keys return `404`; inaccessible tasks return `403`. Identity or database
outages fail closed and never manufacture an incomplete state.

The PR coverage comment includes the entire guidance bridge package with an
80% minimum. Synthetic OAuth/SQLite tests exercise identity, ownership,
configuration, and failure handling; `api-empty-database` also runs
`TestMySQLGuidanceBridgeCanonicalRoundTrip` against MySQL on every PR change.
That test exercises the real bridge and Taskly GraphQL completion routes in both
directions, including undo, retries, and disabled completion. No production data
or credentials are needed.

For the cross-service browser suite, `apps/server/cmd/guidance-e2e` starts the
real Taskly bridge and GraphQL routes with synthetic data and in-process Redis.
It requires `TASKLY_GUIDANCE_E2E=1`, a nonproduction `NODE_ENV`, literal loopback
`ADDR`, a loopback `TASKLY_E2E_MYSQL_DSN` whose database is exactly
`taskly_guidance_e2e`, and a loopback HTTP `AUTH_ISSUER`. It refuses nonempty
databases rather than deleting existing data. Start from `apps/server` so it can
read `schema.sql`, or pass `TASKLY_E2E_SCHEMA_PATH`. Supply the synthetic issuer's
`AUTH_CLIENT_ID` and `AUTH_CLIENT_SECRET`. `/__test/ready` reports readiness;
`/__test/fixture` returns synthetic user IDs, local sessions, task bindings, and
the event namespace. These routes and sessions exist only in this loopback test
command; the production Docker entrypoint builds `cmd/server` exclusively.
The fixture includes an explicitly read-denied user for authorization failure
coverage. Browser tests use the actual `/graphql/` completion mutations for
Taskly-side changes and the guidance proxy for the opposite direction.

Before rollout, verify the synthetic bridge, real-MySQL, and browser cases for
pair creation, both Guidance undo directions, manual-check preservation,
native Taskly single-item behavior and ownership release, retries, scope
changes, user isolation, and failed-write rollback. Record the actual CI results.
