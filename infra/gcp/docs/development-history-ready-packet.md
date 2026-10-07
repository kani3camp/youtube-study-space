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
| Writer proof / bounded deployment if required | 既存receiptでguardを証明できればdeploy不要。証明できなければdev両writerだけのimage/task/reference差分、既存roles/network/config維持、previous digest/referencesへのrollbackを具体化 | whole-stack CDK deployが他Lambda image等を変えるなら追加scopeとしてSTOP。元runtime/config不明のままdeployしない |
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
証明ではない。current CI apply workflowへの配線・activationは未実施で、source adoption=false。
既存11のownership/configが変わったら候補を再reviewする。12という数だけでPASSにしない。

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
