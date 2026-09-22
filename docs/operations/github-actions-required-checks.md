# main の必須 Actions チェック

main の ruleset は `api-build`、`api-empty-database`、`web-build-and-test`、`mcp-build`、`email-compose-build`、`compose-build` の 6 件を必須にする。各名前は対応する pull request ワークフローのジョブ名と一致させる。

必須チェックを生成するワークフローには `pull_request.paths` を設定しない。変更パスでワークフローが起動しない場合、そのチェックは成功にも失敗にもならず、ruleset がマージを待ち続けるため。Compose のイメージ選択はワークフロー内の `plan` ジョブで行う。

チェックが完了しても PR が `BLOCKED` のままなら、`gh api repos/asari194617/taskly/rules/branches/main` で必須コンテキストを確認し、`gh pr checks <PR番号>` の表示名と比較する。
