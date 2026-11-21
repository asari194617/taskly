module.exports = async function resolveRequest(github, context, core) {
  const { owner, repo } = context.repo;
  const { comment, issue, repository } = context.payload;
  const command = /^\/?(terraform|atlas) (plan|apply)(?: (\S+))?$/.exec(comment.body);
  const respond = (message) => core.setOutput('message', message);
  if (command?.[0] !== comment.body) {
    return respond('Usage: `terraform plan`, `terraform apply`, `atlas plan` or `atlas apply`.');
  }
  const [, tool, operation, argument] = command;
  const argumentFormat = operation === 'plan' ? /^[0-9a-f]{40}$/ : /^[1-9][0-9]{0,18}$/;
  if (argument && !argumentFormat.test(argument)) {
    return respond(operation === 'plan' ? 'Use a full 40-character commit SHA.' : 'Use a numeric plan run ID.');
  }

  const { data: pr } = await github.rest.pulls.get({ owner, repo, pull_number: issue.number });
  if (pr.state !== 'open' || pr.draft || pr.head.repo?.id !== repository.id ||
      pr.base.repo?.id !== repository.id || pr.base.ref !== repository.default_branch) {
    return respond('Commands require an open, ready PR from this repository targeting the default branch.');
  }
  if (pr.base.sha !== context.sha) {
    return respond('The default branch advanced. Post a new command.');
  }

  const revision = pr.head.sha;
  if (operation === 'plan') {
    if (argument && argument !== revision) {
      return respond('Review the current PR commit and use its full SHA in the plan command.');
    }
    if (!argument) {
      const { repository: source } = await github.graphql(`
        query($owner: String!, $repo: String!, $sha: GitObjectID!) {
          repository(owner: $owner, name: $repo) {
            object(oid: $sha) { ... on Commit { signature { isValid signer { login } } } }
          }
        }
      `, { owner, repo, sha: revision });
      const signature = source?.object?.signature;
      if (!signature?.isValid || signature.signer?.login !== owner) {
        return respond(`Review the current commit, then post \`${tool} plan ${revision}\`. An argument-free plan requires a verified owner signature.`);
      }
    }
  }

  const { data: comparison } = await github.request('GET /repos/{owner}/{repo}/compare/{basehead}', {
    owner, repo, basehead: `${pr.base.sha}...${revision}`, per_page: 1,
  });
  if (comparison.status !== 'ahead') {
    return respond('Update the PR branch with the default branch, review the new commit, then request a plan.');
  }

  const artifactName = `${tool}-pr-${issue.number}-${pr.base.sha}-${revision}`;
  if (operation === 'apply') {
    const endpoint = argument
      ? 'GET /repos/{owner}/{repo}/actions/runs/{run_id}/artifacts'
      : 'GET /repos/{owner}/{repo}/actions/artifacts';
    const params = { owner, repo, name: artifactName, per_page: 100 };
    if (argument) params.run_id = argument;
    const artifacts = await github.paginate(endpoint, params);
    const eligible = artifacts.filter((artifact) => !artifact.expired && artifact.name === artifactName &&
      artifact.created_at < comment.created_at && artifact.workflow_run?.head_sha === pr.base.sha &&
      artifact.workflow_run.repository_id === repository.id);
    eligible.sort((a, b) => b.created_at.localeCompare(a.created_at) || b.id - a.id);
    const selected = eligible[0];
    if (!selected) {
      return respond(`No saved plan exists for this PR commit. Request \`${tool} plan\` and review its result.`);
    }
    const { data: run } = await github.rest.actions.getWorkflowRun({ owner, repo, run_id: selected.workflow_run.id });
    if (run.path !== '.github/workflows/terraform-pr-comments.yml' || run.event !== 'issue_comment' ||
        run.run_attempt !== 1 || run.conclusion !== 'success' || run.head_sha !== pr.base.sha ||
        run.updated_at >= comment.created_at) {
      return respond(`The saved plan did not come from a successful command run. Request a new \`${tool} plan\`.`);
    }
    core.setOutput('plan_run', String(run.id));
    core.setOutput('artifact_id', String(selected.id));
  }

  core.setOutput('tool', tool);
  core.setOutput('command', operation);
  core.setOutput('revision_sha', revision);
  core.setOutput('base', pr.base.sha);
  core.setOutput('artifact', artifactName);
};
