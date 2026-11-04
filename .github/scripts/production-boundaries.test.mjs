import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { runInNewContext } from 'node:vm';

function job(workflow, name) {
  const source = readFileSync(new URL(`../workflows/${workflow}.yml`, import.meta.url), 'utf8');
  const block = source.match(new RegExp(`^  ${name}:\\n([\\s\\S]*?)(?=^  [a-z][\\w-]*:|$(?![\\s\\S]))`, 'm'));
  assert.ok(block, `${workflow}/${name} exists`);
  return block[1];
}

function enabled(block, github, results = {}) {
  const expression = block.match(/^    if: (.+)$/m)?.[1]
    .replace(/^\$\{\{\s*|\s*\}\}$/g, '')
    .replace(/needs\.([\w-]+)/g, 'needs["$1"]')
    // GitHub expression string equality is case-insensitive.
    .replace(/github\.ref == '([^']+)'/g, 'github.ref.toLowerCase() === "$1".toLowerCase()');
  assert.ok(expression, 'job has an explicit condition');
  return runInNewContext(expression, {
    github,
    needs: Object.fromEntries(Object.entries(results).map(([name, result]) => [name, { result }])),
    always: () => true,
    cancelled: () => false,
  });
}

for (const [workflow, name, results] of [
  ['deploy-prod', 'select-runner', {}],
  ['deploy-prod', 'release-images', { plan: 'success', 'publish-images': 'success' }],
  ['deploy-prod', 'deploy', { plan: 'success', 'release-images': 'success' }],
  ['update-primary-runners', 'install', {}],
]) {
  test(`${workflow}/${name} accepts main and rejects other dispatch refs`, () => {
    const block = job(workflow, name);
    for (const ref of ['refs/heads/feature', 'refs/tags/main', 'refs/pull/1/merge', '']) {
      assert.equal(enabled(block, { ref, ref_protected: true, event_name: 'workflow_dispatch' }, results), false, ref);
    }
    assert.equal(enabled(block, { ref: 'refs/heads/main', ref_protected: true, event_name: 'workflow_dispatch' }, results), true);
    for (const ref of ['refs/heads/main', 'refs/heads/Main']) {
      assert.equal(enabled(block, { ref, ref_protected: false, event_name: 'workflow_dispatch' }, results), false);
    }
  });
}

test('failed/skipped image planning cannot publish or deploy', () => {
  for (const result of ['failure', 'skipped', 'cancelled']) {
    const github = { ref: 'refs/heads/main', ref_protected: true };
    assert.equal(enabled(job('deploy-prod', 'release-images'), github, {
      plan: result, 'publish-images': 'skipped',
    }), false);
    assert.equal(enabled(job('deploy-prod', 'deploy'), github, {
      plan: 'success', 'release-images': result,
    }), false);
  }
  assert.equal(enabled(job('deploy-prod', 'release-images'), { ref: 'refs/heads/main', ref_protected: true }, {
    plan: 'success', 'publish-images': 'skipped',
  }), true, 'a successful plan with no changed images still tags the release');
});

test('production SSH jobs stay on the production runner and environment', () => {
  for (const [workflow, name] of [['deploy-prod', 'deploy'], ['deploy-prod-watch', 'release-drift']]) {
    const block = job(workflow, name);
    assert.match(block, /^    runs-on: \[self-hosted, taskly-prod\]$/m);
    assert.match(block, /^    environment: production$/m);
  }
});

test('release drift checks accept only scheduled or manual main runs', () => {
  const block = job('deploy-prod-watch', 'release-drift');
  assert.equal(enabled(block, { ref: 'refs/heads/Main', ref_protected: false, event_name: 'workflow_dispatch' }), false);
  for (const ref of ['refs/heads/main', 'refs/heads/feature', 'refs/tags/main']) {
    for (const event_name of ['schedule', 'workflow_dispatch', 'workflow_run', 'pull_request']) {
      const expected = ref === 'refs/heads/main' && ['schedule', 'workflow_dispatch'].includes(event_name);
      assert.equal(enabled(block, { ref, ref_protected: true, event_name }), expected);
    }
  }
});

test('failed deployment notifications ignore non-main and successful runs', () => {
  const block = job('deploy-prod-watch', 'failed-deploy');
  for (const head_branch of ['main', 'feature']) {
    for (const conclusion of ['failure', 'success', 'skipped']) {
      assert.equal(enabled(block, {
        event_name: 'workflow_run', event: { workflow_run: { head_branch, conclusion } },
      }), head_branch === 'main' && conclusion === 'failure');
    }
  }
});
