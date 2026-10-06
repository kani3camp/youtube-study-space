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
