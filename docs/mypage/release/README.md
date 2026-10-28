# MyPage release preparation

メンテナンスモードの初期化、開始、解除、fail-closed時の復旧は[maintenance mode v1 runbook](../maintenance-mode.md)を参照する。これは通常release、Terraform apply、Hosting公開とは独立した手動操作であり、同runbookのworkflow以外からFirestore状態を直接変更しない。

このrunbookは未完の実経路Gate・本人判断を残したまま、コード・設定・試験を進める入口。DEVの2026-10-22時点の限定確認は[E2E実測記録](e2e.md#2026-10-22-devの限定確認)を参照する。設定検査や匿名拒否試験の成功は公開承認や実account E2Eの成功ではない。見た目・Phase 1のread-only scope・既存identity/data契約は維持する。

プロダクト方針の正本はCurrent Canonの[04 認証](https://app.notion.com/p/3ec357a8d0ce812f8520f787fedcc6bd)、[05 Privacy](https://app.notion.com/p/3ed357a8d0ce81e7b7d2cc85ec8ab4cf)、[06 Security](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634)、[09 判断記録](https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f)。実装・CI・配備状態はGitHubを正とし、設計の確定を実経路PASSへ読み替えない。wire/実装は[実装contract](../README.md)、[OpenAPI](../openapi.yaml)を参照する。本人への準備事項は[owner packet](owner-packet.md)、実経路の試験・release対・rollbackは[E2E手順](e2e.md)にまとめる。

2026-10-24の限定DEV配備対、上限付きFirestore測定、観測OFF、実409競合拒否、一時権限解除、正常0の認証smokeは[原価測定記録](../dev-cost-measurement-20261024.md)を参照する。表の公開Gate全体を完了した証跡ではない。

## 0. 2026-10-25 現行構成・公開前Gate（過去の手順より優先）

**現行の採用目標**：Development/ProductionともFirebase Hosting (React/Vite) + `/api/**` Cloud Run Go BFF rewrite + Firebase Authentication + Firestoreを継続。CloudflareはRegistrar/DNS onlyで、**Cloudflare HTTP Proxy/WAF、Cloud Armor、専用外部LB、Cloud Run Webは初期採用しない**。標準 `run.app` への直接到達は許容するが、認証・App Check・SAC・Cookie/state/Origin・IP/UID limiter・S01/S04を全経路で検証する。[Notion 07](https://app.notion.com/p/3ef357a8d0ce81b5892ac1dc2c99e587) / [06](https://app.notion.com/p/3ee357a8d0ce81038e27d0ad4c7eb634) を正とし、旧Managed WAF必須のチェック項目は履歴として扱う。

**Phase 0A** は運営者限定でE2E検証を続行可能。**Phase 0B** はURLを知る実ユーザーがアクセス可能となるため、次の受入を追加する。monitor認証と制限Rulesのrepository sourceは実装済みだが、以下のlive受入はすべてPENDINGであり、仕様記載・source・Emulator CIだけでGate完了とは扱わない。

- [ ] [monitor認証必須 #1459](https://github.com/kani3camp/youtube-study-space/issues/1459)：実装済みのOBS Interactログイン、local persistence、monitor claim + UID allowlist、制限Rulesを同じrelease pairとしてDEV/PRODへ段階適用する。repository Rulesは `seats` / `member-seats` / `menu` / `work-name-trend` / `config/constants` の匿名read/listを廃止し、専用monitor主体にのみ必要なreadを許可する。一般MyPage UIDや匿名・別projectは拒否、書込は禁止。
- [ ] OBS/CEFの実再起動・PC再起動・scene変更・long-running token refresh・権限失効時通知/復旧と、既存monitor seat-count制御の回帰を確認。まずDEV、その後ProductionのRulesとmonitorを整合させて段階適用。未対応Rulesだけ先にdeployしない。
- [ ] DEV min0/max1、PROD min0/max3、request-based、timeout/body/query予算をTerraformとGCP実値で照合。Cloud Run max instancesは料金hard capではない。
- [ ] **費用とabuse監視**：環境別にCloud Billing Budgetの多段階アラート・通知先・実着信を検証。Cloud MonitoringでRun request/instance/429/5xx、Firestore reads、YouTube quota、異常アクセスを監視。予算額・thresholdは正常時請求から決め、Budgetは自動停止ではないと明示。Cloud Run Spend Cap previewは利用可能性・同じprojectの他Cloud Run停止影響を確認するまで採用済み扱いしない。Firestoreは別管理。
- [ ] 既存S01（Hostingと`run.app`直接、IP偽装/429/誤制限）・S04（OAuth callbackのcode/state/Cookie/Tokenを全層ログで確認）・Auth/App Check/UID/IDOR・Privacy削除・D02/D03実値、release pair / rollbackを引き続きPASSにする。
- [ ] 実行結果は[公開受入 #1133](https://github.com/kani3camp/youtube-study-space/issues/1133)と[E2E matrix](e2e.md)へ対象environment、revision、時刻、証拠、未確認箇所を記録し、0B/一般公開を個別判定する。

## 1. 現在ある実装と、残る完了条件

DEV Backendのimage更新をCIへ移す経路は [Backend CI手順](backend-ci.md) を参照する。既存image-build→レビュー済みdigest promotion→CIでCloud Run更新/匿名smoke/限定State readbackの順序。専用identityの接続と実CI配備は同手順の実受入記録で判定する。

| 領域 | 既存実装・localで検証できること | 公開前に残ること |
| --- | --- | --- |
| OAuth / identity | [AuthService](../../../system/core/mypage/auth.go)、atomic claim/consume、channel確認、短命token、purpose分離 | 実Google account/Brand Account/channel選択・審査 |
| OAuth transaction TTL | [MyPage専用Terraform](../../../infra/mypage-oauth-ttl/README.md)、dev/prod定義、credential不要mock/限定plan contract | owner-approved target/state、別途承認したplan/apply、両環境のTTL ACTIVE・自然削除観測。applicationの10分失効は維持 |
| MyPage Cloud Run platform IaC | [専用Terraform](../../../infra/mypage-platform/README.md)はdev/prod別root。DEVの配信revision/固定digest、Hosting経由の匿名拒否を2026-10-22にread-only確認 | Production、完全なFE/BE release対、費用/rollback、S01/S04全層、Privacy公開Gateは未完了 |
| API / Firebase | [server assembly](../../../system/core/mypage/server.go)、keyless SDK、署名/claim、Origin、6 endpoints。DEVのApp Check欠落/不正・別Origin・直接Runを限定確認 | 有効App Check/Auth、複数account隔離、実OAuth/cookie、runtime signer/Firestoreの正常経路、緊急遮断/復旧 |
| metadata | refresh、正常なchannel不在のclear、29日cleanup、[定期呼出しrunner・durable監視・期限付きalert source](metadata-monitoring.md)、23時間heartbeat判定 | 実scheduler/独立watchdog/監視sink/alert宛先の結線、実inventory、自然実行 |
| 作業集計 | Firestore snapshot/readTime、JST境界、小数秒fixture、欠損/partial | history coverage・writer atomicity・query indexの実証跡 |
| ServiceAccessControl | [MyPage guardとtrusted moderation CLI](../service-access-control.md)、独立reasonとtransaction/checkpoint、合成QA | 実environmentの権限・runtime・運用結線、全runtime drainと横断削除B |
| support | request/purpose/channel binding、fresh proof、opaque参照、[demo限定dry-run](../../../system/cmd/mypage-operator-dryrun/main.go)、[無効のアプリ内受付](../privacy-intake.md) | 初期公開は返信可能な人的Privacy窓口と既存fresh OAuth / private CLI / Runbookの実運用。実連絡先・担当・期限管理・返信deliveryは未確定 |
| 削除 / operator実行 | [削除workflow・offline CLI](../support-delete.md)と[trusted operator source](../support-operator.md)を合成検証 | live adapter / registry / CLI bootstrapなし。`--execute`拒否を維持。全store横断・復元/再生成防止・完了確認の実証跡 |
| public policy | anonymous routes、明示同意、公開情報差込とdraft文面 | 運営値・実送信/保持との照合、本文承認、D02の規約適合性確認 |
| GA4 | consent世代、同意前非送信、撤回/cross-tab、safe固定UTM、loader。未接続時は同意バナーを非表示にし、旧v1選択を引き継がない | Firebase既存のMyPage用DEV/本番プロパティ・Webストリームを採用済み。環境別Measurement IDのbuild設定反映、Console/全tag inventory・実Network・保持設定 |
| Hosting | [pure候補](../../../mypage/src/hosting.ts)、明示CSP inventory、API優先rewrite、asset検査 | 所有target、実SDKのinventory、callback/access-log、実security headers |
| Bot入口 | 正本で `!app`/alias・固定public入口/UTMを決定済み。設定・受入条件は本runbookで準備 | runtimeコマンド接続は本PRに含めない。別の実装scope承認、Phase 1の実URL設定・live導線/流入確認 |
| 表示・操作 | visual/runtime/privacy/font/bfcache合成QA | 実端末・実account差。受容済みMyPage見た目を再設計しない |

support proofやdry-runは削除実行ではない。古い `mypage-users` mapping向け運用コードを、channel UIDの現在契約へ確認なしで流用しない。metadata cleanupは定期呼出しcallback・durable monitor・期限付きJSON health sinkのsourceまでで、実schedulerや監視宛先が作成済みとは扱わない。`mypage-server` の起動もkeyless credential、Firestore client、OAuth provider、HTTP listenに依存し、現行sourceや配置templateだけでは稼働中のdev Hosting→Cloud Run経路を証明できない。[2026-10-22のread-only確認](e2e.md#2026-10-22-devの限定確認)は既存DEVの匿名拒否と非機密metadataだけであり、[S01/S04実経路手順](e2e.md#canon-s01s04-実経路セキュリティ検証)の複数回線・全層ログや実OAuthはpending。deployやIAM/ログ設定変更は行っていない。

### GA4の環境別セットアップ（Firebase既存MyPageプロパティ採用済み・実送信未有効）

MyPageのGA4は、各Firebaseプロジェクトに既存でリンクされている **`mypage` 用GA4プロパティ**（各Webデータストリーム1つ）を使う。`youtube-monitor` 用プロパティはMyPageでは使用しない。Measurement IDは **DEV `G-E31NHW939V` / PROD `G-ZJ1VPVT2L2`** を採用する。ストリームURLは承認済みの環境別public originと整合させ、開発用アクセスを本番集計へ送らない。ID採用は確定済みだが、環境別build設定への反映・Console設定の受入・実Network/Realtime検証が終わるまではGA4実送信の有効化完了とは扱わない。

- 両プロパティで広告連携、Google Signals、広告パーソナライズを使用せず、保持14か月・新規アクティビティによる保持期間reset OFFを確認する。SPA側で手動page viewを送るため、GA4のEnhanced Measurementのhistory由来自動page view等による重複・余計な送信を無効にする。
- `VITE_MYPAGE_ANALYTICS_READY=true`、`VITE_MYPAGE_GA4_ID=<その環境の測定ID>`、`VITE_MYPAGE_PUBLIC_ORIGIN=<その環境のHTTPS origin>` を環境別frontend build設定に与える。測定ID自体は公開値でありsecret扱いしない。公開manifestの整合性検査とHosting CSPの別承認が必要。
- 未接続時は初回の同意バナーを表示しない。既存のv1同意設定は、接続前に取得した選択なので再利用しない（v2へ更新）。GA4を有効化した最初のアクセスで、ユーザーへGoogle Analytics・Cookie等を使うサイト改善のためのアクセス解析を案内し、明示的に同意/拒否を選んでもらう。
- DEVから先に、同意前・拒否中のGA4 network request 0、許可後の実イベント/URLのサニタイズ、拒否への変更後の非送信とCookie、Realtime表示、CSP・外部送信表示を実ブラウザで確認する。ローカル合成テストではConsole/実vendorの証明にならない。DEVの検証結果をProductionの検証に読み替えない。

## 2. 外部接続なしの設定preflight

public設定は[Vite/Firebase/GA4](../../../mypage/src/firebase.ts)、server設定は[entrypoint](../../../system/cmd/mypage-server/main.go)が使う既存契約へ揃える。設定値の元は選択済みproject/appの管理画面・承認済みinventoryであり、例の値を実projectへ流用しない。

```sh
# repository root: explicit public-only JSONだけを読む。SDK/ADC/通信なし。
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
# private manifestをstdinで渡してもよい。server環境全体をexportしない。
node mypage/scripts/check-release-config.mjs --stdin < /path/to/private-public-config.json

# system: 現在processの設定を既存validatorで確認する。ADC/listen/通信なし。
cd system
go run ./cmd/mypage-server --check-config
```

backend検査だけはserver-only secretを既存の安全なprocess環境から読む。コマンド引数やmanifest/Viteへsecretを移さない。出力は固定status/field/codeのみ。public checkerは64KiB以内の明示JSONを読み、秘密・debug/未知field、project/app/origin/同意版の不一致を拒否する。optional backend companionは非秘密の識別・版だけ、optional Hostingは既存candidateのvalidatorを再利用する。

manifestのtop levelは `schemaVersion: 1`, `deployment`, `frontend`、任意の `backend`, `hosting`, `publicPolicy`。`deployment`/`backend`は `environment`, `projectID`, `projectNumber`, `webAppID`, `publicOrigin`, `privacyVersion`, `termsVersion` の7項目。`frontend`の正確なkeyは[例](public-config.example.json)と[public schema](../../../mypage/src/release-config.ts)を使い、server環境をコピーしない。`publicPolicy`は `VITE_PUBLIC_CONTACT_URL`, `VITE_PUBLIC_PRIVACY_CONTACT_URL`, `VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE`, `VITE_PUBLIC_TERMS_EFFECTIVE_DATE`, `VITE_PUBLIC_JURISDICTION`、任意の `VITE_PUBLIC_OPERATOR_NAME`。`hosting`は[既存HostingInputs](../../../mypage/src/hosting.ts)の明示inventoryを使う。optional入力の省略は `not-supplied` として未検証のまま残す。

両検査とも外部登録・keyの真正性・permission/quota・公開本文承認を確認しない。`configurationValid=true`/`configuration=valid`でもinfrastructure/live E2Eはpending。`MYPAGE_INFRASTRUCTURE_READY=true`は起動に必要な宣言であり、Gateの証拠ではない。prepare時の検査のためにlive起動・Ready flag変更を行わない。

### 設定の受け渡し

Hosting image inventoryにはチャンネル画像用の `https://yt3.ggpht.com` と `https://yt3.googleusercontent.com` の両originを明示する。reCAPTCHA用画像だけのinventoryはpreflightで拒否する。許可元は[Hosting候補](../../../mypage/src/hosting.ts)と[設定検証](../../../mypage/src/release-config.ts)で検査する。これらはimg-src専用で、script/connect/frameを広げない。将来別の画像originが返る場合は実配信元を確認し、inventoryを再レビューする。

| boundary | 項目 | 受け渡し・検証 |
| --- | --- | --- |
| public browser | `VITE_FIREBASE_API_KEY/AUTH_DOMAIN/PROJECT_ID/APP_ID`, `VITE_APP_CHECK_SITE_KEY` | 公開Web app設定。secret/admin key/debug tokenではない。project番号とapp、登録domainは別途照合 |
| 同意版 | browser `VITE_PRIVACY_POLICY_VERSION/VITE_TERMS_VERSION`; server `MYPAGE_PRIVACY_VERSION/MYPAGE_TERMS_VERSION` | 同じrelease対で一致。実際に承認された本文の版を記録 |
| origin | `MYPAGE_PUBLIC_ORIGIN`; analytics用 `VITE_MYPAGE_PUBLIC_ORIGIN` | environmentの固定HTTPS origin。Host/Originをclient headerから推測しない |
| server identity | `MYPAGE_ENVIRONMENT/REGION/PROJECT_NUMBER/WEB_APP_ID/SIGNER_EMAIL`, `GOOGLE_CLOUD_PROJECT` | development=asia-southeast2、production=asia-northeast2。signerは選択projectのもの |
| server secret | `MYPAGE_OAUTH_CLIENT_ID/CLIENT_SECRET`, optional `MYPAGE_YOUTUBE_API_KEY` | OAuth secret/API keyは承認済みsecret供給境界。repo/chat/public artifact/Viteに入れない |
| optional Analytics | `VITE_MYPAGE_ANALYTICS_READY`, `VITE_MYPAGE_GA4_ID` | 実Console・network inventoryが承認されるまで有効化しない。Consentを迂回しない |
| 公開案内 | `VITE_PUBLIC_CONTACT_URL/PRIVACY_CONTACT_URL`, optional `VITE_PUBLIC_OPERATOR_NAME`, `VITE_PUBLIC_PRIVACY_EFFECTIVE_DATE/TERMS_EFFECTIVE_DATE/JURISDICTION` | 公開に使ってよい値だけ。実名・自宅住所を必須にしない。欠落/不正は未設定、draftは維持 |
| メンテナンス告知 | source内のDEV/PROD YouTube posts URL対応表、選択keyは`VITE_FIREBASE_PROJECT_ID` | 非secretの固定公開導線。Firestore・Backend・browser storageへ複製せず、未知projectはfallbackしない。変更はsource reviewと再buildを必須にする |
| Bot設定案 | `MYPAGE_PUBLIC_ENTRY_URL`（runtimeへの接続は未実装） | 承認済みpublic入口だけ。user固有query/tokenを含めない。0A/0Bは未設定、Phase1承認後に設定。実装・設定の承認前に有効化しない |

publicPolicy入力は一般問い合わせとPrivacy請求を分ける。形式が正しいURLでも返信機能・ログイン不要・SLA運用が確認済みとは限らない。

## 3. 再利用するlocal acceptance

依存は各packageのlock/toolchain設定を使う。秘密や実ユーザーdataは不要。以下はrepository-nativeの確認で、real OAuth/Hostingを代替しない。

```sh
# root
python3 .github/scripts/check-mypage-contract.py
python3 .github/scripts/check-doc-references.py
node mypage/scripts/check-release-config.mjs --file docs/mypage/release/public-config.example.json
```

各packageの確認は、それぞれrepository rootから開始する。

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

Go変更の必須checksは[AGENTS.md](../../../AGENTS.md)。生成後の意図しない差分0を確認する。Firestore依存のacceptanceは[既存Emulator script](../../../.github/scripts/run-firestore-integration-tests.sh)をrootから実行する（demo project、real credentialを渡さない）。

browserはlocal Viteをport18081で起動し、[runtime](../../../mypage/scripts/runtime-qa.py)・[privacy](../../../mypage/scripts/privacy-qa.py)・[visual](../../../mypage/scripts/visual-qa.py)・[font](../../../mypage/scripts/font-qa.py)を使う。各scriptは外部requestを遮断する。実bfcacheは[専用static build](../../../mypage/scripts/bfcache.vite.config.ts)と[既存QA](../../../mypage/scripts/bfcache-qa.py)を使い、dev HMRを証拠にしない。設定済みpublic文面は専用testとbrowserで匿名到達/安全なlinkを確認する。実ユーザー画面をpublic QA artifactへ保存しない。

### DEV HostingをCIで配備する（2026-10-25）

基盤のTerraform Applyとページの配備は別の操作だが、DEVページの更新も既存`CI`の手動実行から完結させる。`integration/mypage-canon-20261006`を選び、`operation=mypage-hosting-release` / `environment=development` を指定する。実行SHAは選択refの `github.sha` を使い、手入力しない。PR実行・main・他branch・別owner・再実行・SHA不一致は拒否する。失敗時にRunの再実行で継続せず、現在状態を確認して新しい実行を開始する。

1. 既存Repository/Frontend検査に成功後、cloud credentialのないBuild jobで固定依存の検査・test・DEV公開設定の検査・buildを行う。
2. 同じrunのartifactを別Apply jobへ渡し、source SHA/run ID/30分期限/configと全fileのSHA256を再検証する。Applyは専用WIFと短命tokenを使用する。SA keyやFirebase login tokenは使わない。
3. 対象は`test-youtube-study-space`の`test--study-space`だけ。現行HostingのCSP/rewrites等とAPIの同意版・固定origin・Ready revisionを照合し、configの意図しない変更、別site、途中のHosting/API/branch変更を拒否する。公開版を作成・upload・finalizeしてから、最後にreleaseを切り替える。writeの自動retryはしない。
4. 配備後は全fileのbytes/Cache-Control/CSP等、`/`と`/mypage`、匿名APIの403、API metadata不変を確認する。別のcredentialなしBrowser jobで匿名画面・Cookie設定・GA4 request 0を確認する。失敗はCI Gateへ伝播する。実GA4送信・認証済みE2E・公開Gateの完了を意味しない。

GitHub Environment `mypage-dev-hosting`には、**公開項目だけ**の`MYPAGE_HOSTING_PUBLIC_CONFIG_JSON`と専用provider名`MYPAGE_HOSTING_WIF_PROVIDER`をVariablesとして設定する。前者は既存のpublic manifest schemaで、`VITE_MYPAGE_ANALYTICS_READY=false`、`VITE_PRIVACY_INTAKE_ENABLED=false`、`analyticsApproved=false`を必須とする。server envのexport・OAuth/credential/Secret値を渡さない。公開済みDEVと同じorigin/Firebase app/同意版/CSP inventoryを引き継ぐ。設定変更は既存preflightとレビューを通す。

権限bootstrapは専用`mypage-hosting` WIF pool/providerと`mypage-hosting-deploy` SAへ分離する。providerは固定repo/owner/actor ID、初回workflow_dispatch、integration ref、caller `ci.yml`、reusable `mypage-hosting-authenticated.yml`、同一workflow SHA、Environment `mypage-dev-hosting`を全て検査する。既存のTerraform WIF/SA/Stateは変更しない。

**IAMの限界**：[Firebase公式](https://firebase.google.com/docs/projects/iam/permissions)ではHostingのcustom roleは未対応。DEV projectだけの`roles/firebasehosting.admin`を専用SAへ付与するため、IAM上はDEV内の他Hosting siteの作成/削除等も可能な権限を含む。site1個への限定は固定API/配備契約と強いWIF admissionで行うもので、IAMによるsite単位制限とは呼ばない。API比較のため`mypage-api`1 serviceの`roles/run.viewer`と、quota用のDEV `roles/serviceusage.serviceUsageConsumer`だけを加える。Cloud Run update、Firestore、Auth、Secret、IAM setter、Productionへの権限は付与しない。

失敗時はsanitized release recordのpreviousVersion/previousReleaseと現在配信を照合し、明示的に復旧判断する。自動rollback・古いartifactの再利用はしない。Firebaseのrelease切替APIにcompare-and-swapはなく、直前の一致確認とCI concurrencyは外部operatorの同時更新を完全に防ぐものではない。実行中は別経路のHosting更新を避ける。切替後に検証が失敗した場合は、live更新済み・CI失敗として扱う。recordは切替API呼出し**前**にunknownで保存し、応答を確認できた時だけconfirmedへ更新する。応答喪失時もpreviousVersionを残し、成功/失敗にかかわらず14日間のCI artifactへ保全する。publicファイルと公開設定だけのbuild artifactは1日で期限切れにする。

この経路はDEV frontend専用。Production、Backend image更新、Terraform変更を同時に自動化したとは扱わない。後続のProduction経路は公開Gate・独立Environment/権限・配備対象をレビューしてから追加し、本番未公開を維持する。

### 静的配信cacheの契約

同じpublic manifestでFrontendをbuildした後、実distを専用release directory内の`dist/`へ固定コピーし、exact asset pathを採取する。`node mypage/scripts/prepare-hosting.mjs --manifest /path/to/reviewed-public-config.json --dist /path/to/release/dist --output /path/to/release/firebase.json` は設定候補だけを書き、deployしない。出力先は未作成file、親directoryは作成済みとする。Firebase CLIはproject root外のpublicを拒否するため、symlink解決後もdistは出力configの親directoryの内側にある独立subdirectoryを要求する。manifestやprivate運用fileはdistへ置かない。既知hash付きJS/CSS/fontだけが1年immutableとなり、HTML・API・未知hash pathはno-store。buildを差し替えたら必ず再生成する。target/site対応は既存のreview済みDEV selectorを使い、repository共通firebase設定を上書きしない。

build元は追跡済みのclean Git worktreeとし、同じpublic manifest以外のVITE設定を混入させない。ignoredなQA directory内のarchiveをbuild rootにするとTailwindの自動source走査が変わるため使用しない。JSだけの更新では、前のreview済みdistとCSS/全fontの名前・SHA256集合を機械比較し、不一致ならdeploy前に停止する。style変更が意図的ならその差分をレビューする。deploy後は全staged fileのbody SHAと実headersを再照合し、build rootの問題をhash名の変化だけで見過ごさない。

Hosting Emulatorで既知asset、`/`、`/mypage`、`/api/mypage`、未知`/assets/missing-12345678.js`のCache-ControlとCSP/nosniffを確認する。API rewriteの実Backend headers、実DEVでの各headerと初回/再訪download量は別のrelease証跡とする。asset cacheでAuth/SAC/cache TTLを変更しない。

### DEV専用：MyPageのFirestore読み取り原価計測

設計候補は[Notion D06](https://app.notion.com/p/3f0357a8d0ce81a5b0e4e108231bda0f)。データソースの正本は[03｜統計](https://app.notion.com/p/3ec357a8d0ce81ac94eef9e6ac0c9964)。収集フックのsource実装だけでは、**実DEVでのデプロイ・計測・index適用を行ったことにはならない**。2026-10-24に実施した限定配備・直接SDK測定と、その範囲外のGateは[実測記録](../dev-cost-measurement-20261024.md)を参照する。

- DEVの明示環境設定 `MYPAGE_READ_METRICS=true` の時だけ、BFFのCache Missから生じたFirestore snapshotの**成功したread-only transaction**ごとに `mypage_snapshot_read` を出力する。Productionではこの設定があっても無効。設定追加はDEV Cloud Run / Terraformの既存env allowlistとrelease承認を通して行う。
- 記録内容は `user_lookups`（UserDoc lookup数）、`seat_queries` / `seats_returned` / `seat_query_errors`、`segment_queries` / `segments_returned` / `segment_query_error`、`elapsed_ms` の数値・真偽値のみ。uid/channel ID、氏名、path、作業名、segment本文、OAuth/Cookie、エラー本文などの秘匿値を記録しない。
- `seat_query_errors` は通常席・メンバー席クエリの取得失敗数（0〜2）、`segment_query_error` は履歴クエリの取得失敗（true/false）。返却0件でも、対応する失敗数・フラグが0/falseである場合だけ正常な空結果として扱う。未実行は対応するquery数0で区別する。partial snapshotでquery失敗を捕捉してtransaction自体が成功する場合も、失敗指標を残す。transaction全体の失敗時には従来どおりこの観測callbackは呼ばれず、これを全エラーの集計器とは扱わない。文書のdecode不良・重複等はquery取得失敗とは別であり、既存のavailability判定を維持する。
- **この数値をFirestoreの実課金読み取り数と混同しない。** 返却0件のqueryの最低課金、index entry read、transaction retry、アクセス制御の別経路、既存AWS Batch等の利用は含まれない。実請求は選択GCP projectのCloud Billingと[Query Explain](https://firebase.google.com/docs/firestore/query-explain)を参照する。Explain Analyze自体も対象クエリのread料金が発生するため、小規模な合成データで実施する。
- 合成テスト対象：User不存在、履歴0/10/100/500/1000件（可能なら1001件検知）、同一IDの通常席/メンバー席、JST日曜→月曜・月末境界、coverage欠損、クエリ失敗と認証拒否。受け取った文書数だけでなく、失敗/上限時に履歴を0として表示しないことを確認する。
- 毎回の処理量（read/write/index read/CPU/latency）と1人・1日あたりのrequest回数を組み合わせて、実測原価を計算する。**現行のQuery `work-segments` は`ended-at`と`started-at`の複数の範囲条件を含むため、index entry readの請求をQuery Explain Analyzeで確認する。** 安全な値として`limit(1001)`があっても、同等数の課金readに収まる保証にはならない。
- 代表的な2時間滞在なら、定期refreshは旧60秒120回→新5分24回が**目標値**。手動操作/ログイン/再試行は別集計。複数インスタンスのヒット率、App CheckのreCAPTCHA評価回数、追加書き込み/キャッシュ維持費も別途算出する。
- writer inventory：`workspaceapp/command_seat.go`、`workspaceapp/utils.go`、`workspaceapp/batch.go`、`workspaceapp/command_moderation.go` と中央の`PrepareWorkSegmentWrites`/commitをPR #1442で接続し、月次rollupとrawのatomicity・重複・rollbackをCI/Emulatorで検証した。旧dailyを正本へ復活させない。source実装の確認と全runtimeへの実配備は区別する。DEVには常駐Botがないとownerが確認し、既存Batch/Lambdaの固定imageは監査できたが、現sourceのbuild/deploy receipt照合と切替は別Gate。

### 月次履歴の明示移行と復旧

`cmd/mypage-history-rebuild`はDEVまたはloopbackのdemo Emulatorだけを対象とするoperator CLI。通常APIから呼ばない。Production、暗黙project、credential file、未知引数、4KiB超のstdin、上限超過、未レビューexecuteを拒否する。target UIDをコマンド引数・logへ載せず、制限した手元のJSONをstdinへ渡す。出力は件数・秒・版hash・checkpoint等だけで、入力/reportを実ユーザーの公開artifactへ保存しない。

1. source/CI/全writerの配備・旧fleetとqueueのpause/drain・既存coverage・費用上限を確認する。**mixed writer中にbaseline completeや切替を宣言しない**。MyPage APIのimage更新だけでBot/Batchのwriter切替済みとは扱わない。初回の合成DEV検証は独立したsynthetic UIDだけとし、既存利用者を変更しない。
2. `--operation=rebuild`（既定dry-run）で対象UIDの原記録を上限付きで調べ、指定JST月の値と旧値を比較する。stdinは`uid/month`、継続時は**最初の**`checkpoint`も指定する。既定上限1000、hard max10000、sentinelの追加1件、取得payload 4MiB、transaction 30秒・attempt 1。日付filterで不正rowを見落とさないようUIDの原記録を検査し、上限・不正・重複・失敗なら公開しない。大きいUIDは未切替のままとし、別途承認・レビューした移行が必要。
3. 同じ`checkpoint`とdry-runの`sourceVersion`をstdinへ加え、`--execute`で指定月だけを原子的に公開する。原記録/既存月/停止状態が変われば拒否する。月が再開単位で、同じ原記録を何度再集計しても足し込みにならない。ACK喪失時は旧versionを自動再実行せず、original checkpointを維持した新dry-runで現状・epoch・差分を確認する。削除を跨いだcheckpointの再採取は禁止。
4. 直近7日が触れる月を準備した後、`--operation=cutover`のdry-runで同じcheckpointと`userRevision`を確認し、それを渡した`--execute`でUserの読取markerを切り替える。SAC/User revision、月のbaseline/epoch/整合性、future原記録の不存在を再照合する。coverageの設定は別の確認事項であり、この操作で古い不完全履歴を完全としない。
5. 新旧比較ではJSTの日/月/週境界、秒未満、作業名変更、break、ongoing、正常0、1001件以上、欠損/不正/取得失敗を確認する。旧raw readerの1001件到達は正確な比較値にならないので、既知の合成期待値と月次値を照合する。
6. 退避には`--operation=rollback`を現在User revisionと**同じoriginal checkpoint**でdry-run→executeする。rawは保持し、読取markerだけを外す。旧writerへ戻す場合は先に全対象のmarkerを外し、再導入時はfleet/drain確認と新dry-run/再集計を行う。metadataだけ取り直して旧planを通さない。

repository rootから対象を確認し、system directoryで使用する例（実行には対象ごとの承認済み入力・費用境界が必要）:

```sh
go run ./cmd/mypage-history-rebuild --environment=development \
  --project=test-youtube-study-space --operation=rebuild --max-documents=1000 \
  < /path/to/private-reviewed-selection.json
```

新しいclose writerとrebuildはprivacyDeletionをtransactionで確認し、SAC解除後も旧admission/checkpointの再生成を拒否する。genericなBot moderation/入室/User更新まで共通停止にしたわけではないため、横断削除のlegacy pause/drainは引き続き必要。削除adapter/inventoryは原記録と`work-months`を同じprimary scopeで消し、UserのmarkerもUser削除に含む。inactive SAC checkpoint・独立moderationは保持する。cache/flight・backup/export/restoreまで含むBの完了条件は従来どおりで、このCLIをPrivacy削除の実行経路へ転用しない。

## 4. Gateと承認の順序

| checkpoint | 完了を示す証跡 | 未完時 |
| --- | --- | --- |
| MyPage Ready Gate | [infra正本Phase6](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc)のdev/prod基盤・ownership・Firebase境界・keyless CI | resource本実装/provisioningへ進まない |
| Phase 0A準備 | exact commit/CI、設定preflight、公開文面とAPI前同意、対象/rollback/実試験計画 | local prepareは継続。target値を推測しない |
| 0A実施承認 | 対象project/domain/revision、操作一覧、本人test accounts、infra/security証跡、秘密の供給方式 | live実行/公開/追加IAMを行わない |
| 0B受入 | 0A実経路とCanon S01/S04の検証PASS（必要なら局所修正後）、MONのDEV/PROD monitor認証・OBS永続化・失敗通知・max-seat回帰、COSTの実課金境界・Budget実着信・異常利用監視、D01実装証跡、D02両取消経路の規約適合性確認、D03実値、人的Privacy窓口、7暦日削除/cleanup、実metadata/log/backup inventory | 実ユーザー受入を停止 |
| Phase 1一般公開 | OAuth verification完了、0B実account/端末QA、Bot/外部導線、rollback、monitoring | 日付やCI成功だけで告知しない |

初期公開のPrivacy請求は[人的窓口と既存部品の最小運用](owner-packet.md#privacy請求の受付準備)を前提とする。[アプリ内source package](../privacy-intake.md)の有効化は0Bの必須条件ではない。公開manifestの`VITE_PRIVACY_INTAKE_ENABLED`は現時点で`false`のみ受理し、server側も`EnablePrivacyIntake=false`を維持する。no-sessionも同じ人的窓口で扱い、新しいbearer credentialを導入しない。アプリ内受付・返信UIを後で使う場合は独立した公開判断が必要。

### 0B前の人的Privacy・削除最小運用チェック

- D03で返信できるログイン不要の窓口、担当、受付時刻・期限管理、実公開文面を確定する。匿名の一般意見窓口やOAuth user support emailを自動的に流用しない。
- 依頼目的・environment・対象channel・受付時刻を記録し、fresh OAuthを同じ依頼へ束縛する。削除の7暦日は受付時刻から固定し、proof再発行・ACK喪失・再試行で延長しない。
- 実operator identity/権限、private CLIのlive接続、返信のdelivery acknowledgement、監査の保持・閲覧者が未確定。現状の`--execute`拒否を運用完了へ読み替えない。
- [削除inventory](../support-delete-inventory.md)で実runtime・legacy writer・Auth/BQ/cache/backup/export/vendor/logを確認し、D01 guard、全fleet pause/drain、session revoke、横断削除、復元・古い処理の再生成防止、store別の不存在確認、完了通知まで実証する。部分失敗はpendingのまま再開する。
- D02は外部Google取消検知のrefresh保存・pollingを採用しない確定方針。アプリ内とGoogle設定画面の両取消経路について適合確認が終わるまで公開Gateを閉じる。新しい取消UI/APIも先行追加しない。

Canon S03のchannel譲渡は利用規約で扱い、申告時の[manual block / session revoke](../service-access-control.md)を維持する。自動ownership監視や周期再認証は追加しない。既存moderation CLIのsource機能を、対象environmentの権限・監査・実運用の確認済み証跡と混同しない。S02のFirestore公開範囲はこの文書整合PRの対象外。

正常session revokeで最大約1時間の残存を許容する既定を、削除時の全writer/cache静止の代替にしない。D01方式は2026-10-07 Canonの独立ServiceAccessControlに決定済み。AのguardとBの横断削除/drain完了を区別する。D02は2026-10-09に外部Google解除検知だけを目的とするrefresh token保存・定期pollingを採用しないと確定し、規約適合性と取消後の保持済みAPI dataの扱いをverification/complianceで確認する。

## 5. 証跡と停止条件

[release record例](release-record.example.json)にfrontend commit/Hosting release、backend image digest/revision、rewrite/pinTag、schema/同意版、試験のscopeと参照、owner承認をひと組で記録する。例はD02設計選択のみ確定済みとし、残るGateはpendingのtemplateで、入力しただけではGateを閉じない。private target/個人data/credentialをpublic PRに載せない。

認証/IDOR・誤channel・code/state漏えい・同意前GA4・古いdata復活・保持/削除期限違反・rollback互換不成立はSTOP。部分失敗はunknown/pendingとして記録し、再実行で失敗を消さない。CodeQLの既存OIDC custom-claim問題とGitHub CI Gateは区別し、解析未開始をsecurity scan PASSとは呼ばない。OIDC/IAM変更はこのrunbookのコマンドには含めない。
