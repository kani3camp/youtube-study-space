# YouTube Monitor

`youtube-monitor/` is the Next.js UI rendered in the YouTube livestream. It reads room state from Firestore and renders horizontal/vertical layouts.

> [!IMPORTANT]
> The monitor is **not display-only in the current architecture**. `src/components/MainContent.tsx` also calculates desired general/member seat counts from room definitions and current occupancy, then can call `POST /set_desired_max_seats`. Before changing room definitions or seat counts, read [`../docs/development/architecture.md`](../docs/development/architecture.md).

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
pnpm build:ci
```

These values are **build-only placeholders**. They do not prove connectivity or behavior against a deployed environment, and `build:ci` does not prove that runtime media has been provisioned.

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
