import assert from 'node:assert/strict';
import test from 'node:test';
import resolveRequest from './infra-pr-request.cjs';

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
  path: '.github/workflows/terraform-pr-comments.yml',
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
      comment: { body, created_at: commentAt },
      issue: { number: 3 },
      repository: { id: 7, default_branch: 'main' },
    },
  };
  const github = {
    rest: {
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
        signer: { login: options.signer ?? 'owner' },
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
  for (const tool of ['terraform', 'atlas']) {
    const { outputs, calls } = await resolve(`${tool} plan`);
    assert.equal(outputs.tool, tool);
    assert.equal(outputs.revision_sha, revision);
    assert.deepEqual(calls, ['verify-signature']);
  }

  assert.match((await resolve('terraform plan', { signer: 'someone-else' })).outputs.message, /verified owner signature/);
  const explicit = await resolve(`/terraform plan ${revision}`, { signer: 'someone-else' });
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
  const other = { ...savedPlan, id: 103, name: 'terraform-pr-3' };
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
