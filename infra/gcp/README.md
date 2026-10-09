# GCP Terraform

development履歴tableのまとまった判断packet・本人の先行準備・本番への依存は
[`docs/development-history-ready-packet.md`](docs/development-history-ready-packet.md)を参照してください。

YouTube Study Space の既存GCP resourceを、安全に段階移行するためのTerraform rootです。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 現在のscope

Phase 1のscaffold / S3 state / protected CIと、developmentの主要11 resourceのownership移行は完了しています。2026-10-06時点の通常full-rootは `import0 / no-op11 / drift0 / unknown0` です。後続はIssue #1191を入口に、development残件を片付けてからproduction migration準備へ進みます。

dedicated development aggregate auditの実行記録・revoke手順と、development→production→MyPage releaseの証拠条件は
[`docs/phase2-approval-and-readiness.md`](docs/phase2-approval-and-readiness.md) を参照してください。
source準備や過去のno-op記録を、fresh実環境検証・実行承認・Infrastructure Readyと扱いません。
監査時点でlegacy値のbackfillは不要でしたが、legacy列は残っています。次のschema修復と
別waveでのcanonical import準備は[`docs/user-activity-history-canonicalization.md`](docs/user-activity-history-canonicalization.md)
を参照してください。adoptionはdefault-offのままで、live DROP/import/applyは未承認です。

通常のmigration pathでは以下を行いません。bounded normal-change / state-only例外は、別validator・別approvalで明示的に隔離します。

- 未承認のproduction resource import / apply
- 想定外のworkload create / update / delete / replacement
- manual Scheduler / Pub/Sub / Firestore export trigger
- importと同時のAPI enable / disable
- broad IAM role追加
- Service Account JSON keyの作成
- MyPage resourceのprovisioning

Firestore export FunctionのNode.js 22自然実行E2Eはdevelopment / productionともPASSし、#1173はcompletedでclose済みです。developmentのPub/Sub topic / Cloud Scheduler / Gen1 Functionもそれぞれprotected import-only waveでownership移行済みで、post-planはno-op11 / drift0です。generated subscription / build artifact / source deploymentはownership外を維持します。

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
- `DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED=false`（historyのplan-only waveとclosureで維持。再有効化は未実施security検証も含めた別review / approvalが必要）
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
- `mode=plan`のidentity smokeは`plan-read-only`を使用し、同一SHA OIDC claim、plan identity、exact dev state read、dev project/Function metadata GETとdevの必要GET・mutation permission不在だけを確認する。state本体PUT、apply role/SA credential発行試験、prod / 他productのread/listとprod permission検査は実行せず、Summaryに`SKIPPED`と`Full security gate: NOT VERIFIED`を記録する。Terraform native dev `.tflock`の通常PUT/GET/DELETEはplanに必要な一時操作として別に扱う
- 非plan承認経路の既存security smokeはstateへの条件付きPut拒否、prod / 他productのread/list拒否、wrong EnvironmentのSTS拒否、prod permission不在と別SA impersonation拒否を保持する。unexpected grant / network error / object不在をDENY成功と混同しない。条件付きPutはcurrent stateが存在する間は上書きしないが、GET後の削除と誤許可が重なると空versionを作り得るため、read-only planでは呼ばない。plan成功だけで未実施のsecurity検証やapply有効化を承認しない
- raw init / plan / apply出力はpublic logへ流さない
- saved planはrunner一時領域だけで扱い、artifact / cacheへ保存しない
- public outputは `.github/scripts/terraform_plan_summary.py` が生成するresource address / action count中心のsanitized summaryだけ
- import移行期はcreate / update / delete / replacement / driftをstopする
- plan jobとapply jobでsaved planを渡さず、apply jobは同じ `github.sha` から再planし、sanitized projectionが一致した場合だけ同一job内のplanをapplyする

historyの次applyについて、現行`mode=apply`はapply gate=falseでpreflight停止するため、
この入口を使ってgate変更前のfull negative smokeを実施することはできません。
既存full smokeには正本state keyへのconditional S3 PutObject否定試験があり、誤許可と同時削除の
組合せで空versionを書き得ます。安全なprobe-only経路または具体的リスクを含む別live承認を
先にreviewし、plan-only成功をfull security gate PASSへ読み替えません。

developmentのGitHub Environment / branch trust、AWS GitHub OIDC backend role、GCP GitHub WIF / Terraform Service Accountは#1162で構築・実測済みです。通常PRはcredentiallessのまま、authenticated executionはtrusted integration refと独立Environment approvalへ限定します。production側のbackend / trust / identityは未開始で、#1191の別approval境界です。

production backendは未bootstrapのため、production authenticated plan / applyはbackend準備完了まで有効化しません。

### Single development history plan receipt

