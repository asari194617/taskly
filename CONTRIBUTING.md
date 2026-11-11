# Contributing

Taskly is not currently accepting external code contributions. Pull request
creation is restricted to collaborators in the repository settings. Maintainers
continue to use branches, pull requests, review, and the required checks before
merging.

Public issues may be used for reproducible bug reports and narrowly scoped feature requests,
but maintainers may close requests that contain private data, security details,
or material outside the project's scope.

Do not submit suspected vulnerabilities through a public issue or pull request.
Follow [`SECURITY.md`](SECURITY.md) instead.

Opening an issue or pull request does not grant a license to use Taskly source
code and does not change the repository-wide rights notice in
[`LICENSE`](LICENSE). A contribution policy and contributor license terms will
be published before external code contributions are accepted.

## Local lint checks

Run these from the repository root:

```sh
npm ci
npm run lint
npm run lint:go
python3 -m venv /tmp/taskly-lint-venv
/tmp/taskly-lint-venv/bin/pip install -r requirements-lint.txt
/tmp/taskly-lint-venv/bin/ruff check .
```

The `lint` workflow runs all three checks on every pull request and on main.
Biome is pinned in the root lockfile and checks supported JavaScript, TypeScript,
JSX, JSON, CSS and HTML files across the web app, MCP, email service, email Worker,
and repository scripts. Generated protobuf and build output are excluded.
`npm run lint:fix` applies safe Biome fixes; formatting is not enforced.
Astro-specific checks remain covered by `npm run check` in `apps/web`.

The initial Biome policy enables recommended correctness rules and selected
suspicious-code rules. Unused imports/variables/parameters and React hook rules
are deferred until existing code is cleaned up; style and accessibility policies
are not enabled yet. See `biome.json` for the exact enforced rules.
Ruff initially checks syntax errors and undefined names (`E9`, `F63`, `F7`, `F82`)
in Python, including Airflow, monitoring and deployment scripts.
Go vet runs separately in every tracked Go module, including nested notification
modules that a parent module's `./...` pattern would otherwise skip. It requires
Go 1.26.10 or newer and access to module dependencies.
