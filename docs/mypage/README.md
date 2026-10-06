# MyPage implementation contract

現行仕様から実装に必要な非機密 wire contract だけを抽出したもの。`openapi.yaml` は six-endpoint API の契約、`fixtures/` は実ユーザーに依存しない合成例。

- 本人識別は YouTube channel ID と同じ Firebase custom-provider uid。client から対象 ID を受け取らない。
- OAuth callback の atomic claim と confirm の atomic consume の後に、外部 API / token mint を transaction callback の外で実行する。
- 現在・今日・月曜開始の週・生涯累計・直近7暦日は同じ snapshot / `asOf` に揃える。
- seconds を合計し、表示時だけ分に切り捨てる。取得障害・不完全履歴・矛盾は正常な0に置き換えない。
- account metadata と作業統計の部分障害を分離し、raw ID / credentials / DB path を response・log に載せない。

契約検証:

```sh
python -m pip install -r docs/mypage/contract-requirements.txt
python .github/scripts/check-mypage-contract.py
```

Frontend と backend は独立して検証する。provisioning / production deployment はこの実装 stack に含まない。

## Web authentication storage

`oauth-transactions/{opaque id}` と `web-accounts/{verified channel id}` を server-only に追加する。既存 rules の default deny により browser access は許可しない。10分の期限は application が判定し、TTL 設定・権限・cleanup の実環境設定は provisioning gate 後に別途行う。

Callback は `pending -> processing -> channel_verified`、confirm は `channel_verified -> consumed`。WebAccount upsert と consume は同じ transaction。firstWebLoginAt は authenticated session completion で set-if-absent。users / 作業履歴は web login によって変更しない。transaction record には state hash・同意版・公開 channel metadata・時刻だけを保存し、code / OAuth token / Custom Token は保存しない。

Google provider と Firebase token mint は interface として分離し、demo emulator では合成 provider を使用する。本番への結線・デプロイは未実施。

## Pure aggregation

`WorkSnapshot` は同じ Firestore snapshot から読む adapter との境界。snapshot read time がない入力は受理しない。`Coverage` は完全性を確認した期間だけを指定し、未設定なら登録済み利用者の履歴を `HISTORY_INCOMPLETE` とする。実環境の coverage 開始日・欠損範囲は未確認であり、推測設定しない。

現行 writer は退室時に `users.TotalStudySec` へ入室セッション分を転記する。そのため入室中の lifetime は `TotalStudySec + SeatDoc.CumulativeWorkSec + work 状態の経過秒`。履歴の ongoing 区間は `CurrentSegmentStartedAt` を使い、作業名変更による closed segment と重ねない。break 中もすでに完了した session work は lifetime に残す。

1001件・取得失敗・重複/未来/負の区間・coverage 欠損を正常な0へ変換しない。独立して正しい current / lifetime / account は保持する。この slice は pure calculation の検証であり、実 DB の read-only snapshot adapter と既存 writer の競合整合性検証は次の slice。

## Contract completeness

6 endpointはlogin/supportをpurposeで区別する契約。任意supportChallengeはserver recordへ照合し、support confirmはCustom Tokenを返さない。Frontend support flowとoperator lifecycleは独立した未完了条件。

## Snapshot adapter and writer ordering

User lookup の `DocumentSnapshot.ReadTime` を取得してから JST query window を決め、User / seats / member-seats / work-segments を同じ read-only transaction で読む。不存在Userでも readTime を維持し、seat/history queryは省略する。seatは各区分2件まで、historyは1001件までで打ち切る。

work-segmentsは user-id equality、ended-at > window start、started-at < asOf を用いる。必要な composite index の候補は `query-index.json`。これは未適用のquery要件であり、既存 firebase config / infrastructureを変更・provisionしたものではない。Ready Gate後のownership確認が必要。

既存 writer の identity/history query は `WorkspaceApp.RunTransaction` が渡すcontext内で同じtransactionへ参加する。外側のquery利用は従来どおり。これにより seat更新と並行する read→write が競合判定される。

移動では old segment close と new seat entry に同じtransition instantを使う。休憩のoriginal startは新seatのentryより前でも正当だが、current segmentはnew seat entry以降でなければならない。過去に記録された重複区間は推測修復・許容せず、影響するhistory metricを `DATA_INCONSISTENT`、独立したcurrent/lifetimeは利用可能な限り維持する。

## HTTP and memory cache

