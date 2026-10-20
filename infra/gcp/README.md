# GCP Terraform

YouTube Study SpaceのDevelopment／Productionで採用済みのGCP resourceをTerraform管理するためのrootと運用入口です。実行結果や導入時の判断は[変更履歴](#実行記録と変更履歴)を参照してください。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 通常のDEV Terraform開発

`ci.yml` の本人による初回 `workflow_dispatch` で `terraform_authenticated=true`、
`terraform_target=dev`、`terraform_mode=normal-plan` / `normal-apply` を選択します。
`terraform_expected_sha` には実行するbranchの40桁HEADを指定します。
認証付き通常操作の**ブランチ許可はGitHub EnvironmentのSelected branchesだけ**で管理します。controller / AWS OIDC / GCP WIFはブランチ名を個別allowlistせず、同一refの正規workflow・本人初回dispatch・非`main`のbranch refだけを検証します。
PR/fork、tag、`main`でのDEV通常操作、再実行は認証前またはEnvironmentで拒否します。Productionの通常操作は保護された`main`だけです。
`normal-apply` は同じrunで独立Plan job→Apply jobを実行します。

1. 通常のHCL追加・修正は任意の作業/統合ブランチ向けPRで進める。認証付きPlan/Applyが必要なブランチだけをGitHubの`terraform-dev-plan` / `terraform-dev-apply` Environmentへ明示追加する。`dev`は運用可能な統合結果だけを最終PRで受け入れる。
2. 新しい非秘密入力は `normal.tfvars.json` に追加する。既存Environmentのprivate tfvarsを読み、
   このcommitのファイルで上書きするため、既存リソースの選択・private inventoryを落とさない。
   秘密値はコミットしない。新しいprivate入力は両DEV Environmentのmanaged入力を同じ内容へ更新する。
3. delete/replaceが必要なら、同じPRの `normal-operations.json` に正確なaddressとactionsを記載する。
   create/updateにはリソース別manifestやtype登録を要求しない。
4. 作業PRのcredentialless CIとレビューを確認する。認証付き検証が必要なら対象ブランチをGitHub Environmentへ追加する。専用統合ブランチを使う作業ではCodexがそこへマージしてよい。
5. Environmentで許可した対象ブランチのHEADから認証付き `normal-plan` / `normal-e2e` を実行し、Actionsのログとjob summaryで変更リソース種別・操作・件数を確認する。
6. 適用する場合は同一ブランチ・HEADで `normal-apply` を新規dispatchする。
   これは新しいPlanを作り、Apply identityの再Plan・State/SHA/入力一致を検査してsaved planを適用する。
7. `GCP_NORMAL: PASS; role=apply` と事後full-root no-opを確認する。
   別jobの再Plan HMAC比較はGitHub Actionsの `if` 条件で行い、**HMACをstep envやcommandへ渡さない**。一致しなければApplyを実行せず、セクション別比較の真偽値だけで `GCP_NORMAL_REPLAN: STOP` を表示する。値・address・HMACは公開しない。
8. DEV実地検証と復旧確認を完了してから、専用統合ブランチ→`dev`の最終PR（1本）をレビュー用に残す。`dev`へのマージは本人だけが行う。
   破壊的変更のレビュー指定は次のPRで削除する。未使用の指定も拒否される。

破壊的変更の例（削除順を含めてTerraform Planと完全一致が必要）:

```json
{
  "destructive_changes": {
    "google_pubsub_topic.retiring": ["delete"],
    "google_pubsub_topic.replaced": ["delete", "create"]
  }
}
```

認証境界・State整合検査は[実行契約](docs/terraform-authentication-boundary.md#devの通常planapply)を参照してください。
S3 native lockingとVersioning、環境別concurrencyを継続し、workspace・target apply・force-unlockは使いません。
raw Plan/State/Secret/API応答はログ・artifactへ公開しません。公開結果は固定phase、操作件数、変更リソースの種別・操作別件数です。
種別名は固定providerのschemaに存在する名前だけを使用し、address・for_eachキー・属性名/値は表示しません。
no-opは一覧から省略し、replaceはcreate/deleteの順序も表示します。
変更値はHCL/入力のレビューで確認し、秘密値を含む生Planが必要な障害調査はprivateな管理端末で行います。

### 一回限りのDEV基盤設定

`bootstrap_normal_ops.py --check` はGCPの現行ロール定義を読み、予定権限だけを表示します。
設定実行は `python3 infra/gcp/scripts/bootstrap_normal_ops.py` です。
既存のGCP管理login、GitHub Environment secret管理権限が必要です。長期鍵は作りません。
途中失敗時も同じコマンドを再実行でき、各設定はreadbackで検査します。

- 既存AWS Plan/Apply backend role・DEV key/lock・2 Environmentを再利用する。
- GCPに通常専用Plan/Apply SA、DEV固定`terraform-normal` WIF pool/provider、
  リソース管理用カスタムロール2個を作る。既存migration SA/WIFは変更しない。
- 両DEV Environmentに `GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT` /
  `GCP_TERRAFORM_NORMAL_WIF_PROVIDER` を設定する。既存managed入力はそのまま使う。
- Cloud Run/Functions、Scheduler、Pub/Sub、Artifact Registry、Storage bucket、BigQuery構成、
  Monitoring、Logging、Secret metadata、API有効化、SAの作成/更新/削除をまとめて準備する。
  権限は現在の公式role catalogと明示的な資源/操作の共通部分から作る。型別の実行器は不要。
- データ本文の読書き、SA鍵、token発行、project IAM、WIF管理は通常SAへ付与しない。
  Secret本文、Storage object内容、BigQueryデータ操作は専用の権限審査対象。
  IAM管理のためだけに通常SAへOwner/Editorを付与しない。
- Cloud Run等の実行SAには、project権限なしの共通 `terraform-dev-workload` を準備し、
  通常Apply SAの`actAs`だけを当該SA上で許可する。業務別のruntime権限は別の明示的なIAM変更。

設定後はSAのproject roleとtrust、WIFのissuer・mapping・条件、user-managed keyがないこと、
runtime SAの権限とactAs付与先、既存identityへの影響を実値で確認します。
権限数やpermission一覧は `--check` とIAMのreadbackから取得し、固定値を本書へ転記しません。
GitHub Secretは値を読み戻せないため名前と更新時刻を確認し、実際の認証・Plan/ApplyはActionsで検証します。
設定変更時の確認項目は[認証・実行境界](docs/terraform-authentication-boundary.md#設定変更時の確認)を参照してください。

### GitHub Environmentで許可したブランチの認証付きE2E

AWS DEV backend Roleは移行用workflowと共有する既存の7 claimsカスタムsubjectを維持します。初回設定時にAWSネイティブの `token.actions.githubusercontent.com:actor_id` を `StringEquals: 54093651` で固定し、改変されたworkflowでも他actorからState identityを取得できないようにします。GitHubのrepository-wide subject templateとProduction Roleは変更しません。GCP通常WIFもブランチ名をallowlistせず、repository / owner / Environment / actor / workflow path / same-ref / same-SHAを恒久的に検証します。
**ブランチごとのPlan/Apply許可はGitHub Environmentだけ**で変更します。`infra/gcp/scripts/enable_normal_integration.py --apply --branch <branch>` は `terraform-dev-plan` / `terraform-dev-apply` のSelected branchesへ完全一致のbranch名を追加するだけです。通常利用にAWS Profileやgcloud loginは不要で、GitHub Environmentを管理できる`gh`認証だけを使います。既存migration/Production identityの権限は変更しません。
GitHubの Settings → Environments からも操作できます。Planだけを許可する場合は `terraform-dev-plan`、Applyも許可する場合は `terraform-dev-plan` / `terraform-dev-apply` の両方で、Selected branches and tagsへ対象branchを追加します。取消時は同じ画面から対象branchのruleを削除します。WIFやAWS trustの編集は不要です。上のスクリプトは両Environmentへの追加を行います。

```bash
python3 infra/gcp/scripts/enable_normal_integration.py --branch integration/gcp-terraform-normal-ops
python3 infra/gcp/scripts/enable_normal_integration.py --branch integration/gcp-terraform-normal-ops --apply
# CI成功後、そのbranchを許可してE2Eも1回起動:
python3 infra/gcp/scripts/enable_normal_integration.py --branch integration/gcp-terraform-normal-ops --apply --dispatch-e2e
```

既存AWS DEV backend Roleのtrustでは、移行用workflowとの共用のため、
固定されたrepository/owner/Environment/caller/reusable等の**7 claims**に対し
ブランチ部分だけ`refs/heads/*`を使う場合があります。現行実値がこの**完全一致した形式**
であれば読み取り検証して維持し、AWS trustを狭めて既存migrationを壊しません。
AWS/GCPはブランチ名の運用allowlistを持ちません。実際に認証付きjobへ到達できるbranchはGitHub Environment policyが決めます。
想定外のwildcard、repo/claim変更、default OIDC subjectは拒否します。
`--verify-aws` は全Allow経路を検査し、Federated principalの文字列/配列を正規化して、単一の承認済みWebIdentity許可・audience・subject・本人actor IDを要求します。初回のactor固定は各DEV Roleの既存trustの `Condition.StringEquals` に `token.actions.githubusercontent.com:actor_id: "54093651"` を追加し、readbackで確認します。以後のbranch追加ではAWSを変更しません。

AWS IAMを再監査したい場合だけ認証済みAWS CLIから`--verify-aws`、GCP WIFを再監査したい場合だけ認証済みgcloudから`--verify-gcp`を追加できます。どちらもread-onlyです。通常のbranch追加ではクラウドCLIを使いません。AWS/GCPの恒久trustが不適切ならGitHub Actionsの短期認証がfail-closedで停止し、State Applyへ進みません。GitHub Environmentの実値が想定と異なる場合は**先にSTOP**します。
失敗時は `NORMAL_BRANCH_ACCESS: STOP; phase=<検証段階>` の固定分類だけを表示します。
AWS/GCP CLIエラーの生ログ・認証情報は表示しません。
`--apply`が変更するのはGitHub Environmentのbranch policyだけです。AWS IAM / GCP WIF / Terraform State / DEV business resources / Productionには触れません。
この運用スクリプトは現行の選択branch policyを変更せず、統合branchの明示追加だけを行います。

一括オプション `--dispatch-e2e` はGitHubで指定branchの必須CI成功と実行者IDをreadbackしてから本人の新規dispatchを1回行います（E2E完了までは判定しない）。
GitHub Environmentでbranchを許可した後、CIを `normal-e2e`、branch `integration/gcp-terraform-normal-ops`、
その正確な40桁HEAD SHAで**一度新規dispatch**します。失敗runのRe-runは拒否されます。
既存rootがno-opのときだけ、runnerのprivate source copyに一時的なDEV Pub/Sub topicを追加し、
create→ラベルupdate→名前replace→deleteを実行します。各段階は対象topicだけの変更を要求し、
saved plan Apply・full-root no-op・lock解放を検証します。既存業務リソースはテスト用に変更しません。
最後に元のState resourcesと一致することを検査します。State serial/versionは正常に進みます。
MyPageのHCL・専用Stateを扱いません。

通常操作は有効な非`main` branch refをcontroller/WIFが受け付けますが、**GitHub Environmentで明示許可されていないbranchはcredentialed jobへ入れません**。AWS/GCPの恒久trustが不正なら認証中に拒否されます。
Environment/WIFの保護は維持し、`dev`向け最終PRのマージは本人が実施します。
E2Eが中途失敗した場合は、そのrunのprobeだけの削除Planで復旧を試みます。
他リソースの差分・lock・認証失敗があれば停止します。自動force-unlockやStateの復元はしません。
probeが残った場合は新しいE2Eを起動せず、State内の `google_pubsub_topic.normal_ops_probe` をprivate端末で確認し、
同じtopic名・projectのHCLを一時的にDEV rootへ宣言して通常Planをno-opに戻します。
その後、HCL削除と明示的delete reviewを通常PRで適用し、no-opを確認してからE2Eを再実行します。

## 通常のPROD Terraform開発（main固定）

Productionの通常Plan/Applyは既存DEVと同じ `normal-plan` / `normal-apply` を使います。**実行refは `main` のみ**。Production `normal-e2e`（一時的な実リソース生成）は禁止します。既存の `migration-*` はimport-only/no-opのまま変更しません。通常実行にはPROD専用IAM/WIF・既存State・全plan/StateのHMAC・saved plan・事後full-root no-opを要求します。

1. `infra/gcp/environments/prod/` のHCLをレビュー対象とし、先にDEVで相当する変更を検証します。既存PRODリソースの宣言とprivate inventoryを落としてはなりません。DEVのRoot/StateをPRODへコピーしません。
2. 非秘密の差分だけを `normal.tfvars.json` に記載します。既存30資源の選択情報・private metadataは `terraform-prod-plan` / `terraform-prod-apply` Environmentのmanaged入力を引き続き使います。両者の入力内容はPlan/Applyで論理的に一致させます。
3. delete/replaceはPRODの `normal-operations.json` の `destructive_changes` に正確なresource addressとnative actionsの順序を明示します。create/updateに型別allowlistはありませんが、GCP IAMの明示権限外の操作は拒否されます。
4. 保護された `main` にレビュー済みリリースを取り込んだ後、本人が初回 `workflow_dispatch` で `terraform_target=prod`、`terraform_mode=normal-plan`、`terraform_expected_sha=<mainの40桁SHA>` を指定します。公開ログではresource type/操作/件数だけを確認します。
5. Applyが必要な場合は同一SHAの `normal-apply` を新たに実行します。同runの独立Plan/Apply jobのHMAC/State/SHA/期限が一致したsaved planだけを適用します。事後のfull-root no-opとlock解放を確認します。

### 一度限りの本番設定と有効化条件

**コードをdevへマージするだけではPRODの認証付きPlan/Applyは有効になりません。** 既存PROD移行用AWS S3 backend Roleを再利用し、通常専用のGCP Plan/Apply SA・カスタムロール・WIF pool/provider・runtime SAを追加します。移行用GCP WIF/SA、S3 State、既存workloadは変更しません。

- PROD AWSの既存Plan/Apply Roleは、repository-wide GitHub OIDCの7 claims（`terraform-prod-plan/apply`、`ref=refs/heads/main`、固定caller/reusable）に加え、native `actor_id=54093651` が必要です。予期しないAllow、旧integration branch trust、wildcardが残ればSTOPします。AWSのtrust調整は管理者が差分を独立レビューしてから実施し、このbootstrapではAWSを変更しません。
- GitHub `terraform-prod-plan` / `terraform-prod-apply` のSelected branchesは**完全一致の `main` のみ**とします。既存managed入力・S3 role/bucket/account秘密参照を維持します。旧integration branchの認可は本番運用切替時に整理します。
- AWS・GCP・GitHubで必要な管理者権限を持つ短期認証済み端末で以下を実行します。

```bash
python3 infra/gcp/scripts/bootstrap_production_normal_ops.py --check
python3 infra/gcp/scripts/bootstrap_production_normal_ops.py --apply
```

`--check` はAWS Role、GitHub OIDC/Environment、GCPの現行権限カタログをread-only検査します。`--apply` は通常専用GCP identity/WIFとGitHub Environmentの**新規通常用Secret参照**だけを設定し、AWS Role/State/既存リソース・migration identityは変更しません。readbackに不一致があればSTOPし、生APIエラー・秘密値は表示しません。

- `main` への認証付きworkflowのリリース後、まずPRODの既存30資源を対象とする認証付きfull-root no-op Planを実測してください。その後、必要最小限の変更についてPlan→承認済みApply→事後no-opを確認します。PRODで一時Pub/Sub CRUDテストを実施しません。

**確認範囲：** GitHubコードとcredentialless CIでは本番の実IAM/State readback、実mainでの初回認証付きPlan/Applyの成功を証明できません。これらはSourceの実装と独立した有効化ゲートです。

## 既存import専用経路

`migration-plan` / `migration-apply` は既存baselineのimport/no-op専用として保持します。
DEVの既存importは従来の `dev` / `integration/mypage-canon-20261006` に限定します。共有Environmentに通常用branchを追加しても、そのbranchへimport経路の認可は広がりません。
独自Attestation/Ownership Receiptは再導入しません。
通常経路はこのimport-only制限を解除しておらず、controller/contract/job本文も変更しません。
DEVでbaseline外のHCL/Stateを追加した後は、固定manifestのmigration経路が拒否する場合があります。
その場合は通常経路で運用し、legacy contractを緩めません。Productionの既存importは引き続きmain固定で、通常CRUDも上記のmain-only経路で有効化します。

既存rootのopt-inやprivate baselineから管理対象を落とすと削除Planにつながるため、
未選択の既定値を既存Stateへ適用しません。State初期化やimportは、その専用契約と復旧手順に従います。
新しい資源領域で権限不足となった場合は、権限カタログをレビューして同じbootstrapで更新します。
MyPage固有リソースと独立Stateは専用統合branchで管理し、二重ownershipを作りません。

移行時のState採用件数、各run・障害・未観測事項は[移行記録](#実行記録と変更履歴)を参照してください。
現在の件数や実設定は、新しいPlanと管理APIから確認します。

## ディレクトリと現行の責務

| 場所 | 用途 |
| --- | --- |
| `environments/dev/`、`environments/prod/` | 環境別の独立root・State key・provider lock。workspaceで環境を切り替えない |
| `modules/` | 管理対象の共通Terraform modulesと既存リソース契約 |
| `scripts/`、`tests/` | 現行の設定検証、offline準備、negative / native fixtureテスト |
| `docs/` | 復旧・運用・準備判定の既存runbook。現行の判断入口は以下のリンク |
| `.github/scripts/terraform_migration.py`、`terraform_migration_contract.py` | 既存baseline import-only実行・拒否契約（通常CRUDは`infra/gcp/scripts/terraform_normal_ops.py`） |
| `.github/workflows/ci.yml`、`gcp-terraform-authenticated.yml` | credentialless PR CIと保護された本人起動の運用経路 |

## Version policy

- Terraform CLI: `.terraform-version` と各rootの `required_version` を一致させる
- Google provider: 各rootの `required_providers` と `.terraform.lock.hcl` を正本にする
- provider lockはTerraform operator / CIで使用する `linux_amd64` / `darwin_arm64` のchecksumを事前生成する
- version更新は通常のdependency変更としてPRでreviewする
- Notionにはpatch versionを複製しない

## ローカル検証

通常の検証はcredentiallessです。

```bash
python3 .github/scripts/test_terraform_migration.py
python3 .github/scripts/test_terraform_migration_contract.py
python3 .github/scripts/test_terraform_authenticated_workflow.py
python3 -B infra/gcp/tests/test_terraform_normal_ops.py
python3 -B infra/gcp/tests/test_terraform_normal_ops_native.py
terraform fmt -check -recursive infra/gcp
for env in dev prod; do
  (cd "infra/gcp/environments/$env" && terraform init -backend=false -lockfile=readonly && terraform validate)
done
```

PRでは **GCP Terraform Validate** が必要な安全契約・Terraform root/module・nativeテストを検証します。
`migration-plan` / `migration-apply`、S3 State/Versioning/locking、OIDC/WIF・分離identity、
secret非漏出・import-only/no-op/unknown/drift拒否の実行コードと対応テストは維持します。
復旧・監査は[State account recovery](docs/state-account-recovery.md)、[durable audit](docs/state-durable-audit.md)、
[現行の運用契約](docs/runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)を参照してください。

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

**現行の認証付きplan / apply**

- GitHub OIDC → AWSの短期credentialでS3 backendへアクセスする
- GitHub OIDC / Workload Identity Federation → GCPの短期credentialでtarget workload projectへアクセスする
- long-lived AWS key / Google Service Account JSON keyをGitHub Secretsへ保存しない
- cloud側trustはrepositoryだけでなくtrusted ref / GitHub Environment / workflow等へ可能な限り限定する
- 本人初回workflow_dispatchと対象refの制限を維持し、必要なreview・保護設定を各Environmentで管理する
- plan用identityとapply用identityを分離し、それぞれのEnvironmentから専用least-privilege identityを使う
- full saved plan file / raw stateをpublic Actions artifactへuploadしない
- `terraform show -json` 等のraw sensitive outputをpublic logへ出さない
- public log / PR commentへ出すのはsanitized summaryを原則とする
- application secret valueはTerraformへ極力流さず、Secret Manager等のsecret storeとwrite-only / ephemeralな経路を優先する

外部forkのTerraformコードへcredential付きplanを自動実行しません。authenticated planをPR前に行う場合も、trusted same-repository commitを明示的なgate後に実行する設計とします。

## State recovery

backend bucketのObject Versioningを前提に、誤ったstate更新・削除時は以下の順序で停止・復旧します。

1. apply / importを停止し、対象account/environment/backend keyとnative lockを確認する
2. 対象environmentとbackend bucketを再確認する
3. S3 object VersionIdと事故前後のStateをprivateにread-only比較する
4. 復旧候補versionと現行Stateのlineage / serial / ownership / 後続writeを比較する
5. 現行Stateをprivate領域に保全し、候補versionの盲目的な上書きを禁止する
6. native lock下で現行ownershipと後続writeを保持できる、対象限定の復旧planをreviewする
7. 承認された復旧手順だけを実行し、完全な `terraform plan` とversion / lockを再検証する
8. 差分の原因が説明できるまで通常applyを再開しない

dev / prodを跨いだstate copyは行いません。
SSO/MFA、break-glass、責任者、private contactの確認は
[State account recovery runbook](docs/state-account-recovery.md)を参照します。

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

repository validationはcloud credentialを使いません。

bootstrapや障害調査にはoperatorの短期SSO/更新可能credentialを使い、長期AWS keyやGCP SA JSON keyを作りません。

通常運用では認証をCIへ寄せます。

- S3 backend: GitHub OIDC → AWS IAM role
- GCP provider: GitHub OIDC / Workload Identity Federation → GCP
- ローカルのauthenticated `plan` / state operationはmigration・障害調査等の例外用途とし、AWS SSO / Google ADC等の短期・更新可能credentialを使う
- public PR CIはcredentiallessを維持する
- DEV通常操作は専用WIF/SAとPlan/Apply別Environment・least privilegeを使い、許可する非`main`ブランチはGitHub Environmentで管理する
- Productionの既存importおよび通常CRUDは`main`へ固定し、通常CRUDには別途Production用認証設定と実Plan/Apply検証を要求する

既存AWS runtime → GCP WIFとはTerraform CI trustを分離します。

## Production boundary

production:

- 最初のimportは同resource typeをdevで確立してから
- 本人初回workflow_dispatch限定。実行前の準備判定は[Production runbook](docs/production-readiness.md)に従う
- import直後の想定外diffをapplyしない
- destructive planは停止


## 関連する運用課題

State復旧と継続監査は、通常のPlan/Applyとは別に運用します。
実装・有効化・費用判断の状況は対応するIssueで追跡し、設計案を有効化済みと扱いません。

- [Recovery運用 #1165](https://github.com/kani3camp/youtube-study-space/issues/1165) / [State recovery runbook](docs/state-account-recovery.md)
- [Durable audit #1166](https://github.com/kani3camp/youtube-study-space/issues/1166) / [設定案](docs/state-durable-audit.md)
- [Privacy cleanup #983](https://github.com/kani3camp/youtube-study-space/issues/983) / [Privacy runbooks](../../docs/privacy/)

## 実行記録と変更履歴

設定readback・実施日時・対象SHA・CI/レビュー・Plan/Apply結果・未検証事項は、対応PRとActionsへ記録します。
本書には現在の運用手順と制約を置き、一時点の実測値や進捗を複製しません。

- [DEV通常運用の導入・実設定・検証（PR #1382）](https://github.com/kani3camp/youtube-study-space/pull/1382)
- [独自証跡の撤廃と認証境界の変更（PR #1373）](https://github.com/kani3camp/youtube-study-space/pull/1373)
- [既存State移行と採用結果（Issue #1191）](https://github.com/kani3camp/youtube-study-space/issues/1191#issuecomment-6233630058)
- [旧経路の整理方針（Issue #1191）](https://github.com/kani3camp/youtube-study-space/issues/1191#issuecomment-6244637630)
- [移行当時のコード・runbook（固定コミット）](https://github.com/kani3camp/youtube-study-space/tree/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp)

過去の手順を再実行する場合は、現在の実装・認証済み実環境・復旧契約を確認します。
