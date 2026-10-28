# MyPage implementation contract

実装・DEV配備・実試験の最新状態は[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)を参照する。本文は実装契約であり、配備状態の判定には使わない。公開Gateの完了は、対象releaseの証跡で個別に確認する。

公開準備は[release runbook](release/README.md)、[実E2E/rollback手順](release/e2e.md)を参照する。既存実装と外部/本人判断待ちを区別し、設定検査やlocal合成QAを公開承認・live E2Eの証拠へ読み替えない。

source-onlyの[Privacy request intake](privacy-intake.md)はdefault offで、受付・本人確認・返信の境界と残る運用判断を記録する。Canon 05/09の初期公開経路は返信可能な人的Privacy窓口と既存fresh OAuth・private CLI / Runbookの実運用であり、アプリ内受付UIの有効化は必須条件ではない。CLIのlive adapterは未接続で、現状のsourceだけで公開可能とは扱わない。

横断削除Bのcheckpoint・offline CLI・adapter境界・未確認gateは[Support deletion runbook](support-delete.md)と[inventory](support-delete-inventory.md)を参照する。

現行仕様から実装に必要な非機密 wire contract だけを抽出したもの。`openapi.yaml` は ten-endpoint API の契約（公開maintenance statusを含み、privacy受付2本はdefault off、明細1本はDEV限定/default off）、`fixtures/` は実ユーザーに依存しない合成例。メンテナンスモードの状態・例外・手動切替・復旧は[maintenance mode v1 runbook](maintenance-mode.md)を参照する。

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

Frontend と backend は独立して検証する。provisioning / production deployment はこの契約検証に含まない。

構成・公開段階・実環境の確認事項は [release runbook](release/README.md) を参照する。

## 認証とserver境界

### Web authentication storage

`oauth-transactions/{opaque id}` と `web-accounts/{verified channel id}` を server-only に追加する。既存 rules の default deny により browser access は許可しない。10分の期限は application が判定する。TTL定義とmock/限定plan検査は[MyPage専用Terraform](../../infra/mypage-oauth-ttl/README.md)で準備し、plan/apply・TTL ACTIVEと自然削除観測・権限・cleanupの実環境確認はprovisioning gate後の別途承認に残す。

Callback は `pending -> processing -> channel_verified`、confirm は `channel_verified -> consumed`。WebAccount upsert と consume は同じ transaction。firstWebLoginAt は authenticated session completion で set-if-absent。users / 作業履歴は web login によって変更しない。transaction record には state hash・同意版・公開 channel metadata・時刻だけを保存し、code / OAuth token / Custom Token は保存しない。

Google provider と Firebase token mint は interface として分離し、demo emulator では合成 provider を使用する。配備状態は対象 release の証跡で確認する。

### YouTube OAuth provider adapter

`GoogleYouTubeOAuth`はserver-onlyのclient ID/secretと固定HTTPS originを受け取り、既存のatomic callback serviceへ接続できるadapter。authorizationはyoutube.readonlyだけ・online・select_accountとし、Google Account選択をYouTube channel pickerとは扱わない。code exchangeは1回で、grant scopeを検証してから同じ短命access tokenでmine=trueのchannelを取得する。channelなし・複数・次pageありは拒否し、唯一の公開metadataだけを返す。legacy custom URLを@handleと推測しない。

provider instance/store/responseにYouTube tokenを保存せず、refresh tokenを要求せず、token source/refresh/retryを使わない。callback originはrequestから作らず、外部redirectを追わず、処理全体を8秒に制限する。provider body/URL errorをallowlist errorへ変換しsecret/code/tokenを公開errorへ出さない。