`HTTPHandler` は6つの通常login endpointのlibrary boundary。App Check、Firebase custom provider、WebAccount存在・accessBlocked・同意版を統計アクセス前に検証する。cache hitでもこのgateを毎回通す。opaque cookieはHttpOnly / Secure / Lax / Path=/、Domainなし。callbackは固定pathへredirectし、raw provider errorやdependency detailを公開しない。JSONは未知・重複・大小文字違いのfield、余分なquery/body、4096 byte超を拒否する。

aggregate cacheは環境+uidをkeyにprocess memoryで最大30秒、JST日境界を越えて再利用しない。singleflight内の処理は独自10秒budgetを持ち、最初のcaller切断が他callerの処理を止めない。generatedAtは元snapshotの値を維持する。失敗はcacheしない。metadataは24時間でrefresh対象、失敗時30日未満だけpartial、30日以上はunavailable。

rate limitはprocess単位のbounded memory。deploymentの全体limit、trusted proxy IP抽出、実provider/verifier/minter、server起動とHosting rewriteの結線は未実装。`accessBlocked`のread guardだけで全writer/cache/in-flight deletionのD01要件を満たしたとは扱わない。support purposeとprivacy運用も引き続き独立した完了条件。

HTTP handlerは環境ごとに固定したHTTPS `PublicOrigin` を必須とし、対象外Host、別origin/OriginなしのPOST、重複Originと重複`__session`をdependency検証・OAuth mutation前に拒否する。callback GETはOriginなしを許すが固定Hostの検証を省略しない。clientのX-Forwarded-Host / Proto / Forで許可先やdefault IPを変更しない。Hostingから届く実Hostの確認はdeployment gateに残す。

すべてのrouteにcoarse IP bucket（60/min、burst20）をverification前に置き、invalid App Check / ID tokenの連投も制限する。startの5/10min・20/hour制限もこの位置で行う。verified uid制限とatomic transaction consumeは別に維持する。

## Frontend private memory

`MyPageMemory` はuid変更/logout開始/pagehideで個人データを同期clearしてrequestをabortする。epoch照合によりabortを無視する遅延responseも破棄する。logout失敗でもデータは復活せず再試行可能。bootstrap中にlogin画面を出さない。

visible時だけ完了後60秒でpollし、hidden停止・復帰即refresh・request overlap防止・manual debounce・失敗backoff・Retry-Afterを実装する。401はデータを消して同uidで一度だけtoken refresh、App Checkだけは一度再取得し、継続401/再同意/WebAccountなしはsignOutへ戻す。取得不能section/metricは同uidの直前成功だけをmemory保持し、元asOfとstaleを残す。未取得値を0にはしない。

controllerはsession/API adapterを注入する。browser runtime sliceでFirebase listener、router、画面、bfcache eventへの結線を追加した。MyPage responseやmetadataをstorageへ保存する経路は持たない。

## Display and visual verification

React表示componentはcurrent / summary / recent7Days / account panelを持ち、snapshotの時刻だけでtimelineを描く。未取得値はunavailable、旧成功値は元asOf付き、休憩でもworkNameを維持。0時間の日もkeyboardで選択できる。native dialogのinertに加えてTab循環・Escape・avatarへのfocus復帰を検証する。最新仕様に合わせflat cream背景と通常cardのshadowなしを使う。

`mypage/visual.html` はdev専用の合成fixture入口で、production buildの入口に含めない。product runtimeの結線とは別に、画面コードを独立して実browserで確認する:

```sh
cd mypage
pnpm dev --host 127.0.0.1
# 別shell、system Chromiumが使える環境
python -m pip install -r visual-requirements.txt
python scripts/visual-qa.py --output /tmp/mypage-visual-qa
```

320/390/768/1024/1440px × 12状態、modal keyboard/logoutを合成fixtureで確認する。external requestは遮断し、利用者dataをartifactへ出さない。Approved exportのruntime/provenance gapは `design/README.md` に記録し、元runtimeとのpixel parity確認は未完了。外部fontは取得しておらずlocal fallbackでのQA。

`METADATA_TOO_OLD`は直前成功accountもclearして期限切れmetadataを表示しない。account panelで表示停止の理由を伝える。429 cooldownは共通deadlineに保持し、manual refresh/visibility復帰でも期限前にrequestしない。auth拒否後のsignOut失敗は`LOGOUT_FAILED`を維持し、retryを可能にする。cookie-nameの前後空白を正規化してGo cookie parserと同じ同名cookieを重複拒否する。


## Browser runtime and synthetic integration

