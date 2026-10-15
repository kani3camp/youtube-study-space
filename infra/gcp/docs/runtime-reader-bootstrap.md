# 既存CIにRuntime設定の読取権限を追加する案

この文書と [`bootstrap_runtime_readers.py`](../scripts/bootstrap_runtime_readers.py) は、実行前にレビューするための準備物です。
**本人の通常GCPログインでIAMを変更する例外は、まだ承認されていません。**
元の「IAM変更はGitHub OIDCとGitHub Environment承認を通す」という契約を、このsourceの追加で変更したとは扱いません。
実行手順と具体的な例外判断は担当者が、確定したsource・private差分・独立Security reviewを揃えてからまとめます。

## 変更する範囲

| 作成するcustom role | 含める権限 | 付与先 |
| --- | --- | --- |
| `terraformDevRuntimePoolReadV1` | `iam.workloadIdentityPools.get`、`iam.workloadIdentityPools.getAttestationRules`、`iam.workloadIdentityPoolProviders.get` | devの既存Runtime Pool 1件 |
| `terraformDevRuntimeSaPolicyReadV1` | `iam.serviceAccounts.getIamPolicy` | devの既存Runtime SA 1件 |

各roleを既存のplan/apply用GCP SA 2個だけに付与します。新role 2件、binding 2件、member出現数4件、write試行は最大4回です。
既存CIの名前は、成功したprotected OIDC readと現行sourceのidentity検査に基づきます。新しい本人メールや識別子の提出は不要です。
custom roleの定義先namespaceはdev projectですが、Project IAM policyへのgrantはありません。
既存roleの更新、広いViewer/Admin、CIへの変更権限委譲、元AWSキー経由の変更、prod、WIF trust変更、Terraform、State/lock書込は対象外です。

PoolへのbindingがProvider読取に効く場合、粒度はそのPool配下の現在・将来のProvider全体です。
単一Providerだけに絞れるとは主張しません。AWS federation Poolでのbinding受理・各permissionの適用・Provider継承は未実証です。
過去の`queryTestablePermissions` HTTP400を、その可否の証拠へ読み替えません。失敗時にProject全体のgrantへ広げません。

## 既定動作とprivate manifest

既定はofflineです。private manifestのhash・schema・固定対象・2 roleの全定義を検査し、認証もHTTPも変更も行いません。
`--manifest-file`と`--manifest-sha256`が必須です。入力はcheckout外のcanonical absoluteなowned regular fileに限定し、symlink/hardlink、共有write、4MiB超過、重複JSON key、未知fieldを拒否します。
upload時の0644は、開いたfdで0600へ制限します。ファイルの親はownedかつgroup/other-write不可でなければ停止します。

private manifestは担当者が既存の正本から一括生成します。利用者が各fieldを転記する方式ではありません。
正確なschemaはsourceの`MANIFEST_KEYS`と`validate_manifest`を正本とします。
主な内容は次の通りです。

- dev project ID/number、既存Pool/Provider/Runtime SAの住所、既存CI 2 identity。
- 旧captureのhashは住所の出典だけに使用。旧state/CEL/policy/attestation/history/成功flagを現状証拠に使わない。
- レビュー済みintegration revision、helper自身と同revisionのworkflowのSHA256。
- `ROLES`/`additions()`と完全一致する2 role/2 bindingの差分。
- 固定の`current-normal-gcloud-user` scope、owner例外・既知IAM writer変更静止・空Pool初回設定の各判断。
- 最長8時間のmanifest有効期間。実際の認証待ち・read・write・保存は、さらに呼出開始から1時間以内。

`--live-plan`は明示した既存user sessionによるfresh readとdry planだけを行います。
`--execute-owner-exception`は別途承認された差分の変更候補です。両modeは排他で、どちらもレビュー済み`--source-sha256`がなければ認証前に停止します。
実行modeではmanifestのowner例外・変更静止の判断も必要です。空Pool初回設定にはさらに`--accept-empty-pool-no-cas`とmanifest側の明示判断が必要です。
これらの値やflagがsourceに存在すること自体は、本人の承認を証明しません。

## 認証・先行検査・変更

CLIの現在のactive accountを静かに取得し、1個の通常user accountだけを選び、そのaccountを明示して短期tokenを取得します。
SA account、impersonation、ADC/credential/access-token/login-config overrideは拒否します。新login、key読出し、認証設定の書換えはありません。
stdout/stderrとtokenはメモリ内だけで扱います。account選択と必要permissionの実測を区別し、本人identityを外部から独立認証したとは表示しません。

変更前には以下をすべて完了します。

