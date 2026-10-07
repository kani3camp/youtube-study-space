# Phase 2A runtime WIF / API ownership 実行前 packet

Issue [#1191](https://github.com/kani3camp/youtube-study-space/issues/1191) の担当 workstream。
正本は [Terraform Canon](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc) と
[完了済み WIF migration 記録](https://app.notion.com/p/3d3357a8d0ce81ca9e0fe03749301deb)。

source preparation は development の existing runtime WIF と API ownership のみ。
fresh inventory / live import-only plan / CI least-privilege smoke / import は未実測。
production root に定義を追加せず、共通 authenticated workflow / activation / validators は変更しない。
今夜の daily batch 成功は計画上の仮定であり、自然実行の証跡ではない。

## 最小 read-only inventory を一度に取得する

対象は development `test-youtube-study-space` のみ。正規 read-only connector / 既存 operator 権限で取得し、
public Issue / PR / log / artifact へ本文を出さない。permission がなければその exact read を別承認へ返し、
自動で role / binding を追加しない。

| Read | Exact resource / fields | Permission candidate |
| --- | --- | --- |
| pool GET | `projects/<private-number>/locations/global/workloadIdentityPools/aws-runtime`: name/state/displayName/description/disabled/mode、unexpected trust fields | `iam.workloadIdentityPools.get` |
| pool attestation read | 同 exact pool の `:listAttestationRules`。既存 rules が空であることを確認 | exact endpoint の permission 名は fresh確認が必要（未確定・未付与） |
| provider GET | 同 pool の `providers/aws-provider`: name/state/displayName/description/disabled/aws.accountId/attributeMapping/attributeCondition、他 federation type の有無 | `iam.workloadIdentityPoolProviders.get` |
| SA getIamPolicy | runtime config と一致する default App Engine SA 1件。`options.requestedPolicyVersion=3`、exact role/member/condition と unrelated bindings を private で照合 | `iam.serviceAccounts.getIamPolicy` |
| Service Usage list | `projects/<private-number>/services?filter=state:ENABLED` の全 page。name/config.name/state のみ | `serviceusage.services.list` |
| project metadata | exact development project ID/number（fresh inventory が既存なら重複取得不要） | `resourcemanager.projects.get` |

pool/provider list、SA/key list、project IAM policy read、business data、secret payload は不要。
runtime audience / impersonation SA は source と現行 runtime metadata の一致を親 workstream の既存証跡で確認する。
この一致を確認するためだけに Lambda invoke / ECS task / daily batch / export を起動しない。
SA identity GET はこの provider の IAM member Read には不要。

## Pinned provider の実 Read / destruction semantics

各 root の provider lock を維持し、latest provider から権限を推測しない。

- Pool Read は exact GET **と同 pool の `:listAttestationRules` GET** を無条件で実行する。provider Read は exact provider GET。
  historical inventory の pool GET-only をそのまま least-privilege 根拠にしない。
  [pool source](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/iambeta/resource_iam_workload_identity_pool.go)、
  [provider source](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/iambeta/resource_iam_workload_identity_pool_provider.go)。
  [公式 attestation read](https://docs.cloud.google.com/sdk/gcloud/reference/iam/workload-identity-pools/list-attestation-rules)
  は存在するが、使用 permission 名と existing CI identity の実 read は未実測。
  broad pool list / viewer へ広げず、その exact endpoint の read 要件を親が一度確認する。
- IAM member import / Read は対象 SA の `getIamPolicy` を policy version 3 で取得。
  import は `service_account_id role member [condition title]`。
  同 member / role / condition title に複数 binding がある場合は STOP。
  [SA updater](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/resourcemanager/iam_service_account.go)、
  [member importer / Read](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/tpgiamresource/resource_iam_member.go)。
- `google_project_service` Read は Resource Manager project GET + batched enabled-services list。
  exact service GET-only ではない。API1件ずつの import は可能でも、CI read は development project scope の list が必要。
  `disable_on_destroy=false` は provider Delete で API disable を skip する。依存 API disable も false。
  [service source](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/resourcemanager/resource_google_project_service.go)、
  [batch read source](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/resourcemanager/serviceusage_batching.go)。

公式 [pool GET](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools/get)、
[provider GET](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools.providers/get)、
[SA policy read](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.serviceAccounts/getIamPolicy)、
[Service Usage access control](https://docs.cloud.google.com/service-usage/docs/access-control) も照合する。
実 IAM custom role / 条件付き resource scope の成立と実 principal の権限は、実行前に separately verify する。
import-only の apply identity に WIF create/update/delete、SA setIamPolicy、API enable/disable を付けない。

## Private offline preparation

`scripts/prepare_runtime_ownership.py` は cloud client を持たず、inventory を読み default-off tfvars JSON を生成する。
input/output は checkout 外の private directory、input mode 0600。output は 0600 / exclusive create、symlink・上書き・checkout 内出力を拒否する。
stdout/stderr に identity 値は出さない。入力の capture 時刻・完全性・scope の証明と caller/ownership review は operator の責任。

入力 envelope:

| Key | 内容 |
| --- | --- |
| `project` | fresh `projectId` / string `projectNumber` |
| `pool`, `provider` | 上記 exact GET の REST metadata（CEL は byte-for-byte） |
| `pool_attestation_rules` | 成功した同 exact pool の attestation GET の JSON body。下記の空 response 契約のみ採用 |
| `iam_policy_resource` | policy を read した exact SA resource name。新 identity を生成しない |
| `runtime_config` | `audience` / `service_account_email`、source・現行 runtime と一致確認済み |
| `iam_policy` | version 3 を要求した policy response。無条件 policy の response version 1 は可 |
| `runtime_grants` | `grant-01` 等の review alias → exact `member` / optional `condition_title`。1件ずつ選別 |
| `enabled_services` | 全 page を結合した `{services: [...]}`、残る `nextPageToken` は拒否 |
| `api_classification` | service → `classification` / `dependency_addresses` list / `reason`。全 enabled service を分類 |

分類は `Own` / `Platform/External` / `Investigate` / `Do not own`。
`Own` は Study Space が所有する Terraform resource の explicit dependency / caller evidence があり、既存 enabled を無変更採用できる service のみ。
API 名だけで判断せず、Firebase/Google-managed platform の暗黙 dependency は候補化しない。
候補の一覧が出ても `owned_api_keys` は空のまま。grant 候補も `runtime_wif_grant_keys` は空のまま。
テスト内の invented number / role / condition は実 environment input として使わない。
pool `mode` が remote で省略されている場合は `null` のまま provider-computed とし、`FEDERATION_ONLY` を補完しない。

attestation の [公式 IAM v1 Discovery schema](https://iam.googleapis.com/$discovery/rest?version=v1)
`ListAttestationRulesResponse` は `attestationRules`（array）/ `nextPageToken`（string）の2 optional fields のみ。
成功 response の `{}`、`{"attestationRules": []}`、省略または空 string の `nextPageToken` は空・完了を表せる。
HTTP成功・exact request scope・fresh provenance は capture 時に確認する。未取得 response を `{}` に置き換えない。
helper は `error` envelope / 未知 key / null / 型違い / 非空 rules / 継続 token を fail-closed で拒否し、
candidate を出力しない。未知 response を「rulesなし」として補完しない。

```bash
python3 infra/gcp/scripts/prepare_runtime_ownership.py \
  /private/staging/runtime-inventory.json /private/staging/runtime-candidate.tfvars.json
python3 infra/gcp/tests/test_runtime_ownership.py
```

後者は dummy inventory / mock provider / backend-disabled temporary roots のみ。
target project label もテスト用 scratch root / CLI module 内だけ架空値へ置き換え、実 identity を fixture に記載しない。
default graph0、pool/provider/個別 grant の順序、条件の保持、wrong project / broad member / unrelated role / incomplete page の拒否、
API選別・destruction flags、global import-only guard が mock create を拒否することを検証する。
これを live no-op / runtime smoke / permission smoke と扱わない。

## 承認後の ownership wave と acceptance

1. private fresh inventory、API分類、対象 resource/member と現行実 principal の read permissions をレビュー。
   missing read は existing dev read role/principals/bindings を維持する exact permission 追加案として別承認。
   broad predefined role・mutation・production grant は追加しない。
2. pinned provider / isolated local-backend read-only import plan で remote fidelity を確認。
   raw plan / private tfvars は一時 private 領域だけ、credential/token/state は public 出力しない。
   unknown、sensitivity-only update、表現 drift も通常 guard を緩めず STOP。
3. 親が共通 CI の read smoke・private input handoff・必要な cumulative resource validator と activation をレビュー。
   この PR だけで既存 authenticated CI へ候補は供給されない。
4. reviewed integration SHA の既存 protected full-root route を使う。pool1 → provider1 → exact member1ずつ → Own API1ずつ。
   前 wave の flags / selected aliases を保持。`-target` や別 trust/entrypoint は使わない。
   fresh baseline に履歴 table が追加済みなら resource count をその実測値から計算し、過去の11を固定しない。
5. independent plan/apply Environment approval、same-SHA re-plan、sanitized projection 一致、global import-only guard を維持。
   pre-plan は exact expected imports + その他 managed resources no-op。create/update/delete/replace/drift/unknown/other0。
   saved import-only apply で state ownership のみ採用。API enable/disable や IAM/WIF config mutation は別 wave / 別承認。
6. post は import0 / full-root all no-op / drift0。GCP metadata / 全 SA bindings / enabled APIs 不変、
   S3 lineage / serial / version差分 / lock release を private 確認。公開は address/actions/counts のみ、artifact0・leak audit。
7. runtime regression は自然 Lambda / daily batch の既存 read-only証跡で確認。
   今夜成功の計画仮定を PASS 記録にしない。production は dev 完了後に fresh inventory から別設計。

## Rollback / 残 gate / 見積もり

default-off source 段階は source revert のみで cloud/state 変更なし。
import 後に flags/aliases を外すと destroy plan になるため外さない。
ownership 誤採用は apply を停止し、private current state / S3 version / native lock を確認して、
exact state-only ownership release を別承認で準備する。remote WIF/IAM/API を削除・disableしない。
後続 state write を古い S3 version で上書きしない。

残 gate は fresh private inventory、API ownership判断、実 read smoke / 最小権限承認、
親による共通 CI activation / cumulative validator、protected import と post no-op。
inventory/分類/isolated read-only plan は権限と証跡が揃ってから概算1〜2時間、
protected adoption は各 wave 概算15〜30分＋人の approval/CI待ち。grant/API件数と read不足で変わる。
history source/validator/activation、D01削除、実 cloud IAM/auth/API/import/apply/prod変更は本 source PR の範囲外。
