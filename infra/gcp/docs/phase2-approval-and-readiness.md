# Phase 2: next approval and infrastructure readiness

実行入口は [Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)。
これは公開可能な最小specとsanitized実行記録です。private inventoryや承認そのものを保存しません。
account number、WIF principal全文、既存policy、実集計値、state/planをこの文書へ追加しません。

## Recorded audit and next boundary

dev historyの判断材料、本人の先行準備、offline import検査、production依存とCodeQL/OIDC制約は
[`development-history-ready-packet.md`](development-history-ready-packet.md)へ集約します。
code/tests/specは一つのbundleでreviewし、実行はdeployment/schema/state/securityの影響境界で分けます。

実run `37475212059` はsuccess。ユーザー貼付のsanitized Summaryはlegacy列present yes、
4集計すべてis-zero yesでした。監査当時は列未削除でしたが、その後のmetadata/Stateで
canonical8列とhistory ownershipを確認済みです。再DROP・再import・不要なbackfillは行いません。
runtime temporary access撤回はoperator readback報告、source gate=falseは#1238で完了。
判定のSQL上の意味、offline canonical preparation、未承認のschema修復→別wave importは
[`user-activity-history-canonicalization.md`](user-activity-history-canonicalization.md) を参照してください。

## Historical approval scope: development aggregate audit once

以下は完了済みaudit waveの当初設計です。resource作成やaudit再実行の指示ではありません。
このpacketの作成やsource PRの承認は、新規accessの実行承認を兼ねません。
以下の候補名をprivate read-only inventoryでcollision確認し、exact diffとprincipalを
親スレッドへ提示してから、実行直前の明示承認を待ちます。既存同名resourceは流用しません。

| Target | Proposed addition | Exact permission / restriction |
| --- | --- | --- |
| Project `test-youtube-study-space` | SA ID `dev-user-activity-schema-audit` | 新規専用identity 1件。keyなし |
| Same project | custom role ID `devUserActivityAuditJobs` | `bigquery.jobs.create` だけ |
| Same project IAM | 上記jobs roleのmember 1件 | 専用audit SAだけ。既存binding不変 |
| Same project | custom role ID `devUserActivityAuditTableRead` | `bigquery.tables.get`, `bigquery.tables.getData` だけ |
| `test-youtube-study-space.firestore_export.user-activity-history` IAM | 上記table-read roleのmember 1件 | exact tableだけ。dataset/projectには付与しない |
| Dedicated audit SA IAM | `roles/iam.workloadIdentityUser` member 1件 | 既存GitHub WIFからこのaudit workflowの承認済みrunだけ |
| GitHub Environment `gcp-dev-user-activity-schema-audit` | protected Environmentとprivate input | manual approval、reviewer/self-review/bypass設定をfresh確認して承認済み設定を適用、trusted integration branchだけ |
| Same Environment | secret names | `GCP_USER_ACTIVITY_SCHEMA_AUDIT_SERVICE_ACCOUNT`, `GCP_TERRAFORM_WIF_PROVIDER` |
| Source gate | reviewed activation PR、終了後closure PR | `DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED` false→true→false。common CI routing変更なし |

実SA email、project number、provider resource name、federated member、reviewer IDs、
branch protectionの実diffはprivate packetで確定します。Environment既存時はfresh設定を確認し、
欠けたprotectionだけを明示diffにします。Terraform plan/apply identitiesとそのbindingは変更しません。

既存Environmentのreviewer、`prevent_self_review`、admin bypass設定はsourceだけでは確定できません。
別reviewerが必要とは仮定しません。self-review禁止を選ぶ場合、dispatch actorとreviewerの分離が
必要なので、既存運用と実設定を確認してからその変更を承認packetに含めます。
会話内のaction-time承認と、GitHub Environmentの手動承認は別です。

### IAM capability and stop conditions

