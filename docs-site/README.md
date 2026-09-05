# Documentation Site

The public documentation site is built with Docusaurus. Runtime/tool versions and scripts are defined in [`package.json`](./package.json); use that file as the canonical source instead of duplicating version numbers here.

## Setup

```sh
cd docs-site
pnpm install --frozen-lockfile
```

## Local development

```sh
pnpm start
```

## Verification

```sh
pnpm lint
pnpm typecheck
pnpm build
```

Inspect [`package.json`](./package.json) before assuming whether a command modifies files. The current `lint` and `check` scripts are non-writing; `format` and `check:fix` include `--write` and modify the working tree.

## Normal publication path

The normal publication path is GitHub Actions, defined in [`../.github/workflows/deploy-docs.yml`](../.github/workflows/deploy-docs.yml).

- pushes that change `docs-site/**` on `main` or `release` trigger the deploy workflow;
- `workflow_dispatch` can run the same workflow manually;
- the workflow builds the site and publishes `docs-site/build` to GitHub Pages.

A documentation PR should normally validate the build and then rely on the repository's branch/release workflow. Do not run a manual production publish just to preview a PR.

## Manual `pnpm deploy`

Docusaurus also exposes a package-level manual deploy command, but treat it as an exceptional/operator path rather than the standard release flow. It pushes generated content to the pages branch and therefore mutates external repository state.

```sh
USE_SSH=true pnpm deploy
# or
GIT_USER=<github-user> pnpm deploy
```

Use it only when the task explicitly calls for that manual path and the target/repository state has been confirmed.