1. 固定integration branchの現HEADとreview済みrevisionが一致。同revisionのworkflow hashと、明示した関連gate閉鎖・read時刻の空欄を確認します。State用emitter等に一律falseは要求しません。
2. 固定の`ci.yml`について、activeな`workflow_dispatch`がないことを5状態で確認します。公開APIの403、不明・未完了の応答、現HEAD差異は停止します。active runを実IAM writerと断定せず、安全なscope確認ができない場合も停止します。
3. fixed dev project GETでID/number/ACTIVEを先に照合。Pool/ProviderがACTIVEなAWS federation、attestationが空かつ完了、Pool/SA policyが全fieldを保持したrequested version3であることを確認します。
4. actual tokenについてprojectのrole create/get、exact Poolとexact SAのpolicy get/set permissionを照会し、必要な全permissionが返ることを確認します。
5. 作成予定role ID 2件をexact GET。404以外は停止します。同名roleが同値でもpatch・再利用・ID自動採番をしません。
6. private dry planが固定の2 role/2 bindingだけであり、既存principal/condition/auditConfigを保持することを検査します。

検査完了後、role 2件を作成し、Pool/SA policyを各1回だけ更新します。各更新直前にも既知のGitHub gate/activityとexact policyを再確認します。
write試行前にjournalをfsyncします。HTTP409、timeout、失われた応答、未知shape等は不確実な結果として停止し、再送も自動rollbackも行いません。
policyの全snapshot復元やrole deleteを行う経路もありません。変更後は期待差分だけであることと、fresh Pool/Provider/attestationが不変であることを検査します。

## 空Pool policyとetagの限界

etagがあれば直前の値をそのまま使います。条件付きpolicyやSA policyで必要なetagが得られなければ停止します。
Poolのraw `{}` だけは、標準GA CLIの初回binding追加と同じく、etagなし初期設定の候補とします。
既知writerの変更静止、直前の2回のfull GET一致・空、1回のSET、事後の限定差分をすべて必要とします。

この分岐にatomic compare-and-swapの保証はありません。`updateMask`もCASではありません。
最後のGETからSETまでに未知writerがbindingを追加すると、その追加を消すraceは残り、事後diffでも未観測の消失を証明できません。
静的gate閉鎖、active runなし、2GET一致から全writer静止を推論しません。この限界を含めて独立reviewと本人の具体的例外判断が必要です。
etagを捏造したり、推測したIf-Matchを追加したりしません。

## 結果と途中失敗

結果は新規`yss-runtime-reader-bootstrap-*`フォルダー（0700）へ、各0600で保存します。
公開出力は固定status、mode、write試行数、常にfalseのmigration-ready、結果フォルダーだけです。
privateファイルは`journal.jsonl`、`offline-intent.json`、`preflight.json`、`dry-plan.json`、`write-1.json`〜`write-4.json`、`after-pool-policy.json`、`after-sa-policy.json`、`after-metadata.json`の必要なsubsetです。
raw policy/metadataはprivateファイルに保持しますが、token/error body/credential fileは保存しません。
公開artifactやrepositoryへコピーしません。

最大4 writeはtransactionではありません。STOP時もjournalを残し、確認済みのwriteと、適用の成否が不明な試行を区別します。
roleだけが残った状態を完了扱いにしません。復旧はjournalの独立review後に、fresh policyから今回追加したmemberだけを除去する別の判断です。
既存policy全体の復元や、新roleの自動delete/disableは行いません。

設定完了もCIのread成功・Provider継承・migration/import準備完了の証明ではありません。
設定後の既存plan/apply各Environmentによるprotected readと、exact対象の変更権限が増えていない確認は、担当者が別のレビュー済み有限windowで行います。

## Source検証と根拠

ダミーだけの検証は `python3 infra/gcp/tests/test_bootstrap_runtime_readers.py` で実行します。実認証やAPI接続はありません。
CIにも同じ検証を登録し、通常経路・衝突・誤対象・権限不足・期限切れ・partial write・error非公開・既存policy保持を確認します。

- [PoolへのViewer直接付与の公式例](https://docs.cloud.google.com/sdk/gcloud/reference/iam/workload-identity-pools/add-iam-policy-binding)
- [Policyとetag](https://docs.cloud.google.com/iam/docs/reference/rest/v1/Policy)
- [Pool setIamPolicyとupdateMask](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools/setIamPolicy)
- [custom roleの作成とID制約](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.roles/create)
- [custom role permission support](https://docs.cloud.google.com/iam/docs/custom-roles-permissions-support)
- [Pool GETの権限確認対象](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools/get)、[Provider GETの権限確認対象](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools.providers/get)

標準GA CLIのsourceでは、Poolのadd-bindingはversion3 GET→既存policyへのbinding追加→version3 SETです。
空policyを拒否したりetagを作り出したりする処理ではありません。このsource上の対応と、今回の対象での実適用成功は区別します。
