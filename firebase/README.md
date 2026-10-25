# Firebase / Firestore

This directory contains the repository-managed Firestore Security Rules, indexes, and emulator configuration. Treat these files as the existing project definition. Do **not** run `firebase init firestore` as a routine setup step because it can overwrite or regenerate repository-managed configuration.

## MyPage Phase 0B prerequisite: authenticated OBS monitor (2026-10-25 decision, NOT deployed)

The current `firestore.rules` in this repository still allows anonymous reads of `seats`, `member-seats`, `menu`, `work-name-trend`, and `config/constants`. This is a **known temporary exposure**: private OBS browser source deployment is not a Firestore authentication boundary.

Before the first externally usable MyPage Phase 0B release, [Issue #1459](https://github.com/kani3camp/youtube-study-space/issues/1459) requires:
- Authenticate a dedicated monitor-only Firebase Auth principal in the OBS browser source. The first candidate is one interactive sign-in with Firebase local persistence; persistence after actual OBS/CEF restarts has **not** been proven.
- Require a dedicated, validated UID and monitor-only claim (or equally strict explicit authorization) for the five Client SDK collection/doc reads. Anonymous, normal MyPage users, and all browser writes must be rejected.
- Use Client SDK / Security Rules tests for anonymous, monitor, wrong UID/project, expired sessions, read/list/write; validate the same rules against the selected DEV and PROD projects, including restart, renewal, and operator notification paths.
- Separate Client SDK enforcement from server-side Go/IAM, which bypasses Security Rules. Do not add service-account credentials to browser bundles or accept an API-key/project split as authorization.
- Stage credential/login support **before** enforcing deny-only rules in production; verify rollback without restoring unrestricted anonymous access.

This section describes an **adopted pre-release requirement**, not a completed code change or live rule deployment. Keep existing Go Bot and monitor seat-count-control behavior unchanged unless separately approved. Related [MyPage release gate #1133](https://github.com/kani3camp/youtube-study-space/issues/1133) and [Notion Security Canon](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634).

## Test layers

These checks cover different contracts and are not interchangeable.

### 1. Normal Go tests

```sh
cd system
go test -shuffle=on -v ./...
```

Tests with the `integration` build tag are not included here.

### 2. Firestore Emulator integration tests

From the repository root:

```sh
npm install --global firebase-tools@15.25.1
bash .github/scripts/run-firestore-integration-tests.sh
```

Requirements are Node.js, Java 21 or later, and the pinned Firebase CLI version used by CI. The script starts Firestore Emulator and runs the Go integration suite. It does not require a real Firebase project or production credentials.

The script installs the isolated dependencies in this directory, runs `firestore.rules.test.mjs` with `@firebase/rules-unit-testing`, and then runs the Go integration suite. The Go layer validates Firestore serialization, transactions, and repository/application behavior against Emulator semantics. The Rules test separately exercises browser Client SDK allow/deny behavior.

### 3. Client SDK / Security Rules behavior

When `firestore.rules` changes, reason explicitly about browser/client access separately from server-side Go behavior. Verify the intended allow/deny cases for every changed public boundary, especially `config/*` and any collection consumed by `youtube-monitor`.

A Go test using server credentials can bypass the question that Security Rules answer. Do not treat a passing Go Emulator integration suite as sufficient evidence for a client-access policy change.

The monitor read boundary requires both `monitor_reader: true` in the Firebase ID token and an enabled `_monitor-readers/{uid}` document. Client rules never expose or mutate that allowlist. Provision claims and allowlist documents only through trusted Admin/server operations, and see [`../youtube-monitor/README.md`](../youtube-monitor/README.md) for the staged rollout and fail-closed rollback procedure.

## Existing project: inspect before deploying

Authenticate only when you need to inspect or change a real Firebase project:

```sh
firebase login
firebase projects:list
firebase use <project-id>
```

Before any deploy, confirm the selected project explicitly:

```sh
firebase use
```

Review repository-managed inputs before applying them:

```sh
cat firebase/firestore.rules
cat firebase/firestore.indexes.json
cat firebase/firebase.json
```

Firebase CLI does not provide a CDK-style universal `diff` for Firestore Rules/indexes. Use local/source review plus the emulator checks above, and inspect the target project in Firebase/GCP when real-environment verification is required.

## Targeted deployment

A deployment changes external project state and requires authorization for the target environment. Prefer the narrowest target instead of generic `firebase deploy`:

```sh
cd firebase
firebase deploy --only firestore:rules --project <project-id>
firebase deploy --only firestore:indexes --project <project-id>
```

Deploy both only when both are intentionally changed:

```sh
firebase deploy --only firestore --project <project-id>
```

After deployment, verify the intended client-visible behavior against the selected environment. Never deploy merely to discover whether a rules change is syntactically valid; use local/emulator validation first.

## New standalone Firebase project bootstrap

Only use initialization commands when intentionally creating a separate Firebase project from scratch. They are not part of normal development for this repository. If bootstrap documentation becomes necessary, keep it separate from the existing-environment workflow above so agents do not overwrite checked-in configuration accidentally.
