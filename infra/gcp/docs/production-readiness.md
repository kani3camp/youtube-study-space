# Production backend / bootstrap / import readiness

実行入口は [Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)、設計正本は
[Notion Current Canon](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc)。
これは実変更前のsource/test/runbook packetです。実prod inventory、承認、Infrastructure Readyを
証明しません。private identity、既存IAM全文、raw state/plan、mailbox、business dataは記載しません。

## Scope and evidence

開始時にremote integration `feature/gcp-terraform-iac` のHEAD
`7c7b99c7c8492a1c675c821dbf17762e13edbb80` をfresh確認しました。
既存[prod root](../environments/prod/main.tf)、[backend example](../environments/prod/backend.hcl.example)、
通知2 modulesを再利用します。共通state account/bucketはdevelopmentでbootstrap済みという
repository記録がありますが、このpacketでcloud側を再確認していません。

production backend/key/trust/importは未検証・未承認です。dev batch cutoverや今夜の成功を
production gateの証拠にしません。dev history/runtime WIF/APIの作業は別workstreamです。
source準備、credentialless tests、最小read-only inventory整理はそれらと並行できます。

追加したoffline contractsは次を検証します。

- prod default graphはresource/import/drift各0。既存Email/quota ownershipはfalseのまま。
- prod rootへ別projectを渡すと停止。backend exampleはprod keyとworkspace discovery prefixへ限定。
- Email opt-inはcreate1、quota opt-inはcreate3であり、既存import-only sanitizerが拒否。
- 現行authenticated workflowのprod向け全6 modeはpreflightで停止し、job inputも生成しない。
- 専用PR CIはcredentiallessで、OIDC permission、Environment、secret、remote state操作を持たない。

これらはmocked plan/source guardの証明です。pinned providerのprod実Read、remote no-op、
cloud trust、実Environment approval、locking/recoveryの検証にはなりません。

## Source gaps before production activation

[現行workflow](../../../.github/workflows/gcp-terraform-authenticated.yml) は閉鎖を維持します。
`PROD_AUTHENTICATED_TERRAFORM_ENABLED` をtrueへ変えるだけでは完成しません。
共通CI/identity scriptの変更はこのpacketへ混ぜず、fresh inventory後の別source PRで束ねます。

| Remaining source work | Current evidence / required result |
| --- | --- |
| Independent prod plan/apply gates | 現行prodにはdevと同等の独立apply-enable flagがない。plan smokeだけでapplyを解禁できない構成が必要 |
| Production mode routing | prod branchは`manage_quota` outputを設定せず、dev専用例外modeをprod側で明示拒否していない。prodは通常plan/import-only applyだけへ限定し、各job inputを明示する |
| Actual prod identity smoke | planのidentity smokeはdev条件付き。既存scriptはdev project/keyと既存stateを前提にする。prod plan/apply双方の実identity/negative probeが必要 |
| Initial-state handling | prod keyがabsent/empty/existingのどれか未確認。absentはAccessDeniedと区別する。dev stateをcopyせず、必要なinitial state/output writeは独立した承認対象 |
| Import definitions and exact wave validators | prod import blocksは未準備。fresh Own分類/remote値に基づく定義と、exact address/import/no-op/unknown/drift/output/state shapeのwave契約が必要 |
| Protected execution evidence | same-SHA再plan/projection一致、独立Environment approvals、post no-op、lock/version/lineage、public output監査をprodで実測する |

初回empty graphだけではGCP providerが認証/Readを行った保証がありません。token交換・plan/apply SA
impersonationと無害なexact metadata Readを明示的に証明します。resource0をInfrastructure Readyと
扱わず、想定ownershipの完了を別に判断します。

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
| First import candidates | 本人合意のOwn候補だけについてexact ID/location/provider Read fields、IAM dependency、生成resourceとの境界 | 最初はsmall waveに必要なmetadataだけ。data本文、query、export、source download、Function invocationなし |
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
reviewし、別承認するか、書込みを試みない検証へ限定します。404やnetwork errorをdeny PASSにしません。

