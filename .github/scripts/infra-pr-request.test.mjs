import assert from 'node:assert/strict';
import test from 'node:test';
import resolveRequest from './atlas-pr-request.cjs';
import selfMerge from './self-merge.cjs';

const base = 'a'.repeat(40);
const revision = 'b'.repeat(40);
const commentAt = '2026-11-21T13:00:00Z';
const savedAt = '2026-11-21T12:59:00Z';
const artifactName = `atlas-pr-3-${base}-${revision}`;
const savedPlan = {
  id: 101,
  name: artifactName,
  expired: false,
  created_at: savedAt,
  workflow_run: { id: 201, head_sha: base, repository_id: 7 },
};
const workflow = {
  id: 201,
  path: '.github/workflows/atlas-pr-comments.yml',
  event: 'issue_comment',
  run_attempt: 1,
  conclusion: 'success',
  head_sha: base,
  updated_at: savedAt,
};

async function resolve(body, options = {}) {
  const outputs = {};
  const calls = [];
  const context = {
    repo: { owner: 'owner', repo: 'taskly' },
    sha: base,
    payload: {
      comment: { body, created_at: commentAt, user: { id: 5, login: 'member', type: 'User' } },
      issue: { number: 3 },
      repository: { id: 7, default_branch: 'main' },
    },
  };
  const github = {
    rest: {
      repos: { getCollaboratorPermissionLevel: async () => ({
        data: { permission: options.permission ?? 'write', user: { id: options.userId ?? 5 } },
      }) },
      pulls: { get: async () => ({ data: {
        state: 'open',
        draft: false,
        head: { repo: { id: 7 }, sha: revision },
        base: { repo: { id: 7 }, ref: 'main', sha: base },
        ...options.pr,
      } }) },
      actions: { getWorkflowRun: async ({ run_id }) => {
        calls.push(run_id);
        return { data: { ...workflow, id: run_id, ...options.run } };
      } },
    },
    graphql: async (_query, { sha }) => {
      assert.equal(sha, revision);
      calls.push('verify-signature');
      return { repository: { object: { signature: {
        isValid: true,
        signer: { login: options.signer ?? 'member' },
      } } } };
    },
    request: async () => ({ data: { status: options.comparison ?? 'ahead' } }),
    paginate: async (endpoint, params) => {
      calls.push([endpoint, params.run_id]);
      assert.equal(params.name, artifactName);
      return options.artifacts ?? [savedPlan];
    },
  };
  await resolveRequest(github, context, { setOutput: (key, value) => { outputs[key] = value; } });
  return { outputs, calls };
}

test('plan pins the signed current commit, with an explicit fallback for other signers', async () => {
  const { outputs, calls } = await resolve('atlas plan');
  assert.equal(outputs.revision_sha, revision);
  assert.deepEqual(calls, ['verify-signature']);
  assert.match((await resolve('atlas plan', { signer: 'owner' })).outputs.message, /verified signature from the commenter/);
  const explicit = await resolve(`/atlas plan ${revision}`, { signer: 'someone-else' });
  assert.equal(explicit.outputs.command, 'plan');
  assert.deepEqual(explicit.calls, []);
  assert.match((await resolve(`atlas plan ${'c'.repeat(40)}`)).outputs.message, /current PR commit/);
});

test('closed, forked, divergent and malformed requests are rejected', async () => {
  assert.match((await resolve('atlas plan', { pr: { state: 'closed' } })).outputs.message, /open, ready/);
  assert.match((await resolve('atlas plan', { pr: { head: { repo: { id: 9 }, sha: revision } } })).outputs.message, /open, ready/);
  assert.match((await resolve('atlas plan', { comparison: 'diverged' })).outputs.message, /Update the PR branch/);
  assert.match((await resolve('atlas apply\n')).outputs.message, /Usage/);
  assert.match((await resolve('atlas apply --force')).outputs.message, /numeric plan run ID/);
});

test('apply selects the latest matching saved result that precedes the comment', async () => {
  const later = { ...savedPlan, id: 104, created_at: commentAt };
  const older = { ...savedPlan, id: 100, created_at: '2026-11-21T12:58:00Z' };
  const other = { ...savedPlan, id: 103, name: 'unrelated-pr-3' };
  const newest = { ...savedPlan, id: 102, workflow_run: { ...savedPlan.workflow_run, id: 202 } };
  const { outputs, calls } = await resolve('atlas apply', { artifacts: [older, later, other, newest, savedPlan] });
  assert.equal(outputs.artifact_id, '102');
  assert.equal(outputs.plan_run, '202');
  assert.deepEqual(calls, [['GET /repos/{owner}/{repo}/actions/artifacts', undefined], 202]);
});

