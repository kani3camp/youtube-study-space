# Production backend / bootstrap / import readiness

実行入口は [Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)、設計正本は
[Notion Current Canon](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc)。
これは実変更前のsource/test/runbook packetです。実prod inventory、承認、Infrastructure Readyを
証明しません。private identity、既存IAM全文、raw state/plan、mailbox、business dataは記載しません。

## Current execution boundary — 2026-10-16

全4 Terraform EnvironmentのRequired reviewersは本人が解除済みで、GitHub実APIでも確認した。
独立したplan/apply Environment、integration branch限定、OIDC/WIF trustは維持し、
GitHub本人account ID 54093651・actor/triggering actor kani3campの初回workflow_dispatchだけを通す。
各runの人間承認を再要求せず、botへの切替やrerunで制限を迂回しない。

現在のmigration-plan/migration-apply経路でもproductionの実認証・backend/trust・initial State・importは未完である。
MIGRATION_PRODUCTION_READYはfalseを維持し、Development Exitを実測してから本格移行を進める。
旧PROD_* gateや過去の手動Environment承認記録は、新経路の実readinessを証明しない。
resource0の初期StateだけでInfrastructure/MyPage ReadyGateをPASSにしない。

## Output-only initializer source preparation

[production_initializer.py](../../../.github/scripts/production_initializer.py)はcloud/State操作のないpure adapterで、
現controller/workflowの実行経路には未接続である。CIはsynthetic fixtureとbackendのない一時local rootだけを検証する。

比較した最小案は、新mode追加よりも既存migration-plan/apply内の厳密な条件分岐である。
将来の接続時は、owner初回/ref/Environment/実identityの共通検査後、成功したS3 snapshotの
head/raw/stateがすべてnullで、production targetかつmanifest/resources0の場合だけ選択する。
失敗read、403、既存State、dev、resource/import/drift/unknown/deferred actionは初期化へ流さない。

adapterは既存production_execution_contract.plan_shapeのinitialize契約で、固定production/projectの2 output createを先に検証する。
その2件だけを内部の検証射影でno-op形式へ対応させ、通常full-graph契約も再利用する。
saved binaryは変更せず、通常import-only契約自体も変更しない。public結果はoutput_creates=2を明示し、
専用operation markerと元のoutput changesをprivate HMAC payloadへ保持する。
Coreがresource0で省略するresource_changesだけを空listへ正規化し、その他の未知差分は隠さない。

将来のcontroller接続には、同SHA再plan/HMAC、source copyとsaved-plan seal、native lock、
適用直前のpositive absence再確認、新version/lineageと固定outputs/resource0のState検査、
通常post-planのresource0・output no-op、lock解放が必要である。新bucket/marker/dev State copyは作らない。
このsource準備はproduction activationや実State作成の許可・完了を意味しない。

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

## Source gaps before production activation

[現行workflow](../../../.github/workflows/gcp-terraform-authenticated.yml)のproduction migrationは上記gateで閉鎖を維持します。
以下のPROD_* gate/tableは旧経路のhistorical source packetです。
`PROD_AUTHENTICATED_TERRAFORM_ENABLED` をtrueへ変えるだけでは完成しません。
本packetは独立gate/routing、offline backend proposalとoffline execution validatorまでです。`PROD_EXECUTION_CONTRACT_READY` と
`PROD_DEVELOPMENT_EXIT_READY` はfalseを維持します。fresh inventory後のsource PRでprod専用
identity smoke・initial-state・exact wave検証を完成し、実測development exit後にactivationをreviewします。

| Remaining source work | Current evidence / required result |
| --- | --- |
| Independent prod plan/apply gates | source準備済み。`PROD_AUTHENTICATED_TERRAFORM_APPLY_ENABLED=false`。plan enableから独立し、Environment承認も別 |
| Production mode routing | source準備済み。通常plan/import-only applyのみ。全ownership inputを明示false、dev専用入力/例外mode/rerunを拒否 |
| Actual prod identity smoke | planのidentity smokeはdev条件付き。既存scriptはdev project/keyと既存stateを前提にする。prod plan/apply双方の実identity/negative probeが必要 |
| Initial-state handling | offline output-only/positive absence/state shape validatorを準備。実prod keyのabsent/empty/existing判定、authenticated collector、initial writeのprotected実行は未完。dev Stateはcopyしない |
| Import definitions and exact wave validators | offline exact1 import/full-root no-op/state delta/metadata不変 validatorを準備。実Own inventory、resource-specific定義/metadata adapter、authenticated collectorとsame-SHA workflow接続は未完 |
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

