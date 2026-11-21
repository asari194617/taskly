# PR コメントでの Terraform・Atlas 操作

リポジトリ所有者が、同じリポジトリ内のブランチからデフォルトブランチに向けたレビュー可能な PR に、コマンドを単独でコメントします。PR ブランチには最新のデフォルトブランチが含まれている必要があります。ジョブは GitHub-hosted runner で実行されます。

| 操作 | Terraform | Atlas |
| --- | --- | --- |
| 計画を作成 | `terraform plan` | `atlas plan` |
| 保存した最新の計画を適用 | `terraform apply` | `atlas apply` |
| 計画を復号 | `gpg --decrypt plan.txt.gpg` | `gpg --decrypt plan.sql.gpg` |

1. `plan` は GitHub が所有者の署名を検証できる最新コミットを取得し、実行対象をその SHA に固定します。署名を確認できないコミットを使う場合は、内容を確認したうえで `terraform plan <full-commit-sha>` または `atlas plan <full-commit-sha>` を使います。
2. PR 返信から最新の暗号化された Artifact をダウンロードして復号し、計画を確認します。対応する Environment の `TERRAFORM_PLAN_KEY` または `ATLAS_PLAN_KEY` を使います。
3. `apply` は、コメント投稿前に完了した、同じ PR・コミット・ベースの最新の保存結果を使います。明示する場合は `terraform apply <plan-run-id>` または `atlas apply <plan-run-id>` を使えます。以前のスラッシュ付き形式にも対応しています。

Artifact の保持期間は 3 日です。最新結果が変更なしの場合は、引数なしの `apply` もそこで停止します。PR のコミット、デフォルトブランチ、Terraform の state、Atlas の SQL 差分が変わった場合や Artifact が期限切れになった場合は、最新の状態で plan を作り直します。

## Terraform の初回設定

1. リポジトリ直下の `terraform/` に管理対象のルートモジュールを配置し、state locking を備えたリモートバックエンドを設定します。外部プロバイダーを使う場合は `.terraform.lock.hcl` をコミットします。新規リソースの作成や既存リソースの移行はルートモジュールの導入時に行います。
2. GitHub Environments に `terraform-plan` と `terraform-apply` を作成し、デプロイ元をデフォルトブランチに制限します。各 Environment に同じ値の `TERRAFORM_PLAN_KEY` をシークレットとして設定し、確認担当者と共有します。鍵は `openssl rand -hex 32` などで個別に生成します。
3. 両 Environment が同じ state とロックを参照するようにします。HTTP バックエンドでは Environment variables `TF_HTTP_ADDRESS`、`TF_HTTP_LOCK_ADDRESS`、`TF_HTTP_UNLOCK_ADDRESS` と、必要に応じて Environment secrets `TF_HTTP_USERNAME`、`TF_HTTP_PASSWORD` を使えます。plan には state の読み取りとロックに必要な権限、apply には state の更新に必要な権限を付与します。外部プロバイダーの認証が必要な場合は、ワークフローにその認証ステップを追加します。

Terraform は plan のときも PR に含まれるプロバイダーやデータソースを実行します。所有者は内容を確認したコミットだけを実行対象にし、各 Environment には対象の Terraform 構成に必要な認証情報を設定します。PR 返信と Actions ログには実行結果のみを載せ、詳細な plan は暗号化して保存します。

## Atlas の初回設定

1. `atlas-plan` と `atlas-apply` の GitHub Environments を作成し、デプロイ元をデフォルトブランチに制限します。両 Environment に同じ値の `ATLAS_PLAN_KEY` を設定します。
2. 両 Environment の変数 `ATLAS_DB_HOST`、`ATLAS_DB_NAME`、必要に応じて `ATLAS_DB_PORT` に同じ MySQL 8.4 インスタンスと DB を設定します。接続先は GitHub-hosted runner から到達でき、ホスト名を検証できる TLS 証明書が必要です。独自 CA を使う場合は PEM を `ATLAS_DB_CA` シークレットに保存します。
3. 両 Environment の `ATLAS_DB_USER` と `ATLAS_DB_PASSWORD` に専用アカウントを設定します。plan 用アカウントには対象スキーマの参照権限を与え、apply 用は承認済みの DDL を実行する権限を与えます。初回の動作確認には検証 DB を使います。

Atlas Community はデフォルトブランチの `atlas.hcl` と PR の `apps/server/schema.sql` から、使い捨ての開発 DB を使用して SQL を計画します。apply 前に対象 MySQL の UUID とデータベース名、再計算した SQL が計画と一致することを確認し、保存済みの SQL を実行します。同じ DB に対する別経路の DDL は apply 中に停止してください。破壊的な変更や本番スキーマの基準更新は [スキーマ保守の手順](no-ddl-code-release.md#schema-maintenance)に従います。MySQL DDL が途中で失敗した場合は、実スキーマを確認してから計画を作り直します。