`bigquery.tables.getData` は対象tableのrow read能力も与えます。IAMだけでSQLを4aggregateに
限定することはできません。実行するreviewed CLIは`timestamp` / `taken_at`の4COUNTIFのみで、
row sample、user ID、timestamp sampleを返しません。IAM capabilityと実行SQLの範囲を区別して承認します。
query job作成には課金・一時query resultが伴い、データ/schema read-onlyでもjob metadataは残ります。
scan量/costの見込みと既存quotaをprivate metadataで確認します。

[BigQuery公式resource IAM](https://docs.cloud.google.com/bigquery/docs/control-access-to-resources-iam)
はtableへのcustom-role付与を説明しています。ただし、対象tableの種類、custom-role support、
既存継承権限、現在のAPI support、実role bindingの成立は未検証です。exact2 permissionの
table scopeが成立しない場合、Data Viewerやdataset/project-wide grantへ広げずSTOPします。

[WIF公式attribute mapping](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-deployment-pipelines)
に従い、existing pool/providerのmappingとconditionをprivate GETで確認します。
repositoryだけのprincipalSetやEnvironmentだけのsubjectを「workflow限定」と扱いません。
次をcloud trust / exact mapped principalとGitHub Environmentの組み合わせで実際に拘束できることが条件です。

- repository ID `340900071` / owner ID `54093651`
- ref `refs/heads/feature/gcp-terraform-iac`
- Environment `gcp-dev-user-activity-schema-audit`
- caller `kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac`
- reusable job workflow `kani3camp/youtube-study-space/.github/workflows/gcp-user-activity-schema-audit.yml@refs/heads/feature/gcp-terraform-iac`

existing mappingでこの限定が表現できない場合、provider mapping/condition変更や新providerは
今回の承認scopeへ自動追加しません。exact before/afterとexisting Terraform CIへの影響を示す
別packetが必要です。workflowのshell guardだけでcloud trustの不足を補ったとみなしません。

禁止: table/schema/dataset write、BigQuery Admin/Data Editor、unrelated table read、Firestore read、
state/backend access、production grant、SA key、API enable/disable、runtime deploy、manual export trigger。
operatorのIAM-write能力をaudit SAへ渡しません。

### Approved execution sequence and cleanup

1. Private fresh preflight: exact IAM policies/etags、role-name collision、inherited grants、existing WIF
   mapping/condition、table metadata/type/location、Environment protections、reviewed SHAを確認。
   本文取得や集計queryはこのpreflightでは行わない。
2. 完成したprivate exact diff / target / principal / rollbackを親へ提示し、action-time承認を待つ。
3. 承認scope内で専用SA・exact custom roles・memberだけを作成。policy変更はfresh etagを使い、
   unrelated member/conditionを維持。Environment protectionsと2 private inputsを設定してread-back。
4. Reviewed integration activationを取り込み、同SHAでmanual dispatchを1回だけ承認。
   再run/retry/2回目queryは自動で許可しない。canonicalでlegacy fieldなしならmetadata readだけで終える。
   legacy fieldありならexact4 aggregateを1query。実集計JSON/stderrはrunner tempのみ。
5. Public Summaryはpresenceとzero/non-zeroだけ。exact countsはpublic log/artifact/commentへ出さない。
   runnerのprivate outputはcleanupで破棄されるため、後日そこからexact countsを復元できるとは約束しない。
   exact countsのprivate引き渡しが必要なら、実行前にその経路を別途確定する。
6. 成否にかかわらずworkflow source gateをfalseへ戻す。audit専用WIF member、table-read member、
   project jobs memberを除去し、SAをdisable。専用Environmentのsecretを除去して新runを止める。
   他workflowが使うprovider、Environment、memberは変更しない。
7. Private read-backでexact grant不在、SA disabled、source gate false、既存binding不変を確認。
   既発行tokenが消えたとrevokeだけから推測しない。発行済みcredentialの実lifetime/expiryと
   running job停止を確認し、expiry後の権限不在まで追跡する。Action access-token設定600sだけで
   ADCが発行する全credentialの寿命を保証したとは扱わない。
8. Public logs/Summary/artifactsをleak監査。artifact0、private files cleanup、schema/data/state差分0。
   新SA/custom rolesの恒久保持は許可しない。disable後の削除または期限付きcleanupをprivate結果に記録。

途中失敗時はqueryを再実行せず同じrevoke/disableを先に行います。rollbackは追加memberだけの除去で、
project/table/SAのpolicy全体を古いsnapshotへ上書きしません。専用Environmentを削除する場合も
新規作成分だけです。データ復元やschema rollbackはこのauditでは不要で、schema repairの別waveで扱います。

## Development → production → MyPage ReadyGate

現行は[定常Plan/Apply](runtime-wif-api-ownership.md#定常planapplyへの移行2026-10-16)と
[Productionの実行境界](production-readiness.md#current-execution-boundary--2026-10-17)を優先します。
全4 EnvironmentはRequired reviewersなし・admin bypass=false・integration限定、本人ID54093651の初回dispatchだけを通します。
包括承認内の作業へ都度Chat/Environment人間承認を再導入せず、exact source・独立review・State保護を維持します。

| 判定対象 | 現在の実証 | 判定 |
| --- | --- | --- |
| Development ownership / Exit | [run38945893535](https://github.com/kani3camp/youtube-study-space/actions/runs/38945893535)で両identityの31 no-op/import0/rawdrift0、State current/prior/version/lockと独立review PASS。DevExit後fresh自然回帰5 stage/finding0・Lambda3/6で1,338実行・Errors/Throttles0も独立PASS | PASS。quiet3/未観測の限界とoriginal final receipt欠落は保持。未来の最終no-op後healthへ読み替えない |
| Production backend / CI foundation | AWS roles2/policies2、GCP resources7・個別policies5/member追加8・削除0、Prod2 Environmentの設定14件をreadback・独立review | PASS。既存runtime資産・共通bucket policyを上書きせず、業務resource変更なし |
| Production State / ownership / Exit | H2の[初期化run38961710157](https://github.com/kani3camp/youtube-study-space/actions/runs/38961710157)とH3の初版version GET/checksum・controls9・Dev不変の直接監査／独立PASS。H3の[run39005902978](https://github.com/kani3camp/youtube-study-space/actions/runs/39005902978)まで9件のownershipを検証済み | PENDING。H4の[第10波run39014888191](https://github.com/kani3camp/youtube-study-space/actions/runs/39014888191)は認証付きPlan/Apply・post10成功後、native runner回帰の静的CIとGateで失敗。State10/直前9のversion固定GET・厳密差分照合は独立PASSで、元runの静的CI失敗を保持します。State10照合後にH5の[第11波run39021164334](https://github.com/kani3camp/youtube-study-space/actions/runs/39021164334)を1回発行しました。認証付きPlan/Applyは成功しましたが、native runnerのtimeout/grandchild回帰で静的CIとGateが失敗しました。State11/直前10のversion固定GET・厳密差分照合も独立PASSで、直前10の本文/HEADは第10波の独立確認済みcurrentと完全一致します。元runの静的CI失敗を保持し、第12波は未発行・再送0です。全30件・両identityの最終no-op・State/Production healthは未完了 |
| Infrastructure Ready / IaC拡張の前提 | 両環境のapproved ownership、通常full-root import0/no-op、実trust/negative checks、State/version/lock/recovery境界、回帰、未完項目の明示判断と正本同期が必要 | HOLD。Productionの最終実証受領後に判定する |
| MyPage provisioning / release | MyPage固有のexact IAM/API・Cloud Run/Hosting等、data migration、費用/rollback、機能/Privacy検証は未実行 | HOLD。Infrastructure Readyから自動的にprovision/deployしない |

Infrastructure Readyが成立すれば、MyPageの新規infraをreviewするnormal-change PRへ進むための移行基盤は整います。
現migration workflowはimport-onlyで、一般create権限・MyPageのcreate承認を持ちません。
後続PRで、現行13type・exact import ID・全resource no-op契約とは分離したcreate/update実行経路と検証契約、必要なtype/schema・最小IAM/API・費用・rollbackを整えます。ここでのInfrastructure ReadyはCanonの既存ownership・CI/State・回帰証拠を満たした判定であり、その後続実装やMyPage provisioningの完了を意味しません。
一方、既存workflowのままMyPageを直ちにprovisionできるとも報告しません。

### 回帰証拠の範囲

DevExit cutoff `2026-10-16T18:18:57Z`に対するfresh自然回帰を `2026-10-17T05:08:26Z`に取得し、27 reads・全5 stage・finding0を独立review済みです。Lambda3/6で1,338実行・Errors/Throttles0、quiet3はNone/未観測のままです。
日次SFNの最新成功はcutoff前、5件listはtruncatedです。fresh ECS STOPPED該当0で、cutoff後の日次SFN/ECSや新規native backupは未観測、直接execution/task joinはfalseを保持します。READY backup29件はすべてcutoff前です。先行日次E2E/当時のECS exit0と今回の観測を混同しません。
現在のLambda6/role7/ECS定義/caller設定、Function ACTIVE/Node22、Scheduler ENABLED、backupScheduleは一致し、公開docs HEAD200は補助観測です。この結果はDevExit後のfresh回帰であり、未来の最終source no-op後healthではありません。
Productionの移行後回帰は最終ownership後に別途記録します。未観測・取得不能を成功や0件で補完しません。

### 移行と独立して残す判断

以下の最新判断は今回のexisting ownership移行のnon-blockerですが、すべてOPEN・Acceptance未達です。

- [#1165 contacts / recovery](https://github.com/kani3camp/youtube-study-space/issues/1165#issuecomment-6223368027): MyPage Production release前の運用判断。State Versioning/過去version GETは独立break-glass・MFA・復旧訓練の証明ではない。
- [#1166 durable audit](https://github.com/kani3camp/youtube-study-space/issues/1166#issuecomment-6223368703): 永続S3 data-event監査・retention・継続費用は別判断。今回のCI証跡と混同しない。
- [#983 Privacy cleanup](https://github.com/kani3camp/youtube-study-space/issues/983#issuecomment-6223369320): 旧raw-chat/混在GCS backupの削除は今回の対象外。新規MyPageデータの保持/削除はrelease gateで検証する。

Production Email channel/dangling quota通知先の修復は別waveで、通知経路Readyは未検証です。
CodeQL [PR #1362](https://github.com/kani3camp/youtube-study-space/pull/1362)は本人の明示承認でDefault setupを `not-configured` へ切り替えました。H2追随候補Q2（`92b5ea29`）の全31 CIと[run39001207914](https://github.com/kani3camp/youtube-study-space/actions/runs/39001207914)の3言語解析・SARIF受付が成功し、analysis ID3件・error0、既存open alert1件の保持を確認済みです。
候補と全tree一致のintegration H3（`8b2289c8`）へ統合済みで、H3 pushの[run39001594887](https://github.com/kani3camp/youtube-study-space/actions/runs/39001594887)でも3言語の解析・SARIF受付成功、error0を確認済みです。GCP closureはH2と完全一致し、TerraformのH2当時の実Plan/Apply証拠と区別します。先行拒否・upload失敗の履歴は[history packet](development-history-ready-packet.md#codeql-and-mandatory-oidc-claims)へ残します。
integration refのCodeQL未解決finding6件（critical1/high3/medium2、既存1＋新検出5）は別対応で、dismiss/suppressしません。受付成功を脆弱性0と扱わず、実移行・検証Source H5とH3の解析実証を区別します。最新の運用Sourceと最終runは[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)を参照してください。
最終dev mergeはユーザー本人だけが行います。
