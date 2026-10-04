# GCP Terraform

YouTube Study Space の既存GCP resourceを、安全に段階移行するためのTerraform rootです。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 現在のscope

Phase 1は **scaffold / state設計 / CI validationのみ** です。

この段階では以下を行いません（remote state bootstrap exceptionを除く）。

- 既存workload GCP resourceのimport
- workload resourceのcreate / update / delete
- Cloud Functions / Scheduler / Pub/Subの変更
- workload側のIAM / API変更
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

Issue #1161のread-only preflightにより、state bucketは **AWS Organizations配下に新設する専用Terraform/state control-plane member account** に置くことを決定しています。regionは `ap-northeast-1`（東京）です。development / production workload accountへ共通stateを置かず、production workload accountがOrganizations management / payerを兼務している構成の見直しは別scopeとします。

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

### Existing development backend

Issue #1154で作成した `test-youtube-study-space` 内のdevelopment GCS state bucketは、S3へのmigrationが完了するまで一時的なmigration sourceとして維持します。

- 直ちに削除しない
- production GCS state bucketは作成しない
- dev migrationは専用Issueで実施する
- source / destinationを明示し、dev stateだけを移す
- migrationと旧GCS bucket削除を同じ作業にしない
- rollback確認期間を置いてから旧bucketの扱いを別判断する

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

backend bootstrap / state migrationはoperator credentialを使用できますが、長期AWS access key / secret key、Google Service Account JSON keyは新規作成しません。

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
