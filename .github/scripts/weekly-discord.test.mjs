import assert from 'node:assert/strict';
import { test } from 'node:test';
import { collectActivity, completedJstWeek, discordPayload, sendWeeklyReport } from './weekly-discord.mjs';

test('completed week uses Japanese Monday boundaries', () => {
  const { start, end } = completedJstWeek(new Date('2026-10-12T00:00:00Z'));
  assert.equal(start.toISOString(), '2026-10-04T15:00:00.000Z');
  assert.equal(end.toISOString(), '2026-10-11T15:00:00.000Z');
});

test('collects only events in the finished week and excludes pull requests from issues', async () => {
  const start = new Date('2026-10-04T15:00:00Z');
  const end = new Date('2026-10-11T15:00:00Z');
  const calls = [];
  const fetcher = async url => {
    calls.push(url);
    const path = new URL(url).pathname;
    const data = path.endsWith('/commits') ? [
      { sha: 'a', commit: { committer: { date: start.toISOString() } } },
      { sha: 'b', commit: { committer: { date: end.toISOString() } } },
    ] : path.endsWith('/issues') ? [
      { number: 1, created_at: start.toISOString(), closed_at: end.toISOString() },
      { number: 2, created_at: end.toISOString(), closed_at: null },
      { number: 3, pull_request: {}, created_at: start.toISOString() },
      { number: 4, created_at: '2026-09-01T00:00:00Z', closed_at: start.toISOString() },
    ] : [
      { number: 5, merged_at: start.toISOString(), updated_at: start.toISOString() },
      { number: 6, merged_at: end.toISOString(), updated_at: end.toISOString() },
    ];
    return { ok: true, json: async () => data };
  };
  const result = await collectActivity({ repo: 'owner/repo', token: 'test', start, end, fetcher });
  assert.deepEqual(Object.fromEntries(Object.entries(result).map(([key, items]) => [key, items.map(item => item.number ?? item.sha)])), {
    commits: ['a'], opened: [1], closed: [4], merged: [5],
  });
  assert.equal(calls.length, 3);
});

test('sends one Discord embed with counts and disabled mentions', async () => {
  const requests = [];
  const fetcher = async (url, options) => {
    requests.push({ url, options });
    return url === 'https://example.test/hook'
      ? { ok: true }
      : { ok: true, json: async () => [] };
  };
  await sendWeeklyReport({ repo: 'owner/repo', token: 'test', webhook: 'https://example.test/hook', now: new Date('2026-10-12T00:00:00Z'), fetcher });
  const payload = JSON.parse(requests.at(-1).options.body);
  assert.deepEqual(payload.allowed_mentions, { parse: [] });
  assert.equal(payload.embeds[0].fields.length, 4);
  assert.equal(payload.embeds[0].description, '2026-10-05〜2026-10-11 (JST)');
  assert.equal(discordPayload('owner/repo', completedJstWeek(new Date('2026-10-12T00:00:00Z')), { commits: [], opened: [], closed: [], merged: [] }).embeds[0].fields[0].value, 'None');
});