通常loginのstart / channel / confirm / session-complete、Firebase custom sign-in、App CheckとID token付きsame-origin API、TanStack Routerを接続する。対象uidはFirebaseから読み、clientから対象channel IDを送らない。confirmは自動再送せず、Custom Tokenをmemory controllerやstorageへ保存しない。session-completeのtransient failureだけを最大3回試し、失敗時は当該uidをsignOutする。遅い旧session処理は新uidをsignOutしない。completion前は個人画面へのfetchを抑止する。

public Firebase設定とpolicy versionが揃わない場合はlogin unavailableとしてpublic pageだけを表示する。ViteへOAuth secret / admin credential / App Check debug tokenを設定しない。Firebase SDKの認証session persistence以外の個人metadata / 作業dataはmemory-only。Routerのscroll restorationを無効化しURLをkeyにしない。現在のRouter dependencyはpagehideで空のscroll cache `{}`をsessionStorageへ書くが、URL・challenge・metadataは入らない。

`runtime-visual.html`はdev専用の合成session/API/router入口。production buildには含めない。local Chromiumで以下を検証する:

```sh
cd mypage
pnpm dev --host 127.0.0.1 --port 18081
python scripts/runtime-qa.py --output /tmp/mypage-runtime-qa
```

確認→session complete→MyPage→logout、anonymous route guard、既存session、fresh login、support誤用途防止、期限切れtransaction、uid変更/遅延response、pagehide/pageshow、logout再試行、初期折り畳み、404、設定なしpublic pageを確認する。外部requestは遮断し、合成dataだけを使う。

support本人確認は準備中と明示し通常loginへ流さない。privacy / terms / contactとCookie設定は未完成のdraft UIであり公開承認版ではない。人間が決める運営者・窓口・公開URLを推測で入れない。実Google OAuth / Firebase App Check / Hosting経由のE2E、approved runtimeとのpixel parity、運用privacy/security、D01/D02/D03判断、infrastructure Ready Gate、実provider/verifier/server結線は別の未完了条件。production deploymentはしていない。


## Support record boundary

`FirestoreSupportStore`はoperator/server用libraryで、public管理APIではない。environment / request ID / 対象channel / purpose(delete・revoke・disclosure)を記録し、challengeはhashと24時間期限だけを保存する。同じenvironment+request IDの重複作成はtransactionで拒否する。challenge再発行は元の受付時刻と削除期限(受付から7暦日)を維持し、旧challenge indexを消す。期限超過後の再発行でSLAを延長しない。

SupportBindingはrecord reference / request ID / environment / purpose / challenge hashを保持するserver-only値。reissue前のbinding・目的違い・environment違い・対象channel違い・期限境界・proof replayは拒否する。proof成功はchallengeをconsumeするが、削除実行の承認ではない。完了recordへのpure transitionはchannel/challenge/proof/OAuth referenceを取り除き、最小の匿名監査用時刻を残す。

このsliceはrecord storageとpure state transitionまで。OAuth transactionとのatomic consume、confirm responseのpurpose union、support browser flow、operator CLI、完了時のOAuth record消去/retentionと実削除・revoke・開示実行は後続。D01削除guardを代替せず、既存users/seat/history/Auth userを変更しない。実環境でrecordを作成していない。callerのproject/credential/production確認はReady Gate後のoperator boundaryで実装する。


## Support OAuth atomic boundary

startの任意supportChallengeをserver recordへ解決し、同じFirestore transactionで有効bindingを確認してOAuth recordを作る。request ID/environment/purpose/challenge hashをOAuthへ固定する。channelは固定purposeとfresh OAuthで得たpublic metadataだけを返す。clientからtarget channel/purposeを受け取らない。

support confirmは同じtransactionでOAuth status/ref/同意版、record status/environment/request ID/purpose/旧challenge無効化/対象channel/期限を検証し、SupportRequestへproofを一度記録してOAuthとchallengeをconsumeする。独立した二つのfresh OAuth transactionからも同一依頼のproofは一回のみ。成功responseは`purpose=support`とopaque requestRefであり、Custom Token mint、WebAccount作成、Firebase session completion、ユーザーデータ削除は実行しない。normal consumeへの迂回もstoreで拒否する。OAuth内のchannel metadata/state/refはconsume時にclearする。

通常responseもpurpose=loginを明示する。OpenAPIはchannel/confirmのdiscriminated unionを検証する。frontendの通常login adapterはsupport responseを受け付けず、support目的を通常login画面で確認してしまう経路を閉じる。支援用UIは次slice、完了後のrecord/関連OAuth cleanupとoperator CLIは後続。D01/D02/D03/Ready Gateは継続して未完了。