history=true / mode=plan の検証は、apply=false のまま既存 protected identity と8個の既存 secretを使用する。追加の grant・credential・billing API・query は使わない。`terraform_plan_cost_evidence` は秘密情報を含まない手動 dispatch inputで、欠落時は認証・remote init・native lock の前に停止する。2026-10-09の承認に従い、上限USD0.25はこのrunが追加する当月UTCの費用を対象にする。lifecycle設定や永久削除期限を要求せず、残るlock version/delete markerの保管費は後月の累積費用に継続計上する。Environment承認者は実設定のread証拠に対応する入力をreviewする。既存operator/toolが取得した証拠を使え、所有者本人だけの新しい確認gateを設けない。booleanや金額だけを、未知の課金項目が解決した証拠として扱わない。

JSON input は次のキーだけを許可する。実state・table metadata・bucket/account/role/SA名・privateな証拠本文を入力に貼らない。

| Key | Required evidence |
| --- | --- |
| `model` | `dev-history-plan-monthly-2026-10-09-v1`。旧USD0.01モデルの入力は拒否する。この価格モデルは2026-10-15 UTCに失効する |
| `git_sha` | review・公開された実行対象の完全なSHA |
| `issued_utc`, `expires_utc` | `YYYY-MM-DDTHH:MM:SSZ`。発行済み・期限内、期限は発行後24時間以内かつモデル失効前。15分のplan jobと取消・post readの余裕を確保し、期限はUTC月末の30分前以前 |
| `max_state_bytes` | 非秘密の保守的サイズ上限（正整数、4 MiB以下）。実サイズはCIのexact current object HEADでprivateに確認する |
| `budget_month` | `YYYY-MM`。発行日時・実行時の現在UTC月と一致。旧lifetimeモデル・`lock_retention_days`は受け付けない |
| `cloud_side_cost_usd` | 小数の文字列。下記AWS費用に加算する、このrunの当月追加付随費用の正の上限。実設定のread証拠と保守的な数量・単価に基づく |
| `cloud_side_evidence_reviewed` | 全GCP/provider retry・OIDC/STS・CloudTrail/Cloud Logging・既存sink/replication等の追加課金を含むprivateな数量・単価・保持条件が確認できた場合だけtrue |
| `rates_verified` | 実行時の公開単価が下記ceiling以下であることを確認した場合だけtrue |
| `state_writers_quiescent` | 指定オペレーターだけが実行し、state/workspace prefixの並行writerがないことを確認した場合だけtrue |

費用計算はDecimalで各成分を上方丸めし、当月追加合計USD0.25以下だけを許可する。無料枠・GitHub runnerの所在は仮定しない。256件のAWSリクエストを一律USD0.00001/件、128件の対称KMS処理をUSD0.00001/件、64回分のstate downloadと各リクエスト16 KiB分の応答をUSD0.25/GiB、さらに`cloud_side_cost_usd`を単発・当月費用として予約する。追加するlock bodyとdelete markerを各32 KiBの保守的上限で見積り、USD0.10/GiB/月で丸一月分を予約する。月末のrunでも日割りによる値引きをしない。実サイズは後続HEADでprivateに確認し、上限・STANDARD class・既知の暗号化・有効なcurrent VersionIdを証明できなければstate downloadやremote initへ進まない。UIの丸められたサイズをexact bytesと見なさない。未知の付随費用から確認済みflagを生成せず停止する。

