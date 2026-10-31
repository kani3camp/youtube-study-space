# YouTube Monitor

`youtube-monitor/` は、YouTubeライブ配信に表示するNext.js製のUIです。Firestoreから部屋の状態を読み取り、横型・縦型のレイアウトを描画します。

> [!IMPORTANT]
> 現在の構成では、モニターは**表示だけを担当するわけではありません**。`src/components/MainContent.tsx` は、部屋の定義と現在の利用状況から一般席・メンバー限定席の希望席数を計算し、`POST /set_desired_max_seats` を呼び出すことがあります。部屋の定義や席数を変更する前に、[`../docs/development/architecture.md`](../docs/development/architecture.md) を確認してください。

## セットアップ

実行環境のバージョンは [`package.json`](./package.json) の指定に従ってください。ほかのドキュメントにバージョン番号を重複して記載しないでください。

```sh
cd youtube-monitor
pnpm install --frozen-lockfile
```

## 実行時に必要なアセット

ライブ配信には、`public/` 配下の画像・チャイム・BGMが必要です。これらの一部は意図的にリポジトリの管理対象から外しているため、運用用のビルドや起動を行う前に、必要なファイルをローカルに配置してください。

```sh
pnpm verify:assets
```

検証コマンドは、実行用ソースが参照する画像・チャイムが `public/` 配下に空でないファイルとして存在することと、`public/audio/` に空でない `.mp3` ファイルが1つ以上あることを確認します。次の運用用コマンドでも、この検証が自動で実行されます。

```sh
pnpm build
pnpm start
```

`pnpm build:ci` は、実行時アセットの検証を省略します。非公開のアセットやローカルのメディアを配置していないCIなどで、ソースのコンパイルだけを確認するためのコマンドです。

## 環境変数

以下の環境変数はすべて `NEXT_PUBLIC_` で始まり、ブラウザから見えるコードに組み込まれます。**サーバー専用の認証情報や秘密情報を設定しないでください。** 環境ごとの実際の値も、Issue・ログ・ドキュメントの例には記載しないでください。

| 変数 | 用途 | ブラウザからの参照 |
| --- | --- | --- |
| `NEXT_PUBLIC_DEBUG` | モニターのデバッグ動作を有効にします。`true` または `false` を指定します。 | 可 |
| `NEXT_PUBLIC_CHANNEL_GL` | チャンネル固有の動作を切り替えます。`true` または `false` を指定します。 | 可 |
| `NEXT_PUBLIC_ROOM_CONFIG` | 部屋・レイアウトの設定を選択します。`PROD` または `DEV` を指定します。 | 可 |
| `NEXT_PUBLIC_API_ENDPOINT` | `/set_desired_max_seats` など、モニターが呼び出すAPIのベースURLです。 | 可 |
| `NEXT_PUBLIC_API_KEY` | fetcherがAPIリクエストに使用するクライアント側の設定です。ブラウザへ配布されるため、サーバーの秘密情報として扱うことはできません。 | 可 |
| `NEXT_PUBLIC_FIREBASE_PROJECT_ID` | クライアントが使用するFirebase / FirestoreのプロジェクトIDです。 | 可 |
| `NEXT_PUBLIC_FIREBASE_API_KEY` | クライアントSDKが使用するFirebaseのWeb APIキーです。ブラウザから見える設定値であり、サーバー用の認証情報ではありません。 | 可 |
| `NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN` | モニターが接続するFirebaseプロジェクトのAuthドメインです。 | 可 |

値の検証と利用箇所は、`src/lib/constants.ts`、`src/lib/api-config.ts`、`src/lib/fetcher.ts`、`src/lib/firestore.ts` にあります。

## 実環境へ接続しない検証

静的検査とテストに実際のFirebase / APIプロジェクトは不要です。単体・コンポーネントテストにはVitestとjsdomを使用し、実行時アセットの検証にはNode.jsによる再現可能なテストを用意しています。

```sh
pnpm check
pnpm test
```

ローカル開発中に、変更を監視してテストを実行する場合は次のコマンドを使用します。

```sh
pnpm test:watch
```

