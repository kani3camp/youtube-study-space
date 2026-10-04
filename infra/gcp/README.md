# GCP Terraform

YouTube Study Space の既存GCP resourceを、安全に段階移行するためのTerraform rootです。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 現在のscope

Phase 1は **scaffold / state設計 / CI validationのみ** です。

この段階では以下を行いません。

- 既存GCP resourceのimport
- resourceのcreate / update / delete
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

`terraform init -lockfile=readonly` を通常validationに使い、provider lockの変更を暗黙に許可しません。dependency更新時だけ通常の `terraform init` / `terraform providers lock` でlockを更新し、差分をreviewします。

## Remote state bootstrap

State bucketはTerraform本体の外側にある one-time bootstrap exception とします。

必須条件:

- dev bucketは `test-youtube-study-space` project内
- prod bucketは `youtube-study-space` project内
- dev / prodで別bucket
- Firestore backup bucketと共用しない
- Object Versioning有効
- Uniform bucket-level access有効
- Public Access Prevention enforced
- business data / build artifactを置かない
- public accessを許可しない
- access principalをoperator / 後続のGitHub Actions WIFに限定する

bucket名はglobal uniqueness確認後にoperator作業で決定します。secretやcredentialはREADME / backend configへ保存しません。

作成後、各rootでexampleをcopyしてgitignoredな `backend.hcl` を作成します。

```bash
cp infra/gcp/environments/dev/backend.hcl.example infra/gcp/environments/dev/backend.hcl
# bucketを実値へ変更
terraform -chdir=infra/gcp/environments/dev init -reconfigure -backend-config=backend.hcl
```

productionも同様ですが、devで手順とstate recoveryを確認してから実施します。

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
