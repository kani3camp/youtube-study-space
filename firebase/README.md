# Firebase / Firestore

This directory contains the repository-managed Firestore Security Rules, indexes, and emulator configuration. Treat these files as the existing project definition. Do **not** run `firebase init firestore` as a routine setup step because it can overwrite or regenerate repository-managed configuration.

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

This layer validates Firestore serialization, transactions, and repository/application behavior against Emulator semantics. It does **not** by itself prove what an anonymous/browser Client SDK caller is allowed to access.

### 3. Client SDK / Security Rules behavior

When `firestore.rules` changes, reason explicitly about browser/client access separately from server-side Go behavior. Verify the intended allow/deny cases for every changed public boundary, especially `config/*` and any collection consumed by `youtube-monitor`.

A Go test using server credentials can bypass the question that Security Rules answer. Do not treat a passing Go Emulator integration suite as sufficient evidence for a client-access policy change.

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
