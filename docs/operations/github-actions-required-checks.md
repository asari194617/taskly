# main の必須 Actions チェック

main の ruleset は `api-build`、`api-empty-database`、`web-build-and-test`、`mcp-build`、`email-compose-build`、`compose-build`、`secret-scan` の 7 件を必須にする。各名前は対応する pull request ワークフローのジョブ名と一致させる。

`secret-scan` は GitHub-hosted runner で取得済みの全履歴を検査する。確認済みの誤検出は `.gitleaksignore` の特定 fingerprint のみ除外し、新規の認証情報を広く無視するルールは追加しない。

必須チェックを生成するワークフローには `pull_request.paths` を設定しない。変更パスでワークフローが起動しない場合、そのチェックは成功にも失敗にもならず、ruleset がマージを待ち続けるため。Compose のイメージ選択はワークフロー内の `plan` ジョブで行う。

チェックが完了しても PR が `BLOCKED` のままなら、`gh api repos/asari194617/taskly/rules/branches/main` で必須コンテキストを確認し、`gh pr checks <PR番号>` の表示名と比較する。
