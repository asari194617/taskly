# Picomatch GHSA-fc5q-39xc-vjpf risk acceptance

## Decision

Dependabot alert 384 is accepted temporarily as a non-production-reachable
build-time risk. The decision was recorded on 2026-11-18 and applies only while
all controls below remain true.

The advisory affects every published picomatch version through 4.0.7 and does
not identify a patched release. The proposed upstream fix was closed without
being merged, so upgrading or overriding the package cannot currently remove
the finding with an upstream-supported release.

## Taskly exposure

- `apps/web` is built with Astro's explicit `output: "static"` mode and only
  the generated `dist` directory is deployed to Cloudflare Pages.
- Picomatch is transitive build tooling. Application source does not import it,
  and the generated static artifact does not contain its parser.
- Public request data is never used as a glob pattern. Glob patterns processed
  during the build come from trusted repository source and tool configuration.

The advisory's denial-of-service condition therefore cannot be triggered by a
request to the deployed Taskly application. The remaining exposure is limited
to a developer or CI build evaluating repository-controlled configuration.

## Enforced controls

`npm run test:picomatch-exposure`, run after the production build in web CI,
fails if:

- picomatch becomes a direct application dependency;
- the Astro output is no longer explicitly static;
- application source imports picomatch; or
- picomatch parser signatures appear in the deployed `dist` artifact.

## Reassessment triggers

Remove this acceptance and upgrade or replace the dependency when a supported
patched release becomes available. Reassess immediately if Taskly adds server
rendering, imports picomatch directly, accepts user-controlled glob patterns,
or changes the deployment artifact beyond the static `dist` directory.

References:

- [GHSA-fc5q-39xc-vjpf](https://github.com/advisories/GHSA-fc5q-39xc-vjpf)
- [Upstream remediation proposal](https://github.com/micromatch/picomatch/pull/235)