test('explicit run selection and workflow provenance remain mandatory', async () => {
  const { outputs, calls } = await resolve('/atlas apply 201');
  assert.equal(outputs.artifact_id, '101');
  assert.deepEqual(calls, [['GET /repos/{owner}/{repo}/actions/runs/{run_id}/artifacts', '201'], 201]);
  for (const run of [{ run_attempt: 2 }, { conclusion: 'failure' }, { path: '.github/workflows/other.yml' },
    { updated_at: commentAt }]) {
    assert.match((await resolve('atlas apply', { run })).outputs.message, /successful command run/);
  }
  assert.match((await resolve('atlas apply', { artifacts: [{ ...savedPlan, expired: true }] })).outputs.message, /No saved plan/);
});

test('Atlas runs only for verified repository writers', async () => {
  for (const options of [{ permission: 'none' }, { permission: 'read' }, { userId: 9 }]) {
    assert.deepEqual((await resolve('atlas plan', options)).outputs, {});
    assert.deepEqual((await resolve('atlas apply', options)).outputs, {});
  }
  assert.equal((await resolve('atlas plan', { permission: 'admin' })).outputs.command, 'plan');
});

const { authorizeSelfMerge, mergeOwnPR } = selfMerge;
const sha = 'a'.repeat(40);

function fixture(overrides = {}) {
  const context = {
    repo: { owner: 'owner', repo: 'taskly' },
    payload: {
      comment: { body: `self-merge ${sha}`, user: { id: 25, login: 'member', type: 'User' }, ...overrides.comment },
      issue: { number: 13 },
      repository: { id: 90, default_branch: 'main' },
    },
  };
  const github = { rest: {
    repos: { getCollaboratorPermissionLevel: async () => ({ data: {
      user: { id: 25 }, permission: 'write', ...overrides.member,
    } }) },
    pulls: { get: async () => ({ data: {
      state: 'open', draft: false, user: { id: 25 },
      head: { sha, repo: { id: 12345 } },
      base: { ref: 'main', repo: { id: 90 } },
      ...overrides.pr,
    } }) },
  } };
  return { github, context };
}

test('a repository writer can self-merge a pinned PR of their own, including from a fork', async () => {
  const { github, context } = fixture();
  assert.deepEqual(await authorizeSelfMerge(github, context), {
    owner: 'owner', repo: 'taskly', pull_number: 13, sha,
  });
});

test('readers, authors without write access, bots and other reviewers cannot request a bypass', async () => {
  for (const overrides of [
    { member: { permission: 'read' } },
    { member: { permission: 'none' } },
    { member: { user: { id: 99 } } },
    { comment: { user: { id: 25, type: 'Bot', login: 'member' } } },
    { pr: { user: { id: 99 } } },
  ]) {
    const { github, context } = fixture(overrides);
    assert.equal(await authorizeSelfMerge(github, context), null);
    const app = { request: () => assert.fail('App must not be used for an unauthorized request') };
    await assert.rejects(mergeOwnPR(github, app, context), /no longer matches/);
  }
});

test('closed, draft, retargeted and changed PRs are excluded', async () => {
  for (const pr of [
    { state: 'closed' }, { draft: true }, { head: { sha: 'b'.repeat(40) } },
    { base: { ref: 'development', repo: { id: 90 } } },
    { base: { ref: 'main', repo: { id: 91 } } },
  ]) {
    const { github, context } = fixture({ pr });
    assert.equal(await authorizeSelfMerge(github, context), null);
  }
});

test('merge is bound to the approved SHA and waits for GitHub to report a merge', async () => {
  const { github, context } = fixture();
  const seen = [];
  const app = { request: async (route, args) => {
    seen.push([route, args]);
    return route.startsWith('PUT')
      ? { data: { status: 'pending', details: { uuid: 'request-id' } } }
      : { data: { status: 'merged', details: { sha: 'merge-commit' } } };
  } };
  assert.equal(await mergeOwnPR(github, app, context, async () => {}), 'merge-commit');
  assert.equal(seen[0][1].sha, sha);
  assert.equal(seen[0][1].bypass_rules, true);
  assert.equal(seen[0][1].merge_method, 'merge');
  assert.equal(seen[1][1].uuid, 'request-id');

  const blockedApp = { request: async () => ({ data: { status: 'failed', details: { message: 'Required status check is missing' } } }) };
  await assert.rejects(mergeOwnPR(github, blockedApp, context), /Required status check is missing/);
});
