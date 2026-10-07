# Development history: one decision packet, separate execution boundaries

Goal: `user-activity-history` のschema整理とTerraform adoptionを、安全に判断できる一つの
準備bundleにまとめる。code/tests/specは一つのdraft PRで更新し、証拠が揃った最終候補を
一回reviewする。writer deployment、データ/schema変更、state変更、IAM/security、本番は
それぞれ影響が異なるため、実行scopeと承認を混ぜない。

公開するのは最小specのみ。exact runtime ARN/revision/digest、owner回答、元schema、復元記録、
IAM inventory、raw plan/state、実集計値はprivate operator packetへ保存する。
入口は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)。
[auditの意味とschema修復方針](user-activity-history-canonicalization.md)は維持する。

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
import1/other0とpost import0/no-op12、exact table/provider、canonical8/nested/実列順、
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
証明ではない。protected workflowへのdefault-off配線は下記に準備済みで、source adoption=false。
既存11のownership/configが変わったら候補を再reviewする。12という数だけでPASSにしない。

## Default-off protected history adoption

既存[protected workflow](../../../.github/workflows/gcp-terraform-authenticated.yml)の
`DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED=false`を維持したsource準備です。通常PRは
credentiallessで、mergeやテスト成功からmetadata GET・schema mutation・importを自動実行しません。
共通`ci.yml`、OIDC/trust/Environment/permission、production rootは変更しません。

[workflow metadata helper](../scripts/prepare_user_activity_history_workflow.py)は、別途承認されて
activationしたprotected runでのみ、既発行tokenによるexact dev tableの
[tables.get](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/tables/get)を各stageで1回実行します。
このAPIはtable metadataと`bigquery.tables.get`を使用し、row本文を返しません。
initial plan、apply jobのsame-SHA re-plan前、saved apply後の計3回です。query、getData、token minting、
追加grant、retryは行いません。GETが403ならSTOPし、permissionを自動拡張しません。
公開へはcanonical metadata digestのみ渡し、metadataはrunner tempの0600 fileだけに保存してalways cleanupします。
列順とprivate file pathはGITHUB_ENVへ書きません。runnerの公開step env headerを考慮し、
固定名の0600 metadata/tfvarsを後続stepが直接読みます。Terraformは
[private var-file](https://developer.hashicorp.com/terraform/language/values/variables#variable-definition-files)を使い、
raw metadata・実列順・user counts・field valuesをenv header/Summary/artifactへ渡しません。

helperはlegacy列を除外せず、元のcanonical preparationを再利用してexact8/nested/modes/configを検査します。
plan stageのdigestをapply jobのfresh metadataと照合し、列順の変更・欠損digestでSTOPします。
postではbefore metadataとの同一列順も要求します。metadataやdigestはcurrent legacy値の安全性、
consumer、complete row-policy list、recovery、実行承認の証明ではありません。

[protected plan入口](../../../.github/scripts/terraform_protected_plan.py)はhistory flag=trueの場合だけ
既存strict validatorを追加適用します。devの通常plan/apply、採用済みexport chain、exact12resource、
existing11 no-op、exact history import0/1、canonical8とfresh列順、unknown/drift/move/action0を要求します。
initial importはimport1、既存stateに採用後の通常planはimport0を許可し、postは必ずimport0/no-op12です。
standalone offline CLIはbefore import1のstrict契約を維持します。Email/quota等の例外modeと混ぜません。
global import-only guard、独立Environment approval、同SHA再plan・sanitized projection照合は維持します。

一貫したlive手順は次の順です。今回のsource準備は各actionの承認を兼ねません。

1. Guard自然実行の各task成功、legacy値の安全性、consumer・complete metadata・recoveryをprivate packetで確定。
   計画上の自然実行成功を、実測済み成功と記録しない。schema gateが保留ならここで待つ。
2. 既存packetのexact DROP 1 statementを別承認して実行。unknown/non-zero/新writerがあればSTOP。
   fresh metadataでtimestampだけ消失、他8列・nested・相対列順・config不変を確認する。
3. pinned providerの実isolated import/no-opを確認し、private field-order候補をreview。
   source gate false→trueと対応source testの期待値変更を一つのactivation差分として別承認する。
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

dev history完了だけでInfrastructure Readyにはしない。dev history → runtime WIF/IAM exact
ownership + owned API分類 → dev通常full-root完全no-op → production fresh backend/trust準備 →
approved import-only ownership → dev/prodのno-op/negative probes/state-lock-recovery → Infrastructure
Ready → MyPage固有のrelease gate、という依存を保つ。

本番を先に用意できるのは、既存operatorアクセス経路、予定ownershipとcost/retention条件、
承認者・実行windowの情報まで。production bootstrap/IAM/OIDC/apply/deployは別承認。
Issue記録だけのEmail channel/dangling policyは付随して修復しない。
詳細は[Phase 2 readiness](phase2-approval-and-readiness.md#development--production--mypage-readygate)。

## CodeQL and mandatory OIDC claims

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
