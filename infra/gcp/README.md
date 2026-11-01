# GCP Terraform

YouTube Study SpaceのDevelopment／Productionで採用済みのGCP resourceをTerraform管理するためのrootと運用入口です。実行結果や導入時の判断は[変更履歴](#実行記録と変更履歴)を参照してください。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## Firebase Authenticationのメール認証

モニター用のメール・パスワード認証は、[認証用モジュール](modules/monitor-auth/README.md)で管理します。`beforeCreate`による新規登録制限は廃止し、公開登録と既存アカウントへのパスワード追加を許可します。モニターデータの閲覧はFirestoreの許可リストで制限します。DEVに適用済みの旧登録制限は、同手順の段階撤去を完了してから最終構成を使います。

## 通常のGCP Infrastructure運用

手動操作の入口は `Terraform`（[`terraform.yml`](../../.github/workflows/terraform.yml)）の `workflow_dispatch` です。利用者が選ぶのは次の3項目です。

- `environment`: `development` または `production`
- `operation`: `plan` または `apply`
- `component`: `all`（既定）・`core`・`mypage`

DEVのCoreだけは、任意で `source_branch`（対象ブランチ）・`source_sha`（対象SHA、40桁）を指定し、実行元とは別のブランチ／コミットのHCLをPlan/Applyできます（下記）。両方空欄なら実行元自身のコミットです。

Core GCPとMyPageは別々のTerraform Stateのまま、同じ `Terraform` 実行で扱えます。`plan` は選んだStateのPlanを完了させ、`apply` は選んだPlanがすべて成功した後にCore→MyPageの順でApplyへ進みます。Applyの各jobは自分で再Planし、Plan jobとHMACが一致した自分のplan fileだけを適用します。Stateを物理的に統合してblast radiusを広げず、手動UIだけを統一します。同じrunの最初に対象SHAを1つ確定し（[`terraform_source_pin.py`](../../.github/scripts/terraform_source_pin.py)）、そのコミットの資格情報なしのTerraform検査（`ci-gcp-terraform.yml`）を通してから、同じSHAでPlan/Applyします。検査中に対象ブランチが更新されても、別のコミットへ切り替わりません。Production Applyは `terraform-prod-apply` Environmentの承認を必須とします（required reviewersはownerがGitHubで設定する）。

MyPageの通常経路は最終desired state（現在は `runtime_public` 相当）を常に対象とし、`foundation_metadata` / `foundation` / `registry` / `maintenance_*` / `runtime_private` を利用者が選択しません。これらのstageは初回bootstrapと既存の安全契約を維持する内部実装であり、通常workflowの操作面には露出させません。image build / releaseとmaintenance ON/OFFはTerraform Applyとは別操作です。

### PR Plan（助言用、Applyしない）

ownerが作成した同一repositoryのPRで `infra/gcp/**` などが変わると、`Terraform` の `pull_request` 経路がDEVのCoreとMyPageを **PR Plan** します。PRODのPlan Environmentは `main` 限定のため、PRODは手動Planだけです。

- 手動Planと同じreusable（`gcp-terraform-authenticated.yml` の `pr-plan` mode）、Environment `terraform-dev-plan`、identity（`terraform-dev-normal-plan` SA、`terraform-normal` WIF、`youtube-study-space-terraform-dev-plan` AWS role）を使う。PR専用のidentity・Environmentはない。
- `-lock=false` で実行し、HMAC・saved plan・job outputを残さない。結果はJob Summaryの値なし要約だけで、PRコメントは投稿しない。Applyは必ず手動 `Terraform` で再Planする。
- fork由来PR、owner以外のactor、再実行は資格情報を取得する前に**失敗として**停止する（green skipにしない）。その場合はownerが手動 `Terraform` の `operation=plan` を実行する。
- GCP WIFとAWS trustは、PRをPlan Environmentだけに許可する（PRのref `refs/pull/<n>/merge`、`terraform.yml` と同refのreusable、owner、初回実行）。Apply Environment・Apply identityは手動dispatchだけ。DEVのtrustとEnvironmentは2026-10-31にこの形へ更新済みで、旧PR Plan専用のidentity・Environmentは撤去済み。

認証付きDEV操作の資格情報の境界は**クラウドtrustの本人検査**です。GCP WIF / AWS OIDCはowner immutable ID・repository・初回実行・同refの正規workflowを検証し、DEVはownerの任意の非`main` branchの手動dispatchと、ownerのPR Planを許可します。PR/fork、tag、`main`でのDEV通常操作、再実行は認証前に拒否します。Productionの通常操作は保護された`main`だけです。

DEVのEnvironment（`terraform-dev-plan` / `terraform-dev-apply`）はbranchで制限しません（No restriction）。DEV EnvironmentのSecretはrole ID・SA名・bucket・HMAC key・tfvarsなど資格情報ではない設定値だけで、write権限を持つ協力者のjobが読めても、クラウドの資格情報はactor検査で取得できません。forkのPRにはSecretもOIDCも渡りません。

対象ソースの選択（実行元とは別のブランチ／SHAのHCLをPlan/Applyする機能）は、DEVのCore（`component=core`）だけで使えます。MyPageと本番は常に実行元のコミットを使い、対象ブランチ・SHAを指定すると選択の段階で拒否します。

| source_branch | source_sha | 使用するソース |
| --- | --- | --- |
| 指定 | 空欄 | 選択時点のブランチHEADをSHAへ解決し固定 |
| 空欄 | 指定 | 同一repositoryの通常ブランチ履歴から到達できる指定コミット |
| 空欄 | 空欄 | 実行元自身のコミット |
| 指定 | 指定 | ブランチHEADとSHAが一致する場合だけ実行 |

確定した対象SHAはActionsのSummaryに表示します。Applyで確認済みの同じコミットを使う場合は、`source_branch`を空欄にし、`source_sha`にPlanのSummaryの値を指定します（ブランチだけを指定すると、新しい実行の時点でHEADを再取得します）。資格情報なしの検査（`ci-gcp-terraform.yml`）は対象コミットのソースとテストをcheckoutし、実行元からビルドした隔離コンテナ（[`validate.sh`](../../.github/ci-validation/validate.sh)）内で実行します。対象コードにはrunnerの環境変数・Actionsのキャッシュ用トークン・コマンドファイル・Dockerソケットを渡さず、ソースは読み取り専用でマウントします。検査を指示するworkflow、対象SHAを確定する処理、資格情報付きのPlan／Apply実行器は実行元のコミットを使います。対象ブランチの`.github/`・`infra/gcp/scripts/`・`infra/gcp/tests/`を実行器として使いません。HCLやmodule自体もprovider等による実行を伴うため、選択するソースは本人がレビューしたものを使います。

1. 通常のHCL追加・修正は任意の作業/統合ブランチ向けPRで進める。ownerのPRにはDEVのPR Planが付く。Environmentへのbranch登録は不要。`dev`は運用可能な統合結果だけを最終PRで受け入れる。
2. 新しい非秘密入力は `normal.tfvars.json` に追加する。既存Environmentのprivate tfvarsを読み、
   このcommitのファイルで上書きするため、既存リソースの選択・private inventoryを落とさない。
   秘密値はコミットしない。新しいprivate入力は両DEV Environmentのmanaged入力を同じ内容へ更新する。
3. delete/replaceが必要なら、同じPRの `normal-operations.json` に正確なaddressとactionsを記載する。
   create/updateにはリソース別manifestやtype登録を要求しない。
4. 作業PRのcredentialless CI・PR Plan・レビューを確認する。専用統合ブランチを使う作業ではCodexがそこへマージしてよい。
5. 対象ブランチのHEADから `Terraform` を `environment=development` / `operation=plan` で実行し、Core GCPとMyPage両方のPlan結果を確認する（MyPageのtrustは統合branchのrefに固定）。
6. 適用する場合は同一ブランチ・HEADで `operation=apply` を新規dispatchする。各Stateは独立したPlan/Apply identityとsaved plan検証を維持し、両Plan成功後にApplyへ進む。
7. `GCP_NORMAL: PASS; role=apply` と事後full-root no-opを確認する。
   別jobの再Plan HMAC比較はGitHub Actionsの `if` 条件で行い、**HMACをstep envやcommandへ渡さない**。一致しなければApplyを実行せず、セクション別比較の真偽値だけで `GCP_NORMAL_REPLAN: STOP` を表示する。値・address・HMACは公開しない。
8. DEV実地検証と復旧確認を完了してから、専用統合ブランチ→`dev`の最終PR（1本）をレビュー用に残す。`dev`へのマージは本人だけが行う。
   破壊的変更のレビュー指定は次のPRで削除する。未使用の指定も拒否される。

破壊的変更の指定例です。Terraform Planのactionsと順序まで一致させ、未使用の指定も残しません。

```json
{
  "destructive_changes": {
    "google_pubsub_topic.retiring": ["delete"],
    "google_pubsub_topic.replaced": ["delete", "create"]
  }
}
```

取得するソースは同一repositoryの固定SHAです。別checkoutから`environments/dev/`と`modules/`のHCL・入力・配備データだけを私用作業ディレクトリへコピーし、対象ブランチの`.github/`・`infra/gcp/scripts/`・`infra/gcp/tests/`はコピーしません。symlink、改変済みcheckout、未追跡の入力でcommitを上書きする操作は拒否します。登録制限用の配備ZIP生成・関数配備は撤去しました。

HCLやmodule自体もprovider等による実行を伴うため、選択するソースは本人がレビューしたものを使います。この分離は、未レビューのHCLを安全に実行するsandboxではありません。Environment/OIDC、State、Planの契約は[実行契約](docs/terraform-authentication-boundary.md#devの通常planapply)を参照してください。

S3 native locking、Versioning、環境別concurrencyを継続し、workspace・target apply・force-unlockは使いません。raw Plan/State/Secret/API応答をログ・artifactへ公開せず、公開結果は固定phase・操作件数とprovider schemaで検証したリソース種別/操作別件数に限定します。

**有効化の順序:** このcontrol plane分離は、対応workflowと実行器を含む信頼済み実行元から利用できます。新しい実行元を許可する前に、そのブランチのworkflowと実行器をレビューしてください。HCLを試すだけのfeature branchを実行元へ一時昇格させません。DEV StateへPR #1467の旧登録制限資源を適用した状態では、認証用モジュールの段階撤去を完了するまで、その段階に合う固定SHAを選んでください。古いHCLによる意図しない資源削除は適用しません。

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

### DEVのEnvironment設定と本人起動

DEVの両Environmentは`deployment_branch_policy`を`null`（No restriction）にします。本人・正規workflowパス・同ref/SHA・非mainなどの共通検証はクラウドtrustとpreflightで維持します。

通常のPlan/ApplyはActionsから実行するため、ローカルのAWS/GCP loginやTerraformは不要です。設定の確認・復旧は次の管理用コマンドを使用できます。

```sh
python3 infra/gcp/scripts/enable_normal_integration.py
# 設定反映時だけ実行（GitHub本人認証が必要。AWS/GCP loginは不要）:
python3 infra/gcp/scripts/enable_normal_integration.py --apply
```

プレビューは読み取りだけです。`--apply`はGitHub本人認証を確認して、DEVの2 Environmentのbranch制限を外し（No restriction）、読み戻します。GitHub Settings → Environmentsで両Environmentの「Deployment branches and tags」を「No restriction」にしても同じです。既存Environmentがない場合や、想定外のreviewer・wait timerなどの保護がある場合は、作成・上書きせず停止します。

既存のAWS 7-claim subjectとGCP WIFは、本人のimmutable actor ID、repository、Environment、正規caller/reusable等を検証します。DEVのcloud trustのcallerは`terraform.yml`だけです（旧`ci.yml`のclaimは2026-11-01に削除済み）。管理スクリプトはこれを読み取り検証し、書き換えません。対象source branchをOIDCのworkflow refへ入れません。

`--verify-aws` / `--verify-gcp`は読み取り監査です。`--apply`もAWS IAM・GCP WIF・Terraform State・業務資源・本番・Environment secrets/variablesを更新しません。失敗時は固定のphaseだけを表示し、資格情報やAPIエラー本文を出力しません。

### 基盤変更の検証と復旧

認証・実行基盤の変更は、offlineの契約テストとnative fixtureに加え、認証付きの通常Planで検証します。
実変更の検証が必要なら、レビュー済みのDEV用HCLを通常PRで追加し、通常Plan/Applyと事後no-opを確認します。
削除・置換も`normal-operations.json`の明示レビューを通し、暗黙に一時資源を生成するモードは使いません。

旧`normal-e2e`は撤去しました。過去のrunのprobeが残っている場合は、private端末でState内の
`google_pubsub_topic.normal_ops_probe`を確認し、同じtopic名・projectのHCLをDEV rootへ宣言して通常Planをno-opに戻します。
その後、HCL削除と明示的delete reviewを通常PRで適用し、事後no-opを確認します。
自動force-unlockやStateの巻戻しは行いません。State障害は[State account recovery](docs/state-account-recovery.md)を参照します。

## 通常のPROD Terraform開発（main固定）

Productionの通常Plan/Applyは既存DEVと同じ `normal-plan` / `normal-apply` を使います。**実行refは `main` のみ**。通常実行にはPROD専用IAM/WIF・既存State・全plan/StateのHMAC・saved plan・事後full-root no-opを要求します。Plan属性内の入れ子になったproject-bearing ID、principalSet、SAメールもPRODへ限定し、数値project別名は認証付きResource Manager readbackから取得します。

1. `infra/gcp/environments/prod/` のHCLをレビュー対象とし、先にDEVで相当する変更を検証します。既存PRODリソースの宣言とprivate inventoryを落としてはなりません。DEVのRoot/StateをPRODへコピーしません。
2. 非秘密の差分だけを `normal.tfvars.json` に記載します。既存30資源の選択情報・private metadataは `terraform-prod-plan` / `terraform-prod-apply` Environmentのmanaged入力を引き続き使います。両者の入力内容はPlan/Applyで論理的に一致させます。
3. delete/replaceはPRODの `normal-operations.json` の `destructive_changes` に正確なresource addressとnative actionsの順序を明示します。create/updateに型別allowlistはありませんが、GCP IAMの明示権限外の操作は拒否されます。
4. 保護された `main` にレビュー済みリリースを取り込んだ後、本人が初回 `workflow_dispatch` で `Terraform` を `environment=production`、`operation=plan` で実行します。実行SHAは選択refの `github.sha` をそのまま固定し、手入力させません。公開ログではresource type/操作/件数だけを確認します。
5. Applyが必要な場合は同一main HEADから `operation=apply`、`environment=production` を新たに実行します。`terraform-prod-apply` のrequired reviewersが承認するまでApply jobは開始しません。同runの独立Plan/Apply jobのHMAC/State/SHA/期限が一致したsaved planだけを適用します。事後のfull-root no-opとlock解放を確認します。

### 一度限りの本番設定と有効化条件

**コードをdevへマージするだけではPRODの認証付きPlan/Applyは有効になりません。** PRODのcallerは `terraform.yml` です。callerを切り替えるリリースでは、PRODのPlan/Applyが使えない時間を作らないよう、次の順で行います。(1) `main` へ取り込む前に、PRODのGCP WIF conditionとAWS trust policy（Plan/Apply role）を、`ci.yml` と `terraform.yml` の両方を許可する値にする。(2) `main` へ取り込む。(3) `main` から `Terraform`（`environment=production`、`operation=plan`）を実行し、認証とPlanを確認する。(4) callerを `terraform.yml` だけへ限定する。GCPは [`bootstrap_production_normal_ops.py`](scripts/bootstrap_production_normal_ops.py) のconditionと一致させる。AWSは既存のtrust policyの条件（`aud`・本人の `actor_id=54093651` など）を保持したまま、`sub` のcallerだけを変える。[`production_backend_packet.py`](scripts/production_backend_packet.py) が生成するtrust policyには `actor_id` が含まれないため、そのまま適用しない。`terraform.yml` を許可しないPROD trustでは、PRODの認証は拒否されます。 既存PROD移行用AWS S3 backend Roleを再利用し、通常専用のGCP Plan/Apply SA・カスタムロール・WIF pool/provider・runtime SAを追加します。移行用GCP WIF/SA、S3 State、既存workloadは変更しません。

- PROD用の通常Apply権限はDEVのカタログを基礎としつつ、資源別IAMの付与による自己昇格を防ぐため `*.setIamPolicy` はデフォルトで除外します。Bucket/Secret/Cloud Run等の公開・他主体へのgrantは別途IAMレビューの対象です。
- PROD AWSの既存Plan/Apply Roleは、repository-wide GitHub OIDCの7 claims（`terraform-prod-plan/apply`、`ref=refs/heads/main`、固定caller/reusable）に加え、native `actor_id=54093651` が必要です。予期しないAllow、旧integration branch trust、wildcardが残ればSTOPします。AWSのtrust調整は管理者が差分を独立レビューしてから実施し、このbootstrapではAWSを変更しません。
- GitHub `terraform-prod-plan` / `terraform-prod-apply` のSelected branchesは**完全一致の `main` のみ**とします。既存managed入力・S3 role/bucket/account秘密参照を維持します。旧integration branchの認可は本番運用切替時に整理します。
- 管理端末にはCloud Asset Inventory APIとPolicy Analyzerの読み取り権限が必要です。bootstrapはProjectの最上位祖先を取得し、そのscopeで通常SA/runtime SAの資源別・継承・グループ経由grantを監査します。想定外のrole/resource、条件付きgrant、不完全な解析、既知grantの欠落は書込み前にSTOPします。通常SAへ監査用権限は追加しません。
- `--impersonation-inventory` でProject/folder/organizationからの資格発行・署名・鍵作成・actAs権限を列挙し、既存管理主体を確認した `review_sha256` を `--impersonation-review-sha256` に渡します。custom roleや条件付きgrantも対象とし、主体・条件・資格発行権限が変われば再レビューまでSTOPします。生の委任先一覧は管理端末で扱い、公開ログへ出しません。
- main固定はGitHub/AWS/WIFのCI実行入口の制約です。既存Owner・Google制御プレーン・既存の特権workloadによるGCP直接操作まで禁止するものではありません。bootstrapは既存階層IAMを剥奪せず、その委任を上記fingerprintへ明示的に束縛します。既存アプリ権限の最小化は別途の運用変更です。
- AWS・GCP・GitHubで必要な管理者権限を持つ短期認証済み端末で以下を実行します。

```bash
# Checkout外のowner-only (0600) JSON。値は実設定から確認し、公開しない。
# keys: state_account_id, state_bucket,
#       production_plan_role_name, production_apply_role_name
# 管理端末で既存の偽装経路を確認し、review_sha256を控える（生出力は非公開）:
python3 infra/gcp/scripts/bootstrap_production_normal_ops.py --impersonation-inventory
python3 infra/gcp/scripts/bootstrap_production_normal_ops.py --check --backend-config /private/prod-backend.json --impersonation-review-sha256 "$IMPERSONATION_REVIEW_SHA256"
python3 infra/gcp/scripts/bootstrap_production_normal_ops.py --apply --backend-config /private/prod-backend.json --impersonation-review-sha256 "$IMPERSONATION_REVIEW_SHA256"
# mainの初回 normal-plan が既存30資源・outputともno-opで成功した後のみ:
python3 infra/gcp/scripts/bootstrap_production_normal_ops.py --check --enable-apply --noop-run-id <run-id> --backend-config /private/prod-backend.json --impersonation-review-sha256 "$IMPERSONATION_REVIEW_SHA256"
python3 infra/gcp/scripts/bootstrap_production_normal_ops.py --apply --enable-apply --noop-run-id <run-id> --backend-config /private/prod-backend.json --impersonation-review-sha256 "$IMPERSONATION_REVIEW_SHA256"
```

`--check` はAWS Roleのtrustと全inline/attached権限、GitHub OIDC/Environment、GCPの現行権限カタログと既存通常identityをread-only検査します。backendの既存role名・bucket/accountはprivate設定から読み、新しい名前を推測しません。S3権限は既存 `production_backend_packet.py` のPROD state/lock/限定prefixへ完全一致させ、DEV keyや追加AWS権限を拒否します。

初回 `--apply` は**Plan用のcustom role/SA/WIF/Secret参照だけ**を設定します。Apply用SA・project grant・impersonation・runtime SAは作成せず、既存の早期Apply grantがあればSTOPします。既存移行用の読み取りロール3件からread権限を引き継ぎ、Firestore backup等の既存30資源を欠落させません。AWS Role/State/既存業務リソース・migration identityは変更しません。

`--enable-apply --noop-run-id` は別の明示操作です。GitHub APIで本人初回dispatch・成功したCI・現在のmain SHA・通常PROD Plan jobと専用no-op markerを照合し、それからApply用identity/Secret参照を追加します。migration実績・古いmain・再実行・失敗run・30資源でないPlanは有効化証跡になりません。readbackに不一致があればSTOPし、生APIエラー・秘密値は表示しません。

- `main` への認証付きworkflowのリリース後、まずPRODの既存30資源を対象とする認証付きfull-root no-op Planを実測してください。その後、必要最小限の変更についてPlan→承認済みApply→事後no-opを確認します。PRODで一時Pub/Sub CRUDテストを実施しません。

**確認範囲：** GitHubコードとcredentialless CIでは本番の実IAM/State readback、実mainでの初回認証付きPlan/Applyの成功を証明できません。これらはSourceの実装と独立した有効化ゲートです。

## 完了済み移行経路の撤去

既存資源の取り込みはDEV・PRODとも完了しています。[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)に実行記録を残し、
`migration-plan` / `migration-apply`、空State初期化controllerとlive `normal-e2e`を撤去しました。
固定manifestのimport経路はState復旧には使えず、通常資源追加後のStateにも対応しません。
これらを復旧用の常設コードとして維持せず、必要な過去実装はGit履歴から確認します。
再導入が必要になった場合も、現行State・HCL・IAMとの適合を新しいPRで審査します。

通常Plan/Applyは既存の管理対象・private tfvarsを維持します。既存Secretのwire schema名と4field形式は互換性のため保持し、
入力・JSON/圧縮処理、State読取とprovider schema検証は`terraform_runtime.py`を使います。
新しいimportやStateの初期化が必要な場合は、通常Applyへ流さず、目的と権限を明示して別途設計します。

## ディレクトリと現行の責務

| 場所 | 用途 |
| --- | --- |
| `environments/dev/`、`environments/prod/` | 環境別の独立root・State key・provider lock。workspaceで環境を切り替えない |
| `modules/` | 管理対象の共通Terraform modulesと既存リソース契約 |
| `scripts/`、`tests/` | 現行の設定検証、offline準備、negative / native fixtureテスト |
| `docs/` | 復旧・運用・準備判定の既存runbook。現行の判断入口は以下のリンク |
| `scripts/terraform_normal_ops.py`、`terraform_runtime.py` | 通常Plan/Applyとprivate入力・State・plan shapeの共通検査 |
| `.github/workflows/ci.yml`、`terraform.yml`、`gcp-terraform-authenticated.yml` | credentialless PR CIと、保護された本人起動の運用経路（入口は`terraform.yml`） |

## Version policy

- Terraform CLI: `.terraform-version` と各rootの `required_version` を一致させる
- Google provider: 各rootの `required_providers` と `.terraform.lock.hcl` を正本にする
- provider lockはTerraform operator / CIで使用する `linux_amd64` / `darwin_arm64` のchecksumを事前生成する
- version更新は通常のdependency変更としてPRでreviewする
- Notionにはpatch versionを複製しない

## ローカル検証

通常の検証はcredentiallessです。

```bash
python3 -B infra/gcp/tests/test_terraform_runtime.py
python3 .github/scripts/test_terraform_authenticated_workflow.py
python3 -B infra/gcp/tests/test_terraform_normal_ops.py
python3 -B infra/gcp/tests/test_terraform_normal_ops_native.py
terraform fmt -check -recursive infra/gcp
for env in dev prod; do
  (cd "infra/gcp/environments/$env" && terraform init -backend=false -lockfile=readonly && terraform validate)
done
```

PRでは **GCP Terraform Validate** が必要な安全契約・Terraform root/module・nativeテストを検証します。
S3 State/Versioning/locking、OIDC/WIF・分離identity、secret非漏出・通常Plan/Applyの拒否契約と対応テストを維持します。
復旧・監査は[State account recovery](docs/state-account-recovery.md)、[durable audit](docs/state-durable-audit.md)、
[認証・実行境界](docs/terraform-authentication-boundary.md)を参照してください。

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
- DEV通常操作は専用WIF/SAとPlan/Apply別Environment・least privilegeを使う。DEV Environmentはbranchで制限せず、ownerの任意の非`main`ブランチとownerのPR Planをクラウドtrustのactor検査で許可する
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
