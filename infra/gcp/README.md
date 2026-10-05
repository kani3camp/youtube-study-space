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
- import-only以外のGitHub Actions workload apply（protected CIはIssue #1162）
- Service Account JSON keyの作成
- MyPage resourceのprovisioning

Firestore export FunctionのNode.js 22移行は Issue #1148 の別gateです。Scheduler / Pub/Sub / export Functionのimportは、developmentとproductionのNode.js 22自然実行が安定するまで開始しません。

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

Email ownership is explicitly disabled by `DEV_PRIMARY_EMAIL_IMPORT_ENABLED=false`. Real import preflight found a sensitivity-only `labels` update despite unchanged resource values. The import-only guard correctly rejects it. Keep the guard and sensitive config intact; resolve this as a separately reviewed narrow adoption route before channel import or quota creation. No alert ownership is enabled by this PR; delivery/receipt remains untested. See the channel module README and #1162.
