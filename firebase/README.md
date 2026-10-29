# Firebase / Firestore

This directory contains the repository-managed Firestore Security Rules, indexes, and emulator configuration. Treat these files as the existing project definition. Do **not** run `firebase init firestore` as a routine setup step because it can overwrite or regenerate repository-managed configuration.

## MyPage Phase 0Bの前提となるOBSモニター認証

リポジトリの`firestore.rules`は、`seats`・`member-seats`・`menu`・`work-name-trend`・`config/constants`の読み取りに、専用monitor claimと有効なUID許可リスト項目を必要とする。匿名と通常MyPageユーザーは拒否する。モニターのソースも専用claimを確認してから購読する。これは実装済みだが、実配備の証拠ではない。古いRulesの環境では、従来の匿名公開が残っている場合がある。

外部利用が可能になる最初のMyPage Phase 0B公開前に、[Issue #1459](https://github.com/kani3camp/youtube-study-space/issues/1459)は次を必要とする。

- モニター専用のFirebase Auth本人情報・claim・許可リスト項目を用意し、実装済みの対話ログインとFirebaseローカル永続化を使う。実OBS/CEF再起動後の保持は未検証である。
- 対応するモニターとRulesをレビュー済みの組として配備し、各対象環境で匿名・通常MyPageユーザー・全ブラウザ書き込みの拒否を確認する。
- Client SDK / Security Rulesのテストで、匿名・認可済みモニター・誤UID・無効許可リスト・read/list/write動作を検証し続ける。選択したDEV/PRODで、誤プロジェクトと期限切れセッションの拒否を別途検証し、再起動・更新・運用担当者への通知も確認する。
- Client SDKの制御と、Security Rulesを経由しないサーバー側Go/IAMを分ける。ブラウザバンドルにサービスアカウント認証情報を追加せず、APIキーやプロジェクトの分離を認可の代替にしない。
- 各環境で、拒否ルールの適用前に認証情報とログイン手段を準備する。無制限の匿名アクセスを戻さずに復旧できることを検証する。

この節は実装済みの制御と、未完了の実環境公開条件を記述しており、配備完了は示さない。別途承認がない限り、既存Go Botとモニターの座席数制御を維持する。関連する[MyPage公開条件 #1133](https://github.com/kani3camp/youtube-study-space/issues/1133)と[Notion Security Canon](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634)も参照する。

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

スクリプトはこのディレクトリに独立した依存関係を導入し、`@firebase/rules-unit-testing`で`firestore.rules.test.mjs`を実行してから、Go統合テストを実行する。Go層はEmulatorの動作に対してFirestoreのシリアライズ・トランザクション・リポジトリ/アプリケーション動作を検証する。Rulesテストは、ブラウザClient SDKの許可・拒否を別に検証する。

### 3. Client SDK / Security Rules behavior

When `firestore.rules` changes, reason explicitly about browser/client access separately from server-side Go behavior. Verify the intended allow/deny cases for every changed public boundary, especially `config/*` and any collection consumed by `youtube-monitor`.

A Go test using server credentials can bypass the question that Security Rules answer. Do not treat a passing Go Emulator integration suite as sufficient evidence for a client-access policy change.

モニター読み取りにはFirebase IDトークンの`monitor_reader: true`と、有効な`_monitor-readers/{uid}`文書の両方を必要とする。クライアントルールは許可リストの公開・変更を認めない。claimと許可リスト文書は信頼済みAdmin/サーバー操作だけで作る。段階導入と、拒否を維持する復旧手順は[`../youtube-monitor/README.md`](../youtube-monitor/README.md)を参照する。

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