[AWS OIDC trust公式](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-idp_oidc.html)、
[GitHub OIDC reference](https://docs.github.com/en/actions/reference/security/oidc)、
[GCP WIF deployment-pipeline公式](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-deployment-pipelines)
と、**実際の**subject template/mappingを照合します。private exact diffにはimmutable repo/owner ID、
trusted integration ref、caller/reusable workflow、event、plan/apply Environmentとaudienceを含めます。
repo名やEnvironment名だけでworkflow限定済みとみなしません。repo OIDC templateを変更する案は
既存dev/AWS/GCP/CodeQLへの影響を含む別承認です。

本人が選ぶreviewer/self-review/bypass設定を実API値と照合します。会話内のaction-time承認と
GitHub Environment approvalは別です。既存secret名を再利用する場合もEnvironmentごとの実role/SAを
privateに照合し、planとapplyを同じidentityへ暗黙にまとめません。新secret設定は未承認の実操作です。

## Separate approval and execution units

dev exitが実測完了するまで、以下のproduction実操作は開始しません。source PRのreview/mergeは
これらの承認を兼ねません。各packetはexact targets/before-after/impact/window/rollback/verificationを
完成させてから親へ渡します。approval gateを自動enableしません。

| Unit | Concrete content for owner approval | Result needed before next unit |
| --- | --- | --- |
| P1 Backend/security bootstrap | 共通bucket再利用、prod backend role/trust/policyのexact diff、Environmentとprivate inputs、GCP CI trust/SA/grantsのexact diff。新bucket/accountは含めない | 設定read-back、dev/他product不変、各roleの境界証拠。失敗時は追加grant/trust/inputsだけを撤回 |
| P2 Authenticated plan / initial state | 別reviewed source PRのprod plan routing/smoke、必要ならempty state/output初期化とlockの最大diff | 実AWS/GCP identity、prod state snapshot/version/lineage/resources0、実lock解放、negative probes、public output監査。apply-enableは閉鎖 |
| P3 Exact import wave | Fresh Own候補、exact definition/import ID、full-root pre-plan、provider実Read、state before/after manifest、独立apply activation/approvals | Same-SHA再plan/projection一致、import-only state更新、post import0/no-op、remote metadata不変、lock解放 |
| P4 Exit verification | Approved ownership一覧と未完項目判断、dev/prod通常full-root、trust/state/recovery証拠、正本同期 | Infrastructure Readyを親が判定。MyPage deploy/releaseは別gate |

P1はIAM/Environment/backend/GCP trustの対象別diffを同じ判断packetへ束ねます。実行権限・rollbackが
異なるので、承認scopeを明示します。bucket全policyやSA全policyを古いsnapshotで上書きしません。
stateの復旧は単なる古いversionのcopyではなく、post-write/ownershipを保持する別承認です。

## Production import acceptance

devと同じresource数/順序/retention/region/IDを機械的にコピーしません。
fresh inventory → Own/External/Google-managed/Investigate分類 → exact定義 → full-root import-only plan →
独立protected approvals/same-SHA再plan → exact import-only apply → post通常full-root no-opの順です。

- 1件でもcreate/update/delete/replace/drift/unknown/説明不能actionならSTOP。
- exact expected importだけを許可し、既存managed resource全件はno-op。import数だけでPASSにしない。
- full-rootにdeferred/action invocation/address move/output変化等があればwave契約で説明・拒否する。
- 原則1 resourceまたは分離不可能な最小unit。resource typeのdev実績を先に確立する。
- generated subscription/artifact/source bucket/service agents、project IAM全体、用途不明resourceは巻き込まない。
- notification repair、schema mutation、API enable/disable、Function rebuild/deploy、manual triggerを混ぜない。

production Email channel0 / quota3 dangling refsはIssueの記録であり、ここではfresh未確認です。
現行prod modulesをenableするとcreateを提案するため、import準備や修復としてenableしません。
通知修復はprivate field-mask/rollbackを揃えた別operator waveです。
#983/#1165/#1166は別Issue、D01のholdはこのscope外です。

## Owner preparation and critical path

本人しか確定できない情報を一度にprivateへ揃えます。既知account/profile/regionやSSO loginを
聞き直さず、既存operator経路が使えるか、欠けた情報だけを記録します。

1. prod ownershipの最初の候補と優先順、既存recovery条件/許容cost、import window。
2. 既存state-control-planeの参照とprod role/keyの有無。新credentialsを送らない。
3. prod plan/apply承認者、既存Environment protectionとself-review/bypass運用の選択。
4. GCP Terraform CI trust/SAの既存private参照。runtime WIFと混同しない。
5. state不在時の初期化、negative probes、通知修復を今回含めるかの**別scope**判定。

critical pathは **dev history/runtime WIF/API完了とfresh dev exit → P1 → P2 → P3各wave → P4**。
今夜のdev batch成功だけではdev exitを満たしません。source/tests/runbook、private read-only inventory、
definitionの準備とowner情報整理はdev workと並行可能です。live定義を確定できない部分はdefault-offにします。

| Remaining effort after this source packet | Working-time range | Why still uncertain |
| --- | --- | --- |
| Private minimum inventory + exact P1 packet | 2–5h | prod既存roles/trust/Environment/keyの有無、read経路の権限未確認 |
| Shared source routing/smoke/wave contract + review/CI | 4–8h | 初期state方式、実OIDC/mapping、最初のOwn候補が未確定 |
| Approved P1/P2 execution/read-back | 2–5h | 実protection/権限制約、初期state/lock/recovery条件に依存 |
| Each approved small P3 import wave | 1–3h per wave | 対象数・provider実Read・metadata fidelity・drift未確認 |
| Exit/public audit/canon synchronization | 1–2h | fresh dev/prod ownershipと証拠を揃える必要 |

基本準備と基盤実行は残り **9–20h + 各wave 1–3h** の作業見積りです。
承認待ち、dev critical path、CI queue、権限/remote driftの修復は含めません。暦時間や今夜の完了を保証しません。

## Offline verification

```sh
python3 infra/gcp/tests/test_production_readiness.py
terraform fmt -check -recursive infra/gcp
terraform -chdir=infra/gcp/environments/prod init -backend=false -input=false -lockfile=readonly
terraform -chdir=infra/gcp/environments/prod validate
bash .github/scripts/test-detect-ci-paths.sh
```

testはsource/lock/modulesをfresh temp treeへcopyし、mock providerのplanだけを実行します。
local backend.hcl/state/tfvars/cacheをcopyせず、raw mock planをpublic出力しません。
[専用CI](../../../.github/workflows/gcp-production-readiness.yml) も同じtestをPRで実行し、共通CIを変更しません。
