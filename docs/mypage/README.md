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

現時点の6 endpointは通常login用の最初の契約slice。問い合わせ本人確認の `supportChallenge` / purpose binding と Custom Tokenを発行しないsupport confirmは後続契約であり、login-only schemaを最終対応版とは扱わない。

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

これはsession/API adapterを注入するcontroller slice。実Firebase/browser listener、router、画面とbfcache eventへの結線は後続。MyPage responseやmetadataをstorageへ保存する経路は持たない。

## Display and visual verification

React表示componentはcurrent / summary / recent7Days / account panelを持ち、snapshotの時刻だけでtimelineを描く。未取得値はunavailable、旧成功値は元asOf付き、休憩でもworkNameを維持。0時間の日もkeyboardで選択できる。native dialogのinertに加えてTab循環・Escape・avatarへのfocus復帰を検証する。最新仕様に合わせflat cream背景と通常cardのshadowなしを使う。

`mypage/visual.html` はdev専用の合成fixture入口で、production buildの入口に含めない。実Firebase/API/router結線はこのcomponent sliceの後続。画面コードを独立して実browserで確認する:

```sh
cd mypage
pnpm dev --host 127.0.0.1
# 別shell、system Chromiumが使える環境
python -m pip install -r visual-requirements.txt
python scripts/visual-qa.py --output /tmp/mypage-visual-qa
```

320/390/768/1024/1440px × 11状態、modal keyboard/logoutを合成fixtureで確認する。external requestは遮断し、利用者dataをartifactへ出さない。Approved exportのruntime/provenance gapは `design/README.md` に記録し、元runtimeとのpixel parity確認は未完了。外部fontは取得しておらずlocal fallbackでのQA。
