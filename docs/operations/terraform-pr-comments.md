# PR コメントでの Terraform plan と apply

リポジトリ所有者が、同じリポジトリ内のブランチからデフォルトブランチに向けたレビュー可能な PR に、コマンドを単独でコメントします。PR ブランチには最新のデフォルトブランチが含まれている必要があります。ジョブは GitHub-hosted runner で実行されます。

1. PR のコミットを確認し、`/terraform plan` とコメントします。変更がある場合、返信に暗号化された plan と適用コマンドが表示されます。
2. Artifact をダウンロードし、`gpg --decrypt plan.txt.gpg` で内容を確認します。復号には `TERRAFORM_PLAN_KEY` と同じ鍵を使います。
3. 適用する場合は、返信にある `/terraform apply <plan-run-id>` を新しいコメントとして投稿します。Terraform は指定された実行の保存済み plan を適用し、結果を PR に返信します。

Artifact の保持期間は 3 日です。PR のコミット、デフォルトブランチ、Terraform の state のいずれかが更新された場合や Artifact が期限切れになった場合は、最新の状態で plan を作り直します。失敗後は state と実リソースを確認し、新しい plan を作成してください。

## 初回設定

1. リポジトリ直下の `terraform/` に管理対象のルートモジュールを配置し、state locking を備えたリモートバックエンドを設定します。外部プロバイダーを使う場合は `.terraform.lock.hcl` をコミットします。新規リソースの作成や既存リソースの移行はルートモジュールの導入時に行います。
2. GitHub Environments に `terraform-plan` と `terraform-apply` を作成し、デプロイ元をデフォルトブランチに制限します。各 Environment に同じ値の `TERRAFORM_PLAN_KEY` をシークレットとして設定し、確認担当者と共有します。鍵は `openssl rand -hex 32` などで個別に生成します。
3. 両 Environment が同じ state とロックを参照するようにします。HTTP バックエンドでは Environment variables `TF_HTTP_ADDRESS`、`TF_HTTP_LOCK_ADDRESS`、`TF_HTTP_UNLOCK_ADDRESS` と、必要に応じて Environment secrets `TF_HTTP_USERNAME`、`TF_HTTP_PASSWORD` を使えます。plan には state の読み取りとロックに必要な権限、apply には state の更新に必要な権限を付与します。外部プロバイダーの認証が必要な場合は、ワークフローにその認証ステップを追加します。

Terraform は plan のときも PR に含まれるプロバイダーやデータソースを実行します。所有者は内容を確認したコミットだけを実行対象にし、各 Environment には対象の Terraform 構成に必要な認証情報を設定します。PR 返信と Actions ログには実行結果のみを載せ、詳細な plan は暗号化して保存します。
