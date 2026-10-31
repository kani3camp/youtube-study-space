# Developer Architecture Notes

This document records architecture details that are easy to miss when changing one component in isolation. Runtime code and tests remain authoritative; update this document when the responsibility boundary changes.

## Room state and max-seat control

The livestream monitor is currently both a renderer and one participant in the max-seat control loop.

```mermaid
flowchart TD
    Rooms[Monitor room definitions<br/>basic + temporary rooms]
    Seats[(Firestore seats / member-seats)]
    Constants[(Firestore config/constants<br/>max_seats / desired_* / mode)]
    Monitor[YouTube Monitor<br/>MainContent.tsx]
    API[set_desired_max_seats API]
    Desired[(Firestore desired_max_seats<br/>desired_member_max_seats)]
    Bot[Go Bot<br/>AdjustMaxSeats]
    Current[(Firestore max_seats<br/>member_max_seats)]

    Rooms --> Monitor
    Seats --> Monitor
    Constants --> Monitor
    Monitor -->|calculate desired counts| API
    API --> Desired
    Desired --> Bot
    Constants --> Bot
    Bot -->|increase / shrink / move / exit| Seats
    Bot --> Current
    Current --> Monitor
```

### Values with different roles

| Value | Role | Primary source/consumer |
| --- | --- | --- |
| `max_seats` / `member_max_seats` | Current seat-count limit used by the system. | Firestore `config/constants`; backend reconciliation and monitor layout construction |
| `desired_max_seats` / `desired_member_max_seats` | Requested target. The monitor writes these through the API; the Bot reconciles current limits toward them. | `set_desired_max_seats` Lambda, `WorkspaceApp.AdjustMaxSeats` |
| Basic room seat count | Capacity explicitly represented by the normal room-layout definitions. In fixed mode this becomes the requested capacity. | `youtube-monitor/src/rooms/rooms-config.ts` |
| Temporary rooms | Extra layouts used to cover variable capacity when more seats are required. | `youtube-monitor/src/rooms/rooms-config.ts`, layout builders |

### Fixed-seat mode

When `fixed_max_seats_enabled` is true, the monitor uses the total seats in the **basic room definitions** as the desired general capacity, and the corresponding fixed-member logic for member seats. Temporary rooms are not the mechanism for expanding fixed capacity.

On the backend, shrinking a fixed capacity is operationally significant: `max_seats_adjustment.go` can make users whose seat IDs are above the new limit exit. Therefore changing the number of seats in a basic layout can change system behavior even if the diff appears to be visual-only.

### Variable-seat mode

When fixed mode is disabled, the monitor calculates desired capacity from current occupancy, `min_vacancy_rate`, the basic-room capacity, and temporary-room definitions. Temporary rooms allow the UI to cover capacity beyond the basic rooms while the desired/current values converge.

The backend rejects a shrink that would violate the configured vacancy constraint. When shrinking is valid, users occupying seats above the new limit may be moved before `max_seats` is updated.

### Change checklist for monitor layouts

Before treating a layout change as visual-only, answer all of these:

1. Does the change alter the number of seats in any basic room?
2. Does it alter temporary-room capacity or ordering?
3. What desired counts are produced in fixed and variable modes?
4. Can any occupied seat ID become greater than the requested/current maximum?
5. Are general and member room calculations both covered?

See [`../../youtube-monitor/README.md`](../../youtube-monitor/README.md) for build/test setup.

## Firestore access boundaries

Server-side Go repository tests and browser Client SDK security rules answer different questions:

- normal `go test ./...` validates code without the integration build tag;
- `bash .github/scripts/run-firestore-integration-tests.sh` validates repository/application behavior against Firestore Emulator semantics;
- Firestore Security Rules govern what browser/client SDK callers can read/write and require client-oriented rule validation when that boundary changes.

See [`../../firebase/README.md`](../../firebase/README.md).

## Formal specifications

Seat behavior has formal-spec documentation under [`../../formal-spec/`](../../formal-spec/). The FSL documentation distinguishes contracts that are linked to Go verification from specifications that are currently descriptive/unlinked. Do not weaken a specification to make CI pass, and do not assume an unlinked specification is already enforced by the implementation.

## Privacy and retention

Changes to storage, forwarding, archival, or deletion must also be checked against [`../privacy/`](../privacy/). In particular, historical descriptions of raw live-chat archival must not be reintroduced as current architecture after that path has been retired.

## Current architecture vs desired direction

**Current fact:** the monitor participates in max-seat control as described above.

