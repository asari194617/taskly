const API = 'https://api.github.com';

export function completedJstWeek(now = new Date()) {
  const jst = new Date(now.getTime() + 9 * 60 * 60 * 1000);
  const day = (jst.getUTCDay() + 6) % 7;
  const end = Date.UTC(jst.getUTCFullYear(), jst.getUTCMonth(), jst.getUTCDate() - day) - 9 * 60 * 60 * 1000;
  return { start: new Date(end - 7 * 24 * 60 * 60 * 1000), end: new Date(end) };
}

async function jsonResponse(fetcher, url, options) {
  const response = await fetcher(url, options);
  if (!response.ok) throw new Error(`${new URL(url).host} returned HTTP ${response.status}`);
  return response.json();
}

export async function collectActivity({ repo, token, start, end, fetcher = fetch }) {
  const headers = {
    Accept: 'application/vnd.github+json',
    Authorization: `Bearer ${token}`,
    'X-GitHub-Api-Version': '2022-11-28',
  };
  async function pages(path, stop) {
    const result = [];
    for (let page = 1; ; page++) {
      const separator = path.includes('?') ? '&' : '?';
      const items = await jsonResponse(fetcher, `${API}/repos/${repo}/${path}${separator}per_page=100&page=${page}`, { headers });
      result.push(...items);
      if (items.length < 100 || stop?.(items)) return result;
    }
  }
  const since = encodeURIComponent(start.toISOString());
  const until = encodeURIComponent(end.toISOString());
  const [commits, issueEvents, pulls] = await Promise.all([
    pages(`commits?since=${since}&until=${until}`),
    pages(`issues?state=all&sort=updated&direction=desc&since=${since}`),
    pages('pulls?state=closed&sort=updated&direction=desc', items =>
      items.at(-1)?.updated_at < start.toISOString()),
  ]);
  const within = value => value && value >= start.toISOString() && value < end.toISOString();
  const issues = issueEvents.filter(item => !item.pull_request);
  return {
    commits: commits.filter(item => within(item.commit?.committer?.date)),
    opened: issues.filter(item => within(item.created_at)),
    closed: issues.filter(item => within(item.closed_at)),
    merged: pulls.filter(item => within(item.merged_at)),
  };
}

function line(item) {
  const title = (item.title ?? item.commit?.message?.split('\n')[0] ?? '').replace(/[\r\n]/g, ' ').slice(0, 100);
  return `• ${title} — ${item.html_url}`;
}

function fieldValue(items) {
  if (!items.length) return 'None';
  const lines = [];
  for (const item of items.slice(0, 5)) {
    const next = line(item);
    if (lines.join('\n').length + next.length + 1 > 1024) break;
    lines.push(next);
  }
  return lines.join('\n') || `${items.length} items (links too long to display)`;
}

export function discordPayload(repo, { start, end }, activity) {
  const date = value => new Date(value.getTime() + 9 * 60 * 60 * 1000).toISOString().slice(0, 10);
  const sections = [
    ['Commits', activity.commits],
    ['Opened issues', activity.opened],
    ['Closed issues', activity.closed],
    ['Merged PRs', activity.merged],
  ];
  return {
    allowed_mentions: { parse: [] },
    embeds: [{
      title: `${repo} — Weekly GitHub activity`,
      url: `https://github.com/${repo}`,
      description: `${date(start)}〜${date(new Date(end.getTime() - 1))} (JST)`,
      color: 0x5865f2,
      fields: sections.map(([name, items]) => ({
        name: `${name}: ${items.length}`,
        value: fieldValue(items),
        inline: false,
      })),
    }],
  };
}

export async function sendWeeklyReport({ repo, token, webhook, now = new Date(), fetcher = fetch }) {
  if (!repo || !token || !webhook) throw new Error('GITHUB_REPOSITORY, GITHUB_TOKEN and WEEKLY_DISCORD_WEBHOOK_URL are required');
  const window = completedJstWeek(now);
  const activity = await collectActivity({ repo, token, ...window, fetcher });
  const response = await fetcher(webhook, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(discordPayload(repo, window, activity)),
  });
  if (!response.ok) throw new Error(`Discord returned HTTP ${response.status}`);
  return activity;
}

if (process.argv[1] && import.meta.url === new URL(`file://${process.argv[1]}`).href) {
  sendWeeklyReport({
    repo: process.env.GITHUB_REPOSITORY,
    token: process.env.GITHUB_TOKEN,
    webhook: process.env.WEEKLY_DISCORD_WEBHOOK_URL,
  }).then(activity => {
    console.log(`Weekly Discord report sent: ${activity.commits.length} commits, ${activity.opened.length} opened issues, ${activity.closed.length} closed issues, ${activity.merged.length} merged PRs`);
  }).catch(error => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
