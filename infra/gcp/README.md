# GCP Terraform

YouTube Study Space の既存GCP resourceを、安全に段階移行するためのTerraform rootです。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 現在のscope

Phase 1のscaffold / S3 state / protected CIは構築済みです。Phase 2では、production自然実行E2E（#1173）と独立したdevelopment resourceを、小さいimport-only waveで取り込みます。

この段階では以下を行いません（remote state bootstrap exceptionを除く）。

- production resourceのimport
- workload resourceのcreate / update / delete
- Cloud Functions / Scheduler / Pub/Subの変更
- workload resourceのmutation権限追加 / API有効化
- 未承認のGitHub Actions workload apply（protected CIはIssue #1162）
- Service Account JSON keyの作成
- MyPage resourceのprovisioning

Firestore export FunctionのNode.js 22移行は Issue #1148 の別gateです。development / productionのNode.js 22自然実行E2Eは2026-10-06にPASSし、#1173をcompletedでcloseしました。development export-chain natural-E2E import gateは解禁済みです。最初のtopic定義はdisabledで準備し、別quota state driftの解消とCIの最小GET prerequisiteを満たすまでprotected importを実行しません。

## Directory

```text
infra/gcp/
├── .terraform-version
├── environments/
│   ├── dev/
│   │   ├── main.tf
│   │   └── backend.hcl.example
│   └── prod/
│       ├── main.tf
│       └── backend.hcl.example
└── modules/
    └── README.md
```

dev / prodは別root moduleです。Terraform workspaceで環境を切り替えません。

## Version policy

- Terraform CLI: `.terraform-version` と各rootの `required_version` を一致させる
- Google provider: 各rootの `required_providers` と `.terraform.lock.hcl` を正本にする
- provider lockはTerraform operator / CIで使用する `linux_amd64` / `darwin_arm64` のchecksumを事前生成する
- version更新は通常のdependency変更としてPRでreviewする
- Notionにはpatch versionを複製しない

## Local validation

credentialやbackendを使わない構文・provider validation:

```bash
terraform fmt -check -recursive infra/gcp

for env in dev prod; do
  (
    cd "infra/gcp/environments/$env"
    terraform init -backend=false -lockfile=readonly
    terraform validate
  )
done
```

`terraform init -lockfile=readonly` を通常validationに使い、provider lockの変更を暗黙に許可しません。dependency更新時は `terraform providers lock -platform=linux_amd64 -platform=darwin_arm64` でdev / prod両rootのlockを更新し、差分をreviewします。

## Remote state bootstrap

State infrastructureはTerraform本体の外側にある one-time bootstrap exception とします。

### Architecture

Issue #1155以降の検討を踏まえ、remote stateは **AWS S3の個人開発共通Terraform state control plane** に置きます。管理対象がGCPでもbackendを同じcloudへ置く必要はありません。

原則:

- 個人開発共通のstate bucketは原則1つ
- product / environmentごとにstate keyを分離する
- YouTube Study Spaceは `youtube-study-space/dev/terraform.tfstate` と `youtube-study-space/prod/terraform.tfstate`
- Terraform workspaceでdev / prodを切り替えない
- S3 backendの `use_lockfile = true` を使い、DynamoDB lockは新規採用しない
- S3 Bucket Versioningを有効化する
- S3 Block Public Accessを全面有効化する
- server-side encryptionを有効化する
- state bucketへTerraform state / lock以外のbusiness dataやartifactを置かない
- product / environmentごとにbackend IAM roleを分離し、対象state keyとlock keyだけへ最小権限を付与する
- state file本体には原則 `GetObject` / `PutObject`、lock fileには `GetObject` / `PutObject` / `DeleteObject` を許可し、bucket listも対象prefixへ制限する
- backend roleへAWS workload用の広い権限を付与しない
- 長期AWS access key / secret keyを作らない

Issue #1161で **AWS Organizations配下の専用Terraform/state control-plane member account** と `ap-northeast-1`（東京）のS3 backendをbootstrap済みです。development / production workload accountへ共通stateを置かず、production workload accountがOrganizations management / payerを兼務している構成の見直しは別scopeとします。実account ID / bucket名 / role ARN等はpublic repositoryへ保存しません。

### Public repository CI boundary

このrepositoryはpublicのため、Terraform CIを次の2段階に分離します。

