# コントリビューション

Taskly はフォークからのプルリクエストを受け付けています。フォークでブランチを作成して変更し、Taskly の `main` を対象に PR を開いてください。PR テンプレートに変更内容と確認結果を記載し、画面に変更がある場合はスクリーンショットを添えてください。

外部コントリビューターのワークフローは、メンテナーが変更を確認してから実行を承認します。PR のチェックは GitHub-hosted runner で実行し、マージには署名済みコミットと[必須チェック](docs/operations/github-actions-required-checks.md)が必要です。

公開 Issue は、再現可能な不具合の報告や対象を絞った機能提案に利用できます。Issue や PR に個人情報や機密情報を含めないでください。脆弱性の疑いは [`SECURITY.md`](SECURITY.md) の手順に従って非公開で報告してください。

Taskly の利用には [`LICENSE`](LICENSE) の条件が適用されます。共有する権限を持つ内容を提出してください。外部からの変更をマージする前に、メンテナーは対象ファイルの条件でその変更を取り込み、配布するための書面による許可を確認します。

## ローカルでの静的解析

リポジトリのルートで実行します。

```sh
npm ci
npm run lint
npm run lint:go
python3 -m venv /tmp/taskly-lint-venv
/tmp/taskly-lint-venv/bin/pip install -r requirements-lint.txt
/tmp/taskly-lint-venv/bin/ruff check .
```

`lint` ワークフローはすべてのプルリクエストと `main` で 3 種類のチェックを実行します。Biome のバージョンはルートのロックファイルで固定し、Web アプリ、MCP、メールサービス、メール Worker、リポジトリ内のスクリプトにある JavaScript、TypeScript、JSX、JSON、CSS、HTML を検査します。生成済みの protobuf とビルド出力は対象外です。`npm run lint:fix` は Biome の安全な修正を適用します。フォーマットは強制しません。Astro 固有の検査は引き続き `apps/web` の `npm run check` が行います。

Biome では推奨される正しさのルールと、不審なコードを検出する一部のルールを有効にしています。未使用の import・変数・引数と React フックのルールは既存コードの整理後に導入します。スタイルとアクセシビリティのルールは現在無効です。適用するルールは `biome.json` を確認してください。

Ruff は Airflow、監視、デプロイのスクリプトを含む Python コードの構文エラーと未定義名（`E9`、`F63`、`F7`、`F82`）を検査します。

Go vet は追跡対象の全 Go モジュールで個別に実行します。親モジュールの `./...` では対象にならない通知用のネストしたモジュールも含みます。[lint ワークフロー](.github/workflows/lint.yml)で固定したバージョン以降の Go と、モジュールの依存関係へのアクセスが必要です。
