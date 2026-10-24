# 2026-10-24 DEV原価測定・復旧記録

対象は `test-youtube-study-space / asia-southeast2`。実装は専用の `integration/mypage-canon-20261006` へ統合し、Productionとbase=`dev`のPR #1236は変更・mergeしていない。採算の入力は[cost-model.json](cost-model.json)、生成表は[cost-scenarios.md](cost-scenarios.md)、数値証跡は[DEV evidence JSON](dev-cost-evidence-20261024.json)を参照する。

## 結果と確認範囲

同日raw 0/10/100/500/1000/1001件と、秒未満を含むJST日・月跨ぎ2件を、既存SnapshotReaderとAggregateで実Firestoreへ接続して測定した。月次readerは全fixtureでraw query 0、この日付では1月文書を1回lookupした。0～1000件で記録済み秒数はrawと一致し、1001件ではrawの`HISTORY_LIMIT_EXCEEDED`と月次の記録済み1001秒を区別した。coverage不明をexactな今日/今週の0へ変換しない。境界fixtureの再構築は日別丸め後の月内合計0秒、readerは直近日跨ぎのcarryを含む週の記録済み1秒、累計2秒となった。ただし月跨ぎ原記録は10/1境界で、10/24時点の直近7日window外にある。月跨ぎ原記録の作成・月内再構築は確認できたが、月跨ぎを含む直近windowのraw/月次reader比較と実2月lookupは未実証で、Emulatorの受入とは区別する。

これはowner端末からJakartaへ接続するSDK測定で、HTTP BFF・Cloud Run CPU・課金wall time・cache命中率の計測ではない。raw返却文書数を課金read数と呼ばない。取得失敗・並行writer・Privacy競合・明細の複数instance quotaなどは既存CI/Emulatorの検証で、実DEV障害注入済みとはしていない。

Query Explainは先にAnalyzeなしでCollectionのUID先頭composite `(user-id ASC, ended-at ASC, started-at ASC, __name__ ASC)` を確認し、対象の合成原記録だけを所有revision/hashで照合してからAnalyzeした。空queryはreadOperations=1、1000件ではdocument scan=1000 / index scan=1000 / readOperations=1001、1001件では各1001 / 1001 / 1003だった。Analyze自体も課金対象。別の明細用index・policy・quotaの実DEV検証に流用しない。

raw queryの1000文書を月次1文書へ置換した部分だけなら99.9%減。認可等のsource inventoryである6 SAC + WebAccount1 + User1 + 空seat query最低課金2を加え、Explain値を同条件へ当てはめたcache miss推定は1011→11 read相当（約98.9%減）となる。これはAPIの実請求や利用者平均の測定値ではない。writer追加read/write、App Check、静的配信、Run等を含むTCOは別表で計算する。母集団分布が未測定なので既定のraw平均20・月lookup平均1.2・index1000は仮定のまま残し、fixture感度表では実測index scanへ差し替える。

## 実配備した対

