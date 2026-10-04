# GCP Terraform

YouTube Study Space の既存GCP resourceを、安全に段階移行するためのTerraform rootです。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 現在のscope

Phase 1は **scaffold / state設計 / CI validationのみ** です。

この段階では以下を行いません（remote state bootstrap exceptionを除く）。

- 既存workload GCP resourceのimport
- workload resourceのcreate / update / delete
- Cloud Functions / Scheduler / Pub/Subの変更
- IAM / APIの変更
- GitHub Actionsからのplan / apply
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

Issue #1155のIAM reviewを受け、remote stateはworkload projectから分離した **専用Terraform state project** に置きます。

原則:

- state projectは1つとし、workload resourceを置かない
- dev / prodは別GCS bucket・別prefix・別backend identityを使う
- dev backend identityはdev bucketだけ、prod backend identityはprod bucketだけへアクセスさせる
- backend identityへworkload projectのOwner / Editor / Storage Adminを付与しない
- bucket accessは原則bucket scopeの `roles/storage.objectAdmin` を基準にし、bucket IAM管理権限は付与しない
- operator / recovery主体とCI backend identityを分離する
- GitHub Actions WIF導入時もdev / prodのbackend trustを分離し、既存AWS runtime WIFを流用しない

Google Cloudのproject-level allow policyはproject配下のresourceへ継承されます。stateを専用projectへ分離することで、Firebase / App Engine / Cloud Build等のworkload project IAMがstate bucketへ継承される経路を切ります。

### Bucket requirements

dev / prodの各state bucketは以下を満たします。

- Firestore backup bucketと共用しない
- Object Versioning有効
- Uniform bucket-level access有効
- Public Access Prevention enforced
- business data / build artifactを置かない
- `allUsers` / `allAuthenticatedUsers` を許可しない
- retention lockを初期bootstrapで設定しない
- lifecycle delete ruleを初期bootstrapで設定しない
- soft delete等のplatform既定値は実測して記録する

state project ID、bucket名、billing、recovery principalはbootstrap時に確認して確定します。secretやcredentialはREADME / backend configへ保存しません。

### Existing development bucket

Issue #1154で作成した `test-youtube-study-space` 内のdevelopment state bucketは、専用state projectへのmigrationが完了するまで一時的な既存backendとして扱います。

- 直ちに削除しない
- production stateをworkload project内には作成しない
- dev migrationは専用Issueで実施する
- source / destinationを明示し、dev stateだけを移す
- migrationと旧bucket削除を同じ作業にしない
- rollback確認期間を置いてから旧bucketの扱いを別判断する

### Local backend configuration

各rootでexampleをcopyしてgitignoredな `backend.hcl` を作成します。

```bash
cp infra/gcp/environments/dev/backend.hcl.example infra/gcp/environments/dev/backend.hcl
# dedicated state project内のdev bucketへ変更
terraform -chdir=infra/gcp/environments/dev init -reconfigure -backend-config=backend.hcl
```

backend identityのimpersonationを使う場合はcredential fileを保存せず、GCS backendのservice account impersonation機能または短期credentialを使います。

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

Phase 1のrepository validationはGCP credentialを使いません。

最初のbackend bootstrap / importはoperator credentialを使用できますが、長期Service Account JSON keyは新規作成しません。GitHub Actionsからのkeyless plan/applyは後続Phaseで専用OIDC / WIF principalを作ります。既存AWS runtime WIFとは分離します。

## Production boundary

production:

- 最初のimportは同resource typeをdevで確立してから
- applyはmanual approval必須
- import直後の想定外diffをapplyしない
- destructive planは停止
- Node.js 22 migrationとTerraform ownership移行を同じ変更にしない