**通常PR CI（credentialなし）**

- `terraform fmt`
- `terraform init -backend=false -lockfile=readonly`
- `terraform validate`
- provider lock completeness
- static / security checks

forkを含む通常PRへAWS / GCP credentialを渡しません。

**認証付きplan / apply（後続Phase）**

- GitHub OIDC → AWSの短期credentialでS3 backendへアクセスする
- GitHub OIDC / Workload Identity Federation → GCPの短期credentialでtarget workload projectへアクセスする
- long-lived AWS key / Google Service Account JSON keyをGitHub Secretsへ保存しない
- cloud側trustはrepositoryだけでなくtrusted ref / GitHub Environment / workflow等へ可能な限り限定する
- production applyはGitHub Environmentのmanual approval必須
- plan用identityとapply用identityの分離を検討し、少なくともproduction applyは専用least-privilege identityにする
- full saved plan file / raw stateをpublic Actions artifactへuploadしない
- `terraform show -json` 等のraw sensitive outputをpublic logへ出さない
- public log / PR commentへ出すのはsanitized summaryを原則とする
- application secret valueはTerraformへ極力流さず、Secret Manager等のsecret storeとwrite-only / ephemeralな経路を優先する

外部forkのTerraformコードへcredential付きplanを自動実行しません。authenticated planをPR前に行う場合も、trusted same-repository commitを明示的なgate後に実行する設計とします。

### Authenticated workflow source guard

Issue #1162のsource-control側guardrailとして、default branchにも存在する `.github/workflows/ci.yml` の `workflow_dispatch` を入口にし、同一commitの `.github/workflows/gcp-terraform-authenticated.yml` reusable workflowを呼び出します。専用workflowを直接 `workflow_dispatch` にしないのは、移行中はこの新規workflow fileがdefault branch (`dev`) に存在せず、GitHubのmanual dispatch入口として成立しないためです。

development planとapplyは独立gateを持ちます。planの有効化はtrust構築後、applyの有効化はplan smoke / negative test PASS後の別変更です。

- `DEV_AUTHENTICATED_TERRAFORM_ENABLED=true`（development trust構築後のplan smokeのみ）
- `DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED=true`（development plan smoke / negative test / native lock / public output audit PASS後に有効化）
- `PROD_AUTHENTICATED_TERRAFORM_ENABLED=false`
- development applyは別Environmentの承認と同一SHAの再plan / import-only検証を引き続き必須とする。gate有効化だけではapplyを実行せず、workload importは後続waveで扱う
- authenticated executionの入口は既存 `ci.yml` の `workflow_dispatch` のみ。`terraform_authenticated=true` を明示したrunだけreusable workflowを呼ぶ
- 任意commit SHAはinputで受け取らない
- PR headへcredentialを渡さない
- integration期間中は `feature/gcp-terraform-iac` 以外のrefを拒否し、callerも `ci.yml@refs/heads/feature/gcp-terraform-iac` に固定する
- repository nameに加えてimmutable repository / owner IDもpreflightで確認する
- authenticated jobだけに `id-token: write` を付ける
- external Actionsはfull commit SHAへpinする
- AWS assumed-role ID / role名、GCP project number / SA名も認証Actionより前にmaskする。Environmentには既存5 secretに加えて `AWS_TERRAFORM_BACKEND_ROLE_ID` を設定する
- backendのworkspace discovery prefixは対象environment配下に固定し、`TF_WORKSPACE=default` を使用する
- development planは空resource graphでもGCP WIF token交換とplan SA impersonationを強制し、project metadataのharmless readで認証を証明する
- identity smokeはAWS dev state read、stateへの条件付きPut拒否、prod / 他productのread/list拒否、wrong EnvironmentのSTS拒否、GCP mutation permission / prod permission不在と別SA impersonation拒否を確認する。unexpected grant / network error / object不在をDENY成功と混同しない
- raw init / plan / apply出力はpublic logへ流さない
- saved planはrunner一時領域だけで扱い、artifact / cacheへ保存しない
- public outputは `.github/scripts/terraform_plan_summary.py` が生成するresource address / action count中心のsanitized summaryだけ
- import移行期はcreate / update / delete / replacement / driftをstopする
- plan jobとapply jobでsaved planを渡さず、apply jobは同じ `github.sha` から再planし、sanitized projectionが一致した場合だけ同一job内のplanをapplyする

