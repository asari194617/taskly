import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { runInNewContext } from 'node:vm';

function job(workflow, name) {
  const source = readFileSync(new URL(`../workflows/${workflow}.yml`, import.meta.url), 'utf8');
  const block = source.match(new RegExp(`^  ${name}:\\n([\\s\\S]*?)(?=^  [a-z][\\w-]*:|$(?![\\s\\S]))`, 'm'));
  assert.ok(block, `${workflow}/${name} exists`);
  return block[1];
}

function enabled(block, github, results = {}) {
  const condition = block.match(/^    if: (.+)$/m)?.[1];
  const expression = (condition === '>-' ? block.match(/^    if: >-\n((?: {6}[^\n]+\n)+)/m)?.[1].trim().replace(/\s+/g, ' ') : condition)
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

test('workflow jobs select only GitHub-hosted runners', () => {
  const directory = new URL('../workflows/', import.meta.url);
  const workflows = readdirSync(directory).filter(name => /\.ya?ml$/.test(name));
  assert.ok(workflows.length > 0);
  for (const name of workflows) {
    const source = readFileSync(new URL(name, directory), 'utf8');
    const jobs = source.split(/^jobs:\s*$/m)[1];
    assert.ok(jobs, name);
    for (const block of jobs.trimEnd().split(/(?=^  [a-z][\w-]*:\s*$)/m).filter(block => block.trim())) {
      assert.match(block, /^    runs-on: (?:ubuntu|windows|macos)-latest$/m, `${name}/${block.trim().split(':')[0]}`);
    }
  }
});

test('hosted image result aggregation runs even if a build fails', () => {
  const block = job('compose-build', 'build');
  assert.match(block, /^    needs: \[plan, build-images, test-batches\]$/m);
  assert.equal(enabled(block, {}, { plan: 'success', 'build-images': 'failure', 'test-batches': 'success' }), true);
  assert.match(block, /test "\$PLAN_RESULT" = success/);
  assert.match(block, /test "\$BATCH_TEST_RESULT" = success/);
  assert.match(block, /if \[\[ "\$IMAGE_COUNT" != 0 \]\]; then\s+test "\$BUILD_RESULT" = success/);
});

for (const [workflow, name, results] of [
  ['deploy-prod', 'plan', {}],
  ['deploy-prod', 'release-images', { plan: 'success', 'publish-images': 'success' }],
  ['deploy-prod', 'deploy', { plan: 'success', 'release-images': 'success' }],
  ['nginx-maintenance', 'maintain', {}],
  ['deploy-api-docs', 'publish', {}],
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

test('production SSH jobs use the production environment and pinned host identity', () => {
  for (const [workflow, name] of [['deploy-prod', 'deploy'], ['deploy-prod-watch', 'release-drift'], ['nginx-maintenance', 'maintain'], ['reconcile-code-only', 'reconcile']]) {
    const block = job(workflow, name);
    assert.match(block, /^    environment: production$/m);
    assert.match(block, /secrets\.PROD_SSH_HOST_KEY/);
    assert.match(block, /prepare-prod-ssh\.sh/);
    assert.doesNotMatch(block, /ssh-keyscan/);
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

test('failed deployment notifications accept only failed main pushes or manual runs in this repository', () => {
  const block = job('deploy-prod-watch', 'failed-deploy');
  const repository = 'owner/taskly';
  const run = { event: 'push', head_branch: 'main', conclusion: 'failure', head_repository: { full_name: repository } };
  for (const [overrides, expected] of [
    [{}, true],
    [{ event: 'workflow_dispatch' }, true],
    [{ event: 'pull_request' }, false],
    [{ head_repository: { full_name: 'contributor/taskly' } }, false],
    [{ head_branch: 'feature' }, false],
    [{ conclusion: 'success' }, false],
  ]) {
    assert.equal(enabled(block, {
      repository, event_name: 'workflow_run', event: { workflow_run: { ...run, ...overrides } },
    }), expected);
  }
});

test('reconciliation is manual protected-main only and selects the owned unfinished release', () => {
  const block = job('reconcile-code-only', 'reconcile');
  for (const event_name of ['push', 'pull_request', 'workflow_run', 'workflow_dispatch']) {
    for (const ref of ['refs/heads/main', 'refs/heads/other', 'refs/tags/main']) {
      assert.equal(enabled(block, { event_name, ref, ref_protected: true }), event_name === 'workflow_dispatch' && ref === 'refs/heads/main');
    }
  }
  assert.equal(enabled(block, { event_name: 'workflow_dispatch', ref: 'refs/heads/main', ref_protected: false }), false);
  assert.doesNotMatch(block, /workflow_dispatch:\s+inputs:|RELEASE_SHA: [0-9a-f]{40}/);
  assert.match(block, /recovery-baseline/);
  assert.match(block, /ref: \$\{\{ env\.RECOVERY_TARGET_SHA \}\}/);
  assert.match(block, /prepare-reconcile-baseline\.py/);
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
    assert.match(source, new RegExp(`--committed-digest "\\$${workflow === 'deploy-prod' ? 'SCHEMA' : 'RECOVERY'}_DATABASE_DIGEST"`));
    assert.match(source, /taskly-completed-baseline.json/);
    assert.ok(source.indexOf('name: Verify committed production release') > source.indexOf(workflow === 'deploy-prod' ? 'name: Mark production deployment complete' : 'name: Verify every remaining invariant and complete the exact release'));
  }
});
