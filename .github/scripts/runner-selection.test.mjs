import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const workflow = readFileSync(new URL('../workflows/select-runner.yml', import.meta.url), 'utf8');
const marker = '          script: |\n';
const source = workflow.slice(workflow.indexOf(marker) + marker.length)
  .split('\n')
  .map((line) => line.startsWith('            ') ? line.slice(12) : line)
  .join('\n');
const execute = new Function(
  'github', 'context', 'core', 'process',
  `return (async () => { ${source} })();`,
);

const primary = ['self-hosted', 'taskly-ci-primary'];
const secondary = ['self-hosted', 'taskly-ci-secondary'];
const hosted = ['ubuntu-latest'];

async function select({
  externalFork = false,
  inputOverride = 'auto',
  repositoryOverride = '',
  health = {},
} = {}) {
  let output;
  let failure;
  const calls = [];
  const github = {
    rest: {
      actions: {
        async listWorkflowRuns({ workflow_id: workflowId }) {
          calls.push(workflowId);
          const value = health[workflowId];
          if (value instanceof Error) throw value;
          return { data: { workflow_runs: value ? [{ updated_at: value }] : [] } };
        },
      },
    },
  };
  const core = {
    setOutput(name, value) {
      assert.equal(name, 'label');
      output = JSON.parse(value);
    },
    setFailed(message) { failure = message; },
    warning() {},
  };
  await execute(
    github,
    { repo: { owner: 'owner', repo: 'taskly' } },
    core,
    { env: {
      EXTERNAL_FORK: String(externalFork),
      RUNNER_OVERRIDE: inputOverride,
      REPOSITORY_RUNNER_OVERRIDE: repositoryOverride,
    } },
  );
  return { output, failure, calls };
}

const fresh = () => new Date(Date.now() - 60_000).toISOString();
const stale = () => new Date(Date.now() - 20 * 60_000).toISOString();

test('healthy primary pool is selected first', async () => {
  const result = await select({ health: {
    'runner-health-primary.yml': fresh(),
    'runner-health-secondary.yml': fresh(),
  } });
  assert.deepEqual(result.output, primary);
  assert.deepEqual(result.calls, ['runner-health-primary.yml']);
});

test('healthy secondary pool is selected when primary is stale', async () => {
  const result = await select({ health: {
    'runner-health-primary.yml': stale(),
    'runner-health-secondary.yml': fresh(),
  } });
  assert.deepEqual(result.output, secondary);
  assert.deepEqual(result.calls, ['runner-health-primary.yml', 'runner-health-secondary.yml']);
});

test('GitHub-hosted runner is selected when both pools are unavailable', async () => {
  const result = await select({ health: {
    'runner-health-primary.yml': new Error('unavailable'),
  } });
  assert.deepEqual(result.output, hosted);
});

test('external fork always stays on GitHub-hosted runner', async () => {
  const result = await select({ externalFork: true, inputOverride: 'primary' });
  assert.deepEqual(result.output, hosted);
  assert.deepEqual(result.calls, []);
});

for (const [name, override, variable, expected] of [
  ['manual primary override', 'primary', '', primary],
  ['manual secondary override', 'secondary', '', secondary],
  ['manual hosted override', 'ubuntu-latest', '', hosted],
  ['repository hosted override', 'auto', 'ubuntu-latest', hosted],
]) {
  test(name, async () => {
    const result = await select({ inputOverride: override, repositoryOverride: variable });
    assert.deepEqual(result.output, expected);
    assert.deepEqual(result.calls, []);
  });
}

test('unsupported override fails closed', async () => {
  const result = await select({ inputOverride: 'unexpected' });
  assert.match(result.failure, /Unsupported runner override/);
  assert.equal(result.output, undefined);
});

test('selection does not check out code, access secrets, or use production runners', () => {
  assert.doesNotMatch(workflow, /actions\/checkout|secrets\.|taskly-prod/);
  assert.match(workflow, /permissions:\n  actions: read/);
  assert.match(workflow, /runs-on: ubuntu-latest/);
  assert.match(workflow, /actions\/github-script@[0-9a-f]{40}/);
});