## Support browser flow

support入口は既存Firebase sessionでも通常MyPageへredirectせず、fresh OAuthを開始する。不正なchallengeは専用の無効表示を維持し、通常loginへ切り替えない。serverから受け取ったpurposeと公開channel名を確認画面で示し、支援用buttonから確認する。successはopaque参照番号だけを受付画面のmemoryへ渡し、signIn/session-completeや削除実行を呼ばない。参照番号はroute離脱・pagehide・uid変更でclearし、URL/storageへ保存しない。client申告の目的・対象channelをAPIへ送らない。

product HTMLにno-referrerを指定し、support challenge等のqueryを外部遷移のRefererへ載せない。platform access log/CDN/Hosting設定の実確認はsecurity release gateに残す。支援用CLIと受付窓口の実値は未完了で、公開deployはしていない。


## Consent and security draft boundary

任意Analyticsはunset / granted / denied。unsetでnonblockingな選択を出し、拒否・未選択でも主要操作を使える。選択したgranted/denied文字列だけをfirst-party storageへ保存し、uid/channel/profileを結び付けない。Cookie設定から変更でき、storage eventによる別tabの拒否でも将来送信を停止する。保存に失敗してもcurrent-pageの選択を適用し、保存失敗を表示する。

AnalyticsGateは同意前にstart/sendせず、過去eventをqueue/replayしない。固定route/titleと完全一致した公開UTM組合せだけを渡し、callback、code/state/error/supportChallenge、任意query/fragment、referrer/title入力や本人dataを渡すAPIを持たない。通常loginの4 eventだけをparameterなしで扱う。GA4実sender/script/identifierは未接続であり、公開済みGA4 Network/console設定を検証したとは扱わない。mandatory Firebase/App Check/reCAPTCHA通信とは別の同意である。

public policy/termsは確認用draft。運営主体/窓口/施行日/管轄を推測で埋めず、D01/D02/D03と公開文面の運用照合を残す。production公開はしていない。API全responseにはno-store/nosniff/no-referrer、default-src none / frame-ancestors none、不要device permission拒否を付ける。Hosting HTMLのCSP allowlistとcallback platform access-log redactionは実inventory/Ready Gate後に確認する。

`privacy-qa.py`は設定なしlocalhostのpublic pageと同意UIを実Chromiumで確認する。storageには選択flagだけ、外部requestは遮断・0件。Cookie modalのTab/Escape/focus復帰、実same-origin別tab変更、320/390/1440pxのdraft policyを検証する。実Firebase必須通信/GA4 senderの証拠ではない。


## Support confirmation lifecycle

support confirmのpending requestはidentity・page lifecycle・確認操作の世代に結び付ける。logout、別uid、logout後の同一uid再login、pagehide、unmount、新しい確認操作は旧requestをabortし、transportがabortを無視して成功を返しても参照番号を受け取らない。React側でもreceipt設定と遷移の直前に当該controllerの有効性を確認する。pageshowは取り消したsupport確認を復活させない。

unit回帰は遅延successと各invalidating eventを確認し、synthetic Chromium QAはpending状態からuid変更・logout/relogin・pagehide/pageshow・route離脱後のreceipt非復活と遅いcontact遷移の抑止を確認する。証明済みのserver recordを取り消す操作ではなく、browserで失効した応答の表示を抑止する。


## YouTube OAuth provider adapter

`GoogleYouTubeOAuth`はserver-onlyのclient ID/secretと固定HTTPS originを受け取り、既存のatomic callback serviceへ接続できるadapter。authorizationはyoutube.readonlyだけ・online・select_accountとし、Google Account選択をYouTube channel pickerとは扱わない。code exchangeは1回で、grant scopeを検証してから同じ短命access tokenでmine=trueのchannelを取得する。channelなし・複数・次pageありは拒否し、唯一の公開metadataだけを返す。legacy custom URLを@handleと推測しない。

provider instance/store/responseにYouTube tokenを保存せず、refresh tokenを要求せず、token source/refresh/retryを使わない。callback originはrequestから作らず、外部redirectを追わず、処理全体を8秒に制限する。provider body/URL errorをallowlist errorへ変換しsecret/code/tokenを公開errorへ出さない。

wire request、scope不足、期限切れtoken、channel曖昧さ、metadata形式、応答size、redirect/cancelは合成HTTP transportで検証する。これは実Google接続の証拠ではない。実providerを使うserver、Firebase Auth/App Check verifier・keyless minter、公開metadataのAPI-key refresh、Hosting rewriteとsecurity設定の結線・Ready Gateは後続。

