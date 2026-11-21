# Taskly

Taskly is a class and task management service.

## Repository structure

| Path | Role |
| --- | --- |
| [`apps/web/`](apps/web/) | Astro and React web client deployed to Cloudflare Pages |
| [`apps/server/`](apps/server/) | Go and Fiber API with GraphQL and REST, backed by MySQL and Redis |
| [`apps/mcp/`](apps/mcp/) and [`apps/mcp-client-files/`](apps/mcp-client-files/) | TypeScript MCP HTTP service, stdio bridge, and client distribution files |
| [`apps/email-compose/`](apps/email-compose/) | Node.js gRPC service that renders transactional email |
| [`apps/docs/openapi/`](apps/docs/openapi/) | OpenAPI definition and API documentation page |
| [`batches/`](batches/) | Go jobs for notifications, day-shift rotations, moderation, and database backups |
| [`infra/`](infra/) | Docker Compose stacks, Airflow DAGs and runtime, monitoring, CI runners, and operations scripts |
| [`cloudflare-email-workers/`](cloudflare-email-workers/) | Inbound email forwarding Worker configured by [`wrangler.toml`](wrangler.toml) |
| [`docs/`](docs/) | Development guides, architecture, and operations runbooks |
| [`.github/workflows/`](.github/workflows/) | CI, release, deployment, and scheduled workflows |
| [`scripts/`](scripts/) | Repository tooling, including Go linting |
| [`samples/`](samples/) and [`login/`](login/) | Standalone Airflow Compose example and login screenshot |

### Related repositories

