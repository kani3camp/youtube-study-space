# Phase 2A runtime WIF / API ownership 実行前 packet

Issue [#1191](https://github.com/kani3camp/youtube-study-space/issues/1191) の担当 workstream。
正本は [Terraform Canon](https://app.notion.com/p/3d3357a8d0ce81f589a3d6b9c8e237bc) と
[完了済み WIF migration 記録](https://app.notion.com/p/3d3357a8d0ce81ca9e0fe03749301deb)。

source preparation は development の existing runtime WIF と API ownership のみ。
2026-10-07 の fresh operator inventory 成功は親から受領済み。本文は operator の private 領域に保持し、
この worker は raw inventory を取得していない。live import-only plan / CI principal の read smoke / import は未実測。
production root に定義を追加しない。2026-10-10 の後続 source package は共通 authenticated workflow の無効な plan 選択と累積 validator を追加する。
今夜の daily batch 成功は計画上の仮定であり、自然実行の証跡ではない。

## 受領済み inventory と未確認事項

親の operator receipt では、下記6種類の exact read が既存権限で HTTP200。
pool/provider は ACTIVE な AWS federation、attestation は空・完了、trust と対象 SA の全 bindings/conditions は前回記録と一致。
SA policy は version 3 を要求して response version 1、enabled services は全 page 完了・73件。
private helper は全 API を `Investigate` として成功し、flags=false / selections=0。
これは operator の証跡であり、GitHub Actions の plan/apply identity の権限成功や runtime 自然実行の成功を示さない。

残る照合は、73件と以下の source 候補の private join、現行 AWS caller metadata との対応、CI exact read。
inventory を再取得するための broad list や新権限は不要。既存 receipt にない情報だけを親が不足として扱う。

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
  は存在する。operator の endpoint 成功は受領済みだが、permission 名と existing CI identity の実 read は未確認。
  broad pool list / viewer へ広げず、まず CI の同 exact endpoint の read を別確認する。
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

## Source に基づく API ownership 候補

以下は review 候補であり、73件の実名・state との照合結果ではない。
候補が fresh enabled inventory に存在し、consumer project と ownership 境界が一致したものだけ `Own` へ変更する。
未掲載 service は `Investigate` のまま。非 enabled / 別 project の API を enable して候補へ合わせない。
`owned_api_keys=[]` は分類後も維持し、採用は承認された API1件ずつの wave に分ける。
`dependency_addresses` は以下の resource prefix をそのまま流用せず、採用時の full-root にある exact address/alias に解決する。
default-off dependency は、その prerequisite ownership wave と current baseline を先に確認する。

| Service | 分類候補 | Explicit dependency / source evidence | 採用前の条件 |
| --- | --- | --- | --- |
| `firestore.googleapis.com` | Own | [`google_firestore_backup_schedule.daily`](../environments/dev/native-backup.tf)、[`FirestoreController`](../../../system/core/repository/firestore_controller.go) | dev database / backup の consumer project 一致 |
| `bigquery.googleapis.com` | Own | [`module.firestore_export_dataset.google_bigquery_dataset.retained`](../environments/dev/bigquery.tf)、[`BigqueryClient`](../../../system/core/mybigquery/bigquery_controller.go) | retained dataset/table ownership と同じ project |
| `storage.googleapis.com` | Own | [`module.backup_bucket.google_storage_bucket.retained`](../environments/dev/backup-bucket.tf)、[`StorageClient`](../../../system/core/mystorage/storage_controller.go) | retained bucket の API dependency。generated source bucket は採用しない |
| `pubsub.googleapis.com` | Own | [`module.export_topic[0].google_pubsub_topic.export`](../environments/dev/export-topic.tf) | existing topic ownership。generated subscription は対象外 |
| `cloudscheduler.googleapis.com` | Own | [`module.export_scheduler[0].google_cloud_scheduler_job.export`](../environments/dev/export-scheduler.tf) | existing export scheduler ownership |
| `cloudfunctions.googleapis.com` | Own | [`module.export_function[0].google_cloudfunctions_function.export`](../environments/dev/export-function.tf) | existing Gen1 metadata ownership。deploy/source/build は含めない |
| `monitoring.googleapis.com` | Own | [`module.notification_channels.google_monitoring_notification_channel.primary_email[0]`](../environments/dev/notification-channels.tf)、quota policy module | existing email ownership。quota alert の create gate は別契約のまま |
| `iam.googleapis.com` | Own | [`module.runtime_wif` の exact pool/provider/SA member](../modules/runtime-aws-wif/main.tf) | existing runtime federation metadata / grant ownership |
| `iamcredentials.googleapis.com` | Own | [`GoogleClientOption`](../../../system/internal/awsruntime/credential.go) の existing SA impersonation | 対象 SA project の既存 enabled と caller 照合。新 SA/permission は作らない |
| `sts.googleapis.com` | Own | [`WIF exchange mock`](../../../system/internal/awsruntime/credential_test.go) と existing AWS provider | audience の pool project の既存 enabled と一致 |

IAM / STS / Service Account Credentials は [公式 AWS WIF 手順](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-other-clouds)
でも依存 API とされる。この根拠は ownership 候補の判断だけに使い、手順にある enable / role grant は実行しない。

| Service / category | 分類候補 | 根拠と保留条件 |
| --- | --- | --- |
| `firebaserules.googleapis.com` | Platform/External | [`firebase/firebase.json`](../../../firebase/firebase.json) / [`Firebase CLI ownership`](../../../firebase/README.md) が rules/indexes を管理。[公式 rules API](https://firebase.google.com/docs/rules/manage-deploy) の既存 deploy 面を維持し、この module へ移管しない |
| `firebase.googleapis.com` | Platform/External 候補 | existing Firebase project 管理面に帰属することを private 分類で確認。Firestore client import だけではこの API の必要性を断定しない |
| `cloudbuild.googleapis.com`, `artifactregistry.googleapis.com` | Platform/External 候補 | [`Gen1 ownership 境界`](../modules/firestore-export-function/README.md) が generated build/artifacts を external とする。[公式 build 説明](https://docs.cloud.google.com/functions/docs/building) は platform dependency の根拠。実 project の用途が異なるなら Investigate |
| `cloudresourcemanager.googleapis.com`, `serviceusage.googleapis.com` | Investigate | pinned provider の project GET / enabled-services list に使う control plane。CLI/CI/bootstrap の使用だけで app ownership にしない。API管理者の境界判断が残る |
| `youtube.googleapis.com` | Investigate | [`YouTube client`](../../../system/core/youtubebot/live_chat.go) は Firestore 内の既存 OAuth client から別 token を得る。AWS WIF token の再利用ではない。channel/bot OAuth client の consumer project 一致だけを既存 private receipt で確認し、secret payload を取得しない |
| `run.googleapis.com`, `eventarc.googleapis.com` / その他 | Investigate | Gen1 source、inventory script の参照、enabled であることだけでは explicit app dependency を立証しない。MyPage を根拠に本 workstream で採用しない |

quota MQL 内の `serviceruntime.googleapis.com/quota/...` は metric namespace であり、API enablement/ownership の証拠ではない。
browser の [`Firestore client`](../../../youtube-monitor/src/lib/firestore.ts) も Firestore の caller evidence に留め、
Firebase Auth/Hosting API の必要性を推定しない。enabled API を disable する判断は本分類の成果に含めない。

## Runtime caller の source / private receipt 照合

[`CDK`](../../../aws-cdk/lib/aws-cdk-stack.ts) は共通3 env
`GOOGLE_CLOUD_PROJECT` / `GCP_WIF_AUDIENCE` / `GCP_WIF_SERVICE_ACCOUNT_EMAIL` を Lambda6本と daily-batch に渡す。
既存 [`CDK contract test`](../../../aws-cdk/test/aws-cdk.test.ts) がこの7 caller topology を検証する。

| Caller | Entry point / Google client |
| --- | --- |
| `sns_notify_discord` | [`main.go`](../../../system/cmd/lambda/sns_notify_discord/main.go): shared WIF → Firestore / workspace app |
| `set_desired_max_seats` | [`main.go`](../../../system/cmd/lambda/set_desired_max_seats/main.go): 同 shared WIF |
| `youtube_organize_database` | [`main.go`](../../../system/cmd/lambda/youtube_organize_database/main.go): 同 shared WIF |
| `check_live_stream_status` | [`main.go`](../../../system/cmd/lambda/check_live_stream_status/main.go): 同 shared WIF |
| `update_work_name_trend` | [`main.go`](../../../system/cmd/lambda/update_work_name_trend/main.go): 同 shared WIF。AWS Secrets Manager は GCP Secret Manager dependency ではない |
| `error_log_notify_discord` | [`main.go`](../../../system/cmd/lambda/error_log_notify_discord/main.go): 同 shared WIF |
| `DailyBatchTaskDefinition` の `daily-batch` | [`batch/main.go`](../../../system/cmd/batch/main.go): shared WIF、`transfer-bq` は同 client option を GCS / BigQuery に渡す |

source で特定できるのは caller topology と credential 経路まで。実 role 名を logical ID から生成しない。
親の既存 private AWS receipt で各 Lambda role / ECS **task role** と現行3 env を結び、provider condition と
`attribute.aws_role` mapping の実評価値、対象 SA の existing exact member/condition が一致することを確認する。
role ID が既存 receipt にある場合は同名 role の再作成有無の照合に使うが、新 trust 条件へ追加しない。
7 caller が7 distinct grants を要するとは仮定せず、existing role/member を重複排除して review alias を割り当てる。
追加/不明 caller、unused existing binding、条件不一致は STOP/Investigate。全 SA binding の機械的採用はしない。
ECS execution role、Scheduler/SFN control-plane role、`start_daily_batch` は shared WIF caller に含めない。
caller 照合を理由に Lambda invoke / ECS run-task / batch / export / YouTube API call を実行しない。

## CI exact-read smoke の最小 scope（未実行）

親が既存 protected CI route の plan/apply identity をそれぞれ使い、同じ private dev resource に対して以下を確認する。
operator token を CI principal の証拠へ転用せず、新 WIF provider / SA / role / binding は作らない。
common identity smoke/activation の実装変更は親 workstream が所有し、本 PR はその追加を行わない。

| Probe alias | GET request（値は private handoff） | 必須結果 |
| --- | --- | --- |
| `runtime-pool` | `iam.googleapis.com/v1/<exact-pool-name>` | HTTP200、ACTIVE、existing fields/trust 一致 |
| `runtime-attestation` | 同 exact pool の `:listAttestationRules` | HTTP200、known schema、空 rules、継続 tokenなし |
| `runtime-provider` | `iam.googleapis.com/v1/<exact-provider-name>` | HTTP200、ACTIVE/AWS、mapping/condition byte一致 |
| `runtime-sa-policy` | `iam.googleapis.com/v1/<exact-SA-resource>:getIamPolicy?options.requestedPolicyVersion=3` | HTTP200、全 unrelated bindings/conditions 不変、無条件 response version1 は可 |
| `runtime-project` | `cloudresourcemanager.googleapis.com/v1/projects/<dev-project-id>` | HTTP200、ID/number一致、DELETE_REQUESTED でない |
| `runtime-enabled-services` | `serviceusage.googleapis.com/v1/projects/<private-number>/services?filter=state:ENABLED&pageSize=200`、必要 page 継続 | 全 page HTTP200、完了、private inventory と候補 API の state 一致 |

6種類 × 2 CI principal（page があれば追加 GET）。API1件採用でも project GET / enabled list は必要。
[公式 v1 project GET](https://docs.cloud.google.com/resource-manager/reference/rest/v1/projects/get) と
[enabled service list](https://docs.cloud.google.com/service-usage/docs/reference/rest/v1/services/list) の scope を維持する。
pool/provider/SA/key list、project IAM policy、business data、source/archive/build reads、prod resource は不要。
mask済み alias / status / completion / metadata-match のみ報告し、HTTP body/error・token・URL実値・CEL・member は出力しない。
non-200 / malformed / incomplete / mismatch はその probe で STOPし、自動権限追加も成功値の補完もしない。

GET成功は mutation permission がない証拠ではない。親は既存 least-privilege 契約と role/binding review を別に維持し、
本採用用に WIF create/update/delete/undelete、SA setIamPolicy、API enable/disable を追加しない。
既存 quota normal-change gate 等の別承認権限も、この read smoke を根拠に拡大/転用しない。
attestation の permission 名が未確定でもまず既存 CI principal の exact read を確認し、失敗した exact endpoint だけ別承認へ返す。

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
API 名だけで判断せず、Firebase/Google-managed platform の暗黙 dependency は Own 候補化しない。
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
   無効な source 配線と累積 validator は下記の後続 package で用意した。実行 activation と receipts は未完了。
4. reviewed integration SHA の既存 protected full-root route を使う。pool1 → provider1 → exact member1ずつ → Own API1ずつ。
   前 wave の flags / selected aliases を保持。`-target` や別 trust/entrypoint は使わない。
   runtime/API の live adoption は sibling history wave の post full-root no-op12 実測を待つ。
   default-off source の integration review は並行可能。resource count は fresh baseline 実測値から計算し、11/12を先取りしない。
5. independent plan/apply Environment approval、same-SHA re-plan、sanitized projection 一致、global import-only guard を維持。
   pre-plan は exact expected imports + その他 managed resources no-op。create/update/delete/replace/drift/unknown/other0。
   saved import-only apply で state ownership のみ採用。API enable/disable や IAM/WIF config mutation は別 wave / 別承認。
6. post は import0 / full-root all no-op / drift0。GCP metadata / 全 SA bindings / enabled APIs 不変、
   S3 lineage / serial / version差分 / lock release を private 確認。公開はsanitized counts/outcomes/opaque commitmentsだけ、専用receipt以外artifact0・leak audit。
7. runtime regression は自然 Lambda / daily batch の既存 read-only証跡で確認。
   今夜成功の計画仮定を PASS 記録にしない。production は dev 完了後に fresh inventory から別設計。

## Rollback / 残 gate / 見積もり

default-off source 段階は source revert のみで cloud/state 変更なし。
import 後に flags/aliases を外すと destroy plan になるため外さない。
ownership 誤採用は apply を停止し、private current state / S3 version / native lock を確認して、
exact state-only ownership release を別承認で準備する。remote WIF/IAM/API を削除・disableしない。
後続 state write を古い S3 version で上書きしない。

operator inventory と default-off private preparation は親 receipt で完了。残 gate は API候補の private join / ownership判断、
AWS caller の現行 receipt 照合、CI principal の実 read smoke / 必要な exact read承認、
親による共通 CI activation / execution receipts、protected import と post no-op。
取得済み receipt を使う分類/caller照合と CI read smoke は概算30〜60分＋承認待ち、
isolated read-only plan / 親の共通 CI handoff は概算1〜2時間（read 成立と history post no-op が前提）。
protected adoption は各 wave 概算15〜30分＋人の approval/CI待ち。grant/API件数と read不足で変わる。
history source/validator/activation、D01削除、実 cloud IAM/auth/API/import/apply/prod変更は本 source PR の範囲外。


## 2026-10-10 後続 source package: protected 選択と累積 graph

開始 base は `feature/gcp-terraform-iac` の `77e3c24c6fb9d82f48a59c9c587a5c323b622ffb`。
open PR の確認時点では既存 module/helper が integration PR #1251 に含まれており、runtime配線の重複PRはない。
history は plan-verified のみで **import/apply・post full-root no-op12 は未完了**。
この package の dummy PASS はその prerequisite を完了にしない。

`ci.yml` の `terraform_ownership_wave` は `none` がdefault。
`pool` / `provider` / `grant` / `api` / `noop` はpublicのwave種別だけで、実member・project number・trustを受け取らない。
既存trusted repository/immutable IDs/integration ref/dispatch、plan Environment/OIDC、raw output抑制を保持する。
追加gate `DEV_HISTORY_POST_NOOP12_READY=false` / `DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED=false` は認証前STOP。
Terraform module/rootのruntime flagsと選択はfalse/emptyのまま。既存history/baselineのgateと検証契約は変更しない。
runtime selectionはdevelopment `plan` のみで、`apply`・security probe・exceptional wave・productionはgateを変えても拒否する。

private handoffは明示secret `GCP_RUNTIME_OWNERSHIP_PACKET_JSON` のみ。
このsource変更はsecret/Environment/権限を作成・設定しない。packetをstep envから読むoffline helperは、
認証前にRUNNER_TEMPの固定名 `runtime-ownership-packet.json` / `runtime-ownership.tfvars.json` を0600/exclusiveで作り、
checkout内配置・重複JSON・symlink・既存fileの上書きを拒否する。private valuesはGITHUB_ENV/OUTPUTへ出さない。
private tfvarsだけを通常full-root planの`-var-file`へ渡し、sanitizerが同じpacketから再導出して完全一致を要求する。
常時cleanupで両fileを消す。plan artifact/uploadや`-target`を追加しない。

PR #1274 のoffline legacy v1 schema（後続protected stagingでは拒否、実値はprivateに保持する）:

| Key | 必須値/役割 |
| --- | --- |
| `schema_version` | integer `1` |
| `git_sha`, `wave` | reviewed integration SHA40、dispatchと同じwave種別 |
| `reviewed_utc`, `expires_utc` | UTC `YYYY-MM-DDTHH:MM:SSZ`、review後24h以内かつ未失効 |
| `history_post_noop_run_id` | 正のinteger、実history import/post-noop12の独立review参照 |
| `inventory` | 上記helperのfresh REST inventory envelope、完全page・既存trust/member・分類を保持 |
| `adopted` | 前waveの実stateとpost-noopをreviewした累積selection |
| `selected` | 前selectionを保持し、今回の承認済み対象を1件だけ追加したselection |

両selectionのkeyは `pool` / `provider` (bool)、`grants` / `apis` (重複なしstring list) の4件のみ。
pool→provider→review済みgrantを1件ずつ→review済みOwn APIを1件ずつ。
API waveはinventoryでreview済みのrequired grant selectionをすべて先行採用した場合のみ許可し、候補を自動選択しない。
APIのdependency_addressesは今回のexact managed graphの既存addressへ解決済みであることを要求する。
`noop` は同じ累積selectionを保持する。10 source候補とenabled inventory73は承認selectionではなく、件数/実scopeをcodeで補完しない。
review日時・run ID・adopted宣言のshape検査は実provenance/authorizationの証明ではない。独立reviewと実receiptが別途必要。

`validate_runtime_ownership_plan.py` は既存history validatorへbaseline12を渡して **post/import0** を要求する。
そのvalidatorが既存11のexact identity/action/exclusionとhistory schema/order/descriptionを引き続き検証する。
全graphはduplicate/moved/deposed/data/extra resource、非no-op、wrong import ID、unknown、drift、deferred/provider action、
未完了plan/check、provider/schema version不一致、planned-values投影の不一致を拒否する。
runtime pool/providerのstate/name/project/trust/empty attestationとunsupported federation、個別SA role/member/condition、
APIの既存identityとdisable flagsをexact比較する。sensitivity-only updateも許可しない。
preは新規対象import1＋全resource no-op、postはimport0＋全resource no-op。`noop`はpre/postともimport0。
API enable/disable/recreate、whole-project IAM/SA policy/binding、service agent/key ownershipは許可しない。
Google 8.5.0の実provider schemaをcredentiallessに確認し、既存bucket schema4・その他対象schema0を固定する。
version更新時はこのvalidatorも独立reviewする。省略remote pool modeは既存helperのnull/emptyを保持し、既知federation-only表現のみ比較する。

### Activation要件（すべて未実行）

1. history actual importとpost full-root no-op12、released native lock、state/metadata不変のprivate receiptを独立review。
2. AWS caller/required grantとAPI Own scopeのprivate join、同SHAの完全inventory/selectionを独立review。
3. 既存plan/apply CI principalのexact-read成立と既存least privilegeを確認。新permission/auth/settingsは本packageで変更しない。
4. runtime graphに対応するfresh state/metadata/lockとfactual execution accounting sourceを独立reviewし、実receiptを取得。
   現history receiptはpre-import11専用のためruntime waveでは流用しない。runtime plan gateはこの対応が完了するまでfalseを維持する。
   新しいcost-only blocking machineryは追加しない。既存historyのaccounting/authorization契約はこのpackageでは維持する。
5. runtime applyのsame-SHA/private-input binding、independent plan/apply Environment approval、saved plan seal、
   exact state-only delta、全unrelated SA bindings/API enabled inventory/trust不変、post-noop、lock release/leak auditを実証跡で確認。
   disabled sourceはplan/applyの配線を備えるが、既存apply/emitter/runtime/history gateはfalseで実行を許可しない。
6. 最終dev full-root noopとintegration reviewの実証跡をそろえてから、dev-target #1251のmergeはユーザーが判断する。

検証はpublic dummyのみで、`test_runtime_ownership_package.py` が順序・累積件数・schema/action/identity/unknown/driftの拒否、
73/10でも明示API1件しか選ばれないこと、private staging/consumer/公開summary、閉じたworkflowと既存defaultの互換性を確認する。
`test_runtime_ownership.py` のisolated mock Terraform検証も維持する。ここで実plan/import/probeは行わない。


## Receipt provenance source（disabled、実 execution は未完了）

後続 source は [`terraform_ownership_receipt.py`](../../../.github/scripts/terraform_ownership_receipt.py) と
[`runtime_ownership_provenance.py`](../scripts/runtime_ownership_provenance.py) を使い、submitted run ID の型検査から
source-pinned GitHub run/attempt/job/Environment/receipt artifact の照合へ進める。history は plan-verified のままで、
actual import/post-noop12 は未完了。runtime/apply gates はfalse、追加 `DEV_HISTORY_RECEIPT_EMITTER_ENABLED=false`。
source の追加は execution approval や activation ready を意味しない。

protected staging はstrict v2のみ。v1は既存offline graph fixturesに残し、実provenanceとして昇格しない。
v2はv1から `history_post_noop_run_id` を除き、次の3 fieldsを加える。

| Field | Private contract |
| --- | --- |
| `receipt_binding` | `nonce` (private random32 bytesのhex64)、`backend` (account_id/bucket/key/region/workspace)、`history_metadata` (reviewed full table snapshot) |
| `history_receipt` | exact `run_id`, `attempt=1`, `job_id`, `source_sha`, `receipt_sha256`, `transport=artifact-v1`, `artifact_id`, `artifact_sha256` |
| `adoption_receipts` | ordered `reference`, `wave`, `selected`。actual pool→provider→individual grants→Own APIsのpost receiptsだけ。noopはadoption件数を増やさない |

すべてのreferenceをcanonical repoから独立にGETし、approved exact issuer source、CI caller/reusable workflow、
completed/successのinitial attempt、plan/apply job/critical steps/cleanup、actual approved Environment IDsと
reviewer IDsを照合する。probe/通常CI/pre-plan/コピーJSON/PASS proseはproofとして受理しない。
成功emitter/publish stepが作った専用artifactだけをcanonical GitHub APIから取り、digest/action/root/wave/counts/results/private scopeを比較する。
全jobs pagination、API errors、rerun/race、未承認source、欠落/重複/expired artifactはSTOP。step/job/run logsを取得するfallbackはない。
現在のconsumer run/SHA/packet/catalogにboundした0600/exclusiveのrecordを同じtrusted job内に作り、
cloud credentials前とsanitization直前にGitHubを再照合する。caller提供recordやcached approval booleanは受理しない。
このephemeral recordはsource-pinned jobが生成する内部handoffであり、外部attestation/signatureではない。

source trustは [`runtime-receipt-sources.json`](../../../.github/terraform/runtime-receipt-sources.json) のreviewed issuer catalog。
**現在は空**なのでactual receiptを一つも承認しない。issuer entryはexact SHA→`workflow_id`, `job_names` (plan/apply),
`environments` (plan/applyのid/name), `reviewer_ids`, `receipt_step`, ordered `required_steps`, approved `waves`。
GitHub metadata interfaceのdummy testで実env/reviewer IDsやactual sourceを発明しない。
workflow-run RESTのcaller `path`はexact `.github/workflows/ci.yml` または同pathの
`@feature/gcp-terraform-iac` 形を受理する。reusable `path`はexact repository/workflowの
`@feature/gcp-terraform-iac`、既存 `@refs/heads/feature/gcp-terraform-iac` または `@<exact issuer SHA>` 形のみで、
別field `ref` は常にexact `refs/heads/feature/gcp-terraform-iac`、`sha` はreviewed issuer SHAと一致必須。
任意suffixの除去やref推定はしない。[公式REST例](https://docs.github.com/en/rest/actions/workflow-runs?apiVersion=2026-03-10#get-a-workflow-run)
に加え、このrepositoryの既存成功runのactual `@SHA` 形と、同callerのexact schema-audit companion
(`gcp-user-activity-schema-audit.yml`) を扱う。companionは同じrepo/source/ref/suffix条件を必須とし、
authenticated reusableは必ず1件、companionは最大1件。欠落、重複、未知workflow、余分fieldはSTOP。
actual selective step-log GETは既存非秘密CI stepで404のため、receipt-only sanitized artifact transportへ移行する。
critical checkout/identity/apply/post-noop/emitter/publish/cleanup stepsは省略できず、history issuerでは既存history before/seal/metadata/postと両jobのearly exact AWS RoleId checkも必須。
actual protection policyのreviewer/bypass設定はowner確認が別途必要。APIに存在しないapproval timestamp/job.environment fieldへ依存しない。
API根拠は [workflow runs](https://docs.github.com/en/rest/actions/workflow-runs?apiVersion=2022-11-28) と
[workflow jobs](https://docs.github.com/en/rest/actions/workflow-jobs?apiVersion=2026-03-10) と
[artifacts](https://docs.github.com/en/rest/actions/artifacts?apiVersion=2026-03-10)。
GitHub tokenのActions readを既存callerとconsumer plan/apply jobにsource宣言し、write/PAT/new credentialは追加しない。
archive redirectでtokenを別hostへ送らず、GETはbounded/timeout/no retry。signed URL/error body/private valuesをpublicへ出さない。

artifactは `ownership-receipt-v1-<run>-1-<S>`、payloadはstrict public schema1のcanonical JSON＋改行だけ。
owned/exclusive `RUNNER_TEMP/ownership-public/ownership-receipt.json` を固定file pathでpinned upload Actionへ渡す。
private nonce/backend/state/metadata/packet/binary plan/log/credentialは含めず、directory/glob uploadは禁止する。
overwrite/hidden-filesはfalse、missing fileはerror、ZIP archive true、retention90日。既存Actions runtime-token uploadを使い、PAT/grant/new permissionは追加しない。
consumerはrun artifact1件、exact id/name/repo/head SHA/branch/size/digest/expiryとapply job内のcreation/update時刻を確認する。
attempt/jobの直接attestationはartifact APIにないため、attempt1・review済みissuer source内の唯一のapply uploader・成功publish/cleanup・両Environment approvalも必須。
ZIP取得は64 KiB以下、展開はせず固定regular file1件/16 KiB以下、path/symlink/directory/extra files/encryptionを拒否する。
archive SHA256とcanonical receipt SHA256を別々にhard検証し、artifact/list/runを最後に再読してraceを拒否する。
1 receiptのsingle-page正常経路はGitHub API GET10（うちZIP1）＋allowlisted signed redirect GET1、JSON1 MiB/response以下。
artifact expiry/deletionではSTOPし、永続保管やreceipt recoveryは別承認。同じimportを再実行しない。

public receiptは固定context/result/counts、prior receipt digest、opaque scope/state commitmentsだけ。
state commitmentは別HMAC domainで検証済みfull stateのlineage/serial/resources/outputsを結び、各waveのprior state commitmentも前receiptと一致させる。
GitHub provenanceの一致だけでは現在のstate一致を証明しない。runtime専用fresh state verifierがこのcommitmentとcurrent remote full-state readを照合する。
HMAC-SHA256のprivate nonceでbackend/history stable metadataとadopted runtime identity/trust/member/condition、required grants、selected Own dependenciesを結ぶ。
actual grant aliases/API inventory、account/provider/member/CEL、state/plan/schema/metadata、nonceとその無塩hashを公開しない。
nonceはauth credential/signing trustではなくprivate scope binding。helperの`new-nonce`は暗号学的乱数を
RUNNER_TEMPの固定private fileに0600/exclusiveで作る (valueをstdoutへ出さない)。chainで同じnonceを保持する。
nonce喪失/変更、alias入替え、同count別member/API、前scope削減/skip/batchはSTOP。actual scopeはprivateレビュー入力で決める。
API candidates10/enabled73をselectionに補完しない。

history issuerは別のstrict private envelope `{schema_version:1, receipt_binding}` を既存packet secretから読む。
plan/applyの両jobでformat/context/backend/nonceをcloud credentials前に検査し、fresh metadataとの一致をimport前にも検査する。
plan jobから渡すsanitized `history_receipt_scope` commitmentで両Environmentのprivate bindingを一致させる。実値やnonceをjob outputへ出さない。
既存state11→12/unrelated11不変/lineage/serial/metadata/native-lock/実applyとpost-plan outcome checksの後、
exact post/import0/no-op12 summaryも検査してからprivate machine recordを作る。
専用emit stepはpost成功＋closed issuance gateの条件だけで出力する。cleanup失敗/run失敗のreceiptはconsumerが拒否する。
public accountingは観測したapply/post-plan outcomesとpost verifierのexplicit readsだけ。
provider内部request数・workflow全体総数・実請求額はunknownとして残し、推計をactualにしない。新cost-only blockingはない。
初回history applyはemitter/publication gates両方true、exact trusted context、attempt1を認証前に必須とし、falseの経路ではimportしない。root emission後のcatalog admissionは別closure sourceでhistorical issuer SHAをpinし、catalog準備のためにimportを再実行しない。

sourceで閉じる範囲はissuer success emission、GitHub authenticity、private scope/ordered predecessor joins、protected mandatory wiringとdummy denial tests。
**未完了**: actual history import/post-noop12、real receipts、reviewed issuer pins/Environment policy、private caller/grant/Own approvals、
fresh cloud exact readsとruntime before/after state/metadata/lock/accountingの実receipt、最終full-rootdevnoop。
runtime receipt emitter/apply pathのdummy PASSは実adoptionの証明ではない。
issuer導入前に実history adoptionが終わった場合、元traceとfresh noop/state proofの別承認契約が必要で、blind import/applyを再実行しない。
rollbackは未有効化sourceのrevertとfalse gates維持。実adoption後にmodule flagsをfalseへ戻さない。

local testsはpublic dummy GitHub JSON/ZIPのinjected transportだけを使い、GitHub/AWS/GCPに接続しない。
[`gcp-receipt-artifact-fixture.yml`](../../../.github/workflows/gcp-receipt-artifact-fixture.yml) は同repo PRのopened/synchronize/reopenedだけで、
fixed dummy run101/source `aaaaaaaa...` のpublic schema1 receipt1件をfixture専用名で1日保持する。
Environment/cloud secrets/OIDCなし、checkout credential保存なし。upload後always cleanupし、既存GitHub read接続でbounded archive roundtripを確認する。
PR event/fixture名/dummy sourceはtrusted issuer契約を満たさず、ownership proofへ昇格できない。real publicationは別のowner execution approvalが必要。
実activation前はofficial interfaceへのactual metadata readsとsource/stateの独立reviewも必要。


## Runtime fresh-state / apply receipt package（disabled）

[`terraform_runtime_execution.py`](../../../.github/scripts/terraform_runtime_execution.py) は既存strict v2 packet、
provenance、canonical HMAC state domain、history/full-root graph guardsを使う。baseline11 readerは変更せず、
runtime専用readerでbaseline11 + exact canonical history12 + reviewed adopted graphを検証する。
lineage/serial/outputs/checks、provider/schema、resource containers/instances/dependencies/private fieldsを保持し、
最後のauthenticated receiptの `last_state_after_commitment` とfresh before full-stateを必ず一致させる。
countだけ、historical JSON、run ID、post-import plan-job summaryはexecution proofにならない。

| Phase | Required private evidence |
| --- | --- |
| before (plan/apply) | Exact HEAD→bounded GET→HEAD/version/ETag/length、native lock/workspace absence、full-state semantics、last historical commitment、fresh history + full WIF pool/provider/attestation/SA IAM policy/enabled API inventory |
| plan handoff | Same SHA/packet/nonce/backend/history inputs/runtime vars/existing private environment inputsのHMAC commitmentだけを既存job outputsで渡す。plan jobはstate/version・metadata不変と実init/plan/validation outcomesを確認 |
| apply preapply | 独立apply Environment approval後に同packetを再staging/authenticate。既存same-SHA full-root replan/projection equality、fresh state/version・metadataの再一致、exclusive0600 binary/input/summary seal |
| apply | 保存済planのsealを直前再確認。pool → provider → individual grant1件ずつ → explicit Own API1件ずつ。import-only/global mutation guardsを保持し、native lockを省略しない |
| post / emitter | import0/all-noop、fresh selected full-state、以前の各instance/container/outputs/checks不変、lineage/version不変・serial増加（既存state-only契約の `>`、+1と推測しない）、full metadata/inventory不変、lock/workspace absence。noop waveはfull state不変。実apply/post outcome成功とpost exit0の後だけmachine receiptを作る |

baseline/runtime の射影前に全 resource container の envelope・managed/google provider・非空 instances・
`(module,type,name)` の一意性と各 instance の schema/index/address を検証する。空の未知 container、
同一 grant container の分割、重複 instance は、flatten 後の件数や HMAC が一致しても STOP。
第二 grant/API の追加は既存の一意 container 内へ1 instanceを追加できるが、以前の header/membership は保持する。
emitter も保存済み before/post full-state と差分を再検証し、誤った旧 success record の matching commitment を発行根拠にしない。

fresh metadataは既存read interfaces（project GET、exact pool/provider GET、`:listAttestationRules`、
SA `getIamPolicy` requestedPolicyVersion3、enabled services list、canonical table GET）で取得する。
complete enabled inventoryはbounded pagination・重複/loop/unknown response拒否を使い、unrelated SA bindings/conditions・
all enabled services・WIF trustを比較する。既存runtime config/caller→required grants/API dependenciesはreviewed packetを使い、
そのreviewの代わりにcandidate数からscopeを決めない。CI exact reads/least privilegeの実成立はowner側の未完了条件。

before/preapply/postのobserved helper read countsと実outcomesはprivate evidenceに保持する。公開はcounts/outcomesとopaque
commitmentsだけで、runtime JSON/Markdown projectionにはgrant aliases/API addressesも含めない。provider内部requests/actual costはunknown。
post failure/expiryはscope-bound independent state/lock/workspace/history/runtime metadata safety readsを試行し、false successを発行しない。
owned0600/exclusive filesとexisting always cleanupを使い、public schemaのreceipt1件以外をartifact/cacheへ保存しない。cleanup/job/run失敗はconsumerが拒否する。
issuer catalogのexisting ordered step pinsはhistory/runtime conditional branchesを同source SHAに含められる。
選択されたwaveの全critical stepsと共通checkout/apply/noop/emitter/publish/cleanupがsuccessfulかつ順序一致であることが必須。

public receipt publication用 `DEV_OWNERSHIP_RECEIPT_ARTIFACT_ENABLED` をfalseで追加する。追加secret/PAT/serviceはない。既存 `DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED`, `DEV_HISTORY_POST_NOOP12_READY`,
`DEV_HISTORY_RECEIPT_EMITTER_ENABLED`, apply/history-postnoop/probe gatesはfalse、issuer catalogは空。
history independent post-noop selectorとruntime waveの混在は認証前に拒否する。
[`test_terraform_runtime_execution.py`](../../../.github/scripts/test_terraform_runtime_execution.py) はpublic dummy transport/full stateで
全waveのfreshness→saved seal→one-object delta→実emitter→実consumer、private input/nonce/receipt/state race、
same-count IAM/API/trust交換、metadata/network/lock/cleanup failure、公開漏出拒否を検査する。

historyは未import。ownerのexecution approval後の**初回history import**で既存history emitterのauthentic root receiptを取得し、
独立post-noop12も保持する。receipt導入前にadoption済みだった場合の別bootstrap契約はこのpackageに追加しない。
source integration/reviewはactivation承認ではなく、final issuer SHA/CI、actual workflow/job/Environment/reviewer pinsとprotection/artifact
interface、private nonce/backend/history binding、actual scope/CI exact reads、実history import/postnoop12・ordered runtime receipts・
final dev noop/owner merge判断が必要。既存security probe証拠は再利用し、追加live probe/query/DDL/deployを自動要求しない。
