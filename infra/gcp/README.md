# GCP Terraform

> 2026-10-18: #1251のdevマージ後、完了済みの一回限りの補助コード・旧手順書を退役。証跡は[固定コミット](https://github.com/kani3camp/youtube-study-space/tree/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp)と[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191#issuecomment-6244637630)に保持します。

development履歴tableのまとまった判断packet・本人の先行準備・本番への依存は
[`docs/development-history-ready-packet.md`](https://github.com/kani3camp/youtube-study-space/blob/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp/docs/development-history-ready-packet.md)を参照してください。

YouTube Study Space の既存GCP resourceを、安全に段階移行するためのTerraform rootです。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 現在のscope（2026-10-17）

現行運用はDev31件・Prod30件の固定baselineのfull-root監査planと、承認済みimport/no-op・State-only saved applyを扱う [`migration-plan` / `migration-apply`](docs/runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)です。
入口は既存 `ci.yml` の本人初回 `workflow_dispatch`、専用integration branchです。
第30波と両環境の最終通常Plan/Apply・State監査の実行SourceはH7（`0ed7486b`）です。SHAは実証に用いるSourceの記録であり、文書自体のHEADではありません。最新の運用Sourceと最終runは[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照してください。H4（`93121ab6`）の[#1364](https://github.com/kani3camp/youtube-study-space/pull/1364)でnative10 caseの上限2並列化とrouting/回帰を追加し、H5の[#1366](https://github.com/kani3camp/youtube-study-space/pull/1366)でnative runnerと回帰テストを修正しました。#1370の観測テスト修正をH7（`0ed7486b`）へrequired CI・独立review後に統合し、後続waveと最終no-opを実行しました。#1366は30 checks（12 success/18 skipped）・独立review後に統合済みで、runtime controller5ファイル・認証workflow・HCLはH3/H4と同一です。[CodeQL #1362](https://github.com/kani3camp/youtube-study-space/pull/1362)の実証はH3 pushの[run39001594887](https://github.com/kani3camp/youtube-study-space/actions/runs/39001594887)で3言語解析・SARIF受付成功として保持します。
全4 Terraform EnvironmentはRequired reviewersなし・admin bypass=falseで、plan/apply identityを分離します。
旧issuer、期限付きwindow、毎回のEnvironment人間承認は現行経路へ戻しません。`dev`へのmerge・直接push・auto-mergeは行わず、最終mergeはユーザー本人だけが行います。

| 環境 / 対象 | 実際に確認した範囲 | 証拠の境界・別判断 |
| --- | --- | --- |
| Development | DevExit [run38945893535](https://github.com/kani3camp/youtube-study-space/actions/runs/38945893535)後、H7の[最終run39029559218](https://github.com/kani3camp/youtube-study-space/actions/runs/39029559218)も両identityのimport0/no-op31/rawdrift0・全34 jobの検証完了（選択job成功・非対象skip）。独立full-root no-op31・State serial36/35とversion/lock監査PASS | 最終no-op後healthも全5 stage・finding0。quiet/null・日次処理未観測とoriginal history final receipt欠落は保持 |
| Production backend / CI foundation | 既存共通S3 bucketを再利用し、AWS backend roles2・inline policies2、GCP CI resources7・個別policy5へのmember追加8/削除0、Prod2 Environmentへの設定7種ずつを実readback・独立review済み | 新bucket・長期key・業務resource変更なし。bootstrap7資源はapp root所有30件の対象外として区別する |
| Production State / ownership | H2の[初期化run38961710157](https://github.com/kani3camp/youtube-study-space/actions/runs/38961710157)、H3の初版直接監査を経て、H7の[第30波run39029269335](https://github.com/kani3camp/youtube-study-space/actions/runs/39029269335)で30所有。続く[最終run39029874116](https://github.com/kani3camp/youtube-study-space/actions/runs/39029874116)は両identityのimport0/no-op30/rawdrift0・全34 jobの検証完了（選択job成功・非対象skip）。独立通常full-rootとState32/31のversion/lock監査、最終no-op後health全5 stage・finding0もPASS | 元R10/R11/R20の失敗とreconciliationは[既存runbook](docs/production-readiness.md#current-execution-boundary--2026-10-17)に保持。最終統合Sourceの同等性・authority/State closeout・正本同期とReady判定はIssue #1191 |

Dev31件は既存12件＋runtime Pool/Provider2件＋個別grant7件＋Own API10件、Prod30件は既存11件＋同runtime9件＋Own API10件です。
Production rootの既定選択は0です。Developmentの既存基礎graphと各opt-inはsource定義に従います。
実運用は両環境ともreview済みprivate baselineで既存所有をすべて選択し、候補を1件ずつ追加します。
未選択の既定値を既存Stateへ適用したり、既存所有をbaselineから落としたりしません。
Stateの初回作成はProductionの正規不在・manifest0だけに許す固定outputs2件の例外で、一般resource createは許可しません。

最新の実行結果・source/run・残件は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)、
[Production runbook](docs/production-readiness.md#current-execution-boundary--2026-10-17)、
[Infrastructure / MyPage ReadyGate](docs/phase2-approval-and-readiness.md#development--production--mypage-readygate)に集約します。
TerraformによるIaC拡張の準備と、MyPage固有resourceのprovisioning・Production releaseは別判定です。
新規resourceは専用normal-change PRでtype/schema、create/update専用の実行経路・検証契約、exact IAM/API、費用・rollbackを整えます。HCLや権限の追加だけでは現migration経路から作成できず、import-only契約は維持します。
現行CIのtrustは専用integration refに固定しています。最終devマージ後もこの運用refを必要な間保持し、ref移行・branch削除はworkflow、Environment、cloud trustを整合させる別reviewで判断します。

今回、過去raw-chat/混在backup削除、通知修復、CloudTrail継続課金、MyPage provisioningは行いません。
[recovery運用 #1165](https://github.com/kani3camp/youtube-study-space/issues/1165#issuecomment-6223368027)、
[durable audit #1166](https://github.com/kani3camp/youtube-study-space/issues/1166#issuecomment-6223368703)、
[Privacy cleanup #983](https://github.com/kani3camp/youtube-study-space/issues/983#issuecomment-6223369320)はすべてOPEN・Acceptance未達で、今回のexisting ownership移行のblockerにはしません。
MyPageのProduction release前の運用判断とPrivacyの独立laneへ残します。
CodeQLのintegration refには未解決finding6件（critical1/high3/medium2）があり、dismiss/suppressしません。解析・SARIF受付成功を脆弱性0と扱わず、[別対応の境界](https://github.com/kani3camp/youtube-study-space/blob/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp/docs/development-history-ready-packet.md#codeql-and-mandatory-oidc-claims)を保持します。

## 旧protocolの経緯（履歴・現行の実行条件ではない）

**退役済みの専用実行経路**：`dev-history-observation-sync` の手動dispatch入力、認証付きpreview/applyジョブおよび旧validatorは
2026-10-18の整理で現行treeから削除しました。固定コミットの当時の資料は監査履歴であり、実行手順ではありません。
通常の `migration-plan` / `migration-apply` とS3 State/認証/secret/no-op契約は継続します。


2026-10-16以前のhistory import、issuer、期限付きwindow、個別Environment承認は**移行当時の手順**です。現在のDev31／Prod30件の運用条件には適用しません。
履歴・再現条件は次の既存資料とGitHub実行証拠から参照してください。

- [history schema修復・一回限りのimportと欠落証跡](https://github.com/kani3camp/youtube-study-space/blob/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp/docs/development-history-ready-packet.md)
- [State観測属性同期の旧手順と停止記録](https://github.com/kani3camp/youtube-study-space/blob/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp/docs/history-observation-sync.md)
- [旧Development history execution windowの固定履歴](https://github.com/kani3camp/youtube-study-space/blob/a94eea6474ae6f89c679e306b291701209607718/infra/gcp/docs/development-history-execution-window.md)
- [現行の定常Plan/Applyと所有範囲](docs/runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)
- [移行最終結果・不確実性・残件（Issue #1191）](https://github.com/kani3camp/youtube-study-space/issues/1191#issuecomment-6233630058)

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

## ローカル検証（移行後）

通常の検証はcredentiallessです。移行時だけ利用したmetadata collector、IAM bootstrap、
reviewed-input packer、disconnected prototypeとその専用テストは現行treeから退役しました。
[移行時の固定Source](https://github.com/kani3camp/youtube-study-space/tree/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp)は証跡として参照できますが、
**固定履歴の旧実行ツールを現在の承認で再実行してはいけません**。

```bash
python3 .github/scripts/test_terraform_migration.py
python3 .github/scripts/test_terraform_migration_contract.py
python3 .github/scripts/test_terraform_runtime_execution.py
python3 .github/scripts/test_terraform_identity_smoke.py
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

**終了済み観測ジョブ**：`current-state-read` / `runtime-metadata-read` とIAM authority readの
旧短期Environment経路も撤去済みです。今後必要なState/認証照合は、新しい用途と権限を独立レビューして設計し、
Git履歴の読取用workflowを再有効化する方法は採りません。

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
- 全4 Terraform Environmentは本人初回workflow_dispatch限定・Required reviewersなし。Productionの実行はDevelopment Exitと独立レビュー後のreadiness gateを必須とする
- plan用identityとapply用identityを分離し、それぞれのEnvironmentから専用least-privilege identityを使う
- full saved plan file / raw stateをpublic Actions artifactへuploadしない
- `terraform show -json` 等のraw sensitive outputをpublic logへ出さない
- public log / PR commentへ出すのはsanitized summaryを原則とする
- application secret valueはTerraformへ極力流さず、Secret Manager等のsecret storeとwrite-only / ephemeralな経路を優先する

外部forkのTerraformコードへcredential付きplanを自動実行しません。authenticated planをPR前に行う場合も、trusted same-repository commitを明示的なgate後に実行する設計とします。

### 旧protocol履歴：Authenticated workflow source guard

以下からsingle history plan receiptまでの旧mode/gate/毎回Environment承認は、当時の履歴です。
現行の実行には[定常Plan/Apply](docs/runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)を使い、
旧windowの再発行や完了済みhistoryの再importは行いません。

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
- historyの`mode=apply` plan jobは専用`apply-read-only` checkを使う。`plan-read-only`のMODE=plan/safety admissionへ流用せず、exact OIDC、dev GCP/STS、state HEAD/GET/HEADとlock LISTを確認する。apply jobのFunction checkもこのmodeではdev-only。条件付きPut、wrong-role STS、prod/他product、別SAのnegative requestをapply経路から呼ばない
- full negative smokeは別gate `DEV_TERRAFORM_SECURITY_PROBE_ENABLED=false`の`security-probe` job/modeに限定する。fresh HEADと異なるrandom ETagの`If-Match`による条件付きPutObject拒否は403のみ合格で、前後version/body/lock差分や412/404/409はSTOP。条件付きrequest/headerの結果だけでrole/bucket/session/SCP全体を証明しない。別のlive承認・private policy review・probe結果がapply gateを開く前に必要
- raw init / plan / apply出力はpublic logへ流さない
- saved planはrunner一時領域だけで扱い、artifact / cacheへ保存しない
- public outputは `.github/scripts/terraform_plan_summary.py` が生成するresource address / action count中心のsanitized summaryだけ
- import移行期はcreate / update / delete / replacement / driftをstopする
- plan jobとapply jobでsaved planを渡さず、apply jobは同じ `github.sha` から再planし、sanitized projectionが一致した場合だけ同一job内のplanをapplyする

historyの次applyについて、現行`mode=apply`はapply gate=falseでpreflight停止します。
probe gateもfalseで、source準備だけではfull security gateを満たしません。
live probeとrole/bucket/session/SCPのprivate reviewは別承認で行い、plan-only成功を
full security gate PASSへ読み替えません。

developmentのGitHub Environment / branch trust、AWS GitHub OIDC backend role、GCP GitHub WIF / Terraform Service Accountは#1162で構築・実測済みです。通常PRはcredentiallessのまま、authenticated executionはtrusted integration refと独立Environment approvalへ限定します。production側のbackend / trust / identityは未開始で、#1191の別approval境界です。

production backendは未bootstrapのため、production authenticated plan / applyはbackend準備完了まで有効化しません。

### 旧protocol履歴：Single development history plan receipt

history executionは既存protected identityとsecretを使い、routine Terraform cost blockingのwaiverを安全条件から分離する。新しいbilling API、料金照合、月次予約を実行条件にしない。過去の見積・台帳は履歴であり、実請求額はunknownのまま。新resourceやbulk processingの承認はこのwaiverに含めない。

caller input `terraform_history_plan_safety_evidence`（reusableでは`history_plan_safety_evidence`）は以下のキーだけを許可する。旧`terraform_plan_cost_evidence`をhistory executionに渡すと認証前にSTOPする。state/metadata、bucket/account/role/SA名、privateな証拠本文を公開inputへ貼らない。

| Key | Required evidence |
| --- | --- |
| `schema_version` | 整数`1` |
| `git_sha` | review・公開された実行対象の完全なSHA |
| `issued_utc`, `expires_utc` | `YYYY-MM-DDTHH:MM:SSZ`。発行済み・期限内、発行後最大24時間。planとpost safety readの余裕を確保する |
| `max_state_bytes` | 整数`1..4194304`の承認上限で、実測サイズではない。今回のfresh baselineは固定`4194304`を選び、実サイズは外部未測定とする |
| `state_writers_quiescent` | 対象state/workspace prefixの並行Terraform writerがないと確認した場合だけ`true`。BigQueryの通常writer停止は要求しない |

認証前に0600/exclusiveの`history-plan-admission.json`を生成する。AWS認証後、GCP認証やstate GET/native backend initの前に、既存Environment secretのRoleIdとSTS `UserId` prefix、exact accountを照合する。history plan/applyの両jobで必須。追加grantやnegative probeを使わない。

今回のcurrent-state baseline再発行では`max_state_bytes=4194304`（既存`MAX_BYTES`の4MiB）を承認上限として指定する。
別の外部HEAD jobやbucket locatorによる事前実測は要件にしない。exact実行SHA、新しいUTC発行時刻・最大24時間の期限、
別途fresh確認した既知writer静止は引き続き必要で、上限を選んだことから実サイズやwriter静止を推定しない。
本人のEnvironment承認後、同じprotected jobのidentity検査は最初のState本文GETより前にfresh HEADで
`0 < ContentLength <= 4194304`、有効なVersionId / ETag、STANDARD・既知暗号化等を検査し、bounded GET後に全HEAD応答との一致を再確認する。
後続post-noop snapshotもprofile上限を使ったHEAD→bounded GET→全HEAD再照合をinit/plan前に行う。
この経路ではprofileとidentityの既存固定上限が**同じ4194304のため**一致する。4MiB未満のprofileがidentity段階の上限も下げるとは扱わない。
HEAD検証失敗なら本文GET・init/plan・成功receiptへ進まず、GETや再照合で失敗してもその時点でSTOPする。
実サイズを観測済みと記述できるのは、runnerでこれらの検査が実際に成功し確認できた後だけである。

initial stateはHEAD/GET/HEADとpositive lock/workspace LIST absenceで取得する。exact既存11 managed instance、history未登録、既知Terraform state/check envelope、current VersionIdと全bytesをprivateに検証する。STANDARD class・既知暗号化・exact sizeを確認できなければdownload/initへ進まない。403/通信失敗/欠落/truncated responseを不在と見なさない。

init/plan/sanitizer後は`always()`で再snapshot、stable/未知fieldのstrict tables.get比較、volatile観測値のprivate記録、exact native lockのpositive LIST absenceを行う。期限切れや失敗でも保存済みadmissionの上限でpost safety readを試みるが、fresh evidence・全step成功・strict import1/既存11 no-op/他action0だけが成功receiptを作れる。独立post-noop routeも同じ安全入力を使い、import0/no-op12/他0を要求する。

public Summaryは固定PASS/STOP、sanitized action数、waiverとactual cost unknownだけ。raw state、VersionId/serial/lineage、complete metadata、binary planは0600のprivateファイルで扱い、artifact/cacheへ保存せずalways cleanupする。強制停止・権限不足等で完全receiptが得られなければclosureしない。初回history applyではauthentic root emitterを必須にする。具体的な承認v2、source freeze、独立post-noopとSTOP/recoveryは[history packet](https://github.com/kani3camp/youtube-study-space/blob/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp/docs/development-history-ready-packet.md#one-shot-import-source-contract-not-execution-approval)を参照する。

旧`cost_policy()`と`dev-history-plan-monthly-2026-10-09-v1`は過去のoffline ledger解釈用だけに保持する。live admissionから呼ばない。S3 lock version/delete markerの残存や後月保管費は0・削除済みと見なさない。

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

Phase 1のrepository validationはcloud credentialを使いません。

Issue #1161のdevelopment backend bootstrapはoperatorの短期SSO credentialで完了済みです。長期AWS access key / secret key、Google Service Account JSON keyは作成していません。

通常運用では認証をCIへ寄せます。

- S3 backend: GitHub OIDC → AWS IAM role
- GCP provider: GitHub OIDC / Workload Identity Federation → GCP
- ローカルのauthenticated `plan` / state operationはmigration・障害調査等の例外用途とし、AWS SSO / Google ADC等の短期・更新可能credentialを使う
- public PR CIはcredentiallessを維持する
- development authenticated plan / applyはtrusted ref / GitHub Environment / least privilegeで実装・実測済み
- production backend/trustは実bootstrap済み。正規migration経路は有効で、State初期化・既存30件のownershipと最終no-opは各実行結果で検証する

既存AWS runtime → GCP WIFとはTerraform CI trustを分離します。

## Production boundary

production:

- 最初のimportは同resource typeをdevで確立してから
- 本人初回workflow_dispatch限定・Required reviewersなし。実測Development Exitと独立レビュー済みproduction readiness gateを必須とする
- import直後の想定外diffをapplyしない
- destructive planは停止
- Node.js 22 migrationとTerraform ownership移行を同じ変更にしない


## Bootstrap follow-up

Issue #1161のdevelopment S3 backend bootstrap本体は完了しています。

残件はbootstrapとは分離して追跡します。

- Issue #1165: alternate contacts / recovery運用
- Issue #1166: durable CloudTrail / S3 data events監査
- Issue #1162: public repository向けauthenticated Terraform plan / apply（completed）
- Issue #1191: Development Exit実証済み / Production State・既存ownership移行と最終ReadyGate

#1165の[recovery runbook](docs/state-account-recovery.md)と#1166の
[durable audit source/設定案](docs/state-durable-audit.md)は独立follow-upです。
Audit logging/継続課金は未有効化で、設計案を実装完了と扱いません。

## 完了済みDevelopment waveの履歴

以下の各wave、旧modeとEnvironment承認は当時の記録です。現在は上記の定常migration経路と固定baselineで全所有を保持します。

### Development import wave 1: native backup schedule

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


### Disabled runtime wave execution receipts

Runtime ownership のfresh-state/apply/per-wave receipt sourceは
[runtime ownership contract](docs/runtime-wif-api-ownership.md)を参照。
既存11/history12のexact guard、same-SHA/private packet/plan seal、one-object state delta、full-root post-noopと
complete unrelated IAM/API/WIF metadata保存を検査する。gateはfalse・issuer catalogは空。
original importは成功しましたが、authentic history root receiptは未取得です。既存v1 receipt/v2 packetのimport/apply履歴契約を維持し、別のcurrent-state attestation/v3 packetを追加します。dummy resultsは実receiptやactivation approvalではありません。
