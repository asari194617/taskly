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

test('lightweight jobs use hosted runners without waiting for pool selection', () => {
  const placements = {
    lint: ['biome', 'go-vet', 'ruff'],
    mcp: ['build'],
    'email-compose': ['build'],
    'pr-coverage': ['metadata', 'go', 'web', 'mcp', 'comment'],
    'pr-coverage-fork-comment': ['comment'],
    'weekly-discord': ['report'],
    'deploy-prod-watch': ['failed-deploy'],
    'compose-build': ['test-batches', 'plan', 'build'],
  };
  for (const [workflow, names] of Object.entries(placements)) {
    for (const name of names) {
      const block = job(workflow, name);
      assert.match(block, /^    runs-on: ubuntu-latest$/m, `${workflow}/${name}`);
      assert.doesNotMatch(block, /needs.*select-runner|CI_PRIMARY_ONLY|CI_RUNNER/, `${workflow}/${name}`);
    }
  }
});

test('heavy jobs retain pool selection and production planning forwards its selection', () => {
  for (const [workflow, names] of [
    ['api', ['build', 'empty-database']],
    ['web', ['build-and-test']],
    ['e2e', ['web-api-db']],
    ['compose-build', ['build-images']],
  ]) {
    for (const name of names) {
      assert.match(job(workflow, name), /^    runs-on: \$\{\{ fromJSON\(needs\.select-runner\.outputs\.label\) \}\}$/m);
    }
  }
  const plan = job('deploy-prod', 'plan');
  assert.match(plan, /^    runs-on: ubuntu-latest$/m);
  // Preserve the protected-main gate and runner override for image builds.
  assert.match(plan, /^    needs: select-runner$/m);
  assert.match(plan, /^      runner: \$\{\{ needs\.select-runner\.outputs\.label \}\}$/m);
  for (const name of ['publish-images', 'release-images']) {
    assert.match(job('deploy-prod', name), /^    runs-on: \$\{\{ fromJSON\(needs\.plan\.outputs\.runner\) \}\}$/m);
  }
});

test('hosted image result aggregation runs even if a build or selection fails', () => {
  const block = job('compose-build', 'build');
  assert.match(block, /^    needs: \[plan, build-images, test-batches\]$/m);
  assert.equal(enabled(block, {}, { plan: 'success', 'build-images': 'failure', 'test-batches': 'success' }), true);
  assert.match(block, /test "\$PLAN_RESULT" = success/);
  assert.match(block, /test "\$BATCH_TEST_RESULT" = success/);
  assert.match(block, /if \[\[ "\$IMAGE_COUNT" != 0 \]\]; then\s+test "\$BUILD_RESULT" = success/);
});

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
  for (const [workflow, name] of [['deploy-prod', 'deploy'], ['deploy-prod-watch', 'release-drift'], ['reconcile-code-only', 'reconcile']]) {
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

test('one-time reconciliation is manual protected-main only and binds its release', () => {
  const block = job('reconcile-code-only', 'reconcile');
  for (const event_name of ['push', 'pull_request', 'workflow_run', 'workflow_dispatch']) {
    for (const ref of ['refs/heads/main', 'refs/heads/other', 'refs/tags/main']) {
      assert.equal(enabled(block, { event_name, ref, ref_protected: true }), event_name === 'workflow_dispatch' && ref === 'refs/heads/main');
    }
  }
  assert.equal(enabled(block, { event_name: 'workflow_dispatch', ref: 'refs/heads/main', ref_protected: false }), false);
  assert.match(block, /RELEASE_SHA: 85222d44a5bf082d36d9dd781889eda73f0ff013/);
  assert.doesNotMatch(block, /PROD_ENV|rsync|docker compose up|apply-schema|install-user-cron|secrets.GHCR/);
  for (const workflow of ['deploy-prod', 'reconcile-code-only']) {
    const source = readFileSync(new URL(`../workflows/${workflow}.yml`, import.meta.url), 'utf8');
    assert.match(source, /group: deploy-production/);
    assert.match(source, /cancel-in-progress: false/);
    assert.match(source, /timeout-minutes:/);
    assert.match(source, /prepare-prod-ssh.sh/);
  }
});

test('production source cannot share stdin with children or skip its committed-state postcondition', () => {
  for (const workflow of ['deploy-prod', 'reconcile-code-only']) {
    const source = readFileSync(new URL(`../workflows/${workflow}.yml`, import.meta.url), 'utf8');
    assert.doesNotMatch(source, /bash -s/);
    assert.match(source, /run-prod-script.sh/);
    assert.match(source, /name: Verify committed production release/);
    assert.match(source, /--committed-digest "\$SCHEMA_DATABASE_DIGEST"/);
    assert.match(source, /taskly-completed-baseline.json/);
    assert.ok(source.indexOf('name: Verify committed production release') > source.indexOf(workflow === 'deploy-prod' ? 'name: Mark production deployment complete' : 'name: Verify every remaining invariant and complete the exact release'));
  }
});
