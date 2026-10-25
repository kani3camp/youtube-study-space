# YouTube Monitor

`youtube-monitor/` is the Next.js UI rendered in the YouTube livestream. It reads room state from Firestore and renders horizontal/vertical layouts.

> [!IMPORTANT]
> The monitor is **not display-only in the current architecture**. `src/components/MainContent.tsx` also calculates desired general/member seat counts from room definitions and current occupancy, then can call `POST /set_desired_max_seats`. Before changing room definitions or seat counts, read [`../docs/development/architecture.md`](../docs/development/architecture.md).

## Planned Firebase Auth requirement for OBS (Phase 0B prerequisite; not implemented)

The current monitor uses browser `onSnapshot` listeners to read `seats`, `member-seats`, `menu`, `work-name-trend`, and `config/constants` without signing in. Current Firestore Security Rules explicitly allow anonymous reads. The OBS browser source being unpublished does not hide the Firestore data API.

[Issue #1459](https://github.com/kani3camp/youtube-study-space/issues/1459) requires replacing this with **a dedicated operator-controlled Firebase Auth user**, allowing only that UID and monitor-only claim to read the necessary documents. General MyPage Firebase users must not receive this privilege; no Client SDK writes should be enabled. The existing `/set_desired_max_seats` HTTP control path is independent and must continue to pass its existing checks.

First-choice operator flow: open the OBS browser source's **Interact** view, sign in once using a non-Google embedded-browser-compatible Firebase method, and use Firebase `browserLocalPersistence` for subsequent sessions. A normal Chrome login is not presumed to be shared with OBS/CEF. Verify CEF IndexedDB/persistence, OBS/PC restart, scene switch, token renewal, logout/revocation, and failure notification on the actual OBS version before considering this complete. Do not store service-account keys or administrator tokens in `NEXT_PUBLIC_*`, URLs, or OBS browser parameters.

Deployment ordering: create the monitor identity and implement authenticated subscriptions first; test rules in Emulator and DEV; then enforce new Rules in a coordinated DEV/PROD rollout, verify production live subscriptions, and keep a safe rollback path. **Do not deploy deny-only Rules before the monitor can authenticate.** Phase 0A owner-only MyPage testing may continue; [Phase 0B public availability is blocked](https://github.com/kani3camp/youtube-study-space/issues/1133) until this is verified.

## Setup

Use the runtime versions declared by [`package.json`](./package.json); do not copy version numbers into other docs.

```sh
cd youtube-monitor
pnpm install --frozen-lockfile
```

## Runtime assets

The livestream depends on runtime media under `public/` that is intentionally not fully tracked in this repository. Before an operational build or start, make sure the required images, chimes, and BGM files have been provisioned locally.

```sh
pnpm verify:assets
```

The validator checks that image/chime paths referenced by production source files exist as non-empty files under `public/`, and that `public/audio/` contains at least one non-empty `.mp3` file. Both operational commands run this validation automatically:

```sh
pnpm build
pnpm start
```

`pnpm build:ci` intentionally skips runtime asset validation. It is reserved for CI and other source-only compilation checks where the private/local runtime media is not provisioned.

## Environment variables

All variables below use the `NEXT_PUBLIC_` prefix and are bundled into browser-visible code. **Do not put server-only credentials or secrets in them.** Environment-specific real values should still stay out of issues, logs, and documentation examples.

| Variable | Purpose | Public/client-visible |
| --- | --- | --- |
| `NEXT_PUBLIC_DEBUG` | Enables monitor debug behavior. Must be `true` or `false`. | Yes |
| `NEXT_PUBLIC_CHANNEL_GL` | Selects channel-specific behavior. Must be `true` or `false`. | Yes |
| `NEXT_PUBLIC_ROOM_CONFIG` | Selects the room/layout configuration. Must be `PROD` or `DEV`. | Yes |
| `NEXT_PUBLIC_API_ENDPOINT` | Base URL for monitor API calls such as `/set_desired_max_seats`. | Yes |
| `NEXT_PUBLIC_API_KEY` | Client-side API request configuration used by the fetcher. It must not be treated as a server secret because it is shipped to the browser. | Yes |
| `NEXT_PUBLIC_FIREBASE_PROJECT_ID` | Firebase/Firestore project identifier used by the client. | Yes |
| `NEXT_PUBLIC_FIREBASE_API_KEY` | Firebase web API key used by the client SDK. It is browser-visible configuration, not a server credential. | Yes |
| `NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN` | Firebase Auth domain for the monitor's Firebase project. | Yes |

The validation/consumers are in `src/lib/constants.ts`, `src/lib/api-config.ts`, `src/lib/fetcher.ts`, and `src/lib/firestore.ts`.

## Verification without real external connections

Lint/check and tests do not require a real Firebase/API project. Unit and component tests use Vitest with jsdom, and runtime asset validation has deterministic Node.js tests.

```sh
pnpm check
pnpm test
```

For watch mode during local development:

```sh
pnpm test:watch
```

For a source-only production compilation check, use the same non-secret dummy configuration as CI:

```sh
NEXT_PUBLIC_DEBUG=false \
NEXT_PUBLIC_CHANNEL_GL=false \
NEXT_PUBLIC_ROOM_CONFIG=DEV \
NEXT_PUBLIC_API_ENDPOINT=http://localhost:3000 \
NEXT_PUBLIC_API_KEY=ci-dummy-api-key \
NEXT_PUBLIC_FIREBASE_PROJECT_ID=ci-dummy-project \
NEXT_PUBLIC_FIREBASE_API_KEY=ci-dummy-firebase-api-key \
NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN=ci-dummy-project.firebaseapp.com \
pnpm build:ci
```

These values are **build-only placeholders**. They do not prove connectivity or behavior against a deployed environment, and `build:ci` does not prove that runtime media has been provisioned.

## Monitor operator authentication

The monitor does not subscribe to Firestore until Firebase Auth restores a local session whose ID token contains the dedicated `monitor_reader: true` custom claim. Firestore Security Rules also require an enabled `_monitor-readers/{uid}` allowlist document. Requiring both boundaries prevents an ordinary MyPage Firebase user or a claim-only account from reading monitor collections.

Provision each environment through trusted Firebase/Admin operations:

1. Enable the Firebase Auth Email/Password provider and create a dedicated monitor operator account. Do not add sign-up to the monitor.
2. Set the account's custom claim to `monitor_reader: true` with a trusted Admin SDK operation. Never place Admin credentials or a custom-token signer in this client.
3. Create `_monitor-readers/{uid}` with `{ "enabled": true }` through the Firebase console or a trusted server/Admin SDK. The UID is the document ID. Security Rules deny all browser reads and writes to this collection.
4. Hide the OBS browser source from the live composition, open `/operator-login` through OBS **Interact**, and sign in once. The page uses `browserLocalPersistence`; credentials themselves are not stored by this application.
5. Return to `/` and confirm the monitor reads all required collections. If the session, claim, or allowlist is unavailable, or Firestore returns an unrecoverable read error, the monitor unmounts its data consumers and displays a fail-closed operator status instead of continuing with unauthorized data access.

Do not use `/operator-login` as the livestream browser-source URL, and never expose that credential form in a live scene. Removing the claim, deleting the allowlist document, or changing `enabled` to `false` revokes future Rules-authorized reads after the current ID token is refreshed.

### Rollout and rollback

For each environment, prepare the operator account, custom claim, and allowlist first. Deploy the auth-capable monitor and verify its authenticated subscriptions while the previous Rules are still active; then deploy the restrictive Rules and repeat the read and max-seat-control checks. Record DEV evidence before repeating the process in Production.

Do not restore anonymous Firestore Rules as a rollback. If the monitor fails after lockdown, keep Rules closed, take the source out of the live composition, and roll forward with an auth-capable monitor fix. Rules and monitor deployments are separate authorized operations; source tests alone do not prove OBS/CEF persistence, network-disconnect notification, real App Check behavior, or real-environment access.

## Real-environment verification

Use real environment values only when the task explicitly requires an authorized environment smoke test. Confirm the target environment before opening the monitor because it subscribes to Firestore and can send desired-seat-count requests.

When room/seat layout behavior changes, verify at least:

- horizontal and vertical variants that share the changed logic;
- basic-room seat counts and page boundaries;
- fixed-seat mode and variable-seat mode behavior;
- temporary-room coverage in variable mode;
- that the desired seat counts sent by the monitor are expected for both general/member rooms.

## Seat-count control sources

- Room definitions and basic/temporary room composition: `src/rooms/rooms-config.ts`
- Desired-seat calculation and API request: `src/components/MainContent.tsx`
- API path: `src/lib/api-config.ts`
- Backend endpoint: `../system/cmd/lambda/set_desired_max_seats/main.go`
- Backend reconciliation: `../system/core/workspaceapp/max_seats_adjustment.go`

The current behavior and the desired future responsibility boundary are documented in [`../docs/development/architecture.md`](../docs/development/architecture.md).
