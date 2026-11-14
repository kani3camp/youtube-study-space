# MyPage release preparation

メンテナンスモードの初期化・開始・解除と、安全のため利用を拒否した場合の復旧は[メンテナンスモードv1の手順書](../maintenance-mode.md)を参照する。通常リリース・Terraform apply・Hosting公開とは独立した手動操作であり、その手順書にあるワークフロー以外からFirestore状態を直接変更しない。

この手順書は設定検査・配備・公開条件を案内する。実装仕様は[MyPage](../README.md)、APIは[OpenAPI](../openapi.yaml)、受入項目と復旧試験は[E2E手順](e2e.md)を参照する。配備・検証結果は対象リリースのIssue / PR / Actionsに記録する。現在の統合状況は[PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)、未完の公開受入は[Issue #1133](https://github.com/kani3camp/youtube-study-space/issues/1133)で追跡する。設定検査・合成QA・設計決定を、実経路の合格や公開承認として扱わない。

プロダクト方針は Current Canon の [04 認証](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd)、[05 Privacy](https://app.notion.com/p/3ed357a8d0ce81e7b7d2cc85ec8ab4cf)、[06 Security](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634)、[09 判断記録](https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f) を参照する。

## 構成と配備の入口

開発・本番とも、Firebase Hosting (React/Vite)、`/api/**`の書き換え、Cloud Run Go BFF、Firebase Authentication、Firestoreで構成する。Cloudflareはドメイン登録とDNSだけに使う。初期構成にManaged WAF・Cloud Armor・専用外部LB・Cloud Run Webは含めない。標準の`run.app`にも直接到達できるため、Hostingと直接Runの両方で認証・App Check・SAC・Origin・IP/UID制限・S01/S04を検証する。

公開ドメインは`https://moku.work/`とする。登録のActive状態、DNS/TLS、OAuthコールバック、本番配備は対象環境を再読み取りして確認する。

| 対象 | 手順・契約 |
| --- | --- |
| 基盤・鍵を使わない認証・環境別State | [MyPage基盤](../../../infra/mypage-platform/README.md) |
| OAuthトランザクションのTTL | [専用Terraform](../../../infra/mypage-oauth-ttl/README.md)。アプリの10分失効と非同期TTL削除を区別する |
| バックエンドイメージ更新 | [バックエンドCI](backend-ci.md)。コードPR統合後の検査 → image-build → Registry digest検証 → CIから直接Cloud Run image配備。Terraformは通常リリースで起動しない |
| フロントエンド | 下記のDEV Hosting CIと静的キャッシュ仕様 |
| メタデータ保持・監視 | [削除処理と独立監視](metadata-monitoring.md)。ソースの実装と実スケジューラー・出力先への接続を区別する |
| 横断削除 | [削除手順](../support-delete.md)・[調査対象](../support-delete-inventory.md)。実CLI実装やlocal/mock検証だけを実削除運用の受入完了と扱わない |

## 公開設定

### GA4の環境別セットアップ

MyPageのGA4は、各Firebaseプロジェクトに既に接続された`mypage`用GA4プロパティを使う。各プロパティのWebデータストリームは1つとし、`youtube-monitor`用プロパティは使わない。測定IDはDEV `G-E31NHW939V` / PROD `G-ZJ1VPVT2L2`を採用する。ストリームURLは承認済みの環境別公開オリジンに揃え、開発アクセスを本番集計へ送らない。IDの採用は確定済みだが、環境別ビルド設定への反映、コンソール設定の受入、実通信・Realtimeの検証が終わるまで、実送信の有効化完了とは扱わない。

- 両プロパティで広告連携・Google Signals・広告パーソナライズを使わず、保持14か月と、新規活動による保持期間リセットOFFを確認する。SPA側で手動ページビューを送るため、Enhanced Measurementの履歴変更時の自動ページビューなど、重複・余分な送信を無効にする。
- `VITE_MYPAGE_ANALYTICS_READY=true`、`VITE_MYPAGE_GA4_ID=<その環境の測定ID>`、`VITE_MYPAGE_PUBLIC_ORIGIN=<その環境のHTTPS origin>`を環境別フロントエンドのビルド設定に渡す。測定IDは公開値であり、シークレット扱いしない。公開マニフェストの整合性検査と、Hosting CSPの別途承認が必要である。
- 未接続時は初回の同意バナーを表示しない。既存のv1同意設定は、接続前に取得した選択なので再利用しない（v2へ更新）。GA4を有効化した最初のアクセスで、ユーザーへGoogle Analytics・Cookie等を使うサイト改善のためのアクセス解析を案内し、明示的に同意/拒否を選んでもらう。
- DEVから先に実ブラウザで確認する。同意前・拒否中のGA4通信0件、許可後の実イベントとURLの秘匿処理、拒否変更後の非送信とCookie、Realtime表示、CSP、外部送信表示を対象にする。ローカル合成テストではコンソールや実ベンダーの動作を証明できず、DEVの結果を本番検証に読み替えない。

## 外部接続なしの設定事前検査

公開設定は[Vite/Firebase/GA4](../../../mypage/src/firebase.ts)、サーバー設定は[起動処理](../../../system/cmd/mypage-server/main.go)の既存仕様に揃える。値は選択したプロジェクト・アプリの管理画面や承認済み一覧から取得し、例の値を実プロジェクトへ流用しない。

```sh
# repository root: explicit public-only JSONだけを読む。SDK/ADC/通信なし。
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
# private manifestをstdinで渡してもよい。server環境全体をexportしない。
node mypage/scripts/check-release-config.mjs --stdin < /path/to/private-public-config.json

# system: 現在processの設定を既存validatorで確認する。ADC/listen/通信なし。
cd system
go run ./cmd/mypage-server --check-config
```

バックエンド検査だけは、既存の安全なプロセス環境からサーバー専用シークレットを読む。コマンド引数・マニフェスト・Viteへ移さない。出力は固定のstatus/field/codeだけとする。公開設定チェッカーは64KiB以内の明示JSONを読み、秘密・デバッグ・未知フィールドと、プロジェクト・アプリ・オリジン・同意バージョンの不一致を拒否する。任意のバックエンド補助宣言は、非秘密の識別情報とバージョンだけを持つ。任意のHosting設定には、既存候補の検証処理を再利用する。

マニフェストの最上位項目は`schemaVersion: 1`、`deployment`、`frontend`と、任意の`backend`、`hosting`、`publicPolicy`である。`deployment`/`backend`は`environment`、`projectID`、`projectNumber`、`webAppID`、`publicOrigin`、`privacyVersion`、`termsVersion`の7項目を持つ。`frontend`の正確なキーは[例](public-config.example.json)と[公開スキーマ](../../../mypage/src/release-config.ts)を参照し、サーバー環境をコピーしない。`publicPolicy`は`VITE_PUBLIC_CONTACT_URL`、`VITE_PUBLIC_PRIVACY_CONTACT_URL`、`VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE`、`VITE_PUBLIC_TERMS_EFFECTIVE_DATE`、`VITE_PUBLIC_JURISDICTION`と、任意の`VITE_PUBLIC_OPERATOR_NAME`を受け付ける。既知のFirebaseプロジェクトでは、プライバシー窓口のキーをマニフェストに設定しない。`hosting`には[既存HostingInputs](../../../mypage/src/hosting.ts)の明示的な接続先一覧を使う。任意入力の省略は`not-supplied`として未検証のまま残す。

両検査とも、外部登録・鍵の真正性・権限や利用枠・公開本文の承認は確認しない。`configurationValid=true`/`configuration=valid`でも、基盤と実環境のE2Eは未完了である。`MYPAGE_INFRASTRUCTURE_READY=true`は起動に必要な宣言であり、確認完了の証拠ではない。準備時の検査のために実起動やReadyフラグ変更を行わない。

### 設定の受け渡し

Hostingの画像接続先一覧には、チャンネル画像用の`https://yt3.ggpht.com`と`https://yt3.googleusercontent.com`の両オリジンを明示する。reCAPTCHA画像だけの一覧は事前検査で拒否する。許可元は[Hosting候補](../../../mypage/src/hosting.ts)と[設定検証](../../../mypage/src/release-config.ts)で検査する。img-src専用であり、script/connect/frameは広げない。将来、別の画像オリジンが返った場合は実配信元を確認し、一覧を再レビューする。

| 設定の用途 | 項目 | 受け渡し・検証 |
| --- | --- | --- |
| public browser | `VITE_FIREBASE_API_KEY/AUTH_DOMAIN/PROJECT_ID/APP_ID`, `VITE_APP_CHECK_SITE_KEY` | 公開Web app設定。secret/admin key/debug tokenではない。project番号とapp、登録domainは別途照合 |
| 同意版 | browser `VITE_PRIVACY_POLICY_VERSION/VITE_TERMS_VERSION`; server `MYPAGE_PRIVACY_VERSION/MYPAGE_TERMS_VERSION` | 同じrelease対で一致。実際に承認された本文の版を記録 |
| origin | `MYPAGE_PUBLIC_ORIGIN`; analytics用 `VITE_MYPAGE_PUBLIC_ORIGIN` | environmentの固定HTTPS origin。Host/Originをclient headerから推測しない |
| server identity | `MYPAGE_ENVIRONMENT/REGION/PROJECT_NUMBER/WEB_APP_ID/SIGNER_EMAIL`, `GOOGLE_CLOUD_PROJECT` | development=asia-southeast2、production=asia-northeast2。signerは選択projectのもの |
| server secret | `MYPAGE_OAUTH_CLIENT_ID/CLIENT_SECRET`, optional `MYPAGE_YOUTUBE_API_KEY` | OAuth secret/API keyは承認済みsecret供給境界。repo/chat/public artifact/Viteに入れない |
| optional Analytics | `VITE_MYPAGE_ANALYTICS_READY`, `VITE_MYPAGE_GA4_ID` | 実Console・network inventoryが承認されるまで有効化しない。Consentを迂回しない |
| 公開案内 | `VITE_PUBLIC_CONTACT_URL`、任意の `VITE_PUBLIC_OPERATOR_NAME`、`VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE/TERMS_EFFECTIVE_DATE/JURISDICTION` | 一般窓口は公開マニフェスト／環境変数で指定する。公開できる値だけを使い、実名・自宅住所を必須にしない。文面は草案のままとする。 |
| プライバシー窓口 | [`public-policy-config.ts`](../../../mypage/src/public-policy-config.ts) の `privacyForms` | `VITE_FIREBASE_PROJECT_ID` が `test-youtube-study-space` または `youtube-study-space` の場合は、環境別の固定URLを優先する。未知・未設定のプロジェクトでのみ `VITE_PUBLIC_PRIVACY_CONTACT_URL` を使用する。 |
| ライブ・メンテナンス告知 | `mypage/src/youtube-channel.ts`のDEV/PRODチャンネル対応表、選択keyは`VITE_FIREBASE_PROJECT_ID` | DEV (`test-youtube-study-space`) は`UChBj4xZePvWyGmTKct5FBcg`、PROD (`youtube-study-space`) は`UCXuD2XmPTdpVy7zmwbFVZWg`の`/live`・`/posts`。公開トップとマイページのライブ導線は同じ選択を使う。未知・未設定projectでは両導線を非表示にし、fallbackしない。Firestore・Backend・browser storageへ複製せず、変更はsource reviewと再buildを必須にする |
| Bot設定案 | `MYPAGE_PUBLIC_ENTRY_URL`（runtimeへの接続は未実装） | 承認済みpublic入口だけ。user固有query/tokenを含めない。0A/0Bは未設定、Phase1承認後に設定。実装・設定の承認前に有効化しない |

### お問い合わせ窓口の設定と受入

`/contact` は、使い方・不具合・ご意見の一般窓口と、Google連携解除後の連絡やデータ削除・開示・訂正・全ログイン解除などのプライバシー窓口を分けて案内する。受付と返信は人的窓口で行い、[本人確認・運用CLI](../support-operator.md)を利用する。

- 一般窓口は公開マニフェストの `publicPolicy.VITE_PUBLIC_CONTACT_URL` がデプロイ時の正本で、ビルドでは対応する環境変数を読む。未知・未設定のプロジェクトでプライバシー窓口を使う場合に限り、同じ方法で `VITE_PUBLIC_PRIVACY_CONTACT_URL` を指定する。メールは `mailto:` 付き、WebはHTTPSの窓口URLを使い、認証情報・本人確認リンク・利用者情報・クエリを含めない。一般窓口とこのフォールバックは、未設定・不正値の場合にそれぞれ「準備中」と表示する。一般窓口をプライバシー請求先に流用しない。
- 既知のFirebaseプロジェクトでは、プライバシー窓口をマニフェストに設定しない。ソースの `privacyForms` が正本で、環境変数の値にかかわらず該当する固定URLを表示する。環境の取り違えを防ぐため、URLの変更にはソースレビューと再ビルドが必要となる。
- プライバシー請求にFAQの事前読了やマイページへのログインを求めない。依頼受付後は、原則として同じチャンネルを新しいYouTubeの許可で確認する（fresh OAuth）。通常のログインだけで依頼を実行しない。この確認を利用できない場合は、窓口で個別の方法を案内する。

#### お問い合わせ窓口の公開前 Gate

- マイページ未ログイン・利用制限中でも窓口に到達できることと、Googleに未ログインの状態で環境別のフォームから試験回答を送信できることを確認する。運営者の受信、返信、依頼者への返信到達まで実証する。OAuth同意画面のsupport emailや既存の匿名窓口は自動転用しない。
- フォーム回答の保管場所、閲覧権限、保持期間、削除方法と担当を確定する。受付時刻・返信先・対応担当・期限は人的窓口で管理し、対応結果は元の窓口から返す。実データ削除と7暦日以内の期限管理は別途実証する。
- [Google連携解除の暫定手順](../support-operator.md#google側で連携を解除した利用者への対応)を開発環境で通し確認し、正式手順化と、fresh OAuthを利用できない場合の本人確認の最低基準を公開前に判断する。

これらは公開の受入条件であり、ソースのマージ条件には含めない。

## ローカル検証

依存関係は各パッケージのロックファイルとツールチェーン設定を使う。秘密や実ユーザーデータは不要である。以下はリポジトリ内の検査であり、実OAuth/Hostingを代替しない。

```sh
# root
python3 .github/scripts/check-doc-references.py
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
```

各パッケージの確認は、それぞれリポジトリのルートから開始する。

```sh
cd system
go test -shuffle=on ./...
golangci-lint run --timeout=5m --config=.golangci.yml
I18N_BASELINE=ja go generate ./...
```

```sh
cd mypage
pnpm check
pnpm typecheck
pnpm test
pnpm build
```

Go変更の必須検査は[AGENTS.md](../../../AGENTS.md)に従い、生成後に意図しない差分がないことを確認する。Firestoreに依存する受入検証は、ルートから[既存Emulatorスクリプト](../../../.github/scripts/run-firestore-integration-tests.sh)を実行する。demoプロジェクトを使い、実認証情報は渡さない。

状態遷移は[ランタイムテスト](../../../mypage/src/runtime.test.ts)と[プライバシーテスト](../../../mypage/src/privacy.test.ts)で検証する。ブラウザ確認ではローカルViteをポート18081で起動し、[表示確認の入口](../../../mypage/src/visual.tsx)と[フォントQA](../../../mypage/scripts/font-qa.py)を使う。設定済み公開文面は専用テストと実ブラウザで、匿名到達と安全なリンクを確認する。実ユーザー画面を公開QA成果物に保存しない。

### DEV HostingをCIで配備する

Google OAuthの同意画面に登録済みの`/privacy_policy`は、SPAルーターで`/privacy`へ履歴を置換して移動する互換入口とする。Firebase Hostingの`web.app`・`firebaseapp.com`両ドメインで、匿名のまま同じ既存本文へ到達できる。検索パラメーター・フラグメントは引き継がず、正規の公開URLは引き続き`/privacy`・`/terms`・`/contact`を使う。HTTPリダイレクト設定やOAuthコンソールの登録は変更せず、この互換入口の反映にもフロントエンドの再配備が必要である。

基盤のTerraform Applyとページ配備は別操作である。DEVページの更新は`Deploy MyPage Frontend`（[`deploy-mypage-frontend.yml`](../../../.github/workflows/deploy-mypage-frontend.yml)）の手動実行で行う。`integration/mypage-canon-20261006`を選んで実行する（入力なし、DEVのみ）。実行SHAは選択refの`github.sha`を使い、手入力しない。PR実行・main・他ブランチ・別所有者・再実行・SHA不一致は拒否する。失敗時に同じRunを再実行して継続せず、現在状態を確認して新しく実行する。

1. 同じrunで、まずMyPageのcredentialless CI（`ci-frontend.yml`の`mypage`: audit・cost model・フォント整合性・check・test・build）を通す。続くクラウド認証情報のないBuildジョブで、固定依存の検査（`pnpm audit`）・check・typecheck・テスト・DEV公開設定検査・ビルドを行う。
2. 同じ実行の成果物を別Applyジョブへ渡し、ソースSHA・run ID・30分の期限・設定・全ファイルのSHA256を再検証する。Applyは専用WIFと短命トークンを使い、SA鍵やFirebaseログイントークンは使わない。
3. 対象は`test-youtube-study-space`の`test--study-space`だけとする。現在のHostingのCSP・書き換えなどと、APIの同意バージョン・固定オリジン・Readyリビジョンを照合する。意図しない設定変更、別サイト、途中のHosting・API・ブランチ変更は拒否する。公開版の作成・アップロード・確定後、最後にリリースを切り替える。書き込みを自動再試行しない。
4. 配備後は全ファイルのバイト列・Cache-Control・CSPなど、`/`と`/mypage`、匿名APIの403、APIメタデータの不変を確認する。別の認証情報なしBrowserジョブで、匿名画面・Cookie設定・GA4通信0件を確認する。失敗は`Deploy MyPage Frontend`のそのrunの失敗として表示される（`CI`や`CI Gate`とは別のrun）。実GA4送信・認証済みE2E・公開条件の完了を意味しない。

GitHub Environmentの`mypage-dev-hosting`には、公開項目だけの`MYPAGE_HOSTING_PUBLIC_CONFIG_JSON`と専用プロバイダー名`MYPAGE_HOSTING_WIF_PROVIDER`をVariablesに設定する。前者は既存の公開マニフェストスキーマを使い、`VITE_MYPAGE_ANALYTICS_READY=false`、`analyticsApproved=false`を必須とする。サーバー環境の書き出し、OAuth・認証情報・Secretの値は渡さない。公開済みDEVと同じオリジン・Firebaseアプリ・同意バージョン・CSP接続先一覧を引き継ぐ。設定変更は既存の事前検査とレビューを通す。

権限の初期設定は、専用の`mypage-hosting` WIF pool/providerと`mypage-hosting-deploy` SAに分ける。プロバイダーは、固定リポジトリ・所有者・actor ID、初回workflow_dispatch、統合ref、呼び出し元`deploy-mypage-frontend.yml`（移行前は`ci.yml`。ownerがconditionの`workflow_ref`を置き換える）、再利用先`mypage-hosting-authenticated.yml`、同一workflow SHA、Environment `mypage-dev-hosting`をすべて検査する。既存TerraformのWIF・SA・Stateは変更しない。

[Firebase公式](https://firebase.google.com/docs/projects/iam/permissions)では、Hostingのカスタムロールは未対応である。専用SAに付与するのはDEVプロジェクトだけの`roles/firebasehosting.admin`だが、IAM上はDEV内の他Hostingサイトの作成・削除なども可能になる。サイト1個への限定は固定API・配備仕様と厳格なWIF受入検査で行い、IAMによるサイト単位制限とは呼ばない。API比較のための`mypage-api` 1サービスの`roles/run.viewer`と、利用枠用のDEV `roles/serviceusage.serviceUsageConsumer`だけを追加する。Cloud Run更新・Firestore・Auth・Secret・IAM変更・本番への権限は付与しない。

失敗時は、秘匿処理済みリリース記録のpreviousVersion/previousReleaseと現在の配信を照合し、明示的に復旧判断する。自動ロールバックや古い成果物の再利用はしない。Firebaseのリリース切替APIにはcompare-and-swapがなく、直前の一致確認とCI同時実行制御だけでは外部運用者の同時更新を完全に防げない。実行中は別経路のHosting更新を避ける。切替後の検証失敗は、実環境は更新済みでCIは失敗した状態として扱う。
記録は切替API呼び出し前にunknownで保存し、応答を確認できた場合だけconfirmedに更新する。応答消失時もpreviousVersionを残し、成否にかかわらずCI成果物で14日間保存する。公開ファイルと公開設定だけのビルド成果物は、1日で期限切れにする。

この経路はDEVフロントエンド専用である。本番・バックエンドイメージ更新・Terraform変更まで同時に自動化したとは扱わない。後続の本番経路は、公開条件・独立したEnvironmentと権限・配備対象をレビューしてから追加し、本番未公開を維持する。

### 静的配信キャッシュの仕様

同じ公開マニフェストでフロントエンドをビルドし、実distを専用リリースディレクトリ内の`dist/`へ固定コピーして、正確なアセットパスを採取する。`node mypage/scripts/prepare-hosting.mjs --manifest /path/to/reviewed-public-config.json --dist /path/to/release/dist --output /path/to/release/firebase.json`は設定候補だけを書き、配備しない。出力先は未作成ファイル、親ディレクトリは作成済みとする。Firebase CLIはプロジェクトルート外のpublicを拒否するため、シンボリックリンク解決後も、distは出力設定の親ディレクトリ内にある独立サブディレクトリでなければならない。マニフェストや非公開運用ファイルをdistに置かない。既知のハッシュ付きJS・CSS・フォントだけを1年間immutableとし、HTML・API・未知のハッシュパスはno-storeにする。ビルドを差し替えた場合は必ず再生成する。対象・サイトの対応は既存のレビュー済みDEV指定を使い、リポジトリ共通Firebase設定を上書きしない。

ビルド元は追跡済みで変更のないGit worktreeとし、同じ公開マニフェスト以外のVITE設定を混入させない。Git除外されたQAディレクトリ内のアーカイブをビルドルートにすると、Tailwindの自動ソース走査が変わるため使わない。JSだけの更新では、直前のレビュー済みdistとCSS・全フォントの名前とSHA256の集合を機械比較し、不一致なら配備前に停止する。意図したスタイル変更なら差分をレビューする。配備後は全ステージングファイルの本文SHAと実ヘッダーを再照合し、ハッシュ名の変化だけでビルドルートの問題を見過ごさない。

Hosting Emulatorで既知アセット、`/`、`/mypage`、`/api/mypage`、未知の`/assets/missing-12345678.js`のCache-ControlとCSP/nosniffを確認する。API書き換え先の実バックエンドヘッダー、実DEVの各ヘッダー、初回・再訪時のダウンロード量は別のリリース証跡とする。アセットキャッシュの変更でAuth・SAC・キャッシュTTLを変更しない。

### DEV専用：MyPageのFirestore読み取り原価計測

[原価モデルの前提](../cost-notes.md)に従い、対象リリースと予算を固定して計測する。収集フックのソースだけでは、実DEVの配備・計測・インデックス適用を証明しない。

- DEVの明示環境設定 `MYPAGE_READ_METRICS=true` の時だけ、BFFのCache Missから生じたFirestore snapshotの**成功したread-only transaction**ごとに `mypage_snapshot_read` を出力する。Productionではこの設定があっても無効。設定追加はDEV Cloud Run / Terraformの既存env allowlistとrelease承認を通して行う。
- 記録内容は `user_lookups`（UserDoc lookup数）、`seat_queries` / `seats_returned` / `seat_query_errors`、`segment_queries` / `segments_returned` / `segment_query_error`、`elapsed_ms` の数値・真偽値のみ。uid/channel ID、氏名、path、作業名、segment本文、OAuth/Cookie、エラー本文などの秘匿値を記録しない。
- `seat_query_errors` は通常席・メンバー席クエリの取得失敗数（0〜2）、`segment_query_error` は履歴クエリの取得失敗（true/false）。返却0件でも、対応する失敗数・フラグが0/falseである場合だけ正常な空結果として扱う。未実行は対応するquery数0で区別する。partial snapshotでquery失敗を捕捉してtransaction自体が成功する場合も、失敗指標を残す。transaction全体の失敗時には従来どおりこの観測callbackは呼ばれず、これを全エラーの集計器とは扱わない。文書のdecode不良・重複等はquery取得失敗とは別であり、既存のavailability判定を維持する。
- **この数値をFirestoreの実課金読み取り数と混同しない。** 返却0件のqueryの最低課金、index entry read、transaction retry、アクセス制御の別経路、既存AWS Batch等の利用は含まれない。実請求は選択GCP projectのCloud Billingと[Query Explain](https://firebase.google.com/docs/firestore/query-explain)を参照する。Explain Analyze自体も対象クエリのread料金が発生するため、小規模な合成データで実施する。
- 合成テスト対象：User不存在、履歴0/10/100/500/1000件（可能なら1001件検知）、同一IDの通常席/メンバー席、JST日曜→月曜・月末境界、coverage欠損、クエリ失敗と認証拒否。受け取った文書数だけでなく、失敗/上限時に履歴を0として表示しないことを確認する。
- 毎回の処理量（read/write/index read/CPU/latency）と1人・1日あたりのrequest回数を組み合わせて、実測原価を計算する。**現行のQuery `work-segments` は`ended-at`と`started-at`の複数の範囲条件を含むため、index entry readの請求をQuery Explain Analyzeで確認する。** 安全な値として`limit(1001)`があっても、同等数の課金readに収まる保証にはならない。
- 代表的な2時間滞在なら、定期refreshは旧60秒120回→新5分24回が**目標値**。手動操作/ログイン/再試行は別集計。複数インスタンスのヒット率、App CheckのreCAPTCHA評価回数、追加書き込み/キャッシュ維持費も別途算出する。
- writer は [削除 inventory の起動経路](../support-delete-inventory.md#起動と再生成経路の調査先)と既存の原記録writerを照合する。旧 daily を正本へ戻さず、Bot / Batch / Lambda の実 build・配備・drain を確認する。

### 月次履歴の明示移行と復旧

初期リリースは原記録を直接読む。既存のDEV専用CLIによる月次集計の切替はリリース手順から外す。DEVに残る派生値・マーカーの確認や個人情報削除では、原記録と派生値の両方を対象にする。Bot・Batch・Lambdaのwriter停止と完了待ちを含む既存の削除運用を参照する。

## 確認と承認の順序

| 確認段階 | 完了を示す証跡 | 未完時 |
| --- | --- | --- |
| MyPage Ready Gate | [infra正本Phase6](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc)のdev/prod基盤・ownership・Firebase境界・keyless CI | resource本実装/provisioningへ進まない |
| Phase 0A準備 | exact commit/CI、設定preflight、公開文面とAPI前同意、対象/rollback/実試験計画 | local prepareは継続。target値を推測しない |
| 0A実施承認 | 対象project/domain/revision、操作一覧、本人test accounts、infra/security証跡、秘密の供給方式 | live実行/公開/追加IAMを行わない |
| 0B受入 | 0A実経路とCanon S01/S04の検証PASS（必要なら局所修正後）、MONのDEV/PROD monitor認証・OBS永続化・失敗通知・max-seat回帰、COSTの実課金境界・Budget実着信・異常利用監視、D01実装証跡、D02両取消経路の規約適合性確認、D03実値、人的Privacy窓口、7暦日削除/cleanup、実metadata/log/backup inventory | 実ユーザー受入を停止 |
| Phase 1一般公開 | OAuth verification完了、0B実account/端末QA、Bot/外部導線、rollback、monitoring | 日付やCI成功だけで告知しない |

初期公開のプライバシー請求は[有人窓口と削除の運用](#人的privacyと削除の運用)を前提にする。アプリ内受付・返信UIはPhase 1対象外として撤去した。セッションなしの場合も同じ有人窓口で扱い、新しいBearer認証情報は導入しない。将来受付・返信UIを追加する場合は独立した設計・公開判断を必要とする。

### 人的Privacyと削除の運用

- D03で返信できるログイン不要の窓口、担当、受付時刻・期限管理、実公開文面を確定する。匿名の一般意見窓口やOAuth user support emailを自動的に流用しない。
- 依頼目的・環境・対象チャンネル・受付時刻を記録し、新たなOAuth認証を同じ依頼に結び付ける。削除の7暦日は受付時刻から固定し、証拠再発行・ACK消失・再試行で延長しない。
- 非公開CLIはowner-controlled ADC/GCP IAMへ接続する。実際の最小権限、返信到達、監査の保持・閲覧者、backup/vendor/logのGateを対象環境で確認する。CLI実装だけを運用完了と扱わない。
- [削除対象一覧](../support-delete-inventory.md)で実ランタイム・旧書き込み処理・Auth/BQ/キャッシュ/バックアップ/エクスポート/ベンダー/ログを確認する。D01保護、全実行系の停止・完了待ち、セッション失効、横断削除、復元や旧処理からの再生成防止、ストア別の不存在確認、完了通知まで実証する。部分失敗は保留のまま再開する。
- D02では、外部Google取消検知のためのリフレッシュトークン保存・定期取得を採用しない方針が確定している。アプリ内とGoogle設定画面の両取消経路の適合確認が終わるまで公開を保留する。新しい取消UI/APIも先に追加しない。

Canon S03のチャンネル譲渡は利用規約で扱い、申告時の[手動ブロック・セッション失効](../service-access-control.md)を維持する。自動の所有権監視や定期的な再認証は追加しない。既存moderation CLIのソース上の機能を、対象環境の権限・監査・実運用の確認済み証跡と混同しない。Firestore Client SDKの制限Rulesとモニターの同時適用は、[E2E](e2e.md)のMON受入で確認する。

通常のセッション失効で最大約1時間の残存を許容することを、削除時の全書き込み・キャッシュ停止の代替にしない。D01は独立したServiceAccessControlを使い、Aの保護処理と、Bの横断削除・実行中処理の完了待ちを区別する。

OAuth同意画面のuser support emailは、指定済みの専用値を安全な運用入力で渡す。プライバシー窓口・一般問い合わせ・developer contactに自動転用しない。公開本文は運営者の本名・自宅住所の常時掲載を前提にせず、必要な請求に回答できる運用を用意する。公開前に、返信可能でログイン不要のプライバシー窓口、担当・受付時刻と期限管理、本人テストアカウント・チャンネル、未指定のdeveloper contact、施行日・管轄を実値で照合する。

OAuthスコープは`youtube.readonly`だけ、コールバックは固定の`/api/auth/youtube/callback`とする。D02では、外部Google取消検知だけのためのリフレッシュトークン保存・定期取得を採用しない。アプリ内とGoogle設定画面の両取消経路、保持済みAPIデータの扱いについて検証・規約適合性確認が終わるまで、公開を保留する。設計確定や一般OAuth審査を、この確認の代替にしない。

## 証跡と停止条件

[リリース記録例](release-record.example.json)に、フロントエンドコミット・Hostingリリース、バックエンドイメージのダイジェスト・リビジョン、rewrite/pinTag、スキーマ・同意バージョン、試験範囲と参照、所有者承認を一組で記録する。例はD02の設計選択だけを確定済みとし、残る確認事項は保留のテンプレートである。入力しただけでは完了としない。非公開の対象情報・個人データ・認証情報を公開PRに載せない。

認証・IDORの問題、誤チャンネル、code/state漏えい、同意前GA4送信、旧データ復活、保持・削除期限違反、ロールバックの互換性不足があれば停止する。部分失敗はunknown/pendingとして記録し、再実行で失敗を消さない。セキュリティスキャンの実行結果とCI Gateを区別し、解析未開始を合格とは呼ばない。OIDC/IAM変更は、この手順書のコマンドに含めない。