| 項目 | 固定した値・証跡 |
| --- | --- |
| image build source | `5803ad18c418307b8fdc7de7f7ce3e0ddc0fe774`、[既存CI image build](https://github.com/kani3camp/youtube-study-space/actions/runs/40064524012) |
| image digest | `asia-southeast2-docker.pkg.dev/test-youtube-study-space/mypage/mypage-api@sha256:4020ade3cb0b6114b56f41938620bac253e4951f32d3cff472ec28583312c210` |
| system tree | `b484aa740fd4e05c899082d5bc0cfc9deae4b4b1`。以降の制御修正sourceとも同一で、追加Cloud Run image buildなし |
| Frontend source | `a16dccd672c89228ea3594e211b8125a6cc80722`（PR #1453のAuth復元重複抑止を含む）、DEV site `test--study-space`の所有projectを照合 |
| Hosting release / version | `1792857234818000` / `722779b155355d62`（2026-10-24 15:53:54.818 UTC） |
| Frontend実体 | staged 8 filesをlive body SHA256で照合。JSは`index-Dz5BpYoa.js`、CSSは`index-DjiJpEFS.css`。5分更新・route/online/hidden/backoff修正を含む |
| Cache/security | 正確なhash付きJS/CSS/4 fontは`public, max-age=31536000, immutable`。HTML・SPA未知hash・APIはno-store。CSP/nosniffを維持、匿名APIは403 |
| 最終Cloud Run | `mypage-api-00006-xjf` / generation 6 / 観測OFF。history details、Privacy intake、analyticsはOFF |

5分更新・monthly writer/reader・明細と予算のsourceを含むAPI imageは配備したが、既存ユーザーを月次へ切り替えていない。月次activationは所有する合成Userだけに限定し、終了時にそのUserごと削除した。後段の追加DEV writer配備と、全writerのdrain・既存migrationの受入は別である。明細用indexやpolicy/quotaは追加・有効化していない。

## 観測ON/OFF、競合とState

既存Plan/HMAC手順により[観測ON](https://github.com/kani3camp/youtube-study-space/actions/runs/40077157558)でgeneration 5 / `mypage-api-00005-hgk`となった。live更新後にState反映が失敗したため後続を停止し、[新しいState復旧判断](https://github.com/kani3camp/youtube-study-space/actions/runs/40081768487)と[試験前metadata整合](https://github.com/kani3camp/youtube-study-space/actions/runs/40085407554)で12資源のStateとライブを照合してから合成試験を開始した。

実v2のstale etag付き設定不変PATCHは、mask/If-Matchを含む試験でも成功Operationを返した。これを競合拒否の証拠にはしない。[PR #1450](https://github.com/kani3camp/youtube-study-space/pull/1450)で実適用に必要な更新をv1の元`resourceVersion`付きPUTへ限定し、v2ライブ/Plan前提、完全spec・番号付きSecret参照保持、1回だけの送信、競合時停止、結果のv2照合を維持した。ユーザーからDEV限定full-copy PUTとOFFの追加許可を受けている。

合成削除後、現在のReadyを元に新しいOFF Planを作り、[観測OFF](https://github.com/kani3camp/youtube-study-space/actions/runs/40086892890)が成功した。実GETでgeneration 6 / Ready `mypage-api-00006-xjf`、imageと他設定不変を確認。その最新完全copyに元ON時`resourceVersion`を付けた設定不変PUTは実APIで409拒否され、送信前後のetag/generation/spec不変も確認した。これは当該実競合の拒否証跡であり、全ての並行変更パターンを証明するものではない。

一時付与していた1権限`run.services.update`を既存CI Apply SAから解除し、試験用custom roleを削除状態にした。元IAM etagを使って他binding不変を照合し、Cloud Run image/spec/Readyも維持した。GitHub Environmentの既存runtime入力は両方とも`read_metrics_enabled=false`、`read_metrics_from_revision=""`へ復元。[最終metadata同期CI](https://github.com/kani3camp/youtube-study-space/actions/runs/40087126693)のPlan/Apply/State readbackは成功した。2026-10-24 11:25:01 UTCに、AWSプラグインの既存「terraform state」接続から正確なDEV State keyを独立して読み戻した。S3 versionを固定して読み、serial 14・12資源、暗号化、lock不在を確認。既存契約と同じ正規化で、Cloud Runの全設定とUID/generation/observed generation/etag/更新時刻/Ready・created revisionがライブと完全一致した。State本文・bucket/account・credentialを公開していない。

## 予算と後片付け

データ試験は2026-10-24 10:28:17～10:40:53 UTC（約12分36秒）。承認上限は今回US$1、raw作成3000件、作成+削除6000回、document read 20000、index entry 100000、試験約20分。

- raw作成累計2613件。write/delete予約5260回は将来cleanupも含む保守的な予算で、実請求write数ではない。
- document read見積もり17467（driver17357 + 独立cleanup照合10 + 後続通常認証API allowance100）。SDK再試行等を請求書で確定した値ではない。
- Analyzeの実index scan合計2612、事前allowance13100、結果不明のindex operation 0。文書数limitを一般のindex課金上限とみなさない。
- 予約write/deleteをすべて高いwrite単価で計上したFirestore分は約US$0.013。image保存・Run・ログ・Auth等を含む実請求総額の確定ではなく、合計US$1承認を請求書上で照合できたとはしていない。無料枠を使った控除はしない。

各fixtureの差し替えとcleanupは作成時に保持したrevision/hashを使い、conflict時に新revisionへ追従しない。最後の境界fixtureではQAのtyped hashがJST/UTCの表記差で停止した。rawからの旧期待hash、元SAC checkpoint、全原記録revision/hashをread-onlyで再監査し、3時刻のUTC正規化だけを独立レビューした上で全値一致を確認した。内容一致を確認せず所有権を採用するfallbackや無条件削除は行っていない。

cleanup後に別のread-only照合でUser/raw/月次文書/座席不在と元SAC checkpoint不変を確認した。正規ログインで作成されたWebAccount/Authアカウントは削除せず、既存利用者の履歴・moderation記録を試験へ流用していない。

## 認証経路と未完Gate

試験中にChromeのApp Check/reCAPTCHA取得が失敗し、ログアウト後の再ログインが間に合わなかったため、合成raw/月次のHTTP BFF比較は実施していない。Auth/App Checkを無効化したり、browser credentialを取り出してowner SDKへ流用したりしていない。

ユーザーの再ログイン後、正常な未登録・履歴なしの専用アカウントで今日/今週/累計0と直近7日0の通常画面を確認した。Cloud Run request logは2026-10-24 11:06:52.123491 UTC、revision `mypage-api-00006-xjf`、HTTP 200、latency 0.072165407秒で一致した。この単発latencyをCPU・cache miss/hit比較やSLOにはしない。前のreCAPTCHA失敗は再現せず、根本原因を修正済みとはしていない。

App Check token TTLは3600秒。key metricsの同じ日単位windowで前後合計66→66を得たが、反映遅延・日粒度のため評価数0の証拠や1 poll単位の帰属ではない。組織共有の請求評価数・SKU ID・Billing実績は未照合。API pollingを1/5にしただけでApp Check費用を1/5にはしない。

残る実証は、直近window内の月跨ぎと実2月lookup、認証付きAPIの全文比較・process CPU・内部cache計数（追加UI比較と割当windowは後段参照）、母集団分布とbrowser転送量、請求照合、全writer fleetと既存migration、明細index/共有quotaのlive適用。明細の技術上限50/100件・31日は実装済み候補で、商品としての最終枠や690円Premium発売を決めたものではない。再開条件は既存[Issue #1133](https://github.com/kani3camp/youtube-study-space/issues/1133)で追跡し、この部分完了だけでIssueをcloseしない。

## 継続指示後の180件・認証付きAPI確認

既存上限の残枠で、同じ専用UIDへ180件だけ追加した。新ledgerは旧Finished/checkpoint/累積数の一致を必須とし、raw取得前にUID先頭index・全原記録所有・Analyze成功をguardした。データ窓は12:18:50～12:23:45 UTC（約4分55秒）。原記録/所有User/月次をCASで全削除し、独立readと認証付きの未登録0表示を確認した。累積raw2793、write/delete予約5628、read見積19391、Analyze index scan2792、index allowance22005、不明operation0、データ窓合計約17分31秒である。観測はOFF、image追加0、IAM再付与0を維持した。

SDKではraw180文書/1 query・910ms、月次raw0/1月lookup・762ms、記録済み今日/今週/累計180秒が一致。通常ChromeのAuth/App Check付き実APIでもraw/月次の記録済み今日・今週3分/累計3分を確認した。raw期間の200は205.6/35.0/32.9msの3回、月次は140.6/32.0msの2回。予定2回を超えたraw1回は210 read/2000 index allowanceを保守的に加算した。ログはUID/traceでは紐付けず、phase時刻によるcohort比較である。API本文全体のbyte一致や内部read/hit数を直接取得したとはしない。

raw再取得は次の分に200応答を得てもUIのasOfが21:19のままで、no-store経路のaggregate再利用が見られた。月次のfresh取得は21:22へ進み、値は同じ。内部cache counterと人口hit率は未計測なので、単発latencyをhit率やSLOにしない。

12:18～12:25 UTCの組み込みRun metricは課金対象instance時間1.1秒・CPU割当1.1 vCPU秒・9 requests（overview GET6回、session完了POST3回）。非空overviewは上記5回で、残る1回はcleanup後の正常0表示。1 vCPU/0.5 GiB・Jakarta list priceならRun compute/request分は約US$0.0000425の参考計算だが、請求書と照合した総額ではない。CPU使用率や個々のrequestへの配賦は測っていない。

非空overview5回のHTTP request平均3303 bytes、response平均2092.4 bytes（Cloud Logging message size、headers込み）。browser/Hosting最終wire量とは異なる。モデルではAPI inboundのみ測定参照へ更新し、静的requestの1024 bytes・responseの8192 bytes予算は分離する。session完了もsource上のSAC2+WebAccount2 readと1 documentあたり1回を独立加算し、polling回数とApp Check評価回数へ同じ倍率を掛けない。

実ログには近接したsession完了POSTが2件あった。実イベントへの帰属は断言しないが、同UIDのauth通知とpageshowを重ねる決定的テストで旧コードの二重POSTを再現した。同UID・同auth/confirmation世代・active・未abortに限って未完確認を共有する修正と、pagehide後は新確認を要求する回帰を追加し、全222 Frontend testを確認した。以前のreCAPTCHA失敗の根本原因修正とは扱わない。

公開SKUは[公式Firestore ops group](https://cloud.google.com/skus/sku-groups/firestore-ops)のCloud Firestore service `EE2C-7FAC-5E08`でread/write/delete候補を確認した。Jakarta read `8C8F-C8C6-5DC5`はrender表でも$0.038/100000 countと一致。旧App Engine名のSKUやfree-tier付SKUとは区別し、実accountのどのSKUへ計上されたかは未確認。既存APIのCatalog/Project Billing metadataは403、Catalog reasonはSERVICE_DISABLED、DEVの既存BigQuery billing export dataset候補は0だった。API有効化・新key・新exportを作らず、請求照合は別権限のGateとして残す。

writer監査は接続名だけでtargetを決めず、既存CLIの実identityとstackのGCP projectをDEVへ厳密に照合した。canonical AWSの17有効regionにEC2はなく、対象cluster1・Lambda9（application image7、provider Zip2）、Fargate family最新66、共有family参照はStateMachine4/Task6、読取時running task0。ownerはDEV Botが常駐していないと確認した。現sourceのtracked exportからFargate/Lambda asset2個をoffline synthし、liveの設定と対応する現source build/deploy receiptが未成立であることを確認した。tagや更新日時だけでbinary内容を証明しない。この段落は追加配備前のinventoryであり、その後の配備は次節を参照する。既存UID migrationは行っていない。

## 追加承認後のDEV writer配備・Frontend読み戻し（2026-10-25 JST）

ユーザーが追加2 imageとDEV限定適用を許可した。tracked system tree `b484aa740fd4e05c899082d5bc0cfc9deae4b4b1`から既存DockerfileでLinux amd64のLambda7 binary、arm64のFargate1 binaryを作成し、停止containerからELF/Go版/各binary hashを確認した。imageを実行してglobal batchや通知を試すことはしていない。[PR #1452](https://github.com/kani3camp/youtube-study-space/pull/1452)のasset除外により、ignored QA資料はCDK build contextへ入らない。

| 対象 | Source asset tag | Local config digest | Published/runtime manifest digest |
| --- | --- | --- | --- |
| Lambda amd64 ×7 | `53dafbcdc33356bdeee27d81258516588e9ac38eecef4a123db3ea7752862d67` | `sha256:af4ef8865af248bc072bb63614380dbf6169c80147a655ff6255a066d84bf93a` | `sha256:1bb70bf727e7b8d83127fd8a5548649aa880cd5ce30e5e485c6a37d968a2ac2b` |
| Fargate arm64 ×1 family | `0b343fadd20e1131c9f92cd42305ab20d3772a0e63108d25baf082b1bba18e9c` | `sha256:54db1b370685694d36012aca828a73e5fb2c642ecb2e90c96cf05236fb25fb68` | `sha256:b182e26f4c740c4595c94c1c649733a4991d23fdfc8ff59022424463cc57af69` |

既存immutable ECRへ計2 imageを追加し、registry manifestのconfig digestをlocalと照合した。8 image欄だけを差し替えたsaved candidateのraw SHA256は `34f9f33a77d957b4a8f1cd5009cd37d84a3e44e3a787aa918f8382328e66404d`。既存の暗号化CDK transportでbyte読み戻しし、保存ChangeSetのOriginal templateも候補とparsed全文一致を確認した。ChangeSetはTaskDefinition replacement1＋Lambda Modify7と、task revision参照によるIAM policy再評価4だけ。4 policyは新旧revisionで解決した全文・付与先がliveと一致し、独立reviewでも追加grantなしを確認した。

直前に元stack/更新時刻/role/全旧digest、稼働SFN/task0、既存schedule、IAM全文、保存ChangeSet、ECR digestを再確認した。`users`の月次marker存在検査はprojection＋limit1で0。未移行はraw readerを維持し、新writerの月を自動complete/cutoverしない契約のため既存Schedulerを維持した。実UIDの切替をせず、既存定期起動を止める設定変更も行わない。

適用はUPDATE_COMPLETE。16:06:37 UTCの読戻しで7 LambdaがActive/Successful・固定manifest一致、task familyは66→67・固定manifest一致、全Lambda非image設定・task非image設定・IAM4全文・schedule不変を確認した。ECS `compatibilities`は返却順だけ変わったため、enum集合一致を検査し、他項目は全文比較した。4 StateMachine内の6 task参照は同じunqualified familyを使う。新Fargate imageの実実行は未観測で、既存UIDのmigration/全writer drain完了やPrivacy全経路の受入とは区別する。rollbackは保存した旧manifestへLambda7を戻し、Fargateは同じfamilyへ旧imageの新revisionを作る必要がある。古いrevision番号へ戻したと扱わない。

Frontendは表の新releaseを配備し、8 body SHA、6 immutable asset、HTML/API/未知hashのno-store、CSP/nosniff、匿名403を再確認した。CSS/4 fontは前の承認済みbuildとSHA一致。通常Chromeの既存Auth/App Checkでloading完了・未登録・今日/今週/累計0・7日を確認し、確認tabを閉じた。session重複回数をliveで定量した証拠ではない。追加運用確認を含むread見積/allowanceは19424、原記録作成2793・write/delete予約5628は増やしていない。請求書未照合のためUS$1の実請求保証とはしない。

ECRの公表保存単価は[公式料金](https://aws.amazon.com/ecr/pricing/)の$0.10/GB月を参照。公開したcompressed size合計160,264,117 bytesに対し、モデルは未圧縮合計492,453,123 bytesから丸めた保守的DEV allowance $0.05/月を加えた。free tier・共有layer dedup控除は使わず、通常既存workloadを二重加算しない。旧imageはrollback用に保持し、長期retention削除は実施していない。

適用後16:00〜16:07 UTCの分粒度Lambda metricでは、既存定期処理3関数に呼出7/7/1・Errors各0を取得した。他4関数はsampleなしで、成功実行とは数えない。適用開始分を含む集計のため、全sampleが新imageで動いた証拠や個別requestの検証にはしない。手動invokeは0。証跡生成の独立reviewで浅いcopyが比較元taskを改変するP2を検出し、deep copy＋比較前後のraw hash不変＋保存前manifest一致の機械guardへ修正して読み取り直した。実配備の変更は行っていない。
