# Taskly

Taskly は授業と課題を管理するサービスです。

<a name="repository-structure"></a>

## リポジトリの構成

| パス | 役割 |
| --- | --- |
| [`apps/web/`](apps/web/) | Astro と React による Web クライアント。Cloudflare Pages に配信 |
| [`apps/server/`](apps/server/) | Go と Fiber による GraphQL・REST API。MySQL と Redis を使用 |
| [`apps/mcp/`](apps/mcp/) と [`apps/mcp-client-files/`](apps/mcp-client-files/) | TypeScript 製の MCP HTTP サービス、stdio ブリッジ、クライアント配布用ファイル |
| [`apps/email-compose/`](apps/email-compose/) | 通知や認証メールを生成する Node.js 製の gRPC サービス |
| [`apps/docs/openapi/`](apps/docs/openapi/) | OpenAPI 定義と API リファレンス |
| [`batches/`](batches/) | 通知、日直、モデレーション、DB バックアップを行う Go ジョブ |
| [`infra/`](infra/) | Docker Compose 構成、Airflow の DAG と実行環境、監視、CI ランナー、運用スクリプト |
| [`cloudflare-email-workers/`](cloudflare-email-workers/) | [`wrangler.toml`](wrangler.toml) で設定する受信メール転送 Worker |
| [`docs/`](docs/) | 開発ガイド、システム構成、運用手順 |
| [`.github/workflows/`](.github/workflows/) | CI、リリース、デプロイ、定期実行ワークフロー |
| [`scripts/`](scripts/) | Go の静的解析など、リポジトリ用のツール |
| [`samples/`](samples/) と [`login/`](login/) | 単独で動作する Airflow の Compose 設定例とログイン画面のスクリーンショット |

### 関連リポジトリ

