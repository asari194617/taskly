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
{"client_id":"nnct-corp-guidance-dev","redirect_uri":"http://127.0.0.1:3071/auth/callback"}
```

The setting is an array of these objects. Recreate the development auth service
after changing its configuration. The guidance service uses a public OAuth
client with PKCE S256 and does not need Taskly's confidential client secret or
its internal synchronization token. Do not replace the existing `taskly-web`
client or change Taskly's authentication ownership flags for this setup.

Use `127.0.0.1` consistently for the guidance UI, API, and callback. taskly-auth accepts an
HTTP public-client callback on a loopback IP, but not an HTTP `localhost`
callback. The guidance API also requires the browser Origin to match its
configured UI origin exactly.

The identity provider has its own origin. Keep taskly-auth's default issuer and
authentication UI/API on `localhost`, as described in its setup guide. If that
service already uses a different origin, set the guidance `AUTH_ISSUER` to its
actual issuer; do not change only one side of the provider's browser/API setup.

## Start the database and import private data

Use the Go and Node versions specified by the guidance repository. Python 3.12,
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
`file_name` as its object key. Keep extracted JSON, student assignments, and DB
exports out of both R2 buckets. Use separate administrator write credentials
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
export AUTH_CLIENT_ID='nnct-corp-guidance-dev'
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
NEXT_PUBLIC_API_URL='http://127.0.0.1:3071' npm --prefix apps/web run dev
```

Open `http://127.0.0.1:3000` and sign in through the development auth service.
`GET /healthz` checks MySQL connectivity; `GET /readyz` also requires an imported
dataset. An unauthenticated request to `/api/guidance`, `/api/documents/{id}`, or
`/api/bookmarks` must return `401`. Saved companies belong to the signed-in
Taskly account and are stored in the guidance database.

The static pages are `/` (list), `/search/`, `/favorites/`, `/schedule/`,
`/documents/`, and `/map/`. Production Cloudflare rewrites each page to its
own exported `index.html`, preserving its URL and query string. These are
separate pages; the old `?view=` URLs are compatibility links. The map uses
a fullscreen layout, and PDF/map requests go directly to private R2 rather
than the Go server's filesystem.

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

## Verification and production operations

Use the guidance repository's verification instructions and workflows for its
unit tests, real-MySQL integration tests, browser E2E tests, and Go, frontend,
and Docker builds. PR reports distinguish unit, integration, and browser
coverage. Required checks run for all PR changes, including documentation-only
changes; they must not use path filters that leave a required check unreported.
The measured Go application, frontend, importer, and PDF-extraction tests have
80% minimums. Integration and browser coverage are reported separately from
those combined/unit measurements.

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