このauthenticated reusable workflowを有効化する前に、GitHub Environment / branch trust、AWS GitHub OIDC backend role、GCP GitHub WIF / Terraform Service Accountを構築する必要があります。これらは実環境のtrust / identity mutationなので、Issue #1162のmutation gateに従い明示approval後に行います。

production backendは未bootstrapのため、production authenticated plan / applyはbackend準備完了まで有効化しません。

### Existing development backend

Issue #1154で作成した `test-youtube-study-space` 内のdevelopment GCS state bucketは、Issue #1161で確認した時点でTerraform上の**空state**でした。

- Terraform backend migrationを実行したが、Terraform 1.16.4は空source stateをcopyしないため、migration成功とは扱わない
- S3 destination backendをTerraform自身で初期化済み
- init / validate / state read / native lock / Versioning / read-only recoveryを実測PASS
- source GCS bucket/stateは保持する
- production GCS state bucketは作成しない
- old GCS bucket削除は別判断とする

### Local backend configuration

各rootでexampleをcopyしてgitignoredな `backend.hcl` を作成します。

```bash
cp infra/gcp/environments/dev/backend.hcl.example infra/gcp/environments/dev/backend.hcl
terraform -chdir=infra/gcp/environments/dev init -reconfigure -backend-config=backend.hcl
```

backend credentialはbackend configへ直接書かず、ローカルではAWS SSO等のcredential chain、CIではGitHub OIDCによる短期credentialを使います。

## State recovery

backend bucketのObject Versioningを前提に、誤ったstate更新・削除時は以下の順序で停止・復旧します。

1. apply / importを停止する
2. 対象environmentとbackend bucketを再確認する
3. bucket object generationをread-onlyで確認する
4. 復旧対象generationを特定する
5. 現行stateを別名で退避する
6. 選定したgenerationからstateを復元する
7. `terraform plan` でremote resourceとの整合を確認する
8. 差分の原因が説明できるまでapplyしない

dev / prodを跨いだstate copyは行いません。

## Import policy

既存resourceは configuration-driven `import` block を標準とします。

順序:

1. fresh inventory
2. resource definition
3. import block
4. `terraform plan`
5. remote実値を正しく表すまでconfigurationを修正
6. import直後のplanを原則no-opへする
7. importとは別PR / 別差分で設定改善を行う

import acceptance:

- create: 0
- update: 0
- delete: 0
- replacement: 0

bucket / BigQuery / IAM / Scheduler等でdestroyやreplacementが出た場合は停止します。

## Authentication boundary

Phase 1のrepository validationはcloud credentialを使いません。

Issue #1161のdevelopment backend bootstrapはoperatorの短期SSO credentialで完了済みです。長期AWS access key / secret key、Google Service Account JSON keyは作成していません。

通常運用では認証をCIへ寄せます。

- S3 backend: GitHub OIDC → AWS IAM role
- GCP provider: GitHub OIDC / Workload Identity Federation → GCP
- ローカルのauthenticated `plan` / state operationはmigration・障害調査等の例外用途とし、AWS SSO / Google ADC等の短期・更新可能credentialを使う
- public PR CIはcredentiallessを維持する
- authenticated plan / apply workflowは後続Phaseでtrusted ref / GitHub Environment / least privilegeを実装する

既存AWS runtime → GCP WIFとはTerraform CI trustを分離します。

## Production boundary

production:

- 最初のimportは同resource typeをdevで確立してから
- applyはmanual approval必須
- import直後の想定外diffをapplyしない
- destructive planは停止
- Node.js 22 migrationとTerraform ownership移行を同じ変更にしない


## Bootstrap follow-up

Issue #1161のdevelopment S3 backend bootstrap本体は完了しています。

残件はbootstrapとは分離して追跡します。

- Issue #1165: alternate contacts / recovery運用
- Issue #1166: durable CloudTrail / S3 data events監査
- Issue #1162: public repository向けauthenticated Terraform plan / apply
- production backend: development安定後の別Issue

## Development import wave 1: native backup schedule

`environments/dev/native-backup.tf` は、実環境のdaily backup scheduleをconfiguration-driven importします。これはFirestoreのnative backupであり、#1173で保留しているScheduler / Pub/Sub / export Functionとは別resourceです。

