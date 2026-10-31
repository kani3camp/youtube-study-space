# YouTube Monitor

`youtube-monitor/` is the Next.js UI rendered in the YouTube livestream. It reads room state from Firestore and renders horizontal/vertical layouts.

> [!IMPORTANT]
> The monitor is **not display-only in the current architecture**. `src/components/MainContent.tsx` also calculates desired general/member seat counts from room definitions and current occupancy, then can call `POST /set_desired_max_seats`. Before changing room definitions or seat counts, read [`../docs/development/architecture.md`](../docs/development/architecture.md).

## OBS向けFirebase Authの導入状況

モニターのソースは専用Firebase Auth claimを確認してから、`seats`・`member-seats`・`menu`・`work-name-trend`・`config/constants`のブラウザ`onSnapshot`を購読する。リポジトリのFirestore Security Rulesは、有効なmonitor UID許可リスト項目も必須とし、匿名読み取りを拒否する。これはPhase 0Bの前提となる実装だが未配備であり、実環境で対応するモニターとRulesが稼働済みとは証明しない。

[Issue #1459](https://github.com/kani3camp/youtube-study-space/issues/1459)は、運用担当者が管理する専用Firebase Authユーザーの準備と、ソース・Rulesの一組での配備を必要とする。通常のMyPage Firebaseユーザーにこの権限を付けず、Client SDKの書き込みも有効にしない。既存の`/set_desired_max_seats` HTTP制御は独立しており、従来の検証を引き続き満たす必要がある。

実装済みの運用手順ではOBSブラウザソースを配信シーンから外し、OBSのInteractで`/operator-login`を開く。専用のメール・パスワードアカウントで一度ログインしてから、ソースを`/`へ戻す。以後のセッションにはFirebaseの`browserLocalPersistence`を使う。通常のChromeログインをOBS/CEFと共有しているとは仮定しない。`/operator-login`を配信用ブラウザソースURLにしてはならない。完了とする前に、実際のOBSバージョンでCEFのIndexedDB・永続化、OBS・PC再起動、シーン切替、トークン更新、ログアウト・失効、失敗通知を検証する。サービスアカウント鍵や管理者トークンを`NEXT_PUBLIC_*`・URL・OBSブラウザ引数に保存しない。

配備はモニター用本人情報・claim・許可リストの作成、EmulatorでのRulesテストの順に行う。各環境で認証対応モニターを先に配備・検証してから、読み取りを制限するRulesを適用する。DEVの証跡を揃えた後、本番でも同じ順序を繰り返して実購読を確認する。復旧時もRulesの拒否を維持し、認証対応モニターを修正配備する。匿名読み取りは戻さない。[Rollout and rollback](#rollout-and-rollback)を参照する。Phase 0Aの所有者限定MyPage試験は継続できるが、検証完了までは[Phase 0Bの外部公開を停止する](https://github.com/kani3camp/youtube-study-space/issues/1133)。

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
| `NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN` | モニターのFirebaseプロジェクトのAuthドメイン。 | Yes |

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

## モニター運用担当者の認証

モニターは、専用の`monitor_reader: true`カスタムclaimをIDトークンに持つローカルセッションをFirebase Authが復元するまで、Firestoreを購読しない。Firestore Security Rulesは、有効な`_monitor-readers/{uid}`許可リスト文書も必要とする。両方を求めることで、通常のMyPage Firebaseユーザーやclaimだけのアカウントによるモニター用コレクションの読み取りを防ぐ。

各環境で、信頼済みFirebase/Admin操作を使って次を準備する。

1. Firebase AuthのEmail/Passwordプロバイダーを有効にし、モニター運用専用アカウントを作る。モニターにサインアップ機能は追加しない。
2. 信頼済みAdmin SDK操作で、アカウントのカスタムclaimに`monitor_reader: true`を設定する。このクライアントにAdmin認証情報やCustom Token署名者を置かない。
3. Firebaseコンソールか信頼済みサーバー/Admin SDKで、`_monitor-readers/{uid}`に`{ "enabled": true }`を作る。文書IDはUIDとする。Security Rulesはこのコレクションへの全ブラウザ読み書きを拒否する。
4. OBSブラウザソースを配信シーンから外し、OBSのInteractで`/operator-login`を開いて一度ログインする。ページは`browserLocalPersistence`を使い、アプリケーション自体では認証情報を保存しない。
5. `/`へ戻り、モニターが必要な全コレクションを読めることを確認する。セッション・claim・許可リストが利用不能、または復旧不能なFirestore読み取りエラーの場合、モニターはデータ利用コンポーネントをアンマウントする。認可されないアクセスを続けず、拒否状態を運用担当者に表示する。

`/operator-login`を配信用ブラウザソースURLに使わず、認証フォームを配信シーンに映さない。claimの除去、許可リスト文書の削除、`enabled`の`false`への変更は、現在のIDトークンの更新後に、以後のRulesによる読み取り許可を失効させる。

### Rollout and rollback

各環境で、運用アカウント・カスタムclaim・許可リストを先に準備する。旧Rulesが有効な間に認証対応モニターを配備し、認証付き購読を検証する。その後、制限するRulesを配備し、読み取りと最大座席数制御を再確認する。DEVの証跡を記録してから、本番でも同じ手順を繰り返す。

復旧のために匿名Firestore Rulesへ戻してはならない。制限適用後にモニターが失敗した場合、Rulesの拒否を維持し、ソースを配信シーンから外して認証対応の修正を配備する。Rulesとモニターの配備は個別の承認対象である。ソースのテストだけでは、OBS/CEF永続化、ネットワーク切断通知、実App Checkの動作、実環境アクセスを証明しない。

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