| Repository | Role |
| --- | --- |
| [`taskly-auth`](https://github.com/asari194617/taskly-auth) (private) | Shared login, OAuth, and account settings used by Taskly and NNCT company guidance |
| [`taskly-nnct-corp-guidance`](https://github.com/asari194617/taskly-nnct-corp-guidance) | NNCT company guidance web app, API, and MCP integration with Taskly task completion |
| [`taskly-policy`](https://github.com/asari194617/taskly-policy) | Versioned policy text and release manifests |
| [`taskly-policy-web`](https://github.com/asari194617/taskly-policy-web) | Public policy site that publishes releases from `taskly-policy` |
| [`taskly-environment-ribbon`](https://github.com/asari194617/taskly-environment-ribbon) | Optional Chrome extension that marks production and local development pages |

<a name="local-development"></a>

## ローカル開発

Taskly と関連サービスを `local.` の開発ドメインで開く場合は、[ローカル HTTPS のセットアップ](docs/local-development.md#local-https)を参照してください。以下では、単独開発や自動テスト向けのループバック HTTP の起動方法も案内します。

必要なものは以下のとおりです。

- Docker と Docker Compose v2
- Web クライアントと TypeScript の検証用に Node.js 26（CI は 26.13.1）と npm
- Go のローカル検証用に Go 1.27.4 以上（CI と Docker ビルドと同じバージョン）
- 運用スクリプト用に Bash 4 以上、Airflow のDB設定用に `jq`

Web ログインには別の **taskly-auth** サービスと、その開発用DB、OAuth クライアント登録が必要です。[ローカル開発ガイド](docs/local-development.md)を参照し、認証の構成を選んでから `.env` を設定し、API 起動前にスキーマを適用します。

[企業ガイダンス](docs/nnct-corp-guidance-development.md)を開発する場合は、専用のリポジトリ、DB、OAuth クライアントを用意します。

ローカル開発ガイドに従い、ルートで `.env` を作成してから起動します。

```bash
cp .env.example .env
# 認証の構成を決め、サンプルの秘密値を開発用の値に置き換えます。
chmod 600 .env
install -m 600 .env infra/.env
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server mcp
curl -fsS http://localhost:3063/readyz
curl -fsS http://localhost:3066/readyz
```

MySQL、Redis、email-compose、API、MCP が起動します。Airflow を使う場合は3つのサービスを起動する前に、[Airflow のセットアップ手順](docs/local-development.md#optional-airflow)に従ってメタデータ用DBを初期化します。

Web クライアントは別のターミナルで起動します。

```bash
cp apps/web/.env.example apps/web/.env
npm --prefix apps/web ci
npm --prefix apps/web run dev
```

ループバック HTTP の主な接続先は次のとおりです。

| サービス | URL |
| --- | --- |
| Web クライアント | `http://localhost:4321` |
| API・GraphQL | `http://localhost:3063` / `http://localhost:3063/graphql` |
| API の稼働・準備状態 | `http://localhost:3063/healthz` / `http://localhost:3063/readyz` |
| API のメトリクス | `http://localhost:3064/metrics` |
| MCP HTTP API | `http://localhost:3066` |
| Airflow の画面 | `http://localhost:3065` |

API、メトリクス、MCP、Airflow はホストのループバックで待機します。MySQL、Redis、email-compose には Compose のネットワーク内から接続します。詳細は [本番環境のポート一覧](docs/operations/network-ports.md)を参照してください。

停止するときは、次を実行します。名前付きボリューム内の開発データは保持されます。

```bash
docker compose -f infra/docker-compose.yml down
```

DB スキーマは `apps/server/schema.sql` に定義します。変更を反映するときは、Docker で動作する Atlas を使います。スクリプトが MySQL を起動し、準備が整うのを待ってから適用します。

```bash
infra/ops/apply-schema.sh
```

適用前に計画が表示されます。破壊的な変更を適用する場合は計画を確認し、明示的に `ATLAS_ALLOW_DESTRUCTIVE=true` を指定します。運用スクリプトを使う前に `infra/.env` を `.env` と同期してください。`apps/server/migrations` は過去の記録です。

## Verification

Install each Node package's dependencies first (`npm --prefix apps/<component> ci`;
MCP uses `ci --legacy-peer-deps`). Run the checks relevant to the component you changed:

```bash
(cd apps/server && go test ./...)
(cd batches && go test ./...)
npm --prefix apps/mcp test
npm --prefix apps/mcp run build
npm --prefix apps/email-compose run build
npm --prefix apps/web run check
npm --prefix apps/web run test:unit
npm --prefix apps/web run build
npm --prefix apps/web run test:picomatch-exposure
npm --prefix apps/web run test:canonical
npm --prefix apps/web run test:security-headers
docker compose -f infra/docker-compose.yml config --quiet
```

The web [browser tests](apps/web/tests/browser/README.md),
[real API E2E tests](apps/web/tests/e2e/README.md), and
[Airflow Python tests](docs/operations/airflow-python-tests.md) have additional
setup. Notification delivery channels are separate Go modules: run `go test ./...`
in each of `batches/notifications/delivery-channels/{common,email,discord,web-push}`
when changing them. See [coverage checks](docs/testing/coverage.md) for CI details.

## Documentation

See [`docs/README.md`](docs/README.md) for the documentation index and the
distinction between current references and historical planning material.
Useful entry points include:

- [`docs/Specification.md`](docs/Specification.md): product requirements and planning notes
- [`docs/schema.md`](docs/schema.md): schema overview (the executable source is `apps/server/schema.sql`)
- [`docs/mcp-docs.md`](docs/mcp-docs.md): MCP client and server guide
- [`docs/initial-settings.md`](docs/initial-settings.md): CI/CD and production operations
- [`apps/docs/openapi/openapi.yaml`](apps/docs/openapi/openapi.yaml): OpenAPI definition

## Production deployment

Merges to `main` that match the production paths trigger
[`deploy-prod.yml`](.github/workflows/deploy-prod.yml). It releases commit-SHA
image tags and verifies the application schema against the last committed
production baseline before synchronizing the repository and updating services
with Docker Compose. Application schema changes require separate reviewed
maintenance. Cloudflare Pages deploys the web client separately.

All four service images receive the release SHA tag. Running Airflow containers
are replaced only when their image or Compose configuration changes (or a service
is missing), so their running SHA can differ from the API/MCP/email-compose SHA.
Even a README-only change under `apps/server`, `apps/mcp`, `apps/email-compose`,
`batches`, or `infra` matches the deployment paths. See
[deployment operations](docs/initial-settings.md) and the
[runner pool guide](infra/actions-runner/README.md).

## Source availability and licensing

Taskly is not currently offered under a repository-wide open-source license.
Unless a file or directory contains its own license, all rights are reserved by its copyright holders. In particular,
`apps/mcp-client-files` is separately available under the MIT License contained
in that directory.

See [`LICENSE`](LICENSE) for the scope of these terms and
[`CONTRIBUTING.md`](CONTRIBUTING.md) before proposing a contribution.