- Same: daily recurrence / `(default)` database
- Intended environment difference: developmentは30日、productionは20日保持。既存値を変更しない
- provider ownership: developmentのschedule 1件のみ。Firestore database本体、backup data、Rules / Indexesは対象外
- Google providerのReadはscheduleのGETだけを使う。dev CI custom roleへの追加は `datastore.backupSchedules.get` だけとし、list / create / update / delete、backup本文read、prod権限は追加しない
- `prevent_destroy` と既存CI import-only guardを維持する。providerのlocal `deletion_policy` defaultはimport時に変更せず、no-opを確認する
- credentialless PR CI PASS後にintegration branchへmergeし、同じSHAのauthenticated plan → Environment-approved import-only apply → post-apply no-opを確認する
- production定義 / import、export chain、API ownership / runtime WIFは別wave

2026-10-05の承認済みoperator waveでdevelopment backup bucketのPAPだけを`enforced`へ修正した。両環境のPAPは一致し、他bucket metadata / IAM bindingは不変。`user-activity-history` のdev-only `timestamp` は不要なlegacy driftだが、値・consumer・再流入・復元確認前のDROPとimportは保留する。

## Development backup bucket import

`environments/dev/backup-bucket.tf` imports the existing retained Firestore export bucket through the shared `retained-backup-bucket` module. Objects, backup data, IAM, production resources and export triggers remain outside this wave. `force_destroy=false` / `prevent_destroy` are required. Region, storage class, retention and soft delete preserve inventory values. Review the module README for the dev/prod metadata contract. Accept only import/read/no-op and require post-apply complete no-op through the existing Environment-approved workflow. A missing CI `storage.buckets.get` permission is an approval boundary, not grounds to add Storage admin.

The obsolete empty development GCS state backend was retired in a separate approved operator wave on 2026-10-05. The old operator's executable config/cache was disabled; empty state and noncurrent init lock generations were conditionally deleted, followed by the bucket. Seven-day soft delete remains enabled. Recovery authority is the current versioned S3 state; never reconnect to the retired GCS backend. Detailed verification belongs to #1161 / #1162 and Notion Current Canon.

## YouTube quota alert symmetry

Both dev/prod roots use `modules/youtube-quota-alerts` with environment parameters. Policy ownership is disabled during import-only migration; the existing production policies are untouched. Read-only preflight found no notification channel in either project, while all three production policies reference a missing channel. Actual dev creation is deferred to a separately approved normal-change apply, after channel verification. Mock plans prove three logical types and unchanged import-only rejection of create=3. See the module README for baseline, future import addressing, and remaining gates.

## Private primary Email preparation

Both environments share `monitoring-notification-channels`; quota policies consume its sensitive channel-name output when ownership is enabled. Development native Email channel was created in a separate approved operator wave; production channel/policies are unchanged. Actual mailbox and channel ID are private Environment secret inputs only, and may enter protected S3 state.

Development Email adoption completed through the dedicated protected route with post-plan no-op5 and unchanged GCP metadata. `DEV_PRIMARY_EMAIL_IMPORT_ENABLED=true` now keeps its ownership in ordinary import-only runs. Real import preflight found a sensitivity-only `labels` update despite unchanged resource values. The global import-only guard correctly rejects that update.

The separately approved `terraform_mode=email-adoption` dispatch uses the same trusted ref, plan/apply Environments, identities and same-SHA re-plan. Its dedicated validator requires exactly the existing development channel import, equal before/after values, no unknowns/drift, only the label sensitivity mark, and the four previously managed resources as no-op. Post-apply must be a complete five-resource no-op with no import. The global import-only sanitizer is unchanged; ordinary `plan`/`apply` still use it. Re-running adoption after success is rejected rather than authorizing a wider update. Rollback authority is the versioned S3 state; do not restore an old version over later state writes. No alert creation or GCP mutation grant is enabled by this route. Delivery/receipt remains untested.

Private recipient and channel name are supplied through the existing two development Environments. Only `storage.buckets.get` and `monitoring.notificationChannels.get` were added to the existing custom read role under #1162 approval; its principals and bindings are unchanged. See the channel module README and #1162 for the historical preflight.

## Development quota normal-change preparation

