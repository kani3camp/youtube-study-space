# Runtime AWS WIF ownership preparation

AWS Lambda/Fargate → Google Cloud WIF の runtime 実装・移行は既に完了している。
この module は既存 `aws-runtime` pool / `aws-provider` AWS provider / runtime SA 上の
選択した `roles/iam.workloadIdentityUser` **個別 member** を ownership 移管するための
default-off 定義。working trust を生成・修正しない。

`own_pool=false` / `own_provider=false` / `grant_keys=[]` が既定値。
pool → provider → exact grant の順で、前 wave の ownership を維持したまま追加できる。
grants の address は `grant-01` 等の公開 alias とし、実 AWS role / principal は private input のみに置く。
既存 IAM condition の title / description / expression もそのまま保持する。

default App Engine SA identity、本体 policy / role binding、project IAM、Google-managed
service agent、JSON key、GitHub Terraform CI federation、AWS CDK runtime は管理対象外。
全 resource で `prevent_destroy` を維持し、通常 import-only guard を変更しない。

fresh metadata と承認・protected route はまだ必要。
[実行前 runbook](../../docs/runtime-wif-api-ownership.md) を参照。