| リポジトリ | 役割 |
| --- | --- |
| [`taskly-auth`](https://github.com/asari194617/taskly-auth)（非公開） | Taskly と長岡高専企業ガイダンスで共用するログイン、OAuth、アカウント設定 |
| [`taskly-nnct-corp-guidance`](https://github.com/asari194617/taskly-nnct-corp-guidance) | 長岡高専企業ガイダンスの Web アプリ、API、Taskly の課題完了と連携する MCP |
| [`taskly-policy`](https://github.com/asari194617/taskly-policy) | 版管理されたポリシー本文とリリースマニフェスト |
| [`taskly-policy-web`](https://github.com/asari194617/taskly-policy-web) | `taskly-policy` のリリースを掲載する公開ポリシーサイト |
| [`taskly-environment-ribbon`](https://github.com/asari194617/taskly-environment-ribbon) | 本番環境とローカル開発環境の画面に印を表示する任意導入の Chrome 拡張機能 |

## ローカル開発

Taskly と関連サービスを `local.` ドメインで開くには、[ローカル HTTPS のセットアップ](docs/local-development.md#ローカル-https)を使います。以下のループバック HTTP コマンドも、分離した開発環境や自動テストで利用できます。

- Docker と Docker Compose v2
- Web クライアントと TypeScript の検証に使う Node.js 26（CI は 26.13.1）と npm
- ローカル検証に使う Go 1.27.4 以降（CI と Docker ビルドは 1.27.4）
- 運用スクリプトに使う Bash 4 以降と、Airflow の DB 初期化に使う `jq`

Web ログインは別サービスの **taskly-auth** を使います。認証サービス本体、認証用 DB、OAuth クライアントの登録は別途用意します。API の起動前に、[ローカル開発ガイド](docs/local-development.md)に従って `.env.example` をもとに認証設定を選び、アプリケーションのスキーマを適用してください。

任意で利用できる[長岡高専企業ガイダンスアプリ](docs/nnct-corp-guidance-development.md)は、専用のリポジトリ、DB、OAuth クライアントを使います。この Compose 構成とは別にセットアップします。

ガイドに従い、リポジトリのルートで `.env` を作成・設定します。

```bash
cp .env.example .env
# 続行する前に認証を設定し、仮のシークレットを置き換える。
chmod 600 .env
install -m 600 .env infra/.env
infra/ops/apply-schema.sh
docker compose -f infra/docker-compose.yml up -d --build --wait server mcp
curl -fsS http://localhost:3063/readyz
curl -fsS http://localhost:3066/readyz
```

MySQL、Redis、email-compose、API、MCP が起動します。Airflow を使う場合は、3 つのサービスを起動する前に [Airflow のセットアップ手順](docs/local-development.md#airflow-のセットアップ任意)に従い、専用のメタデータ DB を初期化します。

Web クライアントは別途起動します。

```bash
cp apps/web/.env.example apps/web/.env
npm --prefix apps/web ci
npm --prefix apps/web run dev
```

| コンポーネント | URL |
| --- | --- |
| Web クライアント | `http://localhost:4321` |
| API / GraphQL | `http://localhost:3063` / `http://localhost:3063/graphql` |
| API の生存確認 / 準備状態の確認 | `http://localhost:3063/healthz` / `http://localhost:3063/readyz` |
| API メトリクス | `http://localhost:3064/metrics` |
| MCP HTTP API | `http://localhost:3066` |
| Airflow の管理画面 | `http://localhost:3065` |

API、メトリクス、MCP、Airflow のポートはホストのループバックにのみバインドします。MySQL、Redis、email-compose は Compose ネットワーク内からのみ接続できます。詳細は[本番環境のポート一覧](docs/operations/network-ports.md)を参照してください。

ローカル環境は次のコマンドで停止します。開発データを格納する名前付きボリュームは保持されます。

```bash
docker compose -f infra/docker-compose.yml down
```

DB スキーマの変更は `apps/server/schema.sql` に宣言します。コンテナで動く Atlas でスキーマを適用します。この手順では MySQL を起動し、準備が完了するまで待機します。

```bash
infra/ops/apply-schema.sh
```

コマンドは最初に変更計画を表示します。破壊的変更を適用するには、内容を明示的にレビューしたうえで `ATLAS_ALLOW_DESTRUCTIVE=true` を設定してください。運用スクリプトを実行する前に `infra/.env` を `.env` と同期します。`apps/server/migrations` の過去のファイルは監査用に保存しています。

## 検証

最初に各 Node.js パッケージの依存関係を `npm --prefix apps/<component> ci` でインストールします（MCP は `ci --legacy-peer-deps`）。変更したコンポーネントに応じて確認を実行してください。

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

Web の[ブラウザテスト](apps/web/tests/browser/README.md)、[実 API を使う E2E テスト](apps/web/tests/e2e/README.md)、[Airflow の Python テスト](docs/operations/airflow-python-tests.md)には追加のセットアップがあります。通知の配信チャネルは個別の Go モジュールです。変更した場合は `batches/notifications/delivery-channels/{common,email,discord,web-push}` の各ディレクトリで `go test ./...` を実行します。CI の詳細は[カバレッジの確認](docs/testing/coverage.md)を参照してください。

## ドキュメント

[ドキュメント一覧](docs/README.md)から、現行の資料と過去の要件・提案を参照できます。

- [`docs/Specification.md`](docs/Specification.md) — プロダクト要件と計画
- [`docs/schema.md`](docs/schema.md) — スキーマの概要。実際の定義は `apps/server/schema.sql`
- [`docs/mcp-docs.md`](docs/mcp-docs.md) — MCP クライアントとサーバーのガイド
- [`docs/initial-settings.md`](docs/initial-settings.md) — CI/CD と本番運用
- [`apps/docs/openapi/openapi.yaml`](apps/docs/openapi/openapi.yaml) — OpenAPI 定義

## 本番デプロイ

対象パスに変更がある PR が `main` にマージされると、[`deploy-prod.yml`](.github/workflows/deploy-prod.yml) が起動します。コミット SHA をタグとするイメージをリリースし、アプリケーションのスキーマを直前に確定した本番の基準状態と照合してから、リポジトリを同期して Docker Compose でサービスを更新します。スキーマ変更は別途レビューして保守作業を行います。Web クライアントは Cloudflare Pages が個別にデプロイします。

4 つすべてのサービスイメージにリリースの SHA タグを付けます。実行中の Airflow コンテナを再作成するのはイメージや Compose 設定が変わったとき、またはサービスが存在しないときです。このため Airflow の稼働中の SHA は API、MCP、email-compose の SHA と異なる場合があります。`apps/server`、`apps/mcp`、`apps/email-compose`、`batches`、`infra` 配下は README だけの変更もデプロイ対象です。[デプロイ手順](docs/initial-settings.md)と[ランナープールの運用ガイド](infra/actions-runner/README.md)を参照してください。

## ソースコードの公開とライセンス

現在、Taskly はリポジトリ全体に適用されるオープンソースライセンスを採用していません。個別のライセンスを明記したファイルやディレクトリを除き、著作権者がすべての権利を留保します。`apps/mcp-client-files` は同ディレクトリにある MIT ライセンスのもとで利用できます。

適用範囲は [`LICENSE`](LICENSE) を参照してください。コントリビューションを提案する場合は [`CONTRIBUTING.md`](CONTRIBUTING.md) を事前に確認してください。