`terraform_mode=quota-plan` produces a read-only full-root candidate with existing five resources no-op and exactly three fixed quota creates. `terraform_quota_create_gate.py` validates all configured policy fields, the #1175 query hashes, ratios 0.8/0.8/0.6, duration 60s, target project and the adopted Email only. Unknowns are limited to provider-generated policy/condition identities, creation metadata and local deletion policy; unknown notification/config values are rejected. Import/update/delete/replace/drift/other actions are rejected. Public summaries contain addresses/counts only. Post-create requires all eight resources no-op.

Issue #1162 explicitly approved and provisioned one `monitoring.alertPolicies.get` addition to the existing dev read role and one `monitoring.alertPolicies.create` permission in a separate dev apply-only custom role bound only to the existing apply principal. `quota-create` is closed after the successful fixed three-create apply; independent plan/apply Environments, same-SHA re-plan, projection matching and post-plan no-op8 remain mandatory. Runtime identity checks require plan GET only and apply GET + CREATE, and reject UPDATE/DELETE/list/data/IAM/API/production grants. No predefined role or global import-only guard change is used. Ordinary runs keep all eight resources owned after creation; `quota-plan` never executes apply. After successful creation, a separate reviewed follow-up closes the create gate and maintains persistent quota ownership before an ordinary import-only run.

Rollback of a partial create uses a private manifest of the exact newly created policy names and the existing operator's permission: remove only those new policies, preserve the channel/previous five resources, reconcile Terraform ownership under the native lock, then prove no-op5. Do not run broad destroy or overwrite later state with an old S3 version. Terraform `prevent_destroy` and the normal gate intentionally reject deletion; operator rollback is a separate bounded procedure. After successful creation and no-op8, enable persistent quota ownership in a reviewed follow-up before an ordinary import-only run.

## Development quota empty-label state reconciliation

The three-create apply succeeded, but provider read-back represented absent `user_labels` as `{}` while creation saved `null`. All eight normal resource actions were no-op; three representation-only drift records correctly stopped the strict post guard. Explicit `user_labels = {}` prevents recurrence without adding a real label. Post validation also matches the exact observed provider defaults (empty severity/subject/evaluation and zero trigger percent); arbitrary values remain rejected. These defaults are already identical before/after, so they introduce no additional refresh delta. Global import-only policy remains unchanged.

`quota-refresh` completed under Issue #1162 approval with regular post-plan no-op8/drift0; its one-time gate is closed. Its dedicated validator requires a full-root saved `-refresh-only` plan, zero workload resource changes, exactly the three newly owned policy addresses with only `user_labels: null -> {}` (including its corresponding sensitivity mask), eight resources in the refreshed graph, exact approved policy/channel semantics, unknown zero, output no-op and other drift zero. Public summaries retain drift=3 rather than concealing it. Independent plan/apply Environment approval, same-SHA re-plan and projection matching remain mandatory. The saved refresh plan can persist state only; the final regular plan must pass no-op8/drift0. No IAM/API/workload or production write is authorized by this mode.

State versioning retains the previous version for investigation. Do not overwrite newer state to roll back a representation-only refresh; inspect current state under the native lock and require a new bounded plan for any subsequent reconciliation. The correction changed only the three empty label maps. Ordinary full-root plans now have no representation drift.

## Development export chain adoption

Development quota refresh completed with regular no-op8/drift0. PR #1184/#1186
and protected run37342000553 adopted the existing topic; post-plan is
import0/no-op9/drift0. Only `pubsub.topics.get` was added to the existing read role.
Use topic → Scheduler → Gen1 Function as separate import-only waves.
Generated resources, IAM and source rebuild/upload remain excluded.

## Approved quota state representation correction

Issue #1162 comment5997848683 authorizes the one-time `quota-refresh` route for
exactly the three newly created development quota policies. The separately approved route
does not widen the global import-only policy. The dedicated validator requires a
complete eight-resource full-root refresh-only plan, no workload actions or
unknowns, only `user_labels: null -> {}` and its matching sensitivity map, and
unchanged fixed policy values. Independent plan/apply Environments, same-SHA
re-plan and projection equality remain mandatory. The post-plan is a regular
full-root plan and must be no-op8/drift0. The exceptional gate is closed after success;
never restore an old S3 version over subsequent state writes. No cloud, IAM, API
or production mutation is authorized by this state correction.

## Approved development topic wave