一次資料: [Google OAuth server flow](https://developers.google.com/youtube/v3/guides/auth/server-side-web-apps)、[channels.list](https://developers.google.com/youtube/v3/docs/channels/list)。

### Firebase verification boundary

FirebaseBoundaryは公式Admin SDKのVerifyIDToken/CustomToken portを使い、署名検証済みtokenについて固定projectのissuer/audience、custom provider、tenantなし、channel形式のUID/subject一致、期限/発行/認証時刻を追加検証する。SDK/token errorを公開messageやlogへ転記しない。mintはserverが確認したchannel UIDだけを渡す。

AppCheckVerifierは公式のJWT検証手順に沿ってRS256署名・JWT type・固定project numberのissuer/audience・当該Web App ID・期限/発行時刻を検証する。JWKSは固定v1 endpointから遅延取得し、最大6時間cache、処理2秒・size/key数制限、未知kid/取得失敗のrefresh間隔制限とsingleflightを持つ。constructorは通信/background loopを作らない。SDKのeager AppCheck clientを使わず、request contextと合成HTTP transportで検証できる境界を採用した。

一次資料: [App Check custom backend verification](https://firebase.google.com/docs/app-check/custom-resource-backend)、[Firebase ID token verification](https://firebase.google.com/docs/auth/admin/verify-id-tokens)、[custom token signing](https://firebase.google.com/docs/auth/admin/create-custom-tokens)。

### Server and keyless SDK assembly

`NewMyPageServer`で固定environment/project/app/origin、FirebaseBoundary、OAuth provider、Firestore Auth/Support store、read-only snapshot/BFFを6 endpointへ接続する。constructorはsecret/credential取得・外部通信・listen・通知を実行しない。coverageは検証済み範囲だけを注入し、未設定のhistoryを0へ変換しない。

`NewKeylessFirebaseClient`は既存ADCのtoken sourceと明示した同一projectのsigner emailを受け取り、credential JSON/key file、Auth emulator、project不一致を拒否する。公式SDKへServiceAccountIDを指定し、IAM remote signingを選ぶ。合成IAM transportでsignBlob requestをRSA署名し、SDKのCustom Tokenを公開fixture keyで検証する。実IAM呼び出し/permission grantの証拠ではない。必要なiam.serviceAccounts.signBlobは承認済みsigner resourceに対する別のinfrastructure確認事項で、権限付与コードを含めない。

`cmd/mypage-server`はCloud Run用entrypoint。固定設定の検証と明示したinfrastructure readinessを満たすまでADCを取得せず、real-server bootstrapはemulator/credential-fileを拒否する。設定値・secret・SDK error detailをlogへ出さず、HTTP timeout/header制限とgraceful shutdownを持つ。readiness flagは基盤・securityレビューの代わりではなく、実起動/deployは別の承認済み作業。

Cloud Runへ設定するserver-only項目はenvironment/region/project number/Web App ID/public origin/policy versions/signer email/OAuth client ID/secret。OAuth共通secretは承認済みSecret Manager/Cloud Runのsecret供給境界を使い、Viteへ渡さない。history coverageは実inventory未確認のため未指定とする。

## 作業集計と履歴

### Pure aggregation

`WorkSnapshot` は同じ Firestore snapshot から読む adapter との境界。snapshot read time がない入力は受理しない。`Coverage` は完全性を確認した期間だけを指定し、未設定なら登録済み利用者の履歴を `HISTORY_INCOMPLETE` とする。実環境の coverage 開始日・欠損範囲は未確認であり、推測設定しない。

現行 writer は退室時に `users.TotalStudySec` へ入室セッション分を転記する。そのため入室中の lifetime は `TotalStudySec + SeatDoc.CumulativeWorkSec + work 状態の経過秒`。履歴の ongoing 区間は `CurrentSegmentStartedAt` を使い、作業名変更による closed segment と重ねない。break 中もすでに完了した session work は lifetime に残す。

1001件・取得失敗・重複/未来/負の区間・coverage 欠損を正常な0へ変換しない。coverage欠損だけの場合、有効な区間から確認できた正の合計を任意の`recordedWorkSec`として返す。完全な合計`workSec`はnull/unavailableのままで、UIは記録済み分・集計未確定と区別する。0、取得障害、上限、整合性不良には記録済み値を付与しない。独立して正しい current / lifetime / account は保持する。

### Snapshot adapter and writer ordering

User lookup の `DocumentSnapshot.ReadTime` を取得してから JST query window を決め、User / seats / member-seats / work-segments を同じ read-only transaction で読む。不存在Userでも readTime を維持し、seat/history queryは省略する。seatは各区分2件まで、historyは1001件までで打ち切る。

work-segmentsは user-id equality、ended-at > window start、started-at < asOf を用いる。必要な composite index は [query 契約](query-index.json)と [Firebase 配備定義](../../firebase/firestore.indexes.json)の一致を契約 checker で検証する。実環境への適用・有効状態は release gate で別途確認する。

既存 writer の identity/history query は `WorkspaceApp.RunTransaction` が渡すcontext内で同じtransactionへ参加する。外側のquery利用は従来どおり。これにより seat更新と並行する read→write が競合判定される。

移動では old segment close と new seat entry に同じtransition instantを使う。休憩のoriginal startは新seatのentryより前でも正当だが、current segmentはnew seat entry以降でなければならない。過去に記録された重複区間は推測修復・許容せず、影響するhistory metricを `DATA_INCONSISTENT`、独立したcurrent/lifetimeは利用可能な限り維持する。

### 月次の読み取り用履歴

日別の終了区間件数も保持し、明細queryのscan予算を検証できるようにする。寄与した原区間の最遠endでfuture区間の影響を過去日にも伝える。各scalarは欠損・null・別型を拒否し、0/falseへ暗黙変換しない。

月次 reader はユーザー×月の `work-months`（最大31日の確定 work 秒・日別整合性・版）を最大2文書読み、直近7日を集計する。close 時の月次 write は通常1で、月跨ぎでは増える。月次は同じユーザー内の更新競合と文書転送量が増えるため、実DEVのlatency・競合率・追加write費を確認して切り替える。既存`DailyUserWorkHistoryDoc`はwriterがなく、検証済みの基礎値へ復活させない。raw `work-segments`は原記録として保持する。

切替はserver-onlyのUser field `work-history-epoch=monthly-v1`で明示する。未移行は従来の上限付きreaderを維持し、移行後の通常APIはraw query・同期backfill・明細全page巡回を行わない。`Coverage`は従来どおり独立した履歴完全性の根拠で、月次文書の存在や作成日時から0を推測しない。欠損月・未検証baseline・異なるepochは`HISTORY_INCOMPLETE`、不正区間・逆行watermarkは`DATA_INCONSISTENT`、取得失敗は`SOURCE_UNAVAILABLE`とする。

`work-history-activated-at`と`work-history-closed-through`は明示切替時の同一snapshot時刻で初期化する。future rawがないことも上限1件のqueryで確認し、後続writerがclosed-throughを同じtransactionで単調に進める。切替以後、closed-throughより後の未作成月/日だけは「原記録が存在しない」と検証できる。`closed-through >= activated-at`・未来値なしを必須とし、記録があるはずの月の欠損はunknownのままにする。

writerは`PrepareWorkSegmentWrites`でSAC・冪等event・User・現在Seat・対象月を先にreadし、同じattempt/区間に束縛したplanで原記録・月次・watermarkをwriteする。In＋注文＋席移動も注文write前に準備し、退室はSeat削除前に準備する。作業名変更、休憩/再開、Clear、Out、強制退室、batchは同じ中央writerを使う。現在SeatのUID/session/state/segment開始と一致しない旧区間、User不存在、準備なし/`tx=nil`の作成、genericなraw/月次削除は拒否する。既存の訂正APIはない。管理者の直接raw修正は通常writerを迂回するため、readerを明示rollbackしてから修正・bounded再集計・再切替する。

イベントIDはUser/session/Seat/state/半開区間から決め、同一内容の再送は加算しない。同じIDで内容が違えば競合。SAC checkpointは業務operation全体のretryに保持し、削除解除後に取り直して旧処理を継続しない。late/overlapの派生値はexactにせず、明示再集計で修復する。JST分割後の秒を日別に保持し、日跨ぎの秒未満によるcarryを週集計へ補正して既存rawの丸めを維持する。進行中分は同一asOfのSeat、累計は既存User/Seatの時計を維持する。

snapshot自身はUser 1 lookup、通常/メンバー席各`limit(2)`、月次最大2 lookup。通常はqueryの空結果最低課金を含めsnapshotで4〜5read、壊れた重複席を含む最大は7read相当となる。**APIが2readになるという意味ではない**。metadataがfreshなcache missのleaderでは、既存HTTP/BFFのSAC lookup計6とWebAccount lookup1も加わり、通常11〜12read相当、最大14がソース上の目安。cache hitも認可等4 lookupを維持する。transaction retry・metadata refresh・Auth/App Check・index entry等は別で、これらを実Billingの上限や実測値と呼ばない。

新writerの追加readは通常5（SAC/event/User/Seat/月）、追加writeは月次1と移行済みUserのwatermark最大1。既存Seat/User更新と別に見積もり、retryを加味する。準備対象区間は最大366日、1区間の月数も有限とし、異常に長い区間を黙って捨てない。月次の`days` mapはindex除外候補としてFirebase設定へ記録し、実適用は配備証跡と区別する。移行・復旧・混在writerの手順は[release runbook](release/README.md#月次履歴の明示移行と復旧)を参照する。

### 明細cursorと重い処理の利用予算

`GET /api/mypage/history`はDEV専用・policy未設定ではOFF。public price/Premium発売を意味しない。`mypage-maintenance/history-policy`の[候補](history-policy.example.json)は正式商品枠ではなく、所有者が実費・UXを確認して使う設定例。Productionは拒否する。Frontendも`VITE_HISTORY_DETAILS_ENABLED`が明示trueかつDEV projectの時だけ表示し、既定OFF。

期間は「JSTの[from,to)に完了したwork区間」。日を跨いで開始した区間も完了日に表示する。グラフの期間へのoverlap集計とは別で、グラフのために明細全pageを読み切らない。1request最大31日、初期50/max100件、最大120KiBの明細payload、8秒の処理budget。過去に閲覧できる範囲と1requestの期間を分け、準備済み月次があるrecorded historyを対象にする。未観測の期間を実作業0と確定しない。

sortはended-at＋一意document ID、offsetなし。15分のAES-GCM cursorにverified UID binding、SAC checkpoint、consent、server entitlement/policy revision、期間・page size・固定readTimeを束縛する。原記録のID/UIDをcursorのplaintextやresponseへ出さない。既存server OAuth secretから用途別にkey導出し、新secret/credentialは作らない。page追加の途中でも同じasOfを使うため、同時追加による重複・飛び込みを避ける。

raw query前に、そのsnapshotの月次終了件数と専用indexでscan範囲を限定する。`user-id/segment-type/ended-at` indexのDEV適用とQuery Explain照合は有効化Gateで、Emulatorや件数だけを実index課金の証明にしない。件数上限・欠損月・不正schemaでは通常APIからbackfillしない。明細では重複等のある個々の記録も`sourceConsistent=false`で区別し、グラフのexactな合計へ流用しない。

quotaは`mypage-history-budgets`のUID別singletonに、JST日付・消費単位・opaqueな重複検知・最大4 leaseだけを保存する。新基盤を追加せず、SACとpolicyを同じFirestore transactionで照合して予約する。単位は`page size + sentinel + ceil(maxIndexEntries/1000) + 32`という保守的な技術予算で、quota管理等の費用も含む。原記録のreadより先に判定し、instance/restart/並行requestを跨ぐ。Premiumはtrusted server resolverだけが判定し、既存YouTube会員やclient指定planを自動的にPremiumへしない。既定はStandard、両枠とも有限。

失敗・取消・ACK不明も予約分を消費し、自動refundしない。同じ`Idempotency-Key`は同条件でも409で再実行せず、別条件への転用も拒否する。manual retryは新しいoperationとして予約する。履歴cacheは持たない。重複検知は現在/前JST日の分だけを利用時pruneし、最大2048件。idle singletonは次の利用またはPrivacy削除まで残るため、これをTTL完了や新しい公開retention policyと呼ばない。

leaseはrequest期限より長い15秒で、正常/失敗時に独立1秒budgetで解放する。取消やprocess消失後も期限で回復し、日跨ぎでも旧leaseを維持して同時実行を増やさない。429はRetry-AfterとJST再開時刻を返し、UIは表示済みpageを保持する。Auth/停止/世代変更・pagehideではprivate明細をclearする。overview/logout/Privacyはこの日次枠を消費せず、通常の5分利用を止めない。quotaも既存Privacy primary scopeで消し、旧cursorは元SAC checkpoint不一致で拒否する。

## HTTP・cache

`HTTPHandler` は6つの通常login endpointのlibrary boundary。App Check、Firebase custom provider、fresh ServiceAccessControl、WebAccount存在・同意版を統計アクセス前に検証する。cache hitでもこのgateを毎回通す。旧`accessBlocked`はdecode互換のみで認可に使用しない。opaque cookieはHttpOnly / Secure / Lax / Path=/、Domainなし。callbackは固定pathへredirectし、raw provider errorやdependency detailを公開しない。JSONは未知・重複・大小文字違いのfield、余分なquery/body、4096 byte超を拒否する。

aggregate cacheは環境+uidをkeyにprocess memoryで最大30秒、JST日境界を越えて再利用しない。singleflight内の処理は最大8秒budgetを持ち、最初のcaller切断が他callerの処理を止めない。generatedAtは元snapshotの値を維持する。失敗はcacheしない。metadataは24時間でrefresh対象、失敗時30日未満だけpartial、30日以上はunavailable。

rate limitはprocess単位のbounded memory。deploymentの全体limitとtrusted proxy IP抽出は未実装で、既定は`RemoteAddr`を使う（実経路の確認はCanon S01）。[ServiceAccessControl A](service-access-control.md)はMyPageのread/token/write境界を保護する。横断削除と全runtime drain、support受付完了の実行はBの独立した完了条件。

HTTP handlerは環境ごとに固定したHTTPS `PublicOrigin` を必須とし、対象外Host、別origin/OriginなしのPOST、重複Originと重複`__session`をdependency検証・OAuth mutation前に拒否する。callback GETはOriginなしを許すが固定Hostの検証を省略しない。clientのX-Forwarded-Host / Proto / Forで許可先やdefault IPを変更しない。Hostingから届く実Hostの確認はdeployment gateに残す。

すべてのrouteにcoarse IP bucket（60/min、burst20）をverification前に置き、invalid App Check / ID tokenの連投も制限する。startの5/10min・20/hour制限もこの位置で行う。verified uid制限とatomic transaction consumeは別に維持する。

snapshotのdetached flightも元requestの残りdeadlineを引き継ぎ、最大8秒とresponse用の余裕で上限を切る。authorization前から進むHTTP budgetをリセットしない。短いdeadlineで合流するcallerにも、既知terminal accountをouter/client期限前にpartialで返す余裕を設ける。caller切断は他callerの共有readをcancelせず、flightのmetadata copyとcallerのfallbackを分離する。deadlineまで待つsnapshotと、より短いcallerのsingleflight合流を回帰testで確認する。

shared flight中に判明したchannel不在/metadata terminalと認証gate errorは同revisionのcallerへmutexで共有する。初期accountがpresentの短いcallerも、durable clear失敗時を含め既知terminalを期限前に受け取る。登録・完了・slot削除を同lockで行う最大1024件のsingleflight registryにより、callbackとjoinerの登録gapを作らず、完了結果とflight途中のsanitized verdictだけを共有する。

## Frontendと画面遷移

### Frontend private memory

`MyPageMemory` はuid変更/logout開始/pagehideで個人データを同期clearしてrequestをabortする。epoch照合によりabortを無視する遅延responseも破棄する。logout失敗でもデータは復活せず再試行可能。bootstrap中にlogin画面を出さない。

MyPageのoverview route（末尾slashも含む）がvisibleかつonlineの時だけ成功後5分でpollし、他route・hidden・offlineでは次の取得を停止する。短時間の復帰では残り時間だけ待機し、5分以上経過した復帰時は即refreshする。初回取得・手動更新・request overlap防止・manual debounceを維持する。通常503の自動backoffは絶対時刻で保持し、タブ・route・network・bfcache復帰で短縮しない。明示的な手動更新は通常backoff中も可能だが、429のRetry-Afterは手動更新も待機する。待機は同じuidの復帰に引き継ぎ、別uidへ転用しない。401はデータを消して同uidで一度だけtoken refresh、App Checkだけは一度再取得し、継続401/再同意/WebAccountなしはsignOutへ戻す。取得不能section/metricは同uidの直前成功だけをmemory保持し、元asOfとstaleを残す。未取得値を0にはしない。

制限403は個人データを同期clearし、専用表示へ移る。poll/manual/visibility retryを停止し、logout・public法的文面・support・fresh loginを利用できる。通常503のbackoff/stale表示は維持する。

controllerはsession/API adapterを注入し、Firebase listener、router、画面、bfcache eventへ結線する。MyPage responseやmetadataをstorageへ保存する経路は持たない。

### Browser runtime and synthetic integration

通常loginのstart / channel / confirm / session-complete、Firebase custom sign-in、App CheckとID token付きsame-origin API、TanStack Routerを接続する。対象uidはFirebaseから読み、clientから対象channel IDを送らない。confirmは自動再送せず、Custom Tokenをmemory controllerやstorageへ保存しない。session-completeのtransient failureだけを最大3回試し、失敗時は当該uidをsignOutする。遅い旧session処理は新uidをsignOutしない。completion前は個人画面へのfetchを抑止する。

public Firebase設定とpolicy versionが揃わない場合はlogin unavailableとしてpublic pageだけを表示する。ViteへOAuth secret / admin credential / App Check debug tokenを設定しない。Firebase SDKの認証session persistence以外の個人metadata / 作業dataはmemory-only。Routerのscroll restorationを無効化しURLをkeyにしない。現在のRouter dependencyはpagehideで空のscroll cache `{}`をsessionStorageへ書くが、URL・challenge・metadataは入らない。

認証・route の合成 QA は [design acceptance](design/README.md) を使う。product runtime の成功と実環境での認証・公開受入を区別する。

support本人確認は準備中と明示し通常loginへ流さない。privacy / terms / contactとCookie設定は未完成のdraft UIであり公開承認版ではない。人間が決める運営者・窓口・公開URLを推測で入れない。実Google OAuth / Firebase App Check / Hosting経由のE2E、approved runtimeとのpixel parity、運用privacy/security、D01実装証跡・D02規約適合性確認・D03実値、infrastructure Ready Gateは一般公開前の条件。実環境での確認状況は[release手順](release/README.md)と[実経路E2E](release/e2e.md)を正とする。

### Confirmation return after bfcache

channel確認画面でpagehide/identity変更が起きたらpending確認をabortし、旧channelをclearして中断理由と既存の再開始導線を表示する。bfcacheでmounted画面へ戻っても待機表示を続けず、旧成功応答のreceipt復活・遷移を抑止する。通常loginはloginからやり直し、supportは受付窓口へ戻る。

実 bfcache の確認は HMR を含まない専用静的 build を使い、通常 / support の pending 有無、制限表示、pagehide 後の旧応答と本人データの非復活を確認する。実行方法は [design acceptance](design/README.md) を参照する。

### 表示・合成検証

表示原則・対象状態・browser QA は [design acceptance](design/README.md)、package の検証コマンドは [mypage README](../../mypage/README.md) を参照する。visual/runtime 専用入口は production build に含めない。合成検証では外部 request を遮断し、実ユーザーデータを artifact に保存しない。

`METADATA_TOO_OLD` は直前成功 account も clear して表示停止理由を示す。429 の共通 deadline は manual refresh / visibility 復帰で短縮しない。auth 拒否後の signOut 失敗は `LOGOUT_FAILED` と再試行導線を維持する。cookie-name の前後空白を正規化し、同名 cookie を重複拒否する。

## Support本人確認

### Support record boundary

`FirestoreSupportStore`はoperator/server用libraryで、public管理APIではない。environment / request ID / 対象channel / purpose(delete・revoke・disclosure)を記録し、challengeはhashと24時間期限だけを保存する。同じenvironment+request IDの重複作成はtransactionで拒否する。challenge再発行は元の受付時刻と削除期限(受付から7暦日)を維持し、旧challenge indexを消す。期限超過後の再発行でSLAを延長しない。

SupportBindingはrecord reference / request ID / environment / purpose / challenge hashを保持するserver-only値。reissue前のbinding・目的違い・environment違い・対象channel違い・期限境界・proof replayは拒否する。proof成功はchallengeをconsumeするが、削除実行の承認ではない。完了recordへのpure transitionはchannel/challenge/proof/OAuth referenceを取り除き、最小の匿名監査用時刻を残す。

D01削除guardを代替せず、既存users/seat/history/Auth userを変更しない。

### Support OAuth atomic boundary

startの任意supportChallengeをserver recordへ解決し、同じFirestore transactionで有効bindingを確認してOAuth recordを作る。request ID/environment/purpose/challenge hashをOAuthへ固定する。channelは固定purposeとfresh OAuthで得たpublic metadataだけを返す。clientからtarget channel/purposeを受け取らない。

support confirmは同じtransactionでOAuth status/ref/同意版、record status/environment/request ID/purpose/旧challenge無効化/対象channel/期限を検証し、SupportRequestへproofを一度記録してOAuthとchallengeをconsumeする。独立した二つのfresh OAuth transactionからも同一依頼のproofは一回のみ。成功responseは`purpose=support`とopaque requestRefであり、Custom Token mint、WebAccount作成、Firebase session completion、ユーザーデータ削除は実行しない。normal consumeへの迂回もstoreで拒否する。OAuth内のchannel metadata/state/refはconsume時にclearする。

通常responseもpurpose=loginを明示する。OpenAPIはchannel/confirmのdiscriminated unionを検証する。frontendの通常login adapterはsupport responseを受け付けず、support目的を通常login画面で確認してしまう経路を閉じる。D01実装証跡・D02規約適合性確認・D03実値・Ready Gateは継続して未完了。

### Support browser flow

support入口は既存Firebase sessionでも通常MyPageへredirectせず、fresh OAuthを開始する。不正なchallengeは専用の無効表示を維持し、通常loginへ切り替えない。serverから受け取ったpurposeと公開channel名を確認画面で示し、支援用buttonから確認する。successはopaque参照番号だけを受付画面のmemoryへ渡し、signIn/session-completeや削除実行を呼ばない。参照番号はroute離脱・pagehide・uid変更でclearし、URL/storageへ保存しない。client申告の目的・対象channelをAPIへ送らない。

product HTMLにno-referrerを指定し、support challenge等のqueryを外部遷移のRefererへ載せない。platform access log/CDN/Hosting設定の実確認はsecurity release gateに残す。支援用CLIと受付窓口の実値は未完了で、公開deployはしていない。

### Support confirmation lifecycle

support confirmのpending requestはidentity・page lifecycle・確認操作の世代に結び付ける。logout、別uid、logout後の同一uid再login、pagehide、unmount、新しい確認操作は旧requestをabortし、transportがabortを無視して成功を返しても参照番号を受け取らない。React側でもreceipt設定と遷移の直前に当該controllerの有効性を確認する。pageshowは取り消したsupport確認を復活させない。

unit回帰は遅延successと各invalidating eventを確認し、synthetic Chromium QAはpending状態からuid変更・logout/relogin・pagehide/pageshow・route離脱後のreceipt非復活と遅いcontact遷移の抑止を確認する。証明済みのserver recordを取り消す操作ではなく、browserで失効した応答の表示を抑止する。

### Operator read-only emulator diagnostic

`cmd/mypage-operator-dryrun`は合成local emulatorだけに接続する診断CLI。development、明示demo project、IP loopback endpoint、目的(delete/revoke/disclosure)を必要とし、credential file・real project・production・実行用引数を拒否する。receipt referenceはbounded stdinだけから受け取り、引数/logへ出さない。ADC・Google API・Auth revoke・通知・書き込み/実行portを持たない。

同じread-only transactionでserver受付record、消費済みsupport OAuth binding、受付indexを照合する。受付から7日という削除期限を本人確認/reissueで延長しない。通常login、未確認record、environment/purpose/project/受付bindingの不一致、proof capture時の期限切れを拒否する。OAuth traceがTTLで除去された場合はproofTrace=unknownとする診断だけを返す。受付のproof field欠如や既存traceの不一致は拒否する。

出力は時刻・状態・目的・件数(scope付き)のみ。channel/receipt ID、challenge/proof、氏名、作業内容、raw record/errorを出さない。channel単位と当該receipt単位のqueryを区別し、queryはdocument IDだけ・最大1001件。1000超はat_least=1001、失敗/未確認storeはunknownで0と扱わない。Auth/cache/backup/export/BigQuery/log/legacy/他receiptとのOAuth relationは未確認。readOnly=true、executionAuthorized=false、inventoryComplete=falseを常に明示する。

合成fixtureを準備したlocal emulatorに対して、system directoryで使用する例:

```sh
GOOGLE_CLOUD_PROJECT=demo-youtube-study-space-ci \
MYPAGE_ENVIRONMENT=development MYPAGE_SUPPORT_PURPOSE=delete \
FIRESTORE_EMULATOR_HOST=127.0.0.1:8080 \
go run ./cmd/mypage-operator-dryrun < /path/to/private-synthetic-reference
```

reference fileは手元の制限された場所に保持し、repo/PR/CI artifactへ追加しない。

## 任意Analyticsと同意

### Consent and security draft boundary

任意Analyticsはunset / granted / denied。unsetでnonblockingな選択を出し、拒否・未選択でも主要操作を使える。選択したgranted/denied文字列だけをfirst-party storageへ保存し、uid/channel/profileを結び付けない。Cookie設定から変更でき、storage eventによる別tabの拒否でも将来送信を停止する。保存に失敗してもcurrent-pageの選択を適用し、保存失敗を表示する。

AnalyticsGateは同意前にstart/sendせず、過去eventをqueue/replayしない。固定route/titleと完全一致した公開UTM組合せだけを渡し、callback、code/state/error/supportChallenge、任意query/fragment、referrer/title入力や本人dataを渡すAPIを持たない。通常loginの4 eventだけをparameterなしで扱う。実loaderは`VITE_MYPAGE_ANALYTICS_READY=true`かつmeasurement IDと公開originが一致する場合だけ有効になり、それ以外ではscriptを読み込まない。公開済みGA4 Network/console設定を検証したとは扱わない。mandatory Firebase/App Check/reCAPTCHA通信とは別の同意である。

public policy/termsは確認用draft。運営主体/窓口/施行日/管轄を推測で埋めず、D01実装・D02規約適合性・D03実値と公開文面の運用照合を残す。production公開はしていない。API全responseにはno-store/nosniff/no-referrer、default-src none / frame-ancestors none、不要device permission拒否を付ける。Hosting HTMLのCSP allowlistとcallback platform access-log redactionは実inventory/Ready Gate後に確認する。

### Mockable GA4 command adapter

`GA4Sender`はAnalyticsGateからのみstartする同期command/disable portのadapter。constructorは識別子やbrowser globalを作らず、startで自動config pageviewと広告signalsを無効化し、固定originのroot URL・空referrer・空titleを指定する。event時にもsafe route/固定title/許可UTMを再検証し、任意title/追加parameter/User-IDを転送しない。stopは送信を止め、contextをclearし、runtimeのmeasurement disable flagを設定する。deniedへの変更で送信を誘発し得るgtag commandは呼ばない。同意前eventを再送しない。

adapter自体はgtag.js loader/dataLayer/vendor通信を含まず、読み込みは[GA4 loader](../../mypage/src/ga4-loader.ts)が担う。mockはcommand引数とdisable順序を検証するだけで、実GA4の非送信を証明しない。実loaderでは同意の世代変更後に遅延load callbackを破棄し、queueを再送しないことが必要。ConsoleでEnhanced Measurementのhistory pageview等の自動collectionを無効化し、他tag/pluginを含めたinventoryを確認する必要がある。send_page_view=falseだけではhistory pageviewを防げない。14か月保持・保持reset OFFと、同意前/拒否後のNetwork検証はrelease gateに残る。

一次資料: [GA4 configuration](https://developers.google.com/analytics/devguides/collection/ga4/reference/config)、[manual pageviewsとhistory measurement](https://developers.google.com/analytics/devguides/collection/ga4/views)、[Consent mode](https://developers.google.com/tag-platform/security/guides/consent)。実GA4 Network/Console/自動event/再同意は未検証であり、公開deployはしていない。

### Consent-bound GA4 loader

ConsentTagはgrantedのときだけbrowser loaderを開始し、ロード前/途中のeventを保存・再送しない。撤回/unmountでsenderを停止しpending commandを消し、古いloadの成功/失敗が新しい同意世代を変更しない。browserTagLoaderは専用の単一data layerを使い、safe consent/configをscript挿入前に用意し、固定gtag.js URLとno-referrer、5秒期限を使う。shared tag/globalの既存所有を拒否する。撤回後の再同意は既にロードしたscriptを再取得しない。

productへの結線はVITE_MYPAGE_ANALYTICS_READY=true、公開measurement IDのVITE_MYPAGE_GA4_ID、現在originと一致するVITE_MYPAGE_PUBLIC_ORIGINが明示されたときだけ有効。これらはsecretではない。未設定ではtransportなしを維持し、同意の選択自体は使える。flagはConsole/通信inventory承認を代替しない。

script 除去では既に実行した vendor code を取り消せない。Console の Enhanced Measurement・自動 event・全 tag inventory・保持設定と、実 Network の非送信を公開前に確認する。

一次資料: [Google tagの設置](https://developers.google.com/tag-platform/gtagjs)、[単一data layerと名称変更](https://developers.google.com/tag-platform/devguides/datalayer)、[pageview制御](https://developers.google.com/analytics/devguides/collection/ga4/views)。

## Metadataの保持・失効

### Public channel metadata refresh

公開metadataはserver-only API keyの固定YouTube endpointから取得し、OAuth tokenを使わない。lookupは1秒に制限し、redirectを追わず、request URL/raw errorを公開しない。Firestoreの実document revisionとpolicy/access状態をtransactionで確認し、既存accountの公開metadataだけを更新する。外部callはtransactionの外で行い、accountを新規作成しない。

30日経過したmetadataはlookupの前に同じrevision guardで除去し、同意・account identityを維持し、metadataFetchedAtもclearする。認証/同意が失効した場合はsnapshot取得前に停止する。上流失敗はpartial/unavailableとし0へ変換しない。mock wireとemulatorで同時login、削除、block、policy変更、期限境界、古い応答の拒否を検証する。

server entrypointは任意のserver-only MYPAGE_YOUTUBE_API_KEYが設定された場合だけadapterを接続する。実 key の設定・接続は対象 release の証跡で確認する。戻らないaccountのmetadata保持期限を守るbackground jobは別の運用準備が必要。

metadataが既知のterminal状態なら、work snapshot取得/集計の失敗でも既存200 partial contractでaccountのMETADATA_TOO_OLDを伝える。current/summary/7日すべてをSOURCE_UNAVAILABLEとし、workを0や未登録へ変換しない。snapshotがない応答は生成時刻を使いcacheしない。通常のwork障害では従来の503を維持する。共通の合成wire fixtureをHTTP BFFとbrowser loader/memoryで検証し、旧accountだけの除去と旧work/asOfのstale保持を確認する。

### Terminal metadata state across processes

公開APIのHTTP200・明示items=[]だけをchannel不在として扱い、通信失敗/不正body/曖昧な結果と区別する。不在の場合は期限前でもrevision/policy/access guard付きで公開metadataと取得日時をclearする。消去失敗時にも既知の不在metadataをresponseで表示しない。通常の一時取得失敗は許可された期限内のpartial metadataを維持する。

metadataが空のaccountは、timestampも消去済みの場合を含めMETADATA_TOO_OLDという既存terminal reasonで返す。browserは当該reasonで旧account表示を除去する。BFF cacheとsingleflightはWebAccount document revision/metadata有無に結び付け、他processによる消去後のGETが旧cacheや旧revisionのflightを再利用しない。snapshotの30秒/JST境界制約は維持する。

既存の全 browser へ瞬時に通知する仕組みや、account 削除の全 writer guard を意味しない。

### Inactive-account metadata cleanup and monitoring

MetadataCleanupJobは利用時refreshと独立して、metadataFetchedAtが取得から29日以上のaccountをpage単位で処理する。公開metadataと取得日時の4fieldだけを削除しupdatedAtを更新する。未利用・accessBlocked・旧policyのaccountにも保持期限を適用し、identity/同意/firstWebLoginAt/access状態を変更しない。OAuth/API refreshをせず、accountを作成しない。Firestore adapterは明示projectを確認し、timestampとdocument IDで安定pagination、最大1001件の検出、projectionとexact revision transactionを使う。

並行refresh/削除/revision変更は新しいaccount内容を上書きせずchanged/missingとして扱う。最後の再scanで期限対象が残れば失敗とし、page上限・10分budget・scan/clear/contextの失敗も成功扱いしない。失敗後の再実行は現在revisionからやり直せる。30日以上のdataを検出・除去した場合も保持期限違反を匿名件数で記録する。

JSONMetadataCleanupRecorderは匿名の開始/完了時刻・結果・固定stage・件数だけを出力する。ID/metadata/raw errorを含めず、監視出力失敗そのものもjob失敗にする。cancel後も2秒の独立budgetで失敗観測を試みる。EvaluateMetadataCleanupHealthは未成功、run失敗、保持期限違反、成功coverage時刻から23時間以上のheartbeat欠如をalertとする。dependency errorは内部でunwrap可能なまま返すが、callerはraw errorを公開logへ出さない。

29日eligibilityと処理時間に余裕を持たせるため、接続時には成功scan開始間隔23時間以内・job10分以内を条件とする。少なくとも日次という仕様を満たす設定候補は1日2回だが、実scheduler/alert設定は作成していない。単に24時間間隔の実行を宣言して完了時間の余裕を無視しない。heartbeatは完了日時でなく、成功runのStartedAtを基準にする。日次以上の起動・失敗/heartbeat/期限違反の通知先・再試行・telemetry到達の運用検証はinfrastructure/公開gateに残る。

定期呼出し側のsourceは[MetadataCleanupRunner](../../system/core/mypage/metadata_cleanup_runner.go)へ結線した。`Run`は既存jobとdurable monitorを接続し、終了後のhealth signalまで返す。`CheckHealth`はaccountへ触れない独立watchdog用callback。両者を別々に起動しなければcleanup trigger欠落を監視できない。constructorは同じ明示project/clientからcleanup storeとmonitorを作り、scheduler・credential・通信を開始しない。

[FirestoreMetadataCleanupMonitor](../../system/core/mypage/metadata_cleanup_monitor_store.go)はserver-only `mypage-maintenance/metadata-cleanup`の匿名singletonへ最新観測、成功scan開始時刻、sink未受理の期限違反scan開始時刻をtransaction保存する。実行履歴を増殖させず、client rulesの既存default denyを維持する。Firestoreのmicrosecond保存精度へ時刻を正規化してから比較し、古いscanの遅延到着・再送は新しい失敗を隠さず、同時刻では失敗を優先する。破損状態はresetせず拒否する。monitor read/write・signal delivery失敗は成功扱いしない。出力はdeadline対応sinkへ固定reason/alert/確認時刻だけで、healthyも回復signalとして出す。期限違反は後続の正常scanでも未受理のまま保持し、sink受理後だけexpected scan時刻を比較するtransactionでACKする。ACK後の新しい違反も保護し、ACK失敗は再通知可能なまま残す。sink障害後にmonitorが利用可能ならmonitoring失敗も保存し、次のwatchdogが古い正常観測だけを信頼しない。

callbackの接続例と残る運用依存は[metadata監視source手順](release/metadata-monitoring.md)を参照する。実scheduler・独立watchdog・監視宛先を作成/有効化した証跡ではない。mock/emulatorで29日境界、未利用/blocked/旧policy、pagination、同時refresh/削除/同意変更、残件と再実行、scan/clear/monitoring失敗、heartbeatと期限違反を検証する。新API、実scheduler、IAM、credentials、deploy、実データ消去は追加・実行していない。account削除/revoke/開示の実行承認や全writer guardを代替しない。

JSON recorderはdeadlineを実際に適用できる専用sinkだけを受け入れ、通常のio.WriterをWrite前に拒否する。2秒のwrite deadlineとcancel時のdeadline短縮でblocked pipeを解除し、期限後のwrite成功も失敗扱いにする。deadline非対応の通常file/stdoutはfail closedとなるため、実監視sinkを選定・接続・検証する運用gateが必要。書き込みを無期限goroutineへ逃がさない。

## Hosting

`mypage/src/hosting.ts`のhostingCandidateは明示したenvironment/project/target/Cloud Run service/regionとreview済みSDK接続先から設定候補だけを生成するpure function。developmentはasia-southeast2、productionはasia-northeast2。既存firebase/firebase.jsonを変更せず、CLI init/deployを実行しない。projectIDは別の明示deployment selectorでありfirebase.json本体へ混入しない。候補のpublic=distはmypage directoryを基準とする。

/apiと/api/**をCloud Runへ先にrewriteし、最後にSPAへrewriteする。既定は全pathへno-store/no-referrer/CSPとsecurity headersを設定する。`hostingCandidate(input, assetPaths)`に検証済みの実build全file一覧を渡した場合だけ、既知のhash付きJS/CSS/woff2のexact pathをpublic immutableにする。HTML・API・未知hash pathのSPA fallbackはno-store。CSPは明示したHTTPS sourceだけを許し、wildcard/unsafe-inline/eval/credential/query/fragmentを拒否する。Firebase/App Check/reCAPTCHAの実sourceを推測しない。Analytics sourceは別approvalを必要とする。validateHostingAssetsはstatic API衝突・source map・秘密file向けpathを拒否する。Cloud Run APIは自身のresponse headersも保持する。

原価の仮定と除外範囲は[増分原価モデル](cost-notes.md)を参照する。実測前の推定を請求削減の実績にしない。

CSP は platform logging を制御しない。実 SDK / reCAPTCHA / GA4 の通信 inventory、Hosting の Host / headers / callback / access-log 除外、custom-domain HSTS は対象 release で別途検証する。

一次資料: [Hosting config/priority/headers](https://firebase.google.com/docs/hosting/full-config)、[Cloud Run rewrite/region](https://firebase.google.com/docs/hosting/cloud-run)。
