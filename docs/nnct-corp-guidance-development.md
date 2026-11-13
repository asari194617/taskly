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
user-specific saved companies, and roster-number preferences in MySQL. Source documents and extracted data
remain outside Git. R2 receives only the web application build; authenticated
API routes serve company information, PDF documents, and map images.

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

On first login, set the roster number for Ec4 in the setup dialog, or choose to
do it later. The API accepts numbers with an assignment in the imported guide,
and stores the selection per Taskly account. The setting can be changed from
the header or visit list and is shared across devices. It is not an inference
of the user's enrollment and is not stored in the URL.

Document links are issued after authentication and last at most five minutes,
bounded by the issuing session. The PDF or map URL can open without a viewer
Cookie, but the API rechecks the issuing session and upstream identity. Logout
revokes links from that session. Do not include these temporary bearer URLs in
public logs, issue comments, or test artifacts.

## Verification and production operations

Use the guidance repository's verification instructions and workflows for its
unit tests, real-MySQL integration tests, browser E2E tests, and Go, frontend,
and Docker builds. PR reports distinguish unit, integration, and browser
coverage. Required checks run for all PR changes, including documentation-only
changes; they must not use path filters that leave a required check unreported.
The measured Go application, frontend, importer, and PDF-extraction tests have
80% minimums. Integration and browser coverage are reported separately from
those combined/unit measurements.

Production uses separate `production-ui` and `production-api` GitHub Actions
environments with GitHub-hosted runners. The R2 upload uses bucket-scoped
credentials. API deployment uses a dedicated SSH key restricted to the
deployment command, with a pinned server host identity. Runtime DB credentials,
the session encryption key, and source documents stay on the application server.
See the guidance repository's deployment instructions for environment settings,
data import, readiness checks, and rollback. Taskly's Compose files and
`infra/ops/apply-schema.sh` do not deploy or migrate this separate service.