`history-plan-cost.json`には、当月、profileのdigestであるentry ID、単発・当月side費用上限、当月保管費上限、当月合計、追加保管bytes上限、現行単価による翌月以降一月分の保管費推定、削除期限なしを明記したprivateな`monthly_ledger_entry`を含む。すべて保守的推定で、実測費用や永久費用上限ではない。通常unlockのpositive absenceはcurrent lockが無い証拠であり、過去version/delete markerの永久削除や将来保管費0の証拠ではない。[S3 delete markers](https://docs.aws.amazon.com/AmazonS3/latest/userguide/DeleteMarker.html)も保管費を生じる。

CIのRUNNER_TEMPはcleanupされ、永続台帳にはならない。実行operator/toolは同じreview済みsourceとinputで認証不要の`--phase policy`をローカル実行し、privateなcost receiptの予約行を実行前に月次台帳へ保存する。実run ID/attemptを予約entry IDと結び付け、receipt失敗・取消・lock absenceによって予約を勝手に取り消さない。後月は保管bytesを繰り越し、単価を再確認して既存支出・新しい単発費用と累積する。既存state/lock versionsは別のbaselineであり、このrunの追加上限に含めたと見なさない。旧versionの消失や減額は既存権限のread証拠がある場合だけ反映し、削除操作は行わない。監査/logging等の後月費用も0と仮定せず実設定・月次証拠で照合する。台帳にraw state/metadata/private descriptionを保存・公開しない。

価格根拠は[AWS S3 pricing](https://aws.amazon.com/s3/pricing/)、[KMS pricing](https://aws.amazon.com/kms/pricing/)、[CloudTrail pricing](https://aws.amazon.com/cloudtrail/pricing/)、[Cloud Logging pricing](https://cloud.google.com/products/observability/pricing)。許可されたAPI envelopeとprice ceilingを広げる場合は再reviewが必要。Terraform1.16.4の[backend](https://github.com/hashicorp/terraform/blob/v1.16.4/internal/backend/remote-state/s3/client.go)と固定依存[aws-sdk-go-base beta.72](https://github.com/hashicorp/aws-sdk-go-base/blob/v2.0.0-beta.72/aws_config.go)、[S3 downloader1.17.22](https://github.com/aws/aws-sdk-go-v2/blob/feature/s3/manager/v1.17.22/feature/s3/manager/download.go)を根拠に、小さい単一part state・空のworkspace discovery prefix・native lock一回分に余裕を含む。backendの`max_retries=1`はこの固定AWS v2依存で`WithRetryMaxAttempts(1)`となる。CLIは[AWS_MAX_ATTEMPTS=1](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-retries.html)、認証Actionも1 attempt、planのlock timeoutは0s。native downloader自身のbody attemptを費用に含め、run/plan再実行やforce-unlockは行わない。

既存identity smokeのinitial state GETをHEAD/GET/HEAD付きのprivate snapshotへ置き換える。exact既存11 managed instance、history未登録、正常なTerraform4 state envelope・pass状態の既知check結果、current VersionIdを確認する。provider定義のattributes/identity/privateは値を公開せず、全state bytesを前後完全一致で保持する。新しいstate envelope属性・taint/deposed・unknown checkは拒否する。既存のcanonical8 metadata helperとprivate varfileはそのまま使う。

init/plan/sanitizer後は`always()`で再snapshot・完全なtables.get比較・exact native `.tflock` prefixのpositive LIST absence確認を行う。通常のnative unlock以外の削除は行わない。403/通信失敗/欠落/不正/truncated responseは不在の証拠にしない。失敗または期限切れのjobでも可能なbounded safety readを行うが、成功receiptには全step成功・strict import1/既存11 no-op/他action0を要求する。import0はこの検証経路を通過できない。

public Summaryは固定のPASS/STOPラベルとsanitized action数、当月のみの追加費用枠・後月の保管費継続・削除期限を仮定しない旨の固定文だけ。raw state・VersionId/serial/lineage・完全metadata・費用計算明細はowned0600のprivateファイルで扱い、CIのRUNNER_TEMPは常時cleanupする。artifact/cacheへの保存は禁止する。runner強制停止・権限不足等で完全receiptが得られなければclosureしない。全条件が成立した場合だけ、review済みclosureでhistory=false/apply=falseへ閉じる。

### Existing development backend

Issue #1154で作成したdevelopment GCS state backendは、empty state / inactive lockであることを再確認した後、2026-10-05の承認済みcleanupで退役しました。

- 旧operatorのGCS backend設定 / `.terraform` cacheを無効化
- empty stateと旧lock generationをprecondition付きで削除
- bucketを条件付き削除
- 7日soft deleteを維持
- recovery authorityはVersioning済みAWS S3 stateへ一本化
- 旧GCS backendへ再接続しない
- production GCS state bucketは作成しない

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
- development authenticated plan / applyはtrusted ref / GitHub Environment / least privilegeで実装・実測済み
- production authenticated plan / applyはbackend / trust準備前のためfail-closedを維持する

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
- Issue #1162: public repository向けauthenticated Terraform plan / apply（completed）
- Issue #1191: Phase 2 development残件 / production migration準備

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

Development quota refresh completed with regular no-op8/drift0. PR #1184/#1186 and protected run37342000553 adopted the existing topic; post-plan was import0/no-op9/drift0. Scheduler and Gen1 Function were then adopted in separate protected import-only waves, reaching regular full-root import0/no-op11/drift0. Generated resources, unrelated IAM and source rebuild/upload remain excluded.

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

`environments/dev/export-function.tf` defines the adopted Gen1 Function and remains absent from production. Fresh inventory and protected import confirmed Node22 / ACTIVE / version8, unchanged deployment/source/lock provenance, and the external reserved label/source boundary. The only added Function read permission is `cloudfunctions.functions.get`; source build/upload/redeploy and Google-managed ownership remain outside Terraform. See the [Function module contract](modules/firestore-export-function/README.md).

### Approved development Function activation — 2026-10-06

[#1162 comment6005988785](https://github.com/kani3camp/youtube-study-space/issues/1162#issuecomment-6005988785) approves only `cloudfunctions.functions.get` in the existing development CI read role. Principals/bindings remain unchanged; no list/mutation/call/invoke/sourceCode/IAM/API/broad-role/production additions. The workflow enables the previously reviewed default-off Function definition only for development, after adopted topic and Scheduler dependencies. A harmless exact Function metadata GET supplies the preserved execution identity privately to both jobs.

The unchanged global import-only policy plus `terraform_export_function_gate.py` require Function import1 + existing10 no-op, drift/unknown/unexpected action0, then post import0 / no-op11 / drift0. Independent Environment approvals, same-SHA re-plan/projection equality and saved-plan apply remain mandatory. Runtime Node22/ACTIVE/version8, trigger/retry, environment, execution identity and limits are fixed. Source/redeployment and Google-managed generated resources stay outside ownership; no manual trigger is permitted. See the [Function contract](modules/firestore-export-function/README.md) for provenance and state-only rollback.
