const isRepoWriter = require('./repo-writer.cjs');

async function authorizeSelfMerge(github, context) {
  const { comment, issue, repository } = context.payload;
  const match = /^\/?self-merge ([0-9a-f]{40})$/.exec(comment.body);
  if (match?.[0] !== comment.body) return null;
  if (!await isRepoWriter(github, context.repo, comment.user)) return null;

  const { data: pr } = await github.rest.pulls.get({ ...context.repo, pull_number: issue.number });
  if (pr.user?.id !== comment.user.id || pr.state !== 'open' || pr.draft ||
      pr.base.repo?.id !== repository.id || pr.base.ref !== repository.default_branch ||
      pr.head.sha !== match[1]) {
    return null;
  }
  return { ...context.repo, pull_number: issue.number, sha: match[1] };
}

async function mergeOwnPR(github, app, context, wait = (ms) => new Promise(resolve => setTimeout(resolve, ms))) {
  const params = await authorizeSelfMerge(github, context);
  if (!params) throw new Error('The request no longer matches an open PR owned by a repository writer.');

  const headers = { 'x-github-api-version': '2026-03-10' };
  const endpoint = '/repos/{owner}/{repo}/pulls/{pull_number}/merge-async';
  const { data: initial } = await app.request(`PUT ${endpoint}`, {
    ...params, headers, merge_action: 'direct_merge', merge_method: 'merge', bypass_rules: true,
  });
  let result = initial;
  const uuid = result.details?.uuid;
  for (let attempt = 0; result.status === 'pending' && uuid && attempt < 15; attempt++) {
    await wait(2000);
    const response = await app.request(`GET ${endpoint}/{uuid}`, {
      owner: params.owner, repo: params.repo, pull_number: params.pull_number, uuid, headers,
    });
    result = response.data;
  }
  if (result.status !== 'merged') {
    throw new Error(result.details?.message || 'GitHub has not confirmed the merge.');
  }
  return result.details.sha;
}

module.exports = { authorizeSelfMerge, mergeOwnPR };