一次資料: [Google OAuth server flow](https://developers.google.com/youtube/v3/guides/auth/server-side-web-apps)、[channels.list](https://developers.google.com/youtube/v3/docs/channels/list)。


## Mockable GA4 command adapter

`GA4Sender`はAnalyticsGateからのみstartする同期command/disable portのadapter。constructorは識別子やbrowser globalを作らず、startで自動config pageviewと広告signalsを無効化し、固定originのroot URL・空referrer・空titleを指定する。event時にもsafe route/固定title/許可UTMを再検証し、任意title/追加parameter/User-IDを転送しない。stopは送信を止め、contextをclearし、runtimeのmeasurement disable flagを設定する。deniedへの変更で送信を誘発し得るgtag commandは呼ばない。同意前eventを再送しない。

このadapterはproductへ未接続で、gtag.js loader/dataLayer/vendor通信を含まない。mockはcommand引数とdisable順序を検証するだけで、実GA4の非送信を証明しない。実loaderでは同意の世代変更後に遅延load callbackを破棄し、queueを再送しないことが必要。ConsoleでEnhanced Measurementのhistory pageview等の自動collectionを無効化し、他tag/pluginを含めたinventoryを確認する必要がある。send_page_view=falseだけではhistory pageviewを防げない。14か月保持・保持reset OFFと、同意前/拒否後のNetwork検証はrelease gateに残る。

一次資料: [GA4 configuration](https://developers.google.com/analytics/devguides/collection/ga4/reference/config)、[manual pageviewsとhistory measurement](https://developers.google.com/analytics/devguides/collection/ga4/views)、[Consent mode](https://developers.google.com/tag-platform/security/guides/consent)。実GA4 Network/Console/自動event/再同意は未検証であり、公開deployはしていない。


## Confirmation return after bfcache

channel確認画面でpagehide/identity変更が起きたらpending確認をabortし、旧channelをclearして中断理由と既存の再開始導線を表示する。bfcacheでmounted画面へ戻っても待機表示を続けず、旧成功応答のreceipt復活・遷移を抑止する。通常loginはloginからやり直し、supportは受付窓口へ戻る。

`bfcache-qa.py`は実cross-document離脱→戻るでpageshow.persisted=trueを必須とし、通常/support × pending有無の4ケースで中断表示、再開始操作、旧応答の非復活を確認する。HMRを含むdev serverではなく、専用の合成静的buildで検証する。通常production buildの入口とは独立しdeployしない。

```sh
cd mypage
pnpm exec vite build --config scripts/bfcache.vite.config.ts
python -m http.server 18082 --bind 127.0.0.1 --directory /tmp/mypage-bfcache-qa
# 別shell
python scripts/bfcache-qa.py
```


## Firebase verification boundary

FirebaseBoundaryは公式Admin SDKのVerifyIDToken/CustomToken portを使い、署名検証済みtokenについて固定projectのissuer/audience、custom provider、tenantなし、channel形式のUID/subject一致、期限/発行/認証時刻を追加検証する。SDK/token errorを公開messageやlogへ転記しない。mintはserverが確認したchannel UIDだけを渡す。

AppCheckVerifierは公式のJWT検証手順に沿ってRS256署名・JWT type・固定project numberのissuer/audience・当該Web App ID・期限/発行時刻を検証する。JWKSは固定v1 endpointから遅延取得し、最大6時間cache、処理2秒・size/key数制限、未知kid/取得失敗のrefresh間隔制限とsingleflightを持つ。constructorは通信/background loopを作らない。SDKのeager AppCheck clientを使わず、request contextと合成HTTP transportで検証できる境界を採用した。

合成RSA署名でwrong issuer/audience/app、期限ちょうど、future issue、不正claim type、署名不一致、algorithm/type/kid、並行cache/rotation、上流失敗/cancelを検証する。Firebase Auth側はSDK portをfakeで検証しており、実Firebase公開鍵/ID tokenを検証した証拠ではない。SDK keyless constructor/server結線、IAM Ready Gate、実App Check・Auth E2Eは後続で、credential/IAMを変更していない。

一次資料: [App Check custom backend verification](https://firebase.google.com/docs/app-check/custom-resource-backend)、[Firebase ID token verification](https://firebase.google.com/docs/auth/admin/verify-id-tokens)、[custom token signing](https://firebase.google.com/docs/auth/admin/create-custom-tokens)。


## Server and keyless SDK assembly

`NewMyPageServer`で固定environment/project/app/origin、FirebaseBoundary、OAuth provider、Firestore Auth/Support store、read-only snapshot/BFFを6 endpointへ接続する。constructorはsecret/credential取得・外部通信・listen・通知を実行しない。coverageは検証済み範囲だけを注入し、未設定のhistoryを0へ変換しない。

`NewKeylessFirebaseClient`は既存ADCのtoken sourceと明示した同一projectのsigner emailを受け取り、credential JSON/key file、Auth emulator、project不一致を拒否する。公式SDKへServiceAccountIDを指定し、IAM remote signingを選ぶ。合成IAM transportでsignBlob requestをRSA署名し、SDKのCustom Tokenを公開fixture keyで検証する。実IAM呼び出し/permission grantの証拠ではない。必要なiam.serviceAccounts.signBlobは承認済みsigner resourceに対する別のinfrastructure確認事項で、権限付与コードを含めない。

`cmd/mypage-server`はCloud Run用entrypoint。固定設定の検証と明示したinfrastructure readinessを満たすまでADCを取得せず、real-server bootstrapはemulator/credential-fileを拒否する。設定値・secret・SDK error detailをlogへ出さず、HTTP timeout/header制限とgraceful shutdownを持つ。readiness flagは基盤・securityレビューの代わりではなく、実起動/deployは別の承認済み作業。

Cloud Runへ設定するserver-only項目はenvironment/region/project number/Web App ID/public origin/policy versions/signer email/OAuth client ID/secret。OAuth共通secretは承認済みSecret Manager/Cloud Runのsecret供給境界を使い、Viteへ渡さない。このsliceはsecret作成/設定を行わない。metadata refresherは未接続、history coverageも実inventory未確認。

Firestore emulatorでは構築したserverからsigned App Check＋fake Auth SDKでstart/callback/channel/confirm/session-complete/MyPageを通し、mint replay拒否、users非作成、旧Google provider拒否を検証する。実Firebase ID token/App Check、Google consent/channel選択、Hosting rewrite、platform access-log redaction、runtime IAMは未検証でdeployしていない。


## Public channel metadata refresh

公開metadataはserver-only API keyの固定YouTube endpointから取得し、OAuth tokenを使わない。lookupは1秒に制限し、redirectを追わず、request URL/raw errorを公開しない。Firestoreの実document revisionとpolicy/access状態をtransactionで確認し、既存accountの公開metadataだけを更新する。外部callはtransactionの外で行い、accountを新規作成しない。

30日経過したmetadataはlookupの前に同じrevision guardで除去し、同意・account identityを維持し、metadataFetchedAtもclearする。認証/同意が失効した場合はsnapshot取得前に停止する。上流失敗はpartial/unavailableとし0へ変換しない。mock wireとemulatorで同時login、削除、block、policy変更、期限境界、古い応答の拒否を検証する。

server entrypointは任意のserver-only MYPAGE_YOUTUBE_API_KEYが設定された場合だけadapterを接続する。実keyの設定・Google接続は行っていない。戻らないaccountのmetadata保持期限を守るbackground jobは別の運用準備が必要。


## Consent-bound GA4 loader

ConsentTagはgrantedのときだけbrowser loaderを開始し、ロード前/途中のeventを保存・再送しない。撤回/unmountでsenderを停止しpending commandを消し、古いloadの成功/失敗が新しい同意世代を変更しない。browserTagLoaderは専用の単一data layerを使い、safe consent/configをscript挿入前に用意し、固定gtag.js URLとno-referrer、5秒期限を使う。shared tag/globalの既存所有を拒否する。撤回後の再同意は既にロードしたscriptを再取得しない。

productへの結線はVITE_MYPAGE_ANALYTICS_READY=true、公開measurement IDのVITE_MYPAGE_GA4_ID、現在originと一致するVITE_MYPAGE_PUBLIC_ORIGINが明示されたときだけ有効。これらはsecretではない。未設定ではtransportなしを維持し、同意の選択自体は使える。flagはConsole/通信inventory承認を代替しない。

unitで同意前/ロード中の非送信、撤回/再同意と古いload、失敗/期限/unmount、queue消去を検証する。local Chromiumではgtag.jsをローカルfake responseへ差し替え、script前のsafe引数、no-referrer、将来eventだけの送信、撤回/再同意を検証した。実vendorの通信・自動event・保持設定は検証していない。script除去は既に実行したvendor codeを取り消せないため、Enhanced Measurement等のConsole設定と実Network検証は公開前のgate。

一次資料: [Google tagの設置](https://developers.google.com/tag-platform/gtagjs)、[単一data layerと名称変更](https://developers.google.com/tag-platform/devguides/datalayer)、[pageview制御](https://developers.google.com/analytics/devguides/collection/ga4/views)。


## Hosting security candidate

`mypage/src/hosting.ts`のhostingCandidateは明示したenvironment/project/target/Cloud Run service/regionとreview済みSDK接続先から設定候補だけを生成するpure function。developmentはasia-southeast2、productionはasia-northeast2。既存firebase/firebase.jsonを変更せず、CLI init/deployを実行しない。projectIDは別の明示deployment selectorでありfirebase.json本体へ混入しない。候補のpublic=distはmypage directoryを基準とする。

/apiと/api/**をCloud Runへ先にrewriteし、最後にSPAへrewriteする。全static pathへno-store/no-referrer/CSPとsecurity headersを設定する。CSPは明示したHTTPS sourceだけを許し、wildcard/unsafe-inline/eval/credential/query/fragmentを拒否する。Firebase/App Check/reCAPTCHAの実sourceを推測しない。Analytics sourceは別approvalを必要とする。validateHostingAssetsはstatic API衝突・source map・秘密file向けpathを拒否する。Cloud Run APIは自身のresponse headersも保持する。

unitで候補/routing/inventory拒否を検証し、synthetic static MyPageを同じCSPのlocal Chromiumでrenderして違反0とtimeline/chart表示を確認した。実SDK/reCAPTCHA/GA4を含むinventory、Hosting経由のHost/headers/callback/query access-log除外、custom-domain HSTSは未検証。CSPはplatform loggingを制御しない。設定候補は公開承認やinfrastructure readinessの代わりではない。

一次資料: [Hosting config/priority/headers](https://firebase.google.com/docs/hosting/full-config)、[Cloud Run rewrite/region](https://firebase.google.com/docs/hosting/cloud-run)。


## Operator read-only emulator diagnostic

`cmd/mypage-operator-dryrun`は合成local emulatorだけに接続する診断CLI。development、明示demo project、IP loopback endpoint、目的(delete/revoke/disclosure)を必要とし、credential file・real project・production・実行用引数を拒否する。receipt referenceはbounded stdinだけから受け取り、引数/logへ出さない。ADC・Google API・Auth revoke・通知・書き込み/実行portを持たない。

同じread-only transactionでserver受付record、消費済みsupport OAuth binding、受付indexを照合する。受付から7日という削除期限を本人確認/reissueで延長しない。通常login、未確認record、environment/purpose/project/受付bindingの不一致、proof capture時の期限切れを拒否する。OAuth traceがTTLで除去された場合はproofTrace=unknownとする診断だけを返す。受付のproof field欠如や既存traceの不一致は拒否する。

出力は時刻・状態・目的・件数(scope付き)のみ。channel/receipt ID、challenge/proof、氏名、作業内容、raw record/errorを出さない。channel単位と当該receipt単位のqueryを区別し、queryはdocument IDだけ・最大1001件。1000超はat_least=1001、失敗/未確認storeはunknownで0と扱わない。Auth/cache/backup/export/BigQuery/log/legacy/他receiptとのOAuth relationは未確認。readOnly=true、executionAuthorized=false、inventoryComplete=falseを常に明示する。

Go/race/unitとemulatorで正規のsupport consume後の診断、binding拒否、期限保持、1002件境界、TTL trace不在、CLI成功/失敗の匿名出力を検証する。診断前後の合成document更新時刻は不変。CI emulator scriptへCLI integration対象も明示追加した。実データ/APIへのread-only実行を許可するものではなく、実削除/revoke/開示や運用手順の完成を示さない。

合成fixtureを準備したlocal emulatorに対して、system directoryで使用する例:

```sh
GOOGLE_CLOUD_PROJECT=demo-youtube-study-space-ci \
MYPAGE_ENVIRONMENT=development MYPAGE_SUPPORT_PURPOSE=delete \
FIRESTORE_EMULATOR_HOST=127.0.0.1:8080 \
go run ./cmd/mypage-operator-dryrun < /path/to/private-synthetic-reference
```

reference fileは手元の制限された場所に保持し、repo/PR/CI artifactへ追加しない。


## Terminal metadata state across processes

公開APIのHTTP200・明示items=[]だけをchannel不在として扱い、通信失敗/不正body/曖昧な結果と区別する。不在の場合は期限前でもrevision/policy/access guard付きで公開metadataと取得日時をclearする。消去失敗時にも既知の不在metadataをresponseで表示しない。通常の一時取得失敗は許可された期限内のpartial metadataを維持する。

metadataが空のaccountは、timestampも消去済みの場合を含めMETADATA_TOO_OLDという既存terminal reasonで返す。browserは当該reasonで旧account表示を除去する。BFF cacheとsingleflightはWebAccount document revision/metadata有無に結び付け、他processによる消去後のGETが旧cacheや旧revisionのflightを再利用しない。snapshotの30秒/JST境界制約は維持する。

mock/emulatorで正常不在と失敗の区別、25日metadataの即時clear、2processのTTL前cache破棄、旧flightへの非合流と遅い旧cacheの拒否、2browser memoryの消去を検証する。すべての既存browserへ瞬時に通知する仕組みやaccount削除の全writer guardを意味するものではない。


## Inactive-account metadata cleanup and monitoring

MetadataCleanupJobは利用時refreshと独立して、metadataFetchedAtが取得から29日以上のaccountをpage単位で処理する。公開metadataと取得日時の4fieldだけを削除しupdatedAtを更新する。未利用・accessBlocked・旧policyのaccountにも保持期限を適用し、identity/同意/firstWebLoginAt/access状態を変更しない。OAuth/API refreshをせず、accountを作成しない。Firestore adapterは明示projectを確認し、timestampとdocument IDで安定pagination、最大1001件の検出、projectionとexact revision transactionを使う。

並行refresh/削除/revision変更は新しいaccount内容を上書きせずchanged/missingとして扱う。最後の再scanで期限対象が残れば失敗とし、page上限・10分budget・scan/clear/contextの失敗も成功扱いしない。失敗後の再実行は現在revisionからやり直せる。30日以上のdataを検出・除去した場合も保持期限違反を匿名件数で記録する。

JSONMetadataCleanupRecorderは匿名の開始/完了時刻・結果・固定stage・件数だけを出力する。ID/metadata/raw errorを含めず、監視出力失敗そのものもjob失敗にする。cancel後も2秒の独立budgetで失敗観測を試みる。EvaluateMetadataCleanupHealthは未成功、run失敗、保持期限違反、成功coverage時刻から23時間以上のheartbeat欠如をalertとする。dependency errorは内部でunwrap可能なまま返すが、callerはraw errorを公開logへ出さない。

29日eligibilityと処理時間に余裕を持たせるため、接続時には成功scan開始間隔23時間以内・job10分以内を条件とする。少なくとも日次という仕様を満たす設定候補は1日2回だが、実scheduler/alert設定は作成していない。単に24時間間隔の実行を宣言して完了時間の余裕を無視しない。heartbeatは完了日時でなく、成功runのStartedAtを基準にする。日次以上の起動・失敗/heartbeat/期限違反の通知先・再試行・telemetry到達の運用検証はinfrastructure/公開gateに残る。

このsliceはinjectable job、Firestore adapter、JSON観測とhealth判定まで。mock/emulatorで29日境界、未利用/blocked/旧policy、pagination、同時refresh/削除/同意変更、残件と再実行、scan/clear/monitoring失敗、heartbeatと期限違反を検証する。新API、実scheduler、IAM、credentials、deploy、実データ消去は追加・実行していない。account削除/revoke/開示の実行承認や全writer guardを代替しない。


## Review hardening: bounded telemetry and terminal account on work failure

JSON recorderはdeadlineを実際に適用できる専用sinkだけを受け入れ、通常のio.WriterをWrite前に拒否する。2秒のwrite deadlineとcancel時のdeadline短縮でblocked pipeを解除し、期限後のwrite成功も失敗扱いにする。deadline非対応の通常file/stdoutはfail closedとなるため、実監視sinkを選定・接続・検証する運用gateが必要。書き込みを無期限goroutineへ逃がさない。

metadataが既知のterminal状態なら、work snapshot取得/集計の失敗でも既存200 partial contractでaccountのMETADATA_TOO_OLDを伝える。current/summary/7日すべてをSOURCE_UNAVAILABLEとし、workを0や未登録へ変換しない。snapshotがない応答は生成時刻を使いcacheしない。通常のwork障害では従来の503を維持する。共通の合成wire fixtureをHTTP BFFとbrowser loader/memoryで検証し、旧accountだけの除去と旧work/asOfのstale保持を確認する。
