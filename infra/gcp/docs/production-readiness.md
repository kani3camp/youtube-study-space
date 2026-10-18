# Production backend / bootstrap / import readiness

実行入口は [Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)、設計正本は
[Notion Current Canon](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc)。
この文書はsource/test/runbookと現在の実証範囲を記録します。各実行の証跡はIssue #1191へ集約し、
Infrastructure Readyは個別条件で判定します。private identity、既存IAM全文、raw state/plan、mailbox、business dataは記載しません。

## Current execution boundary — 2026-10-17

全4 Terraform EnvironmentのRequired reviewersは本人が解除済みで、GitHub実APIでも確認した。
独立したplan/apply Environment、integration branch限定、OIDC/WIF trustは維持し、
GitHub本人account ID 54093651・actor/triggering actor kani3campの初回workflow_dispatchだけを通す。
各runの人間承認を再要求せず、botへの切替やrerunで制限を迂回しない。

Development Exitは[owner初回run 38945893535](https://github.com/kani3camp/youtube-study-space/actions/runs/38945893535)で、
plan/apply両identityの31 no-op・imports0・raw drift0、State/bucket policy、完了後に観測できたworkload指標を照合し、独立reviewもPASSした。
DevExit後fresh自然回帰（`2026-10-17T05:08:26Z`）は27 reads・全5 stage・finding0を独立review済み。Lambda3/6で1,338実行・Errors/Throttles0、quiet3はNoneのまま。daily SFN/ECS・backupのcutoff後E2Eは未観測で、先行daily E2Eはcutoff前の証跡である。
Production AWS backend roles2・inline policies2とGCP CI foundationは実作成・readbackを完了し、独立reviewもPASSした。
GCPの新規7 resource、保護対象5件の不変、個別policy5件へのmember追加8・削除0、read custom roleの12/3/1 permissionsを確認した。
実行証跡は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)へ集約し、private IAM本文は公開しない。
この実証を前提にMIGRATION_PRODUCTION_READYだけをtrueとし、旧PROD_* gateはfalseのまま維持する。
Prod2 Environmentへ6参照と固定managed0入力の計7種ずつ、合計14設定を保存し、値を読まずに設定名・保護条件・実行journalを独立照合した。
H2当時の[初期化run38961710157](https://github.com/kani3camp/youtube-study-space/actions/runs/38961710157)はplan/apply成功・固定outputs2件/resource0。H3で初版State0のversion指定GET・FULL_OBJECT checksum・controls9・Dev不変・lock不在を直接監査し、独立review PASSした。初版監査を、後続import後も空Stateという主張へ流用しない。
H3の[run39005902978](https://github.com/kani3camp/youtube-study-space/actions/runs/39005902978)まで既存9件のownership・post no-op・全CI成功を検証済み。sourceH4（`93121ab6`）の[第10波run39014888191](https://github.com/kani3camp/youtube-study-space/actions/runs/39014888191)は認証付きPlan/Apply・post10成功後、native runner回帰の静的CIとGateで失敗した。第10波後のState10/直前9のversion固定GET・checksum・native差分を独立確認し、追加1件・既存値/identity/checks・同lineage/serial増・controls9/DevHEAD不変・live lock/workspace0はPASS。元runの静的CI失敗を保持します。State10照合後にH5の[第11波run39021164334](https://github.com/kani3camp/youtube-study-space/actions/runs/39021164334)を1回発行しました。認証付きPlan/Applyは成功しましたが、native runnerのtimeout/grandchild回帰で静的CIとGateが失敗しました。State11/直前10のversion固定GET・厳密差分照合も独立PASSで、直前10の本文/HEADは第10波の独立確認済みcurrentと完全一致します。元R11の静的CI失敗を保持します。H6の[第12波run39022399909](https://github.com/kani3camp/youtube-study-space/actions/runs/39022399909)は全34 jobと両Auth成功、Plan import1/managed12、Apply post import0/no-op12/rawdrift0/mutation0、cleanup/artifact0/pending0を独立確認済みです。観測テストの修正[#1370](https://github.com/kani3camp/youtube-study-space/pull/1370)はrequired CI・独立review後に統合済みです。H7（`0ed7486b`）で第13波以降へ進み、[第30波run39029269335](https://github.com/kani3camp/youtube-study-space/actions/runs/39029269335)まで30件のownershipを確認しました。元失敗runは再送せず、その失敗判定を保持します。H7の最終通常実証はDevelopment [run39029559218](https://github.com/kani3camp/youtube-study-space/actions/runs/39029559218)・Production [run39029874116](https://github.com/kani3camp/youtube-study-space/actions/runs/39029874116)で、各plan/apply両identityのimport0・no-op31/30・rawdrift/leaf/mutation0、全34 jobの検証（選択job成功・非対象skip）・cleanup・artifact0/pending0を確認しました。独立した通常full-root planも31/30 no-opです。version固定GETでDev serial36/35・Prod32/31を照合し、同lineage・controls9・live lock/workspace0・安定GCP metadata不変を直接監査して独立review PASSです。 H7の最終no-op後healthも両環境の全5 stage・finding0を確認しました。これはH7の実証時点の記録です。最終統合Sourceでの同等性・authority/State closeout・正本同期とInfrastructure Readyの最終判断は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照します。
第30波と両環境の最終通常Plan/Apply・State監査の実行SourceはH7（`0ed7486b`）です。SHAは実証に用いるSourceの記録であり、文書自体のHEADではありません。最新の運用Sourceと最終runは[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照してください。H4の[#1364](https://github.com/kani3camp/youtube-study-space/pull/1364)はnative10 caseの上限2並列化とrouting/回帰を追加し、H5の[#1366](https://github.com/kani3camp/youtube-study-space/pull/1366)でnative runnerと回帰テストを修正した。#1366は30 checks（12 success/18 skipped）・独立review後に統合済み。#1368はloopback cleanupの観測テストを修正し、runner本体・runtime controller・認証workflow・HCLを変更していない。runtime controller5ファイル・認証workflow・HCLはH3/H4と同一で、H2初期化/H3監査・9件/H4第10波の実証を各時点の記録として保持する。
H7の[第20波run39025184389](https://github.com/kani3camp/youtube-study-space/actions/runs/39025184389)はPlan成功後、ApplyのPOST_PLANでreason=EXECUTIONとなり停止しました。元Apply／run全体の失敗と、詳細原因が未観測である事実を保持します。別のread-only監査でState20／直前19・serial21／20・同lineage、IAM member1件だけの追加、既存値・identity・private／sensitivity・checks不変、両version固定GETのFULL_OBJECT checksum・range／HEAD、controls9・Dev全HEAD不変・live lock／workspace0を直接照合し、独立review PASS。続く別LOCAL20診断は同H7の通常full-rootでimport0／no-op20／rawdrift0／mutation0を確認し、独立review PASSでした。これらを元runの成功へ読み替えず、Source変更・元run再送なしで新しい後続waveへ進めました。saved apply／StateDelta到達とAuth logはレビュー済controller／固定readerの実行に基づく間接証拠で、未取得のraw saved planや失敗原因を補造しません。

旧PROD_* gateや過去の手動Environment承認記録は、新経路の実readinessを証明しない。
resource0の初期StateだけでInfrastructure/MyPage ReadyGateをPASSにしない。

H7最終Stateの補足監査は、両環境のcurrent/prior計4版をversion固定GETし、本文とFULL_OBJECT SHA256・全range/HEAD metadataをnative証跡へ完全照合しました。Devは旧R10時点の全HEADを黙って更新せず、直前版がpinned serial35の本文/全HEADと一致し、新current36は公式Dev最終runの時間内に作成、serialとcheck_resultsの順序だけが変わった専用B判定を独立検証しました。Prodは32/31の30所有です。read/auth callsは41回で、controls9・両環境HEADの前後不変・lock/workspace/delete marker0を確認しました。この補足監査自身のState/lock書込は0です。歴史のraw saved import planは未取得で、歴史のStateDeltaやAuth logはreview済みcontroller/strict inspectorの成功に基づく間接証拠として区別します。

## Output-only initializer controller contract

[production_initializer.py](../../../.github/scripts/production_initializer.py)はcloud/State操作のないpure adapterで、
既存migration-plan/apply controllerからproduction target・manifest/resources0・成功したS3 snapshotの
head/raw/stateすべてnullの場合だけ選択する。新modeや別の実行権限は追加しない。
不在はexpected-owner付きの成功した厳密なListObjectsV2からだけ確定する。
403/404を含むCLIエラー、失敗read、既存State、dev、resource/import/drift/unknown/deferred actionを初期化へ流さない。

adapterは固定production/projectの2 output createを先に検証し、その2件だけを内部射影でno-opへ対応させる。
通常import-only契約とsaved binaryは変更しない。Coreがresource0で省略するresource_changesだけを空listへ正規化する。
専用operation marker、元のoutput changes、catalogued hintのpresence/content/path/重複数をprivate HMAC全体と固定名detailへ束縛する。
hintの外側順だけをentry全体のcanonical sortで安定化し、内側順や未知fieldは捨てない。
Terraform sourceと実行validator/adapterの追跡済みsource bytesをclosureへ含める。

plan/applyは同一SHA・同一private inputsから独立planし、overall/section/detail HMACの全一致を要求する。
source copyとsaved-plan seal、native lock、適用直前のpositive absence再確認、新VersionId/lineage・固定outputs2/resource0のState検査、
通常post-planのresource0・output no-op、metadata/State不変・lock解放を維持する。
新bucket/marker/dev State copyは作らない。State resource0はInfrastructure/MyPage Readyを意味しない。

CIは通常のcontroller mockに加え、実pinned Core/Google provider・production full default rootとloopbackのfake S3/STSで検証する。
init/plan/showのState PUT0、saved applyのみ最初のState PUT1、適用後の通常full-root no-op、conditional lockの解放・競合拒否、
caller accountと外側expected-owner/Versioning拒否を確認する。dummy認証・隔離HOME・外向きproxy拒否を使い、実cloudへ接続しない。
このsource接続はProduction gateや実State作成の完了を証明しない。

## Runtime WIF / API定義と採用条件

[production runtime-ownership.tf](../environments/prod/runtime-ownership.tf)はdevelopmentと同じ
共有modules、public grant alias、resource address、private inventory形式とimport ID組立を使う。
production projectだけを許可し、pool/providerはfalse、grant/API選択は空が既定である。
private metadataを与えるだけでは所有を開始しない。信頼条件・mapping・個別IAM conditionを再生成せず保持する。

pool → provider → exact individual grantの順で、それまでの所有を残して取り込む。
SA本体・全IAM policy/role binding・project IAM・CI federationは所有しない。
APIはenabled一覧から一括選択せず、productionの実callerとdependency evidenceを照合した後に
`Own`と分類した既存ENABLED serviceだけを明示する。developmentの分類結果は流用しない。
`disable_on_destroy=false`、`disable_dependent_services=false`、`prevent_destroy`を維持する。

既存readinessテスト入口は、資格情報/HOME/backendを使わないnative mockでproduction full rootと
development runtime定義のdefault0、累積address/import、values/sensitivity不変、順序/分類の拒否を検証する。
synthetic inventoryによるsource検証であり、production実inventory・caller分類・ownership・CI activationの
証明ではない。実行source準備当時の証明範囲を示すものである。現行migrationへの接続・gateの状態は冒頭の実行境界を優先する。

## Historical offline packet scope and evidence

source packetの作成基点は integration `feature/gcp-terraform-iac` のHEAD
`801afa4f00851d01c32eb8c054d067266272e814` です。実行開始時には最新remote HEADを再確認します。
既存[prod root](../environments/prod/main.tf)、[backend example](../environments/prod/backend.hcl.example)、
通知2 modulesを再利用します。共通state account/bucketはdevelopmentでbootstrap済みという
repository記録がありますが、このpacketでcloud側を再確認していません。

production backend/key/trust/importは実環境で未検証です。2026年10月15日の条件付き包括承認Aに加え、
実行者・reviewer運用は上記2026年10月16日のCurrent Canon更新を適用します。
承認範囲内の作業にChat再承認や各runの人間reviewer承認は不要です。devへのagent mergeは禁止です。
過去のdev成功をproduction gateの証拠にしません。dev history/runtime WIF/APIの作業は別workstreamです。
source準備、credentialless tests、最小read-only inventory整理はそれらと並行できます。

以下は旧経路のhistorical/offline contractsです。新migration経路の実readinessとは区別します。

- prod default graphはresource/import/drift各0。既存Email/quota ownershipはfalseのまま。
- prod rootへ別projectを渡すと停止。backend exampleはprod keyとworkspace discovery prefixへ限定。
- Email opt-inはcreate1、quota opt-inはcreate3であり、既存import-only sanitizerが拒否。
- 現行authenticated workflowのprod向け全modeはpreflightで停止し、job inputも生成しない。
- prod plan/applyは独立gate。source契約と実測development exitが未完の間はplanも閉鎖する。
- gateを開いたsynthetic testでもdev例外mode/入力、rerunを拒否し、prodの全ownership outputを明示する。
- offline backend proposalはexact prod state/lockとEnvironment別OIDC subjectを生成し、実値を公開しない。
- 専用PR CIはcredentiallessで、OIDC permission、Environment、secret、remote state操作を持たない。

これらはmocked plan/source guardの証明です。pinned providerのprod実Read、remote no-op、
cloud trust、現Environment保護、locking/recoveryの検証にはなりません。

## 現在の定義・実行・残件

共有export modulesはFunction/Schedulerの既存name、Functionのmemory/timeout/environmentをprivate metadataから保持する。
Production rootの `existing-resources.tf` はnative backup、backup bucket、retained dataset、履歴table2件、export topic/Scheduler/Function、quota3件の計11件を個別選択できる。
Runtime Pool・Provider・個別grant7・明示Own API10を合わせた移行対象は30件である。
rootの既定選択は空のままとし、実運用では完全manifestとreview済み固定baselineを用いる。Developmentの地域、保持期間、列順、IAM member、API分類をコピーしない。

Production enabled APIの分類baseline65件はOwn10／Platform・External5／Investigate50です。productionの実callerと同manifestの非API consumerへ結合したOwn10だけ既存ENABLEDのまま採用し、他55は未選択・enable/disable/delete0です。この65は分類baseline時点の件数で、常時再取得した現在値とは扱いません。CI SA2・CI Pool/Provider2・read custom role3の基盤7資源と個別policy5/member追加8はbootstrap-managed例外で、app rootの実owned30に加算せず、runtime Pool/Provider/grant7と区別します。

quotaは既存3件をimport専用定義で扱い、dangling通知先も現設定のまま保存する。
Production Email channelの作成・検証・通知修復は行わない。legacy Email/quota作成用opt-inとは混在させない。
Pool→Provider→個別grant→依存するOwn APIの順で、それまでの所有を保持して1件ずつ取り込む。
API enable/disable、SA本体・全IAM policy・CI federationのTerraform ownershipは追加しない。

| 項目 | 2026-10-17の実証 / 残件 |
| --- | --- |
| Development Exit | 通常plan/apply両identityの31 no-op・import0・rawdrift0、State/version/lock、観測可能な回帰、独立review PASS |
| Production backend / trust / settings | AWS roles2/policies2、GCP resources7・個別policies5/member追加8、設定14件を実readback・独立review済み。旧UNKNOWNは読み取りreconciliationと残り操作の別実行をjoinし、過去の成功responseを捏造しない |
| 実移行・検証Source / routing | 第12波はH6 `5418c687`で全CI・両Auth成功、managed12。#1370統合H7 `0ed7486b`で第30波と最終31/30 no-op・State監査まで実証。H5第11波の失敗とState11独立PASSは別の記録。最新の運用Source/runはIssue #1191。本人初回dispatch・専用integration・plan/apply別identity。`MIGRATION_PRODUCTION_READY=true`のみ有効、旧 `PROD_*` gateはfalseを保持。都度Environment人間承認なし |
| Production initial State | H2の[run38961710157](https://github.com/kani3camp/youtube-study-space/actions/runs/38961710157)で固定outputs2件/resource0作成、H3の初版version GET/checksum・controls9/Dev不変の直接監査と独立reviewもPASS。dev Stateはcopyしない |
| Production ownership / Exit | H3の[run39005902978](https://github.com/kani3camp/youtube-study-space/actions/runs/39005902978)まで9件・全CI成功を検証済み。H4第10波は認証付きPlan/Apply・post10成功後に静的CIとGateで失敗。State10/直前9のversion固定GET・厳密差分照合は独立PASS。元runの静的CI失敗を保持します。State10照合後にH5の[第11波run39021164334](https://github.com/kani3camp/youtube-study-space/actions/runs/39021164334)を1回発行しました。認証付きPlan/Applyは成功しましたが、native runnerのtimeout/grandchild回帰で静的CIとGateが失敗しました。State11/直前10のversion固定GET・厳密差分照合も独立PASSで、直前10の本文/HEADは第10波の独立確認済みcurrentと完全一致します。元R11の静的CI失敗を保持します。H6の[第12波run39022399909](https://github.com/kani3camp/youtube-study-space/actions/runs/39022399909)は全34 jobと両Auth成功、Plan import1/managed12、Apply post import0/no-op12/rawdrift0/mutation0、cleanup/artifact0/pending0を独立確認済みです。観測テストの修正[#1370](https://github.com/kani3camp/youtube-study-space/pull/1370)はrequired CI・独立review後に統合済みです。H7（`0ed7486b`）で第13波以降へ進み、[第30波run39029269335](https://github.com/kani3camp/youtube-study-space/actions/runs/39029269335)まで30件のownershipを確認しました。元失敗runは再送せず、その失敗判定を保持します。H7の最終通常実証はDevelopment [run39029559218](https://github.com/kani3camp/youtube-study-space/actions/runs/39029559218)・Production [run39029874116](https://github.com/kani3camp/youtube-study-space/actions/runs/39029874116)で、各plan/apply両identityのimport0・no-op31/30・rawdrift/leaf/mutation0、全34 jobの検証（選択job成功・非対象skip）・cleanup・artifact0/pending0を確認しました。独立した通常full-root planも31/30 no-opです。version固定GETでDev serial36/35・Prod32/31を照合し、同lineage・controls9・live lock/workspace0・安定GCP metadata不変を直接監査して独立review PASSです。 H7の最終no-op後healthも両環境の全5 stage・finding0を確認しました。これはH7の実証時点の記録です。最終統合Sourceでの同等性・authority/State closeout・正本同期とInfrastructure Readyの最終判断は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照します。 |
| Infrastructure / MyPage | H7の移行・技術実証はPASS。最終統合Sourceの同等性・authority/State closeout・正本同期を含むInfrastructure Ready判断はIssue #1191を入口にする。既存ownershipの移行基盤ReadyとMyPage provisioning/releaseを分離し、後続PRでtype/schema・create/update専用workflow/検証契約・最小IAM/API・費用/rollbackを整える |

## One minimal private read-only inventory packet

既存正規operator/connectorのread-only経路で、以下を一度に揃えます。新identity、追加permission、
secret、Environmentを作らず、権限不足はSTOPとして記録します。実値の引き渡しは既存private経路のみ。
credential/token、signed URL、raw state/planを会話・public repository・Actions artifactへ送りません。

| Area | Minimal metadata / purpose | Permission or operation boundary |
| --- | --- | --- |
| Shared S3 control plane | 確認済みaccount/bucket/region、Versioning、encryption、BPA、bucket policy、ownership、prod keyの存在/versions/delete markers、exact lockの有無 | Exact bucket設定GET、対象keyだけのHEAD/version metadata。本文/state download不要。absent/AccessDenied/通信失敗を区別する |
| AWS backend roles | prod roleの存在/collision、RoleId、trust、attached/inline policies、boundary/SCPによる実効制限、既存dev roleからprodへの境界 | Exact role GETとそのpolicy metadata。必要なpolicy一覧のみ。新role/provider/permissionなし |
| GitHub trust/settings | 現行repository OIDC subject template、prod plan/apply Environmentの存在、reviewer/self-review/bypass、branch restrictions、secret **names** の存在 | Settings read不可なら未確認。secret値read不要。CodeQL/OIDC制約を解消するためにtrustを弱めない |
| GCP Terraform CI trust | prod側CI pool/provider/plan-SA/apply-SAの存在または不在、mapping/condition、exact SA IAM、roles/grants/API依存 | 既存CI resourceのexact GET。AWS runtime WIFとは別。project番号/member全文/role実値はprivate。permission追加なし |
| First import candidates | 包括承認AのOwn条件を満たす候補だけについてexact ID/location/provider Read fields、IAM dependency、生成resourceとの境界 | 最初はsmall waveに必要なmetadataだけ。data本文、query、export、source download、Function invocationなし |
| Recovery/operator path | 既存SSO/短期operator経路、prod version復旧を行える主体とrunbook、期限/cost条件、既存dev recoveryの参照 | 新backup/restore testを勝手に実行しない。#1165/#1166の完了を推測しない |

state key不在はstate bucket不在を意味しません。既存共通bucketを再利用し、prod用のkey/roles/trustを
確認します。S3 prefix用marker objectや別bucket/GCS backendを「準備」として作りません。
prod keyが既にあればowner/lineage/versionを確定するまでinit/importを止めます。

各metadataには取得時刻、実operator/target、source SHA、比較対象と判定をprivateに添えます。
phase移行やapproval直前に対象だけfresh再readし、snapshot期限を過ぎた値を流用しません。
公開報告は存在/不在/未確認、差分件数、Own分類とSTOP理由のみです。

## Backend and trust design to make reviewable

[S3 backend公式](https://developer.hashicorp.com/terraform/language/backend/s3) に従い、
default workspaceのprod state keyと`.tflock`を分離して権限をreviewします。

| Capability | Protected plan role | Import-only apply role |
| --- | --- | --- |
| State object `youtube-study-space/prod/terraform.tfstate` | Exact GetObject、Put/Deleteなし | Exact GetObject/PutObject、Deleteなし |
| Native `.tflock` object | Exact GetObject/PutObject/DeleteObject | 同じexact3操作 |
| Bucket ListBucket | prod state/discoveryに必要なexact prefixesだけ | 同じ。dev/他product prefixなし |
| GCP provider Read | waveで実証したexact metadata GETと最小認証能力だけ | 同じRead。import-only applyはworkload writeを必要としない |
| Recovery/version restore | operatorの別scope | CIへ自動追加しない |

planもnative lockの作成/削除を伴います。remote init、first state/output write、state移行、lock操作を
単なるread-only inventoryへ含めません。`-lock=false`、`-target`、lock file手動削除でgateを迂回しません。
default workspaceだけを使い、workspace discoveryはprod配下へ固定します。workspace作成のために
state ARN wildcardを足しません。既存encryptionがSSE-S3なら維持し、KMS移行やgrantは別scopeです。

prod smokeを準備するとき、stateがabsentの状態で条件付きPutObjectを「必ずdenyされるread」と
扱いません。許可が誤ってあればobjectを作成します。negative test自体の最大mutation/costを先に
reviewし、包括承認Aの安全条件を満たす独立unitへ分離するか、書込みを試みない検証へ限定します。404やnetwork errorをdeny PASSにしません。

[AWS OIDC trust公式](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-idp_oidc.html)、
[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、
[GCP WIF deployment-pipeline公式](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-deployment-pipelines)
と、**実際の**subject template/mappingを照合します。private exact diffにはimmutable repo/owner ID、
trusted integration ref、caller/reusable workflow、event、plan/apply Environmentとaudienceを含めます。
repo名やEnvironment名だけでworkflow限定済みとみなしません。repo OIDC templateを変更する案は
既存dev/AWS/GCP/CodeQLへの影響を含む別判断です。このpacketは既存templateを変更しません。

現行の4 EnvironmentはRequired reviewersなし・admin bypass=false・integration限定を実API値と照合します。
本人初回dispatch制限を保ち、都度のEnvironment人間承認を再要求しません。既存secret名を再利用する場合もEnvironmentごとの実role/SAを
privateに照合し、planとapplyを同じidentityへ暗黙にまとめません。承認範囲内のEnvironment/Secrets構築は実値をprivateに保ち、保護条件を維持して実施します。

## Separate protected execution units

Development Exitは実測完了済みです。以下のunit順でProductionを進めます。source PRのreview/mergeは
live readinessを兼ねません。各packetはexact targets/before-after/impact/window/rollback/verificationを
完成させてから実行します。包括承認A内は都度Chat/Environment人間承認不要とし、
本人初回dispatch、branch/trust/必要CIと独立レビューを維持します。

| Unit | Concrete reviewed execution packet | Result needed before next unit |
| --- | --- | --- |
| P1 Backend/security bootstrap | 共通bucket再利用、prod backend role/trust/policyのexact diff、Environmentとprivate inputs、GCP CI trust/SA/grantsのexact diff。新bucket/accountは含めない | 設定read-back、dev/他product不変、各roleの境界証拠。失敗時は追加grant/trust/inputsだけを撤回 |
| P2 Authenticated plan / initial state | 別reviewed source PRのprod plan routing/smoke、必要ならempty state/output初期化とlockの最大diff | 実AWS/GCP identity、prod state snapshot/version/lineage/resources0、実lock解放、negative probes、public output監査。一般resource createは閉鎖 |
| P3 Exact import wave | Fresh Own候補、exact definition/import ID、full-root pre-plan、provider実Read、state before/after manifest、独立review・本人初回dispatch | Same-SHA再plan/projection一致、import-only state更新、post import0/no-op、remote metadata不変、lock解放 |
| P4 Exit verification | Approved ownership一覧と未完項目判断、dev/prod通常full-root、trust/state/recovery証拠、正本同期 | Infrastructure Readyを親が判定。MyPage deploy/releaseは別gate |

P1はIAM/Environment/backend/GCP trustの対象別diffを同じ判断packetへ束ねます。実行権限・rollbackが
異なるので、承認scopeを明示します。bucket全policyやSA全policyを古いsnapshotで上書きしません。
stateの復旧は単なる古いversionのcopyではありません。fresh post-write/ownershipを保持する
正規手順をreviewし、blind restoreを行いません。

## Production import acceptance

devと同じresource数/順序/retention/region/IDを機械的にコピーしません。
fresh inventory → Own/External/Google-managed/Investigate分類 → exact定義 → full-root import-only plan →
独立review・本人初回dispatch → plan/apply別identityでsame-SHAの全HMACを照合する再plan → exact import-only apply → post通常full-root no-opの順です。

- configured change、create/update/delete/replace、unknown、説明不能action、未許可driftはSTOP。許可されたBigQuery computed-only2leafの観測差分は現行定常契約に従い、post通常planではraw drift0を確認する。
- exact expected importだけを許可し、既存managed resource全件はno-op。import数だけでPASSにしない。
- full-rootにdeferred/action invocation/address move/output変化等があればwave契約で説明・拒否する。
- 原則1 resourceまたは分離不可能な最小unit。resource typeのdev実績を先に確立する。
- generated subscription/artifact/source bucket/service agents、project IAM全体、用途不明resourceは巻き込まない。
- notification repair、schema mutation、API enable/disable、Function rebuild/deploy、manual triggerを混ぜない。

quota3の通知先参照はreview済み既存metadataどおり保持します。Email channelの新規作成・delivery/receipt検証・通知修復は今回行いません。
legacy Email/quotaの作成用opt-inはcreateを提案するため、import準備や修復としてenableしません。既存resourceは上記のimport専用選択を使います。
通知修復はprivate field-mask/rollbackを揃えた別operator waveです。
#983/#1165/#1166は別Issue、D01のholdはこのscope外です。

## Offline backend proposal

[production_backend_packet.py](../scripts/production_backend_packet.py) はcloud API/CLIを呼ばず、
fresh private inventoryから一つのprivate proposalを生成します。既存OIDC subject templateの順序が
[development smoke](../../../.github/scripts/terraform_identity_smoke.py) のcontractと一致しない場合はSTOP。
account/bucket/region/role名を推測せず、prod/dev4 identitiesの衝突を拒否します。

権限は上表のexact State/lockと、exact default state key・native lock key・prod workspace discovery prefixに限ります。
planはState Put/Deleteなし、applyもState Deleteなし、両方ともdev/other-product/object-version/KMS/IAM/API
権限なし。これは生成された単独policyの提案であり、attached policy、bucket policy、SCP、session policyを
含む実効権限や実際のdeny証拠ではありません。実identity/negative probes/read-backはP1/P2で別途検証します。

private inputは次の**全fieldのみ**を持つowner-only JSONです。credential/tokenは入力しません。

| Field | Required observed value |
| --- | --- |
| `schema_version` | `production-backend-private-inventory-v1` |
| `source_git_sha` / `observed_at` | packet作成対象の40桁SHA / timezone付きinventory取得時刻。次の実行前にfresh再readする |
| `state_account_id` / `state_bucket` / `state_region` | 既存共通State control planeの確認済み実値 |
| `state_status` / `bucket_encryption` | exact prod keyの`absent`・`empty`・`existing`、既存SSE-S3の`AES256`。403/unknownは拒否 |
| `production_plan_role_name` / `production_apply_role_name` | collision確認済みのprod専用role名 |
| `development_plan_role_name` / `development_apply_role_name` | fresh確認した既存dev role名。prodとの衝突防止のみ |
| `oidc_include_claim_keys` | 順序付き `repository_id, repository_owner_id, environment, ref, workflow_ref, job_workflow_ref, event_name` |

input/outputをcheckout外のprivate directoryへ置きます。inputはowner-only、outputは0600で排他的に
新規作成し、既存packetを上書きしません。stdout/stderrは固定labelだけです。

```sh
python3 infra/gcp/scripts/production_backend_packet.py \
  --inventory /private/path/production-backend-inventory.json \
  --output /private/path/production-backend-proposal.json
```

上記pathはprivate operatorが解決する例です。outputに実値を含むためGitHub/Notion/Actions artifactへ
公開しません。GCP trust/SA/GET grants、GitHub Environment settings、initial state/recovery、exact import
waveはこのgeneratorの範囲外であり、P1/P2/P3 packetへfresh実測のexact diffを追加します。
新bucket/account/marker object、CloudTrail継続課金、repo-wide OIDC template変更を生成しません。

根拠は [Terraform S3 backend](https://developer.hashicorp.com/terraform/language/backend/s3)、
[AWS OIDC trust](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-idp_oidc.html)、
[GitHub OIDC subject](https://docs.github.com/en/actions/reference/security/oidc)。IAM action名は
[AWS service reference S3](https://servicereference.us-east-1.amazonaws.com/v1/s3/s3.json) と
[STS](https://servicereference.us-east-1.amazonaws.com/v1/sts/sts.json) で確認しました。
SDK sourceからruntime権限を推定するpacketではなく、上表で明示されたbackend操作に限定します。

## Offline production execution contract

[production_execution_contract.py](../scripts/production_execution_contract.py) の作成基点は
integration `18d41d08bd8221e11db51d5f9f093d19784327e3` です。このscriptはcloud/CLI/Terraformを
呼ばず、private collector inputを検証するだけです。このbundle CLI自体はprotected executionの入口ではありません。現controllerはinitializerで一部pure検証を再利用しますが、CLI単体から実認証・State操作は行いません。
結果の `live_readiness_attested=false` を維持し、offline PASSを実認証・Environment approval・
import完了・Infrastructure Readyの証拠として使いません。

private bundleは以下のexact契約です。raw State/plan、provider values、metadata本文は公開しません。

| Field | Required contract |
| --- | --- |
| `schema_version` | `production-private-validation-bundle-v1` |
| `contract` | version `production-execution-contract-v1`、same `git_sha`、UTC `issued_utc`/`expires_utc`（最大24h）、exact prod `project_id`/private `project_number`/`state_bucket`/`state_key`、`wave`、`adopted`、`candidate` |
| `adopted` / `candidate` | fresh inventoryからreviewした全既存ownershipと追加1件。各descriptorは`address`/`type`/`schema_version`/exact `import_id`/完全なprovider `values`。Developmentの件数・設定・順序を仮定しない |
| `before` / `after` | exact `snapshot` と、全選択addressの`metadata`。metadata envelopeはprod `project_id`/exact provider `resource_id`/完全なGET `payload`。前後をstrict比較し、unknown/volatile fieldも黙って除外しない |
| `snapshot` | UTF-8 raw `state_body`（最大4MiB）、`head_before`/`get_metadata`/`head_after`、exact State/lock/workspace `*_listing`。HEAD/GET version・ETag・length・SSE-S3をjoinし、positive non-truncated LISTを要求する |
| `plan_before` / `plan_after` | 完全なpinned Terraform plan JSON。全選択address/provider/schema/values、outputs、import IDをjoinし、後段は通常full-root no-op |

waveは混ぜません。

- `initialize`: positive exact State LIST0、State/HEAD/GETなし、adopted/candidate/resource0、既存rootの
  `environment=production` と `project_id=youtube-study-space` の2 output作成だけを許可。
  後段Stateはversion4/valid lineage/serial、resource0、post full-root no-opを要求する。
- `import`: initialized existing Stateと既存全ownershipを要求し、exact candidate1件のみimport。
  通常retained typeのimport IDはprovider `values.id`へ完全一致を要求し、SA IAM memberだけはpinned
  space区切りimport IDと、condition全fieldを含むslash区切りState IDを個別に組み立ててjoinする。
  全plan actionはno-op、lineage/version/outputsと旧container/instance/private属性を保持し、serial増加と
  新S3 VersionIdを要求する。persisted `check_results` の変化も現contractではSTOP。
- `noop`: import0、同じState本文/VersionId/HEAD/GET/listings、同じmetadataと全ownership no-opを要求する。

initial State分類のうち `initialize` が扱うのはpositive LISTで証明した **absentだけ** です。
既存resource0のStateがproductionの2 outputsを既に持つ場合は`noop`で検証できます。
既存空Stateに`outputs={}`しかない場合、旧CLI/schema/不明outputsがある場合は未対応としてSTOP。
既存lineage/versionを保持するoutput-only adapterは別source条件であり、このpacketで全初期化ケースが
実装済みとは扱いません。fresh inventoryがどのケースかを確認するまでlive初期化を選択しません。

whole project/SA IAM policy/binding、未知resource type、未知schemaは拒否します。allowlistは既存migration
sourceのretained typesだけで、Google providerのschemaはbucket4/その他0。SA IAM memberは既存runtime用の
`roles/iam.workloadIdentityUser` とexact project-number配下の`principalSet`に限定します。一般IAM principalの
case normalizationは未対応でSTOP。SA/member/condition titleのwhitespaceもpinned importerのtoken解釈が
曖昧になるためSTOPし、必要なら別typed adapterをreviewします。APIは1件importの独立wave、disable flags falseを要求します。
create/update/delete/replace/read/forget、unknown、drift、deferred/action invocation、address move、deposed、
output差分、root graphの欠落/追加、失敗したGET/LISTや403をabsenceとして扱う入力はすべてSTOP。
Stateのidentity schema versionは実証済みのomitted/0だけを許可し、未知versionを受理しません。

```sh
python3 infra/gcp/scripts/production_execution_contract.py \
  --bundle /private/path/production-validation-bundle.json \
  --git-sha FULL_REVIEWED_COMMIT_SHA
```

inputは全checkout外のowner-only regular file（bundle最大16MiB）。symlink/FIFO、duplicate JSON key、
non-finite値を拒否します。stdoutはprivate値を含まないcountsとoffline status、stderrは固定STOPだけ。
`resource_change_actions=0` は検証したplanのresource action数であり、実cloud変更件数を報告しません。
scriptはStateやreceiptをwriteせず、誤ったStateのrestore・lock削除・API操作を行いません。

**このoffline CLIが証明しない範囲:** actual AWS/GCP identityとnegative probes、expected bucket owner/roleの検証、
complete metadataとprovider valuesを結ぶresource-specific adapter、authenticated HEAD/GET/HEAD/LIST collector、
same-SHA re-plan/保存plan sealとplan/apply別identity、post full-root/no-opと実workload回帰、private cleanupが
このoffline CLIだけでは確認できません。現行controllerの実結果は冒頭の実行境界とIssue #1191で判断します。渡されたsnapshotの真正性・full-root実行・writer静止をこのoffline scriptは証明しません。
APIのOwn classification/dependency provenance、WIF trust/mapping、SA IAM grantの必要性と実caller/targetのjoinは
typed waveの残条件です。このvalidatorのtype/shape PASSからOwn分類を自動判定しません。
metadataのvolatile差分や候補check追加が必要なら、exact候補の別reviewed adapterを実装するまでSTOPを保持します。
このsource準備だけでは `PROD_EXECUTION_CONTRACT_READY` をtrueにしません。

## Critical path and remaining work

**実測Development Exit → P1 → P2 → P3各wave → P4** を維持します。
Development ExitとP1/P2は実証・独立review済みです。P2はH2初期化とH3初版Stateの直接監査を区別して記録しました。
P3はH3で9件・全CI成功を検証済みです。H4第10波は認証付きPlan/Apply・post10成功後に静的CIとGateで失敗しました。State10/直前9のversion固定GET・厳密差分照合は独立PASSで、元runの静的CI失敗を保持します。H5の#1366を統合し、State10照合後にH5の[第11波run39021164334](https://github.com/kani3camp/youtube-study-space/actions/runs/39021164334)を1回発行しました。認証付きPlan/Applyは成功しましたが、native runnerのtimeout/grandchild回帰で静的CIとGateが失敗しました。State11/直前10のversion固定GET・厳密差分照合も独立PASSで、直前10の本文/HEADは第10波の独立確認済みcurrentと完全一致します。元R11の静的CI失敗を保持します。H6の[第12波run39022399909](https://github.com/kani3camp/youtube-study-space/actions/runs/39022399909)は全34 jobと両Auth成功、Plan import1/managed12、Apply post import0/no-op12/rawdrift0/mutation0、cleanup/artifact0/pending0を独立確認済みです。観測テストの修正[#1370](https://github.com/kani3camp/youtube-study-space/pull/1370)はrequired CI・独立review後に統合済みです。H7（`0ed7486b`）で第13波以降へ進み、[第30波run39029269335](https://github.com/kani3camp/youtube-study-space/actions/runs/39029269335)まで30件のownershipを確認しました。元失敗runは再送せず、その失敗判定を保持します。同H6のdocs PRでECONNRESETによる観測テスト失敗を確認し、#1370の観測テスト修正をH7へ統合済みです。閉鎖成功はECONNREFUSEDだけとし、他の正常な整数結果は期限まで未閉鎖として再観測し、継続すればFAIL、型不正・例外は即FAILを維持します。native並列化による実運用時間の改善完了とは扱いません。P4のH7最終通常full-root・State/version/lock・両環境の観測回帰は実証済みです。最終統合Sourceの同等性・authority/State closeout・正本同期を含む最終判断は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照します。
resource0、source PR、provider-mocked planだけではInfrastructure Readyにしません。

[ReadyGate](phase2-approval-and-readiness.md#development--production--mypage-readygate)は、各実証と未完項目の判定を揃えて更新します。
MyPageのInfrastructure追加準備は、既存ownershipと正規CI/Stateの安定性を確認する判定です。
現在のmigration workflowは13type・exact import ID・resource no-opを扱い、create/updateを拒否します。新規MyPage resourceは後続normal-change PRでtype/schema、専用の実行・検証契約とworkflow、最小IAM/API、費用・rollbackを整えます。
MyPage provisioning・Production release・data migration/機能検証を移行完了から自動承認しません。

### 移行を止めない独立残件

以下はすべてOPEN・Acceptance未達です。今回の既存ownership移行のblockerにはせず、完了扱いもしません。

- [#1165 最新判断](https://github.com/kani3camp/youtube-study-space/issues/1165#issuecomment-6223368027): contacts・MFA実運用・独立break-glass・復旧責任をMyPage Production release前に判断。State current/prior GETとVersioning確認は復旧訓練ではない。
- [#1166 最新判断](https://github.com/kani3camp/youtube-study-space/issues/1166#issuecomment-6223368703): durable CloudTrail/S3 data eventsの保存先・範囲・retention・費用は別途判断。今回のimportのためだけに継続費用を追加しない。
- [#983 最新判断](https://github.com/kani3camp/youtube-study-space/issues/983#issuecomment-6223369320): 過去raw-chat/混在GCS exportの破壊的cleanupは対象外。Privacy/DRの独立laneで継続し、保持設定を今回のimportで変更しない。

Production quotaのdangling通知先、Emailのdelivery/receiptは未修復・未検証です。通知経路Readyの主張には使いません。
CodeQL [PR #1362](https://github.com/kani3camp/youtube-study-space/pull/1362)は本人の明示承認後にDefault setupを `not-configured` へ切り替えました。H2追随候補Q2（`92b5ea29`）の全31 CIと[run39001207914](https://github.com/kani3camp/youtube-study-space/actions/runs/39001207914)の3言語解析・SARIF受付が成功し、analysis ID3件・error0、既存open alert1件の保持を確認済みです。
候補と全tree一致のintegration H3（`8b2289c8`）へ統合済みで、H3 pushの[run39001594887](https://github.com/kani3camp/youtube-study-space/actions/runs/39001594887)でも3言語の解析・SARIF受付成功、error0を確認済みです。GCP closureはH2と完全一致し、TerraformのH2当時の正規Plan/Apply証拠と区別します。先行拒否・upload失敗の履歴は[history packet](development-history-ready-packet.md#codeql-and-mandatory-oidc-claims)へ残します。
integration refの未解決CodeQL finding6件（critical1/high3/medium2、既存1＋新検出5）は別対応で、dismiss/suppressしません。受付成功を脆弱性0と扱わず、第12波Source H6・最終移行実証Source H7とH3の解析実証を区別します。

## Offline verification

```sh
python3 infra/gcp/tests/test_production_readiness.py
python3 infra/gcp/tests/test_production_backend_packet.py
python3 infra/gcp/tests/test_production_execution_contract.py
terraform fmt -check -recursive infra/gcp
terraform -chdir=infra/gcp/environments/prod init -backend=false -input=false -lockfile=readonly
terraform -chdir=infra/gcp/environments/prod validate
bash .github/scripts/test-detect-ci-paths.sh
```

testはsource/lock/modulesをfresh temp treeへcopyし、mock providerのplanだけを実行します。
local backend.hcl/state/tfvars/cacheをcopyせず、raw mock planをpublic出力しません。
[専用CI](../../../.github/workflows/gcp-production-readiness.yml) も同じtestをPRで実行します。CI callerの`ci.yml`やdevelopment gate値は変更しません。
