# Discord への GitHub 活動の週次レポート

`.github/workflows/weekly-discord.yml` は毎週月曜日の 09:00（日本時間）にレポートを投稿します。対象期間は前週月曜日 00:00 から当週月曜日 00:00（日本時間）までです。既定ブランチのコミット、作成・クローズされた Issue、マージされたプルリクエストを集計し、各カテゴリの件数と最大 5 件のリンクを掲載します。GitHub Actions の定期実行は遅れることがあります。

対象の Discord チャンネルの Webhook URL をリポジトリの Actions シークレット `WEEKLY_DISCORD_WEBHOOK_URL` に設定します。GitHub データの取得には読み取り専用の `GITHUB_TOKEN` を使います。認証情報の不足や API・Webhook のエラーがある場合は実行に失敗します。

ワークフローの **Run workflow** ボタンで、直近に完了した週のレポートを再投稿できます。実際に Discord へ投稿するため、手動実行で投稿が重複する場合があります。

ローカルテストは以下を実行します。

```sh
node --test .github/scripts/weekly-discord.test.mjs
```
