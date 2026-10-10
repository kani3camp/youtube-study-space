# Phase 2: next approval and infrastructure readiness

実行入口は [Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)。
これは公開可能な最小specとsanitized実行記録です。private inventoryや承認そのものを保存しません。
account number、WIF principal全文、既存policy、実集計値、state/planをこの文書へ追加しません。

## Recorded audit and next boundary

dev historyの判断材料、本人の先行準備、offline import検査、production依存とCodeQL/OIDC制約は
[`development-history-ready-packet.md`](development-history-ready-packet.md)へ集約します。
code/tests/specは一つのbundleでreviewし、実行はdeployment/schema/state/securityの影響境界で分けます。

実run `37475212059` はsuccess。ユーザー貼付のsanitized Summaryはlegacy列present yes、
4集計すべてis-zero yesでした。監査時点のlegacy値backfillは不要ですが、列は未削除です。
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

2026-10-06の#1191最新記録はdev full-root `no-op11 / drift0`、resources11、serial13です。
これは記録の引用であり、このcheckoutでのauthenticated revalidationではありません。
#1192–#1197のsource統合だけではruntime deployやschema修復を意味しません。
専用auditは上記runで成功・cleanup済み。schema修復/importとguardのactual runtime deploymentは別gateです。

| Gate | Required fresh evidence before advancing | Separate approval boundary |
| --- | --- | --- |
| Dev audit | 1回audit成功、sanitized判定はユーザー貼付、runtime/source cleanup完了。外部consumerは未確定 | この実行waveは完了。再実行は未承認 |
| Dev schema repair | schema guardの対象runtimeへの反映、external BI/cross-project/manual SQL/wildcard/view確認、tmp canonical、recovery期限とrollback、audit意味の判定 | runtime deploy、DROP/backfillは各exact diffで別承認 |
| Dev history adoption | repair完了後のfresh canonical8 field/order。exact table import1 + existing11 no-op、drift/unknown/other0 → post no-op12 | import-only別wave。schema mutationと混ぜない |
| Dev runtime WIF/IAM | pool/provider mapping/condition/AWS account/SA IAM exact memberのprivate fresh GET、provider実Read権限、ownership分類 | CI permission追加/importは別承認。project IAM/SA本体をownershipしない |
| Dev APIs | fresh enabled servicesとdependency分類、Own exact serviceだけ、destroy/disable semantics確認 | import-only。enable/disableを混ぜない |
| Dev exit | 上記実行結果とactual SHA一致の通常full-root完全no-op、unknown/drift0、state recovery/lock証拠 | 想定外差分時STOP。resource数を11に固定しない |
| Prod backend/trust | fresh S3 key/account/roles/trust/lock/version/recovery、prod Environment/OIDC、GCP WIF/plan/apply最小権限、public output guard | bootstrap/IAM/trust/security変更を対象別packetで承認 |
| Prod imports | fresh inventory→classification→定義→small import-only full-root→independent protected approvals/same-SHA再plan→post no-op | each wave。create/update/delete/replace/drift/unknownが1件でもSTOP |
| Infrastructure Ready | dev/prodのapproved ownership、fresh normal full-root no-op、実trust/negative probes、state/lock/recovery、未完了項目の明示判定、記録と実測SHA一致 | code/tests/docs PASSだけではReadyにしない |
| MyPage release ReadyGate | Infrastructure Ready証拠を親/MyPage taskへ渡し、MyPage固有IAM/API/Cloud Run/Hosting/data migration・機能検証の別gateと照合 | MyPage release/deploy承認はinfra承認から継承しない |

順序はdev残件完了→production基盤→MyPage release ReadyGateです。MyPageの独立source/tests準備は並行できます。
productionのEmail channel0/quota policy3 dangling refsはIssue記録だけで、fresh再確認していません。
notification repairはfresh metadata/minimal field-mask/rollbackを揃えた別operator承認waveです。
production importsへ混ぜず、通知経路Readyの判定が必要なら未検証のままPASSにしません。
#983 cleanup、#1165 recovery、#1166 durable auditは別Issueとして残し、今回へ吸収しません。