PR #1184 defines the existing topic. Issue #1162 comment5998188927 approves
only `pubsub.topics.get` in the existing development read role and topic import.
The authenticated workflow keeps topic ownership enabled for both identities;
the regular root default remains disabled for credentialless validation. An
additive exact nine-resource validator preserves the global import-only guard:
accept import1/topic no-op plus existing8 no-op, unknown/drift/other0; after
adoption accept import0/no-op9. Independent Environments and same-SHA re-plan
are unchanged. No subscription/IAM/generated/source ownership or cloud config
mutation. Scheduler/Function permissions remain separate approval boundaries.

## Approved development Scheduler wave

Issue #1162 comment6005196430 separately approves only `cloudscheduler.jobs.get`
in the existing development CI read role and one Scheduler import. Existing
principals/bindings, production IAM and APIs are unchanged. The root default
stays false for credentialless validation; protected CI enables persistent
Scheduler ownership only for development, with the adopted topic dependency.

An additive exact ten-resource validator preserves the global import-only
policy: Scheduler import1/no-op plus existing9 no-op, no drift/unknown/mutation
or generated ownership. Both CI identities must have only the approved topic
and Scheduler GETs in the export permissions checked; list/create/update/delete/
run/pause/resume/publish and Function GET remain absent. Resume uses the IAM
permission `cloudscheduler.jobs.enable`, which is explicitly checked as absent.
Independent Environments, same-SHA re-plan, projection equality and saved
import-only apply remain mandatory. Post-plan must be import0/no-op10/drift0.

See [the Scheduler runbook](modules/firestore-export-scheduler/README.md) for
fresh fields, state/metadata audits and rollback boundaries. Function ownership
waits for Scheduler completion; Function GET remains a separate approval.

### Scheduler completion and default-off Function definition — 2026-10-06

Scheduler activation #1188 completed on integration SHA `eb46cb9603f320dbf2a729cafecf8a88ac598c8a`, protected run [37388902196](https://github.com/kani3camp/youtube-study-space/actions/runs/37388902196). Separate plan/apply approvals, same-SHA re-plan and projection equality passed. Pre-plan: Scheduler import1 + existing9 no-op; post-plan and independent regular full-root: import0 / no-op10 / drift0 / other actions0. Only `cloudscheduler.jobs.get` was added to the existing dev read role, with bindings unchanged. Cloud metadata in both environments, IAM/API and generated resources remain unchanged by import; S3 serial12 / resources10 / lineage unchanged / native lock released. Public logs/artifacts leak audit passed.

`environments/dev/export-function.tf` now defines the next Gen1 wave, disabled by default and absent from production. Existing topic/Scheduler CI flags do not activate it. Fresh inventory and an isolated provider import confirm Node22 / ACTIVE / version8, unchanged deployment/source/lock provenance, import1 / no-op1 / drift0 / unknown0, and the external reserved label/source boundary. See the [Function module contract](modules/firestore-export-function/README.md).

`cloudfunctions.functions.get` is the exact next read candidate, freshly verified from pinned provider Read and the official API. Function IAM/CI activation/import/apply remain stopped for separate user approval. No source build/upload/redeploy or Google-managed ownership is part of this definition.

### Approved development Function activation — 2026-10-06

[#1162 comment6005988785](https://github.com/kani3camp/youtube-study-space/issues/1162#issuecomment-6005988785) approves only `cloudfunctions.functions.get` in the existing development CI read role. Principals/bindings remain unchanged; no list/mutation/call/invoke/sourceCode/IAM/API/broad-role/production additions. The workflow enables the previously reviewed default-off Function definition only for development, after adopted topic and Scheduler dependencies. A harmless exact Function metadata GET supplies the preserved execution identity privately to both jobs.

The unchanged global import-only policy plus `terraform_export_function_gate.py` require Function import1 + existing10 no-op, drift/unknown/unexpected action0, then post import0 / no-op11 / drift0. Independent Environment approvals, same-SHA re-plan/projection equality and saved-plan apply remain mandatory. Runtime Node22/ACTIVE/version8, trigger/retry, environment, execution identity and limits are fixed. Source/redeployment and Google-managed generated resources stay outside ownership; no manual trigger is permitted. See the [Function contract](modules/firestore-export-function/README.md) for provenance and state-only rollback.
