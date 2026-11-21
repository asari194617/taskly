# PR のレビューとセルフマージ

`main` に向けた PR のマージには、リポジトリの書き込み権限を持つメンバーからの承認が 1 件必要です。書き込み権限を持つメンバーが自分で作成した PR は、承認の代わりに本人が `self-merge <full-commit-sha>` とコメントできます。対象コミットは PR の先頭の 40 桁の SHA を指定します。必須 CI とコミット署名はどちらの場合も必要です。

ワークフローは PR が開いていてレビュー可能な状態であること、コメント投稿者と PR 作成者のアカウント ID が一致すること、投稿者に現在も書き込み権限があることを確認します。GitHub App は指定された SHA に対してマージを依頼し、結果を返信します。コミットが変わった場合は新しい内容を確認してからコメントし直します。

## GitHub の設定

1. 非公開の専用 GitHub App を登録します。Repository permissions の Contents を Read and write にし、Webhook とイベント購読を無効にします。インストール対象は Taskly のみにします。
2. `pr-self-merge` の GitHub Environment を作成し、デプロイ元を `main` ブランチだけに制限します。App のクライアント ID を Environment 変数 `SELF_MERGE_APP_CLIENT_ID`、秘密鍵を Environment シークレット `SELF_MERGE_APP_PRIVATE_KEY` に設定します。
3. `main` に対し、承認 1 件を要求するレビュー専用ルールセットを追加します。For pull requests only のバイパスを専用 GitHub App にだけ付与します。既存の `Protect main branch` ルールセットが必須 CI、署名、PR 経由のマージを引き続き強制します。
