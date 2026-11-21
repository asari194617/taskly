# PR コメントでの Atlas 操作

リポジトリに書き込み権限があるメンバーが、同じリポジトリ内のブランチからデフォルトブランチに向けたレビュー可能な PR にコメントします。PR ブランチには最新のデフォルトブランチが含まれている必要があります。ジョブは GitHub-hosted runner で実行されます。

1. `atlas plan` とコメントします。通常は GitHub がコメント投稿者本人の署名を検証できる最新コミットを取得し、実行対象をその SHA に固定します。別の署名者や未署名のコミットを使う場合は、内容を確認したうえで `atlas plan <full-commit-sha>` と指定します。
2. PR 返信から最新の暗号化された Artifact をダウンロードします。対応する Environment の `ATLAS_PLAN_KEY` を使い、`gpg --decrypt plan.sql.gpg` で計画を確認します。
3. `atlas apply` とコメントします。投稿前に完了した、同じ PR・コミット・ベースの最新の保存結果を使います。対象を指定する場合は `atlas apply <plan-run-id>` を使えます。スラッシュ付き形式にも対応しています。

Artifact の保持期間は 3 日です。最新結果が変更なしの場合は `apply` が停止します。PR のコミット、デフォルトブランチ、Atlas の SQL 差分が変わった場合や Artifact が期限切れになった場合は、計画を作り直します。

## Atlas の初回設定

1. `atlas-plan` と `atlas-apply` の GitHub Environments を作成し、デプロイ元をデフォルトブランチに制限します。両 Environment に同じ値の `ATLAS_PLAN_KEY` を設定します。
2. 両 Environment の変数 `ATLAS_DB_HOST`、`ATLAS_DB_NAME`、必要に応じて `ATLAS_DB_PORT` に同じ MySQL 8.4 インスタンスと DB を設定します。接続先は GitHub-hosted runner から到達でき、ホスト名を検証できる TLS 証明書が必要です。独自 CA を使う場合は PEM を `ATLAS_DB_CA` シークレットに保存します。
3. 両 Environment の `ATLAS_DB_USER` と `ATLAS_DB_PASSWORD` に専用アカウントを設定します。plan 用アカウントには対象スキーマの参照権限を与え、apply 用は承認済みの DDL を実行する権限を与えます。初回の動作確認には検証 DB を使います。

Atlas Community はデフォルトブランチの `atlas.hcl` と PR の `apps/server/schema.sql` から、使い捨ての開発 DB を使用して SQL を計画します。apply 前に対象 MySQL の UUID とデータベース名、再計算した SQL が計画と一致することを確認し、保存済みの SQL を実行します。同じ DB に対する別経路の DDL は apply 中に停止してください。破壊的な変更や本番スキーマの基準更新は [スキーマ保守の手順](no-ddl-code-release.md#schema-maintenance)に従います。MySQL DDL が途中で失敗した場合は、実スキーマを確認してから計画を作り直します。