本人が選ぶreviewer/self-review/bypass設定を実API値と照合します。会話内のaction-time承認と
GitHub Environment approvalは別です。既存secret名を再利用する場合もEnvironmentごとの実role/SAを
privateに照合し、planとapplyを同じidentityへ暗黙にまとめません。承認範囲内のEnvironment/Secrets構築は実値をprivateに保ち、保護条件を維持して実施します。

## Separate protected execution units

dev exitが実測完了するまで、以下のproduction実操作は開始しません。source PRのreview/mergeは
実Environment approvalやlive readinessを兼ねません。各packetはexact targets/before-after/impact/window/rollback/verificationを
完成させてから親へ渡します。包括承認A内でもRequired reviewersとbranch/trust保護を維持します。

| Unit | Concrete reviewed execution packet | Result needed before next unit |
| --- | --- | --- |
| P1 Backend/security bootstrap | 共通bucket再利用、prod backend role/trust/policyのexact diff、Environmentとprivate inputs、GCP CI trust/SA/grantsのexact diff。新bucket/accountは含めない | 設定read-back、dev/他product不変、各roleの境界証拠。失敗時は追加grant/trust/inputsだけを撤回 |
| P2 Authenticated plan / initial state | 別reviewed source PRのprod plan routing/smoke、必要ならempty state/output初期化とlockの最大diff | 実AWS/GCP identity、prod state snapshot/version/lineage/resources0、実lock解放、negative probes、public output監査。apply-enableは閉鎖 |
| P3 Exact import wave | Fresh Own候補、exact definition/import ID、full-root pre-plan、provider実Read、state before/after manifest、独立apply activation/approvals | Same-SHA再plan/projection一致、import-only state更新、post import0/no-op、remote metadata不変、lock解放 |
| P4 Exit verification | Approved ownership一覧と未完項目判断、dev/prod通常full-root、trust/state/recovery証拠、正本同期 | Infrastructure Readyを親が判定。MyPage deploy/releaseは別gate |

P1はIAM/Environment/backend/GCP trustの対象別diffを同じ判断packetへ束ねます。実行権限・rollbackが
異なるので、承認scopeを明示します。bucket全policyやSA全policyを古いsnapshotで上書きしません。
stateの復旧は単なる古いversionのcopyではありません。fresh post-write/ownershipを保持する
正規手順をreviewし、blind restoreを行いません。

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
呼ばず、private collector inputを検証するだけです。protected execution workflowへ未接続で、production全gateはfalse。
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

**残るlive requirements:** actual AWS/GCP identityとnegative probes、expected bucket owner/roleの検証、
complete metadataとprovider valuesを結ぶresource-specific adapter、authenticated HEAD/GET/HEAD/LIST collector、
same-SHA re-plan/保存plan sealと独立Environment approval、post full-root/no-opと実workload回帰、private cleanupが
未接続です。渡されたsnapshotの真正性・full-root実行・writer静止をこのoffline scriptは証明しません。
APIのOwn classification/dependency provenance、WIF trust/mapping、SA IAM grantの必要性と実caller/targetのjoinは
typed waveの残条件です。このvalidatorのtype/shape PASSからOwn分類を自動判定しません。
metadataのvolatile差分や候補check追加が必要なら、exact候補の別reviewed adapterを実装するまでSTOPを保持します。
このsource準備だけでは `PROD_EXECUTION_CONTRACT_READY` をtrueにしません。

## Critical path and remaining work

**実測Development Exit → P1 → P2 → P3各wave → P4** を維持します。
source PRやprovider-mocked resource0をInfrastructure Readyとして扱いません。

- P1: private metadataをfresh収集し、prod backend roles/trust/policy、GCP CI trust/SA/grants、
  GitHub Environment/Secretsのexact差分を作る。既存devや共通bucket全policyを上書きしない。
- P2 source: prod専用AWS/GCP identity smoke、initial state不在の安全な契約、nonmutating negative probes、
  state version/lineage/native-lock/recovery証拠を作る。現source-ready gateの解除にはこれらが必要。
- P2 live: 本人が各GitHub Environmentで承認後、実identity/plan/lock/stateを検証する。apply gateは閉じる。
- P3: fresh Own分類とexact import definition/validatorを作り、1件または最小unitずつsame-SHA import-onlyと
  post full-root no-op、metadata不変を検証する。通知修復は別wave。
- P4: 通常full-root no-op、drift/unknown0、回帰、public出力監査、正本同期、ReadyGate判断。

既知private参照や包括承認範囲は聞き直しません。個人しか決められない未確定reviewer/復旧責任者等だけを
既存private情報の確認後に提示します。#1165/#1166/#983の未完項目と費用・削除のscopeは別に保持します。

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
