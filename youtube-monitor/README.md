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

## Environment variables

All variables below use the `NEXT_PUBLIC_` prefix and are bundled into browser-visible code. **Do not put server-only credentials or secrets in them.** Environment-specific real values should still stay out of issues, logs, and documentation examples.

| Variable | Purpose | Public/client-visible |
| --- | --- | --- |
| `NEXT_PUBLIC_DEBUG` | Enables monitor debug behavior. Must be `true` or `false`. | Yes |
| `NEXT_PUBLIC_CHANNEL_GL` | Selects channel-specific behavior. Must be `true` or `false`. | Yes |
| `NEXT_PUBLIC_ROOM_CONFIG` | Selects the room/layout configuration. Must be a non-empty string. | Yes |
| `NEXT_PUBLIC_API_ENDPOINT` | Base URL for monitor API calls such as `/set_desired_max_seats`. | Yes |
| `NEXT_PUBLIC_API_KEY` | Client-side API request configuration used by the fetcher. It must not be treated as a server secret because it is shipped to the browser. | Yes |
| `NEXT_PUBLIC_FIREBASE_PROJECT_ID` | Firebase/Firestore project identifier used by the client. | Yes |
| `NEXT_PUBLIC_FIREBASE_API_KEY` | Firebase web API key used by the client SDK. It is browser-visible configuration, not a server credential. | Yes |

The validation/consumers are in `src/lib/constants.ts`, `src/lib/api-config.ts`, `src/lib/fetcher.ts`, and `src/lib/firestore.ts`.

## Verification without real external connections

Lint/check and tests do not require a real Firebase/API project:

```sh
pnpm check
pnpm test --runInBand
```

For a production build, use the same non-secret dummy configuration as CI when the goal is only to prove that the application compiles:

```sh
NEXT_PUBLIC_DEBUG=false \
NEXT_PUBLIC_CHANNEL_GL=false \
NEXT_PUBLIC_ROOM_CONFIG=DEV \
NEXT_PUBLIC_API_ENDPOINT=http://localhost:3000 \
NEXT_PUBLIC_API_KEY=ci-dummy-api-key \
NEXT_PUBLIC_FIREBASE_PROJECT_ID=ci-dummy-project \
NEXT_PUBLIC_FIREBASE_API_KEY=ci-dummy-firebase-api-key \
pnpm build
```

These values are **build-only placeholders**. They do not prove connectivity or behavior against a deployed environment.

## Runtime Roomの実寸プレビュー

Runtime Room画像は、最終配信フレーム（1920 × 1080）の左上1520 × 1000へ表示します。右400pxはSidebar、左1520pxの下80pxはMessage（920 × 80）とTicker（600 × 80）です。新規Runtime RoomのClean Imageは当面1520 × 1000を正とし、16:9画像を暗黙に引き伸ばしません。

Storybookの `Development/Runtime Room Preview` では、本番と同じ `SeatsPage` / `SeatBox` / CSS / 左上原点の座標変換を使い、全体フレーム上で実寸相当の配置を確認できます。Firestoreや実環境APIへは接続しません。

```sh
cd youtube-monitor
NEXT_PUBLIC_DEBUG=false \
NEXT_PUBLIC_CHANNEL_GL=false \
NEXT_PUBLIC_ROOM_CONFIG=DEV \
NEXT_PUBLIC_API_ENDPOINT=http://localhost:3000 \
NEXT_PUBLIC_API_KEY=preview-dummy-api-key \
NEXT_PUBLIC_FIREBASE_PROJECT_ID=preview-dummy-project \
NEXT_PUBLIC_FIREBASE_API_KEY=preview-dummy-firebase-api-key \
pnpm storybook
```

プレビュー上部のボタンで `general`（140 × 100）と `member`（230 × 150）を切り替えます。同じ画像・同じSeat Anchorを維持したまま、メンバー席でのstress testができます。チェックボックスでSeat Zone、Protected Visual Zone、main circulation、Seat Anchor、回転後外接矩形、validation結果を個別に表示できます。赤い外接矩形と `FAIL` は、SeatBox同士の正の面積を持つ重なり、Room外へのはみ出し、保護領域・主動線への侵入、指定Seat Zone外のいずれかを表します。境界へ接しているだけの場合は衝突にしません。

fixtureは [`src/dev/runtime-room-preview/fixtures.ts`](./src/dev/runtime-room-preview/fixtures.ts) に追加します。`RuntimeRoomOverlayMap` は `RoomLayout`そのものへ開発専用metadataを隣接させた型です。`room_shape`、`seat_shape`、`seats[].x/y/rotate` を一度だけ定義し、その同じオブジェクトをDOMプレビュー、validator、最終 `RoomLayout` として使ってください。Clean Imageを `public/` 以下へ置き、`floor_image`へ絶対パスを設定します。座標は0〜1へ正規化せず、Room左上を原点とする1520 × 1000論理座標で記述します。

Runtime Room追加時は、次を両profileで確認します。

- 全席の椅子・作業面とSeatBoxの対応が読める
- 一般席で合格し、同じ画像を共用する場合はメンバー席でも合格する
- 空席、利用中、長い作業名、休憩、メンバー、プロフィール画像ありの表示が背景と競合しない
- 回転後外接矩形が相互に重ならず、1520 × 1000内に収まる
- Protected Visual Zoneとmain circulationを覆わず、各席が指定Seat Zone内に収まる
- 失敗時はSeatBoxを縮小せず、Seat Plan、カメラ、家具間隔、動線の順で再構図する

`Calo current相当 — 既知UI互換性問題` は成功例ではなく、10席の短いベイでの衝突、港景、横動線との競合をvalidatorが検出し続けるための回帰fixtureです。

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
