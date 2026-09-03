# AGENTS.md

This file is the repository-wide operating contract for coding agents. Keep it focused on decisions an agent must make repeatedly. Put product/user documentation in the relevant README or docs, and keep tool/runtime versions in their canonical config files instead of duplicating them here.

## Communication and Safety

- Use Japanese as the primary language for user-facing communication and pull request discussion unless asked otherwise.
- Before changing external content or production state through MCP/API/deploy tools, obtain explicit user authorization for that external mutation.
- Do not expose, log, copy, or move credentials/secrets into client-readable locations.
- Preserve important `NOTE` comments. Temporary review-only explanations may use `[NOTE FOR REVIEW]` and should not become permanent clutter.

## Start Every Task

1. Read `README.md`, this file, and only the task-relevant package/docs files.
2. Start normal development work from the latest `dev` on a **fresh branch for that PR**. Never reuse the head branch of a merged or closed PR for new work.
3. An intentionally stacked PR is allowed when the dependency is real. In that case, base it on the immediate predecessor and document the stack/order in every affected PR. Do not accidentally carry unrelated history forward.
4. Before substantial implementation, establish a task contract in working notes:
   - **Goal**: observable outcome.
   - **Invariants**: behavior/data/security/UX that must stay true.
   - **Non-goals**: nearby work intentionally excluded.
   - **Acceptance criteria**: externally observable completion conditions.
   - **Verification**: deterministic checks proving those conditions.
5. Resolve ambiguity from current code, tests, docs, and existing contracts first. Ask for user judgment only when materially different product semantics remain possible.

## Canonical Sources

Do not hard-code volatile versions or duplicate package metadata in this file.

- Node.js version: root `.node-version` / `.nvmrc` and each package's `engines`.
- pnpm version: each package's `packageManager`.
- Go version/dependencies: `system/go.mod` and tool-specific `go.mod` files.
- CI behavior and path routing: `.github/workflows/` and `.github/scripts/detect-ci-paths.sh`.
- Firestore rules/configuration: `firebase/`.
- User-facing commands and behavior: implementation/tests plus the maintained docs site.

Major areas:

- `system/`: Go backend, repository/application logic, batch/Lambda entrypoints.
- `youtube-monitor/`: Next.js livestream UI.
- `firebase/`: Firestore rules and emulator configuration.
- `aws-cdk/`: AWS infrastructure.
- `docs-site/`: Docusaurus documentation.
- `tools/`: standalone operational/development tools.

Prefer the closest package README/config and the closest nested `AGENTS.md` if one is added later.

## Quality Ownership

Generation may be probabilistic; acceptance must be deterministic whenever practical.

When implementation or review reveals a repeatable failure mode, fix the instance and then choose the cheapest reliable prevention layer:

1. regression/integration/contract test for observable behavior;
2. security rule, schema, or data constraint for access/data invariants;
3. formatter/linter/static analysis for mechanical rules;
4. CI/path-routing check for workflow invariants;
5. `AGENTS.md` for agent decisions that cannot be encoded reliably elsewhere;
6. architecture boundary when the invalid state should be impossible by construction.

Do not rely on self-review alone for a repeatable problem that a machine can reject. Avoid tests coupled to implementation trivia; prefer one named externally observable contract per test and semantic assertions over weak existence/truthiness checks.

## Validation by Area

Run the smallest set that fully covers the changed contract, then let GitHub Actions remain the final repository gate.

### Go backend (`system/`)

```sh
cd system
go test -shuffle=on ./...
golangci-lint run --timeout=5m --config=.golangci.yml
I18N_BASELINE=ja go generate ./...
```

After generation, verify there is no unintended generated diff. For Firestore repository/application behavior that depends on Emulator semantics, run the repository's emulator script when applicable **from the repository root**:

```sh
bash .github/scripts/run-firestore-integration-tests.sh
```

For transactional behavior, verify both commit and rollback/atomicity where failure can leave partial state. Preserve error identity with wrapping that still supports `errors.Is` when callers depend on it.

### YouTube monitor (`youtube-monitor/`)

```sh
cd youtube-monitor
pnpm check
pnpm test --runInBand
pnpm build
```

Keep horizontal/vertical or other variants sharing domain/data behavior unless the requirement is intentionally variant-specific. When shared behavior changes, cover the relevant variants rather than assuming one rendering path represents all of them.

### AWS CDK (`aws-cdk/`)

```sh
cd aws-cdk
pnpm test
pnpm cdk:synth
```

Do not deploy infrastructure without explicit authorization.

### Documentation (`docs-site/`)

Use the package scripts defined in `docs-site/package.json` for lint/typecheck/build. Be careful: some format/check scripts write files; inspect the script before running a command whose name is ambiguous.

### CI and path routing

When changing `.github/workflows/**`, `.github/actions/**`, or CI path detection, test the routing contract as well as syntax:

```sh
bash .github/scripts/test-detect-ci-paths.sh
```

A new file/path that should trigger existing validation must be classified by CI. Do not let a new area silently bypass its deterministic checks.

## Implementation Rules That Matter Across the Repository

- Include useful context when returning external/dependency errors; preserve unwrap/error identity when required.
- Use Firestore transactions where a business invariant spans multiple reads/writes or collections.
- Keep server-only configuration and credentials behind server authorization boundaries. A public client contract should expose only the minimum explicit fields it needs.
- When changing i18n source/locales/generation, regenerate typed artifacts and verify the generated diff.
- Avoid unrelated refactors in a behavioral/security fix unless they are required to make the invariant enforceable.

## Pull Requests

- Normal development PRs target `dev`, not `main`, unless the user explicitly requests another base.
- When using `gh pr create` for a normal development PR, pass `--base dev` explicitly rather than relying on the repository default.
- Keep commits and PRs coherent. Split independent concerns rather than creating a review mega-bundle.
- PR descriptions should state the goal, key invariants/non-goals, migration/deploy ordering when relevant, and the checks actually run.
- Never claim a check ran locally when it only ran in GitHub Actions.
- For stacked PRs, show the stack and base relationship clearly. Before merging/rebasing, re-check that each PR contains only its intended delta.
- If review uncovers a repeatable defect, include the prevention-layer decision (test/lint/CI/rules/architecture/instruction) as part of completing the fix.
