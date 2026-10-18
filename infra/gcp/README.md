# GCP Terraform

> 2026-10-18: #1251のdevマージ後、完了済みの一回限りの補助コード・旧手順書を退役。証跡は[固定コミット](https://github.com/kani3camp/youtube-study-space/tree/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp)と[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191#issuecomment-6244637630)に保持します。

YouTube Study SpaceのDevelopment／Productionで採用済みのGCP resourceをTerraform管理するためのrootと運用入口です。移行時の個別判断は後半の固定履歴を参照してください。

設計上の正本はNotion「GCP Terraform / IaC移行」、実装・CI・import状態の正本はこのrepository / GitHubです。

## 現行の運用範囲（2026-10-18）

PR #1373では独自Attestation/Ownership Receipt/issuer catalog/期限付き証跡と旧認証入口を撤廃し、
`migration-plan` / `migration-apply`へ一本化します。[依存調査・維持する安全機構・GitHub実設定と保護提案](docs/terraform-authentication-boundary.md)を参照してください。
現在のStateとクラウドmetadata、full-root plan、native lockingの検査は継続します。
過去の証跡を新たな実行の承認根拠には使いません。

現行運用はDev31件・Prod30件の固定baselineのfull-root監査planと、承認済みimport/no-op・State-only saved applyを扱う [`migration-plan` / `migration-apply`](docs/runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)です。
入口は既存 `ci.yml` の本人初回 `workflow_dispatch`です。Devは各Environmentの選択branch、Prodは`main`だけを許可します。
両環境の最終実行・検証証跡は実証Source H7（`0ed7486b`）に紐付きます。PR #1251の`dev`マージと最終Sourceの扱いを含む実績は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191#issuecomment-6233630058)を参照してください。現在のcommitで同じTerraformを再実行した証拠とは扱いません。
GitHub管理APIで全4 Terraform EnvironmentのRequired reviewersなし・選択branch制限を確認しました。plan/apply identityは分離され、現在は人間の承認待ちを必須にしていません。
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
実際のcloud trustとの整合は別途readbackが必要です。今回のsource更新だけではcutover完了と判定しません。

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

## ディレクトリと現行の責務

| 場所 | 用途 |
| --- | --- |
| `environments/dev/`、`environments/prod/` | 環境別の独立root・State key・provider lock。workspaceで環境を切り替えない |
| `modules/` | 管理対象の共通Terraform modulesと既存リソース契約 |
| `scripts/`、`tests/` | 現行の設定検証、offline準備、negative / native fixtureテスト |
| `docs/` | 復旧・運用・準備判定の既存runbook。現行の判断入口は以下のリンク |
| `.github/scripts/terraform_migration.py`、`terraform_migration_contract.py` | 通常の固定baseline import-only plan/apply実行・拒否契約 |
| `.github/workflows/ci.yml`、`gcp-terraform-authenticated.yml` | credentialless PR CIと保護された本人起動の運用経路 |

