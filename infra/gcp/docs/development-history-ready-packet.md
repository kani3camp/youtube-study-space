# Development history: one decision packet, separate execution boundaries

Goal: `user-activity-history` のschema整理とTerraform adoptionを、安全に判断できる一つの
準備bundleにまとめる。code/tests/specは一つのdraft PRで更新し、証拠が揃った最終候補を
一回reviewする。writer deployment、データ/schema変更、state変更、IAM/security、本番は
それぞれ影響が異なるため、実行scopeと承認を混ぜない。

公開するのは最小specのみ。exact runtime ARN/revision/digest、owner回答、元schema、復元記録、
IAM inventory、raw plan/state、実集計値はprivate operator packetへ保存する。
入口は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)。
[auditの意味とschema修復方針](user-activity-history-canonicalization.md)は維持する。

## 完了済みhistory adoptionの旧実行枠

history Tableは既に12件目としてTerraform-ownedで、canonical8列を実metadata/Stateで確認済みです。
再DROP・再importは行いません。original apply final receipt欠落と
`historical_delta_attested=false`は保持し、現行運用は
[定常Plan/Apply](runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)を優先します。

[execution window contract（旧版の固定履歴）](https://github.com/kani3camp/youtube-study-space/blob/a94eea6474ae6f89c679e306b291701209607718/infra/gcp/docs/development-history-execution-window.md)は、当時の本人承認付きhistory-only
importと独立no-op12を扱った旧手順です。過去のgate/Environment承認/receiptを
現行運用の追加条件へ読み替えず、欠落した歴史証拠は現在の再検証で復元しません。

Developmentの現行ownershipは[run38945893535](https://github.com/kani3camp/youtube-study-space/actions/runs/38945893535)で31件・import0/no-op31/rawdrift0をplan/apply両identityで検証済みです。DevExit後のfresh自然回帰はLambda3/6で1,338実行・Errors/Throttles0、全5 stage・finding0を独立review済みで、quiet3/未観測の限界を保持します。 H7の[最終run39029559218](https://github.com/kani3camp/youtube-study-space/actions/runs/39029559218)も両identityで同じ31 no-opを確認し、独立native/State監査は36/35版・属性/private/identity/mask/output不変、最終cutoff後のfresh healthは全5 stage・finding0・Lambda3/6で19実行でした。DevExitの旧観測と最終H7の観測を別時刻の記録として保持します。
以下のschema repair、deployment候補、one-shot承認、旧Environment review手順は当時のpacketです。現在のhistory再DROP・再importや、ユーザーへの再入力依頼を要求しません。
現行の運用は[定常Plan/Apply](runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)、最終判断は[ReadyGate](phase2-approval-and-readiness.md#development--production--mypage-readygate)を優先します。

## Evidence needed for a decision

| Decision | Evidence and acceptance | Insufficient evidence |
| --- | --- | --- |
| Legacy values | 既存auditのSummaryと取得元、snapshot時刻、以後の全writer対応をprivate照合。監査時点のzeroから不要なbackfillを追加しない | 昔のzeroだけで現在の値を保証しない。再queryは自動追加しない |
| Actual writer guard | 日次・手動のcurrent StateMachine reference → exact task revision/image digest → reviewed source/build/deploy receiptの対応。#1192と同じguard codeが実imageにあること | source merge、ACTIVE、stack更新日時、revision番号、digest単独では証明できない |
| Export/append path | 実user-activities export/loadとtmp metadataの既存対応証拠、canonical required fields、全writer coverage | tmpは他collectionで上書きされる。system fieldsだけの一時schemaはuser-activitiesの証拠にならない。job DONEもsuccessではない |
| Consumers | 外部BI/Redash、cross-project views/jobs、保存/手動SQL、legacy SQL/wildcard/列位置依存についてownerが現行利用と列依存を確認 | 同datasetのVIEW0や一projectのjob一覧だけで外部consumerなしとは言えない |
| Metadata | exact dev table/location、nested/modes/相対列順、annotations、expiration/partition/clustering/CMEK/constraints、complete row-policy list | 古いinventory、部分response、policy listの403を0として扱わない |
| Recovery | 元schema/列順、復元source cutoff、短期保存期限、既存復元proof、DROP後の新write保持、必要な既存権限とcost | time-travel設定、ADD COLUMN、table overwriteだけではproofにならない |

sourceのAWS targetは[CDK README](../../../aws-cdk/README.md)、両writerは
[CDK stack](../../../aws-cdk/lib/aws-cdk-stack.ts)が正本。construct IDをphysical ARNと同一視しない。
operator readbackをchildの独立live検証と区別する。古い日時とguard mergeの前後関係だけで
絶対に未deployとは断言しない。

## Conditional execution packet

一つの判断packetで以下のtarget/diff/権限/cost/STOP/rollbackを見渡せるようにする。
準備の承認や一般の「進めて」はlive実行承認ではない。各段階のexact scopeを完成させてから
action-time承認を得る。途中で範囲を広げたり、不足permissionを追加したりしない。

| Stage | Exact candidate and execution condition | Failure boundary |
| --- | --- | --- |
| Writer proof / bounded deployment if required | 既存receiptでguardを証明できればdeploy不要。証明できなければ既存CDK管理でbatch Imageの最小差分を具体化。共有familyの日次reset/update/transferと手動3処理を影響範囲に含め、roles/network/configを保持。rollbackはprevious digestの新revision | unqualified familyはACTIVE revision登録だけでも切り替わり得る。登録を準備扱いしない。Lambda image等を変えるwhole-stack synthや元runtime不明のままdeployしない |
| Schema-only repair | dev `test-youtube-study-space.firestore_export.user-activity-history`、location `asia-southeast2`、timestamp DROP 1 statement。全preflightと値安全性・consumer・recoveryがPASS、既存operator権限内で実行 | unknown、新writer、column/config変化、非zero等ならSTOP。backfill/再作成/順序統一/retryを追加しない |
| Post-repair metadata | fresh GETでtimestampだけ消失、他8列・nested・相対順序・config不変。既存helperでprivate default-off inputsを準備 | 修復失敗やcanonical以外ならimportしない |
| Import-only adoption | pinned providerの実isolated planとapproved full-root plan、exact table import1 + existing11 no-op、その他0。reviewed source activation/validator配線とsame-SHA再plan、独立protected approval後saved plan適用 | import+updateは必ず拒否。state/lock/caller/SHA差異ならSTOP |
| Independent post-check | import0/no-op12/unknown・drift・その他0、exact ownership/state lineage/lock、通常full-root no-op | 全stateの古いsnapshotで後続writeを上書きしない。unexpected diffを自動applyしない |

監査後の値安全性を既存writer/export証拠から保証できない場合、追加aggregateが必要かを
packetで判断する。query/dry-runのexact scope、identity、cost、上限、cleanupは新しい承認対象。
disabled audit SAの再利用・新grant・自動re-auditを含めない。

candidate DDLは[canonicalization文書](user-activity-history-canonicalization.md#separate-live-waves-all-still-unapproved)
にある1 statementだけ。[BigQuery DROP制限](https://docs.cloud.google.com/bigquery/docs/reference/standard-sql/data-definition-language#alter_table_drop_column_statement)
をconsumer/recovery方式に照合する。新backup/snapshotや復元試験が必要なら、target/期限/
権限/cost/新write保全の具体案を先にreviewし、作成・query・restoreを別承認する。

## Offline managed batch deployment candidate

[batch:prepare-candidate](../../../aws-cdk/lib/batch-image-candidate.ts)はcloud clientを持たず、
既存dev `AwsCdkStack`のtemplateとCDK artifactからcandidate/rollbackのcloud assemblyを作る。
必要なprivate inputはdeployed `GetTemplate` Original、同stackの`DescribeStacks`からStackId/
StackStatus/RoleARN/EnableTerminationProtection、既存CDK synthまたはdeploy receiptのstack artifact、
`DailyBatchTaskDefinitionArn` output、元task logical ID、既存operator記録のcurrent task ARN、
同一既存ECR repositoryのcandidate/previous **digest URI**。
source synthだけを実deployed templateの代用にしない。old task/imageとtemplateの対応、receiptの
鮮度、candidate source/build→ECR digest、他のfamily呼出し元のcoverageはoperator側で照合する。

```sh
cd aws-cdk
pnpm batch:prepare-candidate --input /private/batch-candidate-input.json --out /private/fresh-candidate
```

inputのJSON keysは`baselineTemplate`、`stackArtifact`、`stackReceipt`、`taskLogicalId`、
`currentTaskDefinitionArn`、`candidateImage`、`previousImage`。`stackArtifact`はmanifestの
`artifacts.AwsCdkStack`、`stackReceipt`は上の5fieldsのみ（outputは同名keyへprojection）。
current ARNとCDK outputのrevision不一致を拒否する。task ARN依存はsymbolic revisionと
`Join/Split/Select`で検証し、revision依存の残る値と証明できないintrinsicを拒否する。
現行4 IAM policyの`family:*`はrevision非依存で有効権限が変わらない。CloudFormationが再評価/
再適用する可能性は承認scopeに含め、実change set確認は別承認後とする。task outputは更新後に
新revision ARNへ変わる。inputはowner-only regular file、outputは新規directory限定。
consoleは値やpathを出さない。raw input、template、review、assembly、image artifactを公開CI/
PR/Issueへ添付しない。公開testsのdigest/receiptは合成fixturesでlive proofではない。

生成物は`original.template.json`、`candidate/`、`rollback/`、`review.json`。candidateは
既存taskの`ContainerDefinitions[0].Image`だけ変更し、IAM/roles、network、JOB、SFN、Lambda、
parameters、outputs、全logical IDを保持する。rollbackも同じfieldだけをprevious digestへ変更する。
元templateのtag再利用ではold image復元を保証できないため、previous digestを明示的にpinする。
CloudFormationの[ContainerDefinitions更新はreplacement](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-resource-ecs-taskdefinition.html#cfn-ecs-taskdefinition-containerdefinitions)。
rollbackは元revision番号への復帰ではなくold digestを使う新revisionの登録になる。
[RunTaskのfamily参照](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html)は
revision省略時latest ACTIVEを解決する。SFN定義が無差分でもreset/updateを含む全呼出し元が影響する。

assemblyは既存CDK deploy/execution roleとbootstrap条件、現行termination protectionを保持する。
old template S3 URLとasset dependenciesを除き、asset publicationを0にする。
candidate imageは別途承認された既存ECRへのpublishとdigest照合が必要。生成器はbuild/push/
registration/change set/deployを実行しない。sourceの通常full synthは共有`system/`の変更で
Lambda assetにも波及するため、この最小候補の代わりに適用しない。次回通常CDK更新では
このpinned imageとの整合と全asset差分を明示的にreviewする。

private reviewには1fieldのbefore/after/rollback、template内全family taskとJOB、hashが入る。
入力templateを越えるlive inventoryやtask稼働状況の証明ではない。familyを特定できないASL、
他repository/tag、production、role不一致、runtime変化はSTOP。最終approvalはexact baseline/hash、
build/image、全family window、既存権限、cost、old digest保持とrollback条件を含める。

## Offline plan review already prepared

[validate_user_activity_history_plan.py](../scripts/validate_user_activity_history_plan.py) は
cloud clientを持たず、private canonical metadataとcomplete full-root plan JSONを検査する。
既存11resourceは既存Function post契約を再利用し、new tableだけを加えた12resourceを要求する。
import1/other0とpost import0/no-op12、exact table/provider、canonical8/nested/実列順/列description一致、
deletion protection、unknown/drift/追加import/output変化を検査する。
Terraform 1.16.4の[JSON plan定義](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/command/jsonplan/plan.go#L67)
はresource変更と別にaction_invocations/deferred_action_invocationsを持つため、両方とも
省略または空配列だけを許可する。resource no-opでもactionを含むplanは拒否する。
同様に[previous_address](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/command/jsonplan/plan.go#L275)
を持つmove-only resourceもno-op表示になり得る。全12resourceのstate addressを保持するため拒否する。

```sh
# 既存private operator環境のexecution SA email変数だけを参照。credentialは入力しない。
python3 infra/gcp/scripts/validate_user_activity_history_plan.py \
  --phase before --metadata /private/canonical-table.json --plan /private/approved-plan.json
python3 infra/gcp/scripts/validate_user_activity_history_plan.py \
  --phase post --metadata /private/canonical-table.json --plan /private/post-plan.json
```

`TF_VAR_export_function_execution_service_account_email` は既存Function identityの照合用。
inputはowner-only regular file、symlink/duplicate JSON key/oversizeを拒否。結果はvalue/pathを
含まないPASS/STOPのみ。実provider plan、metadata provenance、live prerequisitesや承認の
証明ではない。protected plan配線は下記に準備済みで、apply gateはfalse。
既存11のownership/configが変わったら候補を再reviewする。12という数だけでPASSにしない。

## Protected history plan, with apply still disabled

既存[protected workflow](../../../.github/workflows/gcp-terraform-authenticated.yml)の
`DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED=true` はcanonical修復後のprotected plan用です。
`DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED=false`を維持したsource準備です。通常PRは
credentiallessで、mergeやテスト成功からmetadata GET・schema mutation・importを自動実行しません。
共通`ci.yml`、OIDC/trust/Environment/permission、production rootは変更しません。

[workflow metadata helper](../scripts/prepare_user_activity_history_workflow.py)は、別途承認されて
activationしたprotected runでのみ、既発行tokenによるexact dev tableの
[tables.get](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/tables/get)を各stageで1回実行します。
このAPIはtable metadataと`bigquery.tables.get`を使用し、row本文を返しません。
initial plan、apply jobのsame-SHA re-plan前、saved apply後の計3回です。query、getData、token minting、
追加grant、retryは行いません。GETが403ならSTOPし、permissionを自動拡張しません。
公開へはcanonical metadata digestのみ渡し、metadataはrunner tempの0600 fileだけに保存してalways cleanupします。
列順・列descriptionとprivate file pathはGITHUB_ENVへ書きません。runnerの公開step env headerを考慮し、
固定名の0600 metadata/tfvarsを後続stepが直接読みます。Terraformは
[private var-file](https://developer.hashicorp.com/terraform/language/values/variables#variable-definition-files)を使い、
raw metadata・実列順・列description・user counts・field valuesをenv header/Summary/artifactへ渡しません。

helperはlegacy列を除外せず、元のcanonical preparationを再利用してexact8/nested/modes/configを検査します。
既存列descriptionはtop-level/nestedのcanonical field pathからsensitive module入力へ保持します。
plan stageのdigestは列順とdescription mapを含み、apply jobのfresh metadataと照合して
変更・欠損digestでSTOPします。postでもbefore metadataとの同一列順・descriptionを要求し、
protected validatorはprivate tfvarsのkey/value全体をfresh metadataの候補と完全一致で検査します。
planのbefore/after schemaもdescriptionを含めてfresh metadataと比較します。
入力はsensitiveですが、resource schemaは既存provider importと同じ非sensitive markを
保持してsensitivityだけのupdateを防ぎます。raw plan/state/logは従来のprivate経路にのみ保存し、
public CIは合成dummy descriptionのみで検証します。
metadataやdigestはcurrent legacy値の安全性、
consumer、complete row-policy list、recovery、実行承認の証明ではありません。

[protected plan入口](../../../.github/scripts/terraform_protected_plan.py)はhistory flag=trueの場合だけ
既存strict validatorを追加適用します。devの通常plan/apply、採用済みexport chain、exact12resource、
existing11 no-op、beforeのexact history import1、canonical8とfresh列順、unknown/drift/move/action0を要求します。
既存stateに採用後のbefore planもimport0を拒否し、postは必ずimport0/no-op12です。
standalone offline CLIはbefore import1のstrict契約を維持します。Email/quota等の例外modeと混ぜません。
global import-only guard、独立Environment approval、同SHA再plan・sanitized projection照合は維持します。

一貫したlive手順は次の順です。今回のsource準備は各actionの承認を兼ねません。

1. Guard自然実行の各task成功、legacy値の安全性、consumer・complete metadata・recoveryをprivate packetで確定。
   計画上の自然実行成功を、実測済み成功と記録しない。schema gateが保留ならここで待つ。
2. 既存packetのexact DROP 1 statementを別承認して実行。unknown/non-zero/新writerがあればSTOP。
   fresh metadataでtimestampだけ消失、他8列・nested・相対列順・config不変を確認する。
3. pinned providerの実isolated import/no-opを確認し、private field-order候補をreview。
   history plan gateは既にtrueで、apply gateはfalseを維持する。後者の変更は別review/承認。
4. Reviewed SHAのfull-root protected planでexact history import1 + existing11 no-op、その他0。
   別apply Environmentの承認後、同SHA再plan・metadata digest・projection一致からsaved import-only apply。
5. Fresh post metadata、import0/no-op12、state lineage/exact ownership、version/lock/live lock0、公開漏えい/artifact0を照合。
   同SHAの独立通常full-root完全no-opを確認してadoption完了とする。

apply後のpost-check失敗は追加apply・DDL・state restoreをせずSTOPしてprivate調査します。
import済みならownership gateをfalseへ閉じてresourceをrootから落としません。state account全体の
旧snapshot restoreやschema rollbackは別承認です。fresh no-op成立後はhistory ownership=trueを維持します。
新しいWIF/API ownershipでbaseline graphが増える場合、activation/import前にこのexact12契約を
reviewし直します。resource数だけ緩めたり、rootの一部planで迂回したりしません。

## Owner information that can be prepared first

確認済みaccount/profile/region、table metadata、audit Summary、SSO loginを聞き直さない。
本人しか持っていない次の情報だけをprivateに揃える。

1. **外部利用の現況**: BI/Redash、別project view/job、保存/手動SQL、legacy/wildcard/列位置依存が
   このtable/列を使うか。未使用ならその現況、使用ならownerと影響確認を記録。SQL全文、user data、
   access token/credential、接続URLの秘密値を送らない。
2. **既存build/deploy証跡**: source SHAとimage digest、両writerのtask/referenceに対応するreceipt。
   なければ「なし」を記録し、guard deployment候補を具体化。広域inventoryで代用しない。
3. **既存recovery証跡と選択条件**: 既知snapshot/復元proofの有無、短期保存期限と許容cost、
   変更後writeを保つ方式の制約。なければ記録し、試験やbackupを勝手に作成しない。

## Production and MyPage dependencies

Development Exitは実証済み、Production backend/CI foundationと2 Environmentへの設定は実readback・独立review済みです。
Productionの正規State初期化はH2当時の[run38961710157](https://github.com/kani3camp/youtube-study-space/actions/runs/38961710157)のplan/applyで成功しました。
H3で初版State0のversion GET/checksum・controls9・Dev不変を直接監査／独立PASS。H3の[run39005902978](https://github.com/kani3camp/youtube-study-space/actions/runs/39005902978)まで9件のownershipと全CI成功を検証済みです。実行Source H4（`93121ab6`）の[第10波run39014888191](https://github.com/kani3camp/youtube-study-space/actions/runs/39014888191)は認証付きPlan/Apply・post10成功後、native runner回帰の静的CIとGateで失敗しました。State10/直前9のversion固定GET・厳密差分照合は独立PASS。元runの静的CI失敗を保持します。State10照合後にH5の[第11波run39021164334](https://github.com/kani3camp/youtube-study-space/actions/runs/39021164334)を1回発行しました。認証付きPlan/Applyは成功しましたが、native runnerのtimeout/grandchild回帰で静的CIとGateが失敗しました。State11/直前10のversion固定GET・厳密差分照合も独立PASSで、直前10の本文/HEADは第10波の独立確認済みcurrentと完全一致します。元R11の静的CI失敗を保持します。H6の[第12波run39022399909](https://github.com/kani3camp/youtube-study-space/actions/runs/39022399909)は全34 jobと両Auth成功、Plan import1/managed12、Apply post import0/no-op12/rawdrift0/mutation0、cleanup/artifact0/pending0を独立確認済みです。観測テストの修正[#1370](https://github.com/kani3camp/youtube-study-space/pull/1370)はrequired CI・独立review後に統合済みです。H7（`0ed7486b`）で第13波以降へ進み、[第30波run39029269335](https://github.com/kani3camp/youtube-study-space/actions/runs/39029269335)まで30件のownershipを確認しました。元失敗runは再送せず、その失敗判定を保持します。H7の最終通常実証はDevelopment [run39029559218](https://github.com/kani3camp/youtube-study-space/actions/runs/39029559218)・Production [run39029874116](https://github.com/kani3camp/youtube-study-space/actions/runs/39029874116)で、各plan/apply両identityのimport0・no-op31/30・rawdrift/leaf/mutation0、全34 jobの検証（選択job成功・非対象skip）・cleanup・artifact0/pending0を確認しました。独立した通常full-root planも31/30 no-opです。version固定GETでDev serial36/35・Prod32/31を照合し、同lineage・controls9・live lock/workspace0・安定GCP metadata不変を直接監査して独立review PASSです。 H7の最終no-op後healthも両環境の全5 stage・finding0を確認しました。これはH7の実証時点の記録です。最終統合Sourceでの同等性・authority/State closeout・正本同期とInfrastructure Readyの最終判断は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照します。
包括承認内の作業へ都度Chat/Environment人間承認を戻さず、本人初回dispatch・exact source・独立review・State保護を維持します。
通知先の修復や古いraw dataの破壊的cleanupをimportへ混ぜません。MyPage固有のprovisioning/releaseは別gateです。
現在の判定は[Phase 2 readiness](phase2-approval-and-readiness.md#development--production--mypage-readygate)へ集約します。

## CodeQL and mandatory OIDC claims

2026-10-17時点の[PR #1362](https://github.com/kani3camp/youtube-study-space/pull/1362)は、本人の明示承認後にDefault setupを `not-configured` へ切り替え、実readbackを確認しました。
H2追随候補Q2（`92b5ea29`）は全31 CI成功、[run39001207914](https://github.com/kani3camp/youtube-study-space/actions/runs/39001207914)の3言語の解析・SARIF受付成功、analysis ID3件・error0、既存open alert1件の保持を確認済みです。この解析はPR merge commit `6922b666`（parents H2 / Q2）で実行しました。
Q2と全tree一致のintegration H3（`8b2289c8`）へ統合済みで、H3 pushの[CodeQL run39001594887](https://github.com/kani3camp/youtube-study-space/actions/runs/39001594887)も3言語の解析・SARIF受付成功を確認しました。H3のGCP source closureはH2と完全一致し、H2当時のProduction初期化証拠をH3での再実行成功へ読み替えません。
移行実証Source H7のpush [run39023320295](https://github.com/kani3camp/youtube-study-space/actions/runs/39023320295)でも3言語の解析・SARIF受付成功/error0を確認しました。最終統合Sourceの解析結果はIssue #1191へ記録し、H7の結果を後続SHAでの再実行と扱いません。
先行[pull_request run38942869203](https://github.com/kani3camp/youtube-study-space/actions/runs/38942869203)のattempt2も3言語の解析・SARIF受付・Go全package compileに成功しています。当時の解析sourceはPR merge commit `55e3c307`（parents `bbf4b9e2` / `7fb8a54e`）でした。
先行attempt1のDefault setup併用によるupload失敗と、本人の個別承認前に自動承認審査が設定変更を拒否した履歴は保持します。Terraformの実Plan/Applyとは別に評価し、OIDC trustやTerraform Environmentを緩めていません。
第30波と両環境の最終通常Plan/Apply・State監査の実行SourceはH7（`0ed7486b`）です。SHAは実証に用いるSourceの記録であり、文書自体のHEADではありません。最新の運用Sourceと最終runは[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照してください。H4のnative10 case上限2並列化とrouting/回帰、H5の[#1366](https://github.com/kani3camp/youtube-study-space/pull/1366)のrunner/回帰修正は、runtime controller5ファイル・認証workflow・HCLを変更していません。integration refの未解決finding6件（critical1/high3/medium2、既存1＋新検出5）は保持し、dismiss/suppressしません。受付成功は脆弱性0を意味しません。
以下は旧OIDC failureの経緯と設計検討です。

確認済みの[CodeQL run 37494025459](https://github.com/kani3camp/youtube-study-space/actions/runs/37494025459)
は全3jobが開始前fail。errorは `OIDC error: the claim 'environment' cannot be null or empty`。
既存devでも同じerrorが確認されている。要求claimsはrepository_id、repository_owner_id、
environment、ref、workflow_ref、job_workflow_ref、event_name。この実行でenvironmentを供給できず
失敗したことと、現行default/advanced setupの設定内部は403で未確認という点を分ける。
設定403を迂回しない。setup構成と実producerを確認する前に、設定変更で直るとは断定しない。
現行[identity smoke](../../../.github/scripts/terraform_identity_smoke.py)はimmutable repository/owner、
Environment、ref、caller/reusable workflow、eventとsame-SHAを照合する。この境界を弱めない。

[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)ではcustom subjectに
含めたEnvironmentが必要で、job_workflow_refはreusable jobで供給される。subject customizationを
変更するならAWS subject trust、GCP mapping/condition/SA bindingとの整合が必要になる。
repo OIDC template/cloud trust/auth/permission変更0を維持する。

診断が裏付けた場合の最小source候補は、明示的なOIDC request/cloud action/secret mappingを追加しない
[公式advanced CodeQL workflow](https://github.com/actions/starter-workflows/blob/main/code-scanning/codeql.yml)。
公式templateはid-token writeを要求しないが、内部OIDC requestゼロの保証ではない。
template単独で必須environment/job_workflow_refを満たせるとも扱わない。要求claimsに対応する
候補として専用reusable callee + cloud許可対象外のsecretless Environmentを検討する。
実際にpinするAction/依存と実producerの挙動は別検証で、今回の失敗修復も未証明。Terraform Environmentを
流用しない。[default→advanced切替](https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/configure-code-scanning/configuring-advanced-setup-for-code-scanning)
は別settings actionなので、source PRだけで修復済みとは報告しない。

## One-shot import source contract (not execution approval)

2026-10-09の[plan run 37925757786](https://github.com/kani3camp/youtube-study-space/actions/runs/37925757786)は
SHA `244a5a3e455139621717ca2496d35d2fb5742150` のfull-rootでimport1/既存11 no-op、
create/update/delete/replace/read/drift/other0、stateとcomplete table metadata不変、
native lock不在、cleanup成功、artifact0、実apply0を確認した。saved planはcleanup済みで再利用不可。
plan-onlyではnegative security probesを実行しておらず **Full security gate: NOT VERIFIED**。
apply gateはfalseのまま。月内累積保守見積USD0.507609043は過去runの記録であり、次のapplyの枠ではない。

### Before an owner could authorize one apply

source準備・live設定・protected executionを一つのgoalとしてreviewする。今回のapply/emitter/post-noop/runtime/probe/production activation gateとissuer catalogは現在閉じたまま。source準備のPRだけでは以下の実行を承認しない。

1. disabled sourceの3前提を先に揃える: routine cost waiverと安全入力の分離、両jobのearly exact AWS RoleId照合、初回importのmandatory root emitter。review済みruntime sourceが同時期に入る場合も、historyのためのruntime executionを要求しない。
2. 既存private inputからbackend値・既存identity secret名・canonical table metadataを回収する。実行承認後、必要なfresh exact table metadata GET一回とprivate nonceでhistory envelope `{schema_version:1, receipt_binding}` を作り、既存`GCP_RUNTIME_OWNERSHIP_PACKET_JSON`へ両development Environment同一内容を準備する。raw state/metadata/nonceをpublicへ出さない。新grant、API enable、credential/trust変更を含めない。
3. apply/emitter/independent post-noopに必要なhistory source activationをまとめてstageし、review後にcombined source SHA **S**を固定する。runtime-ready/runtime-plan/security-probe/production gateはfalseを維持する。旧plan/sourceは流用せず、全source準備を終えてから新protected full-root planを一回作る。
4. `terraform-dev-plan` reviewerがS、fresh safety input、exact history import1/既存11 no-op/他0とsanitized summaryをreviewする。新plan run **P**とsummary digest **D**にboundした承認v2を準備し、apply dispatchのplan/apply両Environment review後、同Sの再plan/projection/binary digestでstate-only saved-plan applyを一回実行する。
5. 初回apply内post import0/no-op12/他0、state11→12/unrelated11不変、lineage/serial、stable metadata、native lock absence、actual apply/post-plan outcomeを照合してroot receiptをemitする。同Sの別protected planで独立post-noop12を一回確認する。成功後にexecution gatesを閉じるが、adopted history resourceをrootから落とさない。
6. root receiptのconsumer admissionは別のclosure/catalog source **C**で、historical issuer Sと`waves=["history12"]`だけをpinする。S自身のcatalog entryは初回emissionに不要。catalogを作るための再import/replanは行わない。新runtime waveのissuerはそれぞれ別review・実receiptが必要。

既存Environment protectionのread証拠でname/id、reviewer id、branch policy、self-review/admin-bypass設定をpacketに回収する。所有者に既存値を再入力させない。所有者の残る判断は、このbounded live scopeの承認とstate writerのquiet window、fresh P/D/SのEnvironment review。private canonical inputの不足はexact path/keyとして挙げ、dummy値で補完しない。

routine Terraform cost blockingはwaived。新しいcost-only checks、月次上限や確認済みbooleanを要求しない。actual invoice、provider/auth/native-backendの全request数はunknownと記録する。過去probe [38029011597](https://github.com/kani3camp/youtube-study-space/actions/runs/38029011597)のconditional PUT403/state不変/lock PASSはそのrequestだけの証拠。再probeせず、SCP/session/inherited IAMの未確認範囲をfull security proofへ読み替えない。権限不足やidentity不一致ならSTOPし、新grantで迂回しない。

### Bounded execution and one-shot approval v2

`terraform_history_import_approval` は次のキーだけ: `schema_version=2`、`target=dev`、`git_sha=S`、正整数`plan_run_id=P`、`summary_sha256=D`、`issued_utc`、`expires_utc`。発行から最大24時間、実行時刻も範囲内。費用digest/金額/旧schemaを拒否する。public inputにprivateな証拠本文やbucket/account/role/SA名を貼らない。

自動照合はtarget/SHA/期限、plan jobとapply再planのsanitized summary digest、apply直前のlocal saved binary digest。`plan_run_id`は正整数形式のreview参照であり、それだけで実runをauthenticateしない。Environment reviewerが実Pのsource/結果/summaryと照合する。両jobでfirst attempt、exact trusted caller/ref/repository、mandatory emitterを認証前に要求し、AWS認証直後にexact RoleId/accountを要求する。same-account別roleを受理しない。

正常成功経路のsource helperが明示するrequest数は以下。実観測値やworkflow全体のrequest ceilingではない。

| Protected stage | S3 helper | STS identity | GCP helper | GitHub OIDC claim GET |
| --- | ---: | ---: | ---: | ---: |
| Fresh plan | 9 | 2 | 7 | 1 |
| Apply dispatch: plan job | 4 | 2 | 6 | 1 |
| Apply job | 13 | 1 | 5 | 0 |
| Independent post-noop plan | 14 | 2 | 7 | 1 |
| Total explicit helpers | 40 | 7 | 25 | 3 |

このほかprivate binding用exact table metadata GET一回。credential ActionsのOIDC/STS/token exchange、Terraform provider refresh、native backend state/lock処理、内部retry、logging/billingはこの表に含めずunknownのまま。workflow dispatchは最大3、Terraform planは最大5、init4、apply1。plan job15分/apply20分。scriptのAWS attemptとcredential Action retryは1、backend `max_retries=1`も維持するが、全内部retryゼロの証明とはしない。失敗経路のbounded post-safety readは追加され得る。自動rerun・force-unlock・scope拡張をしない。

before/preapplyはstateに既存11件のみ、exact table import1/既存11 no-op/他0、元state不変、stable table metadata/description一致、positive lock absenceを要求する。binaryは4 MiB以下でsanitizer/承認前とapply再plan直後に確認し、local saved-plan digestをapply直前にも照合する。旧plan binaryはcleanup済み、job間artifactで渡さない。

postは同lineage・増加serial、既存11の全state値とoutputs/既知checks不変、追加table1件/schema、stable tables.get不変、positive lock absence、post full-root import0/no-op12/他0を要求する。列descriptionはnestedも比較する。output-only `etag`/`lastModifiedTime`/`streamingBuffer`/row・bytes・partition統計はprivate観測し、通常BigQuery writerの変化だけでSTOPしない。他fieldは未知fieldもstrict一致。metadataだけでrow本文の同一性を証明せず、business-data query/DDL/table overwriteはこのscope外。

authentic root receiptは、review済みissuer Sの実GitHub run/attempt1、成功したexact plan/apply jobs、両Environment approval、required emitter/publish/cleanup steps、専用sanitized artifactのexact identity/ZIP digest/receipt digest、private bindingをconsumerが再照合して初めて受理する。初回applyはemitter/publication gates両方を必須とするが、現在はfalse。copy JSON、PASS prose、old probe/plan、public dummy artifactはroot proofにならない。issuer catalogは現在空。selective step-log GET404を受けて専用artifactへ移行し、source独立reviewとbounded public dummy roundtripの成立をactivation前に確認する。real publicationは別のowner execution approvalが必要。詳細は[runtime receipt contract](runtime-wif-api-ownership.md)を参照する。

### Failure classification and recovery

post receiptの`state=unchanged`はimport未記録、`state=imported`は許容state差分、`unknown`は
部分write/競合/取得失敗を含む。metadata STOP、lock unknown、apply/post-plan失敗は成功扱いしない。
いずれも自動retry、`force-unlock`、旧S3 versionの上書きrestore、`state rm`、DDL、table再作成を
行わず、Terraform state writerの競合を隔離してprivateにcurrent S3 version/serial/lineage、exact table metadata、
lock holder、Terraform apply logとstate差分を照合する。stateだけimport済みならdata/tableは
そのままで通常full-root no-opを独立確認し、同一importの再applyは拒否する。state未変更なら
失敗原因と期限/identityを再評価して新planと新承認を得る。state異常またはtable metadata
変化時は別scopeのstate-only修復またはdata recoveryを候補にし、新writeとprivacy retention、
復元先と費用をreviewしてから別承認する。古いsnapshotを後続writeの上へ戻さない。
