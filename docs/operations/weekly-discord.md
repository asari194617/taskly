# Weekly GitHub activity on Discord

`.github/workflows/weekly-discord.yml` posts a report each Monday at 09:00 JST.
The report covers the previous Monday 00:00 through this Monday 00:00 JST
and includes default-branch commits, issues opened or closed, and merged pull requests.
Each category shows its total and up to five links. GitHub Actions schedules can run late.

Set the repository Actions secret `WEEKLY_DISCORD_WEBHOOK_URL` to the target
Discord channel's webhook URL. The workflow uses its read-only `GITHUB_TOKEN`
for GitHub data. Missing credentials or an API/webhook error fail the run.

Use the workflow's **Run workflow** button to post the most recently completed
week again. This is a real Discord post, so manual runs can duplicate a report.

Run the local tests with:

```sh
node --test .github/scripts/weekly-discord.test.mjs
```