古い実行ツールと旧windowの入口は現行treeに残しません。過去のコード、テスト、個別runbookの内容は[固定移行コミット](https://github.com/kani3camp/youtube-study-space/tree/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908)から確認できます。

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
python3 .github/scripts/test_terraform_authenticated_workflow.py
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

### 完了済みの一回限り手順

旧history import、短期のissuer/window、metadata reader、quota/Email/exportの各adoption waveの詳しい手順は運用対象外です。
当時の意思決定・失敗・再検証・復旧の限定証拠は[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191) と[固定コミットの旧README](https://github.com/kani3camp/youtube-study-space/blob/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp/README.md)から辿れます。
古いgateや承認windowは再有効化せず、現在の認証・State・import/no-op境界は現行sourceを優先してください。

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

## 移行時の履歴と継続保守

- 既存Dev31件／Prod30件の採用・通常no-op確認と未観測事項：[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191#issuecomment-6233630058)
- 旧waveのコード・テスト・実行runbook：[移行当時の固定コミット](https://github.com/kani3camp/youtube-study-space/tree/2a92a8874f8d56077d5bb6edf4ef00cfaf7cc908/infra/gcp)
- post-mergeで退役した実行経路とテスト：[Issue #1191の整理方針](https://github.com/kani3camp/youtube-study-space/issues/1191#issuecomment-6244637630)
- 今後の管理対象とMyPage向けcreate/updateは別の変更承認／権限／検証契約として設計する。現在のimport-only guardを迂回しない

古いSourceに含まれる手順をそのまま実行せず、現在のGitHub・認証済み実環境・Notion Current Canonから最新の安全条件を照合します。

## Terraform認証実行のブランチ移行（段階的cutover、2026-10-18）

- **目標（2026-10-18更新）**：Developmentの認証付きTerraformは `dev` を基本とし、GitHubの `terraform-dev-plan`／`terraform-dev-apply` Environmentの **Selected branches and tags** に必要な統合ブランチを本人が追加・削除できる運用にする。Productionは `main` のみ。credentialed jobはEnvironmentのbranch selection、same-branch caller/reusable、exact SHA、本人初回dispatch、plan/apply独立identityを検証する。preflight単体ではEnvironment allowlistを取得できず、**選択ブランチの最終認可はEnvironmentで強制**する。
- **任意ブランチ**：PRのTerraform fmt / credentialless init（`-backend=false`）/ validate / mock・negativeは引き続き無認証。Devの認証付きPlan/ApplyはGitHub Environmentへ**明示追加されたブランチ**だけが実行できる。Planだけ許可、Apply未許可も可。未選択ブランチ・外部fork・タグへcredentialを渡さない。統合branchでは実行前にreview済みの最新workflow/headを検証する。Selected branches方式でbranch protectionはtoken発行の技術上の必須条件ではないが、許可branch内のコード改変を防ぐために推奨する。
- **このPRの境界**：`dev`にソース・テストがマージされても、AWS trust policy、GCP WIF attribute condition / SA IAM、GitHub Environmentのbranch restriction/secret/required reviewerは自動更新されない。既存 `feature/gcp-terraform-iac` の実運用sourceも自動更新されない。**PRのマージだけでcutoverが完了したとは判定しない**。
- **前提**：現行 `main` にはTerraform rootとauthenticated workflowが未導入。通常の `dev → main` 昇格PRでレビュー・CIを通してからProductionで使う。昇格とIAM trust切替の順序・ロールバック・停止期間を別の運用reviewで決める。
- **cutover手順**：①既存trusted実行とState/version・lock・protected Environment/role/SAを読み取り確認、②**初回のみ**Dev AWS IAM trustを固定repo/owner/Environment/caller/reusable/eventのままブランチ3か所だけ`StringLike`／`refs/heads/*`へ、Dev GCP WIFを固定actor・initial attempt・same-SHAを維持しcaller/reusableが**同じ`assertion.ref`**と等しいCEL条件へ変更、GitHub Dev plan/apply Environmentは `Selected branches and tags` で `dev` と承認済み統合ブランチだけに限定。本番AWS/GCP trustは `main` 完全固定、③非許可ref/不一致caller/別actor/tag/旧branchのnegativeと許可対象の認証付read-only Planを検証、④必要なApplyとno-op・State安全性を別承認で検証、⑤旧branchとRulesetは最後に退役する。
- **変更禁止の範囲**：このソースPRでは `main` への直接push、`dev`のbotマージ、Cloud IAM/WIF、remote Terraform State、live Plan/Apply、workflow dispatch、ブランチ削除を実行しない。cloud trustの設定readが不可の場合は未確認と明示し、推測したまま切り替えない。

- **Devのcloud trustは一度だけ変更が必要**：AWS IAMは`StringEquals(aud)`／`StringLike(sub)`を用い、repo/owner/Dev Environment/caller/reusable/eventは厳密に保持、ref・workflow_ref・job_workflow_refのブランチ部分だけワイルドカード化する。ただしIAMの複数ワイルドカードは相互に同じブランチであることまで証明しないため、GitHub Environmentのブランチ許可・GCP WIFの`workflow_ref == fixed_caller + assertion.ref` / `job_workflow_ref == fixed_reusable + assertion.ref`・same-SHAチェック・workflow preflightの多層防御が必要。GCP SA impersonation IAM memberが旧branch subjectに固定なら、そこも一度だけ移行する。以降の追加/削除操作はGitHub Environmentのみ。
- **注意**：Devで最初に`terraform-dev-plan`へ統合ブランチを追加し、実Plan/readを確認してからApplyも必要なら追加する。旧`feature/gcp-terraform-iac`はGitHub Environmentでの許可を失効させるまでDev IAM/WIFのワイルドカードtrustを有効にしない。現在のimport-only controllerでMyPage新規resourceを作成できるわけではない。MyPage用のcreate/update実行contract、最小権限、デプロイ実装は別対応。

監査入口：[PR #1251](https://github.com/kani3camp/youtube-study-space/pull/1251)・[PR #1372](https://github.com/kani3camp/youtube-study-space/pull/1372)・[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)。
