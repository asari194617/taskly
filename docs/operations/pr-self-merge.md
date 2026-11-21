# PR のレビューとセルフマージ

`main` に向けた外部からの PR には、リポジトリの書き込み権限を持つメンバーからの承認が 1 件以上必要です。

書き込み権限を持つメンバーが作成した PR は、専用 GitHub App が最新コミットを自動で承認します。必須 CI とコミット署名の要件を満たしたら、通常どおり **Merge pull request** を押します。

通常の PR は作成時に、ドラフト PR は **Ready for review** に切り替えた時点で自動承認の対象になります。コミットを追加した場合は App が更新後のコミットを承認します。共通実行先の設定とログは [Taskly PR automation](https://github.com/asari194617/taskly-pr-automation) で管理します。