ソースのみで本番用のコンパイルを確認する場合は、CIと同じダミー設定を使用してください。以下の値に秘密情報は含まれていません。

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

これらは**ビルド確認専用の仮の値**です。配備済み環境への接続や動作を保証するものではありません。また、`build:ci` では実行時メディアの配置状況も検証しません。

## モニター運用者の認証

モニターは、専用のカスタムクレーム `monitor_reader: true` を含むIDトークンを持ったローカルセッションがFirebase Authで復元されるまで、Firestoreを購読しません。Firestore Security Rulesでも、有効な `_monitor-readers/{uid}` の許可リスト文書を要求します。両方を必須にすることで、通常のMyPage用Firebaseユーザーや、クレームだけを持つアカウントによるモニター用コレクションの読み取りを防ぎます。

各環境で、信頼できるFirebase / Admin操作を使って次の準備を行ってください。

1. [Terraformの認証用モジュール](../infra/gcp/modules/monitor-auth-registration/README.md)でメール・パスワード認証と一般利用者の新規登録制限を適用し、Firebaseコンソール／Admin SDKでモニター運用者専用のアカウントを作成します。モニターにユーザー登録機能は追加しません。
2. 信頼できるAdmin SDK操作で、アカウントに `monitor_reader: true` のカスタムクレームを設定します。Admin用の認証情報やカスタムトークンの署名処理を、このクライアントに置かないでください。
3. Firebaseコンソールまたは信頼できるサーバー / Admin SDKから、`_monitor-readers/{uid}` を `{ "enabled": true }` で作成します。文書IDにはUIDを使用します。Security Rulesは、このコレクションに対するブラウザからの読み書きをすべて拒否します。
4. OBSでモニターを開き、サーバーログを確認します。未ログインを検知すると、`[monitor] 未ログインです。ログインURL: http://localhost:18080/monitor-login` と表示します。起動だけでは出力せず、認証確認中・認証済み・認証状態の取得エラーでも出力しません。同じ認証不足の状態が続く間はブラウザから1回通知し、サーバー側でもプロセス・状態ごとに1分間の重複を抑制します。権限不足の場合は、ログイン済みであることと `monitor_reader: true` の確認をログに表示します。通知APIが利用できない場合も、未認証時の購読停止は維持します。URLのポートは実際の起動ポートに対応します。別のPCのOBSから接続する場合は、`localhost` を通常のモニターURLのホスト名に置き換えてください。リバースプロキシ経由で接続する場合も、通常のモニターと同じオリジンの `/monitor-login` を使用します。
5. 別シーンへ切り替えるなどして認証画面が配信へ映らない状態にし、対象ブラウザソースの「プロパティ」でURLを `/monitor-login` のURLに変更します。そのソースの**「対話」**を開き、運用者アカウントで一度ログインします。通常のブラウザでのログインではなく、OBS内で行ってください。このページは `browserLocalPersistence` でセッションを永続化します。アプリケーションが入力したパスワードを独自に保存することはありません。
6. ログイン成功後は「ログインしました。」と表示し、ページは移動しません。ブラウザソースの設定URLを元のモニターURLへ戻し、必要なコレクションをすべて読み取れることを確認してから配信画面へ戻します。セッション・クレーム・許可リストが利用できない場合や、Firestoreから復旧不能な読み取りエラーが返った場合は、モニターがデータを利用するコンポーネントをアンマウントし、アクセスを停止した状態でエラー表示に切り替えます。

配信向けの認証・読み取りエラー画面は、ライブチャットまたは緊急連絡フォームへの連絡を案内します。OBSの操作手順やクレーム・許可リストなどの技術的な説明は表示しません。運用者用のログイン画面は、ログイン成功とモニター閲覧権限の有無を分けて扱います。ログイン済みなら成功を表示し、その下に閲覧設定の確認結果を表示します。

`/monitor-login` を配信用ブラウザソースのURLに設定したままにせず、認証情報の入力画面を配信へ映さないでください。クレームの削除、許可リスト文書の削除、または `enabled` の `false` への変更によって、Rulesで許可される今後の読み取りを失効させます。クレームの削除がIDトークンに反映されるには、トークンの更新が必要です。