**Design direction:** prefer moving control decisions out of the presentation client so the monitor can eventually become a read/render-only component. This is a target boundary, not a statement that migration is complete. Until code changes remove the request path, reviews and tests must assume the current control loop exists.

## CI validation boundaries

[CI](../../.github/workflows/ci.yml) is the pull-request orchestrator: it detects changed paths, calls same-tree reusable validation workflows, and publishes the required checks. The validation commands remain in independent jobs so their parallelism and per-job timeouts are preserved.

| Reusable workflow | Responsibility |
| --- | --- |
| [ci-system.yml](../../.github/workflows/ci-system.yml) | Go lint/tests, generated files, Firestore Emulator |
| [ci-frontend.yml](../../.github/workflows/ci-frontend.yml) | YouTube Monitor and Docs Site checks/builds |
| [ci-tools.yml](../../.github/workflows/ci-tools.yml) | Room Image Prompt, Menu Image Generator, Video Maker Simulator, Figma plugin, Formal Spec |
| [ci-infra.yml](../../.github/workflows/ci-infra.yml) | AWS CDK tests and synth; no deployment |
| [ci-repository.yml](../../.github/workflows/ci-repository.yml) | Base-image report tests, Node LTS expiry, documentation links, CI workflow contracts/actionlint |

The modules accept typed selection flags (plus the pinned source SHA for repository checks), use read-only repository permissions, and receive no operational secrets or Environments. [Path detection](../../.github/scripts/detect-ci-paths.sh) selects the same validation domains as before; changes to workflows, shared CI actions, or CI selection/result contracts select the full suite. Unchanged domains can skip. Repository checks always run after change detection and source selection, including during authenticated Terraform dispatches.

`CI Gate` and `GCP Terraform Validate` remain uniquely named direct jobs in `ci.yml`, preserving the existing Required Status Check contexts. Other validation check names acquire their caller prefix (for example, `System / Go Lint`); they are covered by `CI Gate`. The [gate result policy](../../.github/scripts/check-ci-results.py) rejects failed/cancelled reusable calls, missing or malformed detection outputs, unsuccessful detection/repository checks, and skipped selected modules. An unrequested schema audit may skip; a requested audit must succeed. Authenticated Terraform retains its existing separate operational result and is not added as a quality-gate dependency.

Manual CI uses one `execution_mode` choice: `全検査のみ`, `検査＋plan` (default), or `検査＋apply`. Validation-only runs select all quality groups without Terraform credentials. Plan/Apply runs select the GCP Terraform and repository checks, then call authenticated Terraform only after `CI Gate` succeeds. The choice maps to the reusable workflow's existing `normal-plan` / `normal-apply` modes. Plan/Apply runs retain `cancel-in-progress: false`; PRs cannot enter this operational path.

For Plan/Apply, the controller's [source resolver](../../.github/scripts/ci-terraform-source.py) uses the existing owner/ref/source checks to pin `changes.outputs.validation_sha` before validation. GCP Terraform and repository checks checkout this exact commit, including its tests, with read-only permissions, no Environment secrets or OIDC, and no persisted checkout credentials. The authenticated reusable receives that SHA as `expected_sha` with an empty `source_branch`, so it cannot resolve a newer branch HEAD after validation. It rechecks DEV commit reachability, then uses this same SHA for both Plan and Apply. Source resolution failure or a missing/malformed validation SHA fails the gate. The resolver, workflow definitions, CI Gate and credentialed executor remain on the controller commit; PROD still requires the controller's exact `main` SHA. PRs and validation-only runs continue to validate `github.sha`.

Authenticated Terraform calls, dispatch inputs, cancellation rules, explicit secrets and OIDC permissions stay in `ci.yml`. Its workflow path is part of the cloud trust contract. Moving the caller requires a separate change that verifies actual AWS/GCP trust and its rollout; this change requires no Ruleset, IAM, or cloud changes. Existing CodeQL, GCP Production Readiness and GCP Runtime Ownership Contracts workflows retain their separate responsibilities.

Local source verification (with the pinned parser from `.github/codeql/requirements.txt` installed):

```sh
python3 -m unittest discover -s .github/scripts -p 'test_ci_*.py' -v
bash .github/scripts/test-detect-ci-paths.sh
```

CI additionally runs pinned actionlint against the orchestrator, all five modules, and the two operational reusable workflows. Selection contracts exhaust all 8,192 combinations of detection flags; result contracts exercise failures, cancellations, skipped selected modules, and incomplete detection. The independent CodeQL source-contract workflow continues to validate its own source and actionlint scope.
