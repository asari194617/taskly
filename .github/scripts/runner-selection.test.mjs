import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

const workflow = readFileSync(new URL('../workflows/select-runner.yml', import.meta.url), 'utf8');
const bootstrap = workflow.match(/runs-on: \$\{\{ (.+) \}\}/)[1];
const output = workflow.match(/LABEL: \$\{\{ (.+) \}\}/)[1];
const selfHosted = ['self-hosted', 'taskly-ci'];
const hosted = ['ubuntu-latest'];

for (const [name, eventName, headRepo, override, variable, expected] of [
  ['internal PR', 'pull_request', 'owner/taskly', 'auto', '', selfHosted],
  ['external fork PR', 'pull_request', 'external/taskly', 'auto', '', hosted],
  ['deleted fork repository', 'pull_request', '', 'auto', '', hosted],
  ['fork cannot override trust check', 'pull_request', 'external/taskly', 'self-hosted', '', hosted],
  ['main deploy', 'push', '', 'auto', '', selfHosted],
  ['manual default', 'workflow_dispatch', '', 'auto', '', selfHosted],
  ['manual paid override', 'workflow_dispatch', '', 'ubuntu-latest', '', hosted],
  ['repository paid override', 'pull_request', 'owner/taskly', 'auto', 'ubuntu-latest', hosted],
  ['trusted comment handler', 'workflow_run', '', 'auto', '', selfHosted],
]) {
  test(name, () => {
    const context = {
      github: { event_name: eventName, repository: 'owner/taskly', event: {
        pull_request: { head: { repo: { full_name: headRepo } } },
      } },
      inputs: { runner_override: override }, vars: { CI_RUNNER: variable },
      fromJSON: JSON.parse,
    };
    assert.deepEqual(runInNewContext(bootstrap, context), expected);
    assert.deepEqual(JSON.parse(runInNewContext(output, context)), expected);
  });
}

test('selection does not need checkout, secrets, or production access', () => {
  assert.doesNotMatch(workflow, /uses:|secrets\.|taskly-prod/);
  assert.match(workflow, /permissions: \{\}/);
});