### ログイン成功後もモニターを表示できない場合

メール・パスワードでのログイン成功だけではモニター閲覧権限は付与されません。Firebaseコンソールでユーザーを作成した後、同じUIDへの `monitor_reader: true` と `_monitor-readers/{uid}` の `{ "enabled": true }` の両方を設定してください。クレーム未設定の場合はFirestoreの購読開始前に停止します。クレームがあっても許可リストがない場合は、制限済みのFirestore Rulesが読み取りを拒否します。ログイン後にクレームを追加した場合は、新しいクレームを含むIDトークンへの更新が必要です。ログイン成功画面が表示されていることだけで、クレームや許可リストの設定完了とは判断しないでください。

ログイン画面では、クレームがある場合だけ `config/constants` を `getDocFromServer` で一度読み、キャッシュではなくサーバーの読み取り許可を確認します。文書の内容は診断に使わず、許可リスト自体の参照権限も追加しません。診断では席数制御APIを呼びません。

- クレーム不足：`monitor_reader: true` とIDトークンへの反映を案内します。この段階ではFirestoreを読みません。
- 読み取り拒否：`_monitor-readers/{UID}` の `enabled: true`、接続先プロジェクト、Security RulesとApp Checkの確認を案内します。拒否エラーだけから許可リストの欠落・無効やRulesの問題を特定することはできません。
- 設定文書の欠落：読み取りは許可されたものの `config/constants` が存在しないことを案内します。
- 接続失敗：権限不足と断定せず、ネットワークとFirebase設定の確認を案内します。

これらの診断結果とモニターの購読エラーは、固定の分類だけを通知APIへ送り、同じ案内をサーバーログにも記録します。パスワード・IDトークン・UID・文書の内容・生のエラー本文は送信しません。Firestoreの設定・接続エラーではログインURLを出さず、未ログイン・クレーム不足の場合だけ出力します。

### 段階的な導入とロールバック

環境ごとに、まず運用者アカウント・カスタムクレーム・許可リストを準備します。認証対応のモニターを配備し、従来のRulesが有効な状態で認証済みの購読を確認してください。その後、読み取りを制限するRulesを配備し、読み取りと席数制御を再確認します。DEVでの検証結果を記録してから、本番でも同じ手順を実施してください。

ロールバックとして、匿名読み取りを許可するFirestore Rulesに戻さないでください。制限後にモニターが動かなくなった場合はRulesを閉じたままにし、ブラウザソースを配信画面から外して、認証に対応したモニターの修正版を配備します。Rulesとモニターの配備は、それぞれ対象環境への操作として承認が必要です。ソースのテストだけでは、OBS / CEFでのセッション永続化、ネットワーク切断時の通知、実際のApp Checkの動作、実環境へのアクセスは確認できません。

## 実環境での検証

実際の環境の値を使用するのは、実環境での動作確認が明示的に依頼され、承認されている場合だけです。モニターはFirestoreを購読し、希望席数のリクエストを送ることがあるため、開く前に接続先の環境を確認してください。

部屋・座席レイアウトの動作を変更する場合は、少なくとも次の項目を確認してください。

- 変更したロジックを共有する横型・縦型の表示
- 基本ルームの席数とページの境界
- 固定席数モードと可変席数モードの動作
- 可変席数モードでの臨時ルームによる席の表示範囲
- 一般席・メンバー限定席の両方で、モニターが送る希望席数が想定どおりであること

## 席数制御の関連コード

- 部屋の定義と基本・臨時ルームの構成：`src/rooms/rooms-config.ts`
- 希望席数の計算とAPIリクエスト：`src/components/MainContent.tsx`
- APIパス：`src/lib/api-config.ts`
- バックエンドのエンドポイント：`../system/cmd/lambda/set_desired_max_seats/main.go`
- バックエンドでの席数調整：`../system/core/workspaceapp/max_seats_adjustment.go`

現在の動作と、将来的に目指す責務の分担は [`../docs/development/architecture.md`](../docs/development/architecture.md) に記載しています。
