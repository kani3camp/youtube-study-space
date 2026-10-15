# Phase 2A runtime WIF / API ownership 実行前 packet

## 2026-10-15 JST protected current-state metadata検証

[run38606660109](https://github.com/kani3camp/youtube-study-space/actions/runs/38606660109)
（source `24093a4e9c8dfab26aeb36b7380eb8dba5ba0219`、attempt1、17:46 UTC完了）は
本人による `terraform-dev-plan` Environment承認後、専用metadata readerでPASS。
State12件・history canonical8列・State bytes/S3 HEAD metadataと安定したtable/project metadataの
前後不変、lock/workspaceのpositive LIST不存在、private cleanupを検証しました。
State読取は各HEADで4MiB上限を強制しています。Terraform実行・State/lock書込・artifact発行は0。
公開logのcredential/raw-State/raw-Plan/signed-URL/未mask identity対象patternの検出は0でした。

これはfull-root no-op、writer quiescence、runtime六種metadata、original history deltaの証明では
ありません。`historical_delta_attested=false`を維持します。完了済みcanonical tableを再DROP・再importしません。
重複して準備したread run38612550915は承認前に取消し、read gateをfalseへ閉鎖しました。
次は同一環境でfresh admissionに基づく独立protected no-op/issuer closureです。

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

## Codex Cloud の調査用 read-only 認証経路

private selectors と trust の正本は親ページ
[インフラ改善](https://app.notion.com/p/3d3357a8d0ce8191a8b1d8876b6840a9) の
「Codex Cloud 2026-10-12」節。対象 role ARN / account / Service Account は同 private 正本に保持し、
この repository や公開ログへ複製しない。既存 Personal Vault の AWS keys は State account の source IAM user 用で、
調査では次の2経路を使う。

| 対象 | 既存の認証経路 | 調査範囲 |
| --- | --- | --- |
| AWS development / production metadata | 元の source IAM user → private 正本にある各環境の exact ReadOnlyRole → 900秒の scoped STS session | 対象環境の既存 Lambda / ECS 等の必要な metadata read |
| GCP development / production metadata | 元の source IAM user → 専用 `codex-cloud/aws-state-reader` WIF → private 正本の対象環境専用 SA impersonation | 既存 metadataReader が許可する project / enabled services / SA / bucket / dataset / topic metadata read |

### 再利用手順と identity の境界

1. Codex Cloud の credential readiness と元の source STS identity を確認し、private 正本の account / ARN / UserId に照合する。
   source account が development / production workload account と異なることは、AssumeRole 前の期待された構成である。
   この時点の account 差だけで keys を無効と判定しない。公開出力は照合結果だけとし、identity 本文を出さない。
2. AWS 調査は private 正本で指定された exact 2 ReadOnlyRole から環境に対応する1件を選び、
   元の keys を source に `DurationSeconds=900` で AssumeRole する。
   session credentials はその read command / isolated child process にだけ渡し、元の environment と global credential chain を保持する。
   AssumeRole 後に対象 account / exact role を private 照合してから、必要な read を行う。
   別 role の推測、trust の変更、global AWS profile / credentials の上書きは行わない。
3. GCP 調査は元の source IAM user の environment を使い、repository 外の owned temp に
   external-account config と SA impersonation の設定を0600で作る。
   専用 WIF の trust は元 IAM user の account / ARN / UserId の3点を明示的に要求する。
   AWS 調査用 AssumeRole 後の session をこの WIF へ渡すと source が変わるため、GCP の child process は元の environment から分離して起動する。
   ADC / gcloud config はその child process の temp だけに指定し、既存 global 設定を保持する。temp は失敗時も削除する。
4. token / session credentials / private configs / API body / error body を公開ログや artifact へ出さない。
   status、pagination completion、identity-match 等の固定結果を記録し、認証成功と各 endpoint の権限成功を分けて判断する。

AWS Core connector はこの Codex Cloud environment keys と別の接続である。
connector が再認証を要求した場合はその connector の接続状態を扱い、Codex Cloud keys の失効証拠へ転用しない。
既存 keys とこの read-only 経路は調査専用であり、State write / lock create / resource 設定変更 / protected apply には使用しない。
実 migration は repository の Actions OIDC / GCP WIF、専用 plan/apply identity、GitHub Environment の本人承認を通す。

### 2026-10-15 JST の fresh 調査結果と残る metadata gap

両環境で AWS ReadOnlyRole と専用 GCP WIF / SA impersonation の fresh AUTH は PASS。
同日取得の endpoint 結果は次の通り。ARN / account / SA / role metadata / response 本文は private に保持する。

| Exact read | Development | Production | Endpoint 契約 |
| --- | --- | --- | --- |
| Project metadata | PASS | PASS | canonical `cloudresourcemanager.googleapis.com/v1/projects/<exact-project-id>` GET |
| Enabled services | 全page完了・73件 | 全page完了・65件 | `serviceusage.googleapis.com/v1/projects/<private-number>/services?filter=state:ENABLED` GET。pagination を最後まで取得 |
| Runtime Pool | HTTP403 | HTTP403 | `iam.googleapis.com/v1/<exact-runtime-pool-name>` GET |
| Runtime Provider | HTTP403 | HTTP403 | 同 Pool の exact `providers/aws-provider` GET |
| Pool attestation rules | HTTP403 | HTTP403 | 同 exact Pool の `:listAttestationRules` GET。未取得 response を空 rules としない |
| Runtime SA IAM policy | HTTP403 | HTTP403 | exact SA の `:getIamPolicy?options.requestedPolicyVersion=3`、空 body の POST（read）。GET へ置換しない |
| History table metadata | HTTP403 | HTTP403 | `bigquery.googleapis.com/bigquery/v2/projects/<exact-project>/datasets/firestore_export/tables/user-activity-history` GET。row query / data read はしない |
| ECS task definition | `DescribeTaskDefinition` AccessDenied | `DescribeTaskDefinition` AccessDenied | 既存 exact task-definition を指定。task role と execution role を区別 |

各環境で Lambda6本の actual execution role を取得し、6 distinct roles であることを確認した。
3 GCP environment（project / audience / SA）の一貫性を確認し、runtime trust / IAM member との照合は未完了。
ECS task-role metadata は未取得のため、full7caller map は未完成である。
この operator 経路の結果は CI plan/apply identity 各6種の read smoke や workload の自然実行回帰を完了にしない。
metadata gap は失敗した exact endpoint / principal / scope を private に切り分け、既存 least-privilege 設定と pinned provider を照合する。
permission 名が未検証なら推測せず、broad role / API enable / mutation grant で埋めない。
AUTH PASS を migration Ready や Terraform import の完了として扱わず、fresh private inventory / caller join / protected wave の残条件を維持する。

## 最小 read-only inventory を一度に取得する

対象は development `test-youtube-study-space` のみ。正規 read-only connector / 既存 operator 権限で取得し、
public Issue / PR / log / artifact へ本文を出さない。permission がなければその exact read を別承認へ返し、
自動で role / binding を追加しない。

| Read | Exact resource / fields | Permission candidate |
| --- | --- | --- |
| pool GET | `projects/<private-number>/locations/global/workloadIdentityPools/aws-runtime`: name/state/displayName/description/disabled/mode、unexpected trust fields | `iam.workloadIdentityPools.get` |
| pool attestation read | 同 exact pool の `:listAttestationRules`。既存 rules が空であることを確認 | `iam.workloadIdentityPools.getAttestationRules`（CI の実 read・付与状況は未確認） |
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
  は存在する。operator の endpoint 成功は受領済みだが、existing CI identity の実 read は未確認。
  [公式 IAM audit logging](https://docs.cloud.google.com/iam/docs/audit-logging#ListAttestationRules)
  は pool の read に `iam.workloadIdentityPools.getAttestationRules` を記載する。
  managed identity 用の `iam.workloadIdentityPoolManagedIdentities.getAttestationRules` は今回の pool endpoint の対象外。
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
Own API の根拠は API 以外の既存 managed consumer address とする。
当該 API 自身や別 API の `google_project_service` ownership address は consumer evidence ではないため、
認証前 packet preparation と protected full-root graph の両方で拒否する。
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

## CI exact-read smoke の最小 scope（初回STOP・未完了）

親が既存 protected CI route の plan/apply identity をそれぞれ使い、同じ private dev resource に対して以下を確認する。
operator token を CI principal の証拠へ転用せず、新 WIF provider / SA / role / binding は作らない。
common identity smoke/activation の実装変更は親 workstream が所有し、本 PR はその追加を行わない。

| Probe alias | Read request（値は private handoff） | 必須結果 |
| --- | --- | --- |
| `runtime-pool` | `iam.googleapis.com/v1/<exact-pool-name>` | HTTP200、ACTIVE、existing fields/trust 一致 |
| `runtime-attestation` | 同 exact pool の `:listAttestationRules` | HTTP200、known schema、空 rules、継続 tokenなし |
| `runtime-provider` | `iam.googleapis.com/v1/<exact-provider-name>` | HTTP200、ACTIVE/AWS、mapping/condition byte一致 |
| `runtime-sa-policy` | `iam.googleapis.com/v1/<exact-SA-resource>:getIamPolicy?options.requestedPolicyVersion=3`、空 body POST | HTTP200、全 unrelated bindings/conditions 不変、無条件 response version1 は可 |
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
permission 名の文書確認は実 principal の成功を証明しない。まず既存 CI principal の exact read を確認し、
不足時は既存 read role / principals / bindings を維持する最小 permission 差分を最新 Current Canon の承認範囲に照合する。

### Protected runtime metadata read の source route（本8時間窓の診断準備・actual未実行）

`ci.yml` の既存 `workflow_dispatch` で `terraform_authenticated=true` / `terraform_target=dev` /
`terraform_mode=runtime-metadata-read` を指定する専用 source route を用意する。
`current-state-read`、Terraform plan/apply、runtime ownership waveとは独立する。
`DEV_RUNTIME_METADATA_READ_ENABLED=true` とし、UTC `2026-10-15T04:32:00Z` から
`2026-10-15T12:32:00Z` の独立した8時間窓でreview済み診断を実測できるよう準備する。actualは未実行。
このactivationの独立review・required CI・integration統合後のexact SHAから新規attempt1をdispatchし、
本人が既存両Environmentをそれぞれ承認してから観測する。Environment承認待ちでwindowを延長せず、
各jobの認証前・各read前・完了時にも再確認する。実測後は別source変更でread gate=false・時刻空へ閉鎖する。

2個の明示jobは既存 `terraform-dev-plan` / `terraform-dev-apply` Environmentをそれぞれ使い、
各Environmentの既存WIF provider / exact Terraform SA secretで独立に短期tokenを取得する。
同じrepo/immutable owner/ref/initial attempt、caller・reusable workflowのsame SHAとexact Environment OIDCを検証する。
apply Environmentのこのjobはmetadata read専用で、通常apply jobやそのgateへ流入しない。
新secret、AWS認証、State/lockのread/write、Terraform実行、artifact、ownership receiptは使わない。

上の6種類を各identityで前後2回readし、全JSON field、CEL文字列、unrelated IAM binding/condition、
project scope、完了した全enabled-servicesをprivate memory内で比較する。
known REST envelope、existing offline shape、空attestation、20 page/4000 service/4 MiBのsnapshot上限を検証し、
unknown/error/incomplete/duplicate/範囲違い/変化はSTOPする。redirectを追わず、error bodyを出力しない。
metadata本文やtokenをdiskへ書かず、公開SummaryとActions stdoutは同じ固定aliasとPASS/STOP/未実行を出す。
各metadata aliasのbefore/afterとproject permission readは、固定reason aliasを併記する。
reasonは `PASS` / `未実行` / `HTTP403` / `HTTP_OTHER` / `RESPONSE_REJECTED` / `TRANSPORT_FAILED` / `ADMISSION_STOP` / `BOUNDARY_REJECTED` だけを許可し、
公開sinkで全keyと値をallowlist検証する。HTTP403は当該HTTP応答を実観測した場合だけ、HTTP_OTHERは他のnon-200、
RESPONSE_REJECTEDはHTTP200のJSON/shape等の拒否、TRANSPORT_FAILEDはHTTP status未確定のdependency失敗、
ADMISSION_STOPはscope/window/byte budgetの拒否、BOUNDARY_REJECTEDは有効なHTTP200 permission応答が禁止権限を含む境界違反を表す。
raw HTTP status、body、URL、exception textは出さず、
有限reasonも権限追加が必要という証拠へ読み替えない。Summaryへの書き込み失敗時もstdoutに固定結果を残し、全体結果はSTOPにする。
project GETを先行し、dev project ID/number不一致なら他endpointへ進まない。照合後は各interfaceを独立に検査し、
1件の403等で他の実成功を失敗へ書き換えない。各aliasのPASSは前後両read成功だけを表し、片方の失敗を他方で補完しない。
全6種成功・全field前後一致・project-scoped permission確認・window内完了の全条件以外は全体STOPを維持する。

既存のruntime least-privilege契約に合わせ、development projectの `testIamPermissions` で
WIF pool/provider create/update/delete/undelete、SA setIamPolicy、API enable/disableが空responseであることも確認する。
これはproject scopeの観測であり、conditional/resource-specific grant全体やsource/private caller joinを証明しない。
read成功や前後一致をimport承認、fresh inventoryの保存、full-root no-op、historical receipt、Infrastructure Readyに読み替えない。
non-200や不足permissionでは停止し、grantやroleを自動で追加しない。

### 2026-10-15 JST runtime metadata read の初回実測とwindow閉鎖

[run 38663394483](https://github.com/kani3camp/youtube-study-space/actions/runs/38663394483) は
integration commit `ede97cb79734c3ba52b872f9fe163c2bcc7cb531` の専用metadata routeで実行した。
本人が既存 `terraform-dev-plan` / `terraform-dev-apply` の両Environmentを承認し、
両jobのexact Environment/same-SHA OIDC検証とGCP短期認証はSUCCESS、metadata observe stepはSTOP、
always cleanupはSUCCESSで完了した。認証成功をmetadata成功へ読み替えない。

当該sourceは個別固定aliasの結果をStep Summaryだけへ書き、Actions logには全体STOPだけを出していた。
API/connector経路では個別Summaryを取得できなかったが、本人が両jobの固定Summaryを共有した。
同じfirst attemptで得られた結果は両identityとも以下である。

| Fixed alias | Plan | Apply |
| --- | --- | --- |
| `runtime-pool` | STOP | STOP |
| `runtime-attestation` | STOP | STOP |
| `runtime-provider` | STOP | STOP |
| `runtime-sa-policy` | STOP | STOP |
| `runtime-project` | PASS | PASS |
| `runtime-enabled-services` | PASS | PASS |
| `runtime-stable` | 未実行 | 未実行 |
| `runtime-project-permissions` | PASS | PASS |
| `runtime-window` | PASS | PASS |
| `runtime-cleanup` | PASS | PASS |

当該sourceは失敗reasonを保持しておらず、この記録からHTTP403、HTTP200のshape拒否、その他の原因を判別できない。
後続sourceの有限reasonを過去runへ遡って補完せず、不足permissionや原因を推定しない。
後続sourceは同じ固定結果と有限reasonをstdoutにも出し、body/error/URL/identity/CEL/raw HTTP statusは出さない。

使用済みの `DEV_RUNTIME_METADATA_READ_ENABLED` はfalse、発行・失効UTCは空へ閉鎖する。
既存Environment approvals、required reviewers、trusted ref/immutable owner/same-SHA OIDC、plan/apply分離、
runtime ownership/apply/productionの閉じたgateと既存least-privilege構成を保持し、grant/secretを追加しない。
Terraform/State/lock、runtime mutation、artifact、ownership receiptへ進まない。
共有された当該runの固定結果は追加readなしで保存する。失敗分類の追加実測が必要なら、
別reviewで新しい最大8時間のsource windowを設定し、本人の両Environment承認を経て新規runを行う。
過去runの再実行やwindowの延長で代用しない。

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
このpackage作成時点ではhistoryはplan-verifiedのみだった。2026-10-13時点ではimportと独立full-root no-op12は成功したが、original final receiptは失敗しており、下記current-state bootstrapを別契約として扱う。
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
| `selected` | 前selectionを保持し、採用waveは承認済み対象を1件追加、`noop` は追加0件のselection |

両selectionのkeyは `pool` / `provider` (bool)、`grants` / `apis` (重複なしstring list) の4件のみ。
pool→provider→review済みgrantを1件ずつ→review済みOwn APIを1件ずつ。
API waveはinventoryでreview済みのrequired grant selectionをすべて先行採用した場合のみ許可し、候補を自動選択しない。
APIのdependency_addressesは今回のexact managed graphの既存addressへ解決済みであることを要求する。
`noop` は空でない同じ累積selectionを保持する。poolのみ、pool＋provider、grantの部分採用、全required grant、API採用後の各段階で使える。
providerや全required grantを`noop`のために追加しない。API採用の全required grant prerequisiteは維持する。
10 source候補とenabled inventory73は承認selectionではなく、件数/実scopeをcodeで補完しない。
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
source-pinned GitHub run/attempt/job/Environment/receipt artifact の照合へ進める。historyのactual importと独立post-noop12は後に成功したが、original final receiptは未取得。
runtime plan/root-ready gatesはfalse。元history emitter/publication設定はそのまま保持し、新bootstrap emission/root-ready gatesもfalse。
source の追加は execution approval や activation ready を意味しない。

original-history-importのprotected stagingはstrict v2のみ。別のcurrent-state-bootstrapは下記strict v3を使用する。v1は既存offline graph fixturesに残し、実provenanceとして昇格しない。
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
**未完了**: original final receipt、別current-state rootの実発行/受理、ordered real runtime receipts、reviewed issuer pins/Environment policy、private caller/grant/Own approvals、
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
| post / emitter | import0/all-noop、fresh selected full-state、以前の各instance/container/outputs/checks不変、lineage/version不変・serial増加（既存state-only契約の `>`、+1と推測しない）、full metadata/inventory不変、lock/workspace absence。noop waveはraw state bytesとS3 HEAD応答（VersionId/ETag/length/metadata等）がbeforeと完全一致。実apply/post outcome成功とpost exit0の後だけmachine receiptを作る |

baseline/runtime の射影前に全 resource container の envelope・managed/google provider・非空 instances・
`(module,type,name)` の一意性と各 instance の schema/index/address を検証する。空の未知 container、
同一 grant container の分割、重複 instance は、flatten 後の件数や HMAC が一致しても STOP。
第二 grant/API の追加は既存の一意 container 内へ1 instanceを追加できるが、以前の header/membership は保持する。
emitter も保存済み before/post full-state と差分を再検証し、誤った旧 success record の matching commitment を発行根拠にしない。
noop emitterは保存済みraw bytesも再比較する。canonical state HMACやdecoded JSONの一致だけではformat/key order変更を検出できないため、
同じ意味のstateでもbytesやstorage headerが変わればSTOPし、独立safety observationsを続けてsuccess receiptを発行しない。

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
runtime plan/root-ready gatesはfalse、issuer catalogは空。元history emitter/apply/post-noop設定は後続の承認済みsource値を保持し、新bootstrap gateに読み替えない。
history independent post-noop selectorとruntime waveの混在は認証前に拒否する。
[`test_terraform_runtime_execution.py`](../../../.github/scripts/test_terraform_runtime_execution.py) はpublic dummy transport/full stateで
全waveのfreshness→saved seal→one-object delta→実emitter→実consumer、private input/nonce/receipt/state race、
same-count IAM/API/trust交換、metadata/network/lock/cleanup failure、公開漏出拒否を検査する。

このpackageのoriginal-history-import経路はauthentic history root receiptを必要とする。現在はimport済みだがそのfinal receiptは失われているため、再import・遡及発行で復旧しない。
独立post-noop12も保持する。この旧経路は別bootstrapを受理せず、新契約は次の専用sectionで扱う。
source integration/reviewはactivation承認ではなく、final issuer SHA/CI、actual workflow/job/Environment/reviewer pinsとprotection/artifact
interface、private nonce/backend/history binding、actual scope/CI exact reads、別rootの実発行/受理・ordered runtime receipts・
final dev noop/owner merge判断が必要。既存security probe証拠は再利用し、追加live probe/query/DDL/deployを自動要求しない。

## 2026-10-13 部分採用の proof-bound no-op（source-only）

PR #1289後の専用branch `82b363eaf19207162b090cee6bd647e9a0e26779` をbaseとする。
上記の過去packageのgate／実行状況は当時の記録であり、現在のgate値の正本はauthenticated workflow。
この変更はgate値、空issuer catalog、receipt schema／commitment、権限、Environment、運用承認を変更しない。
`DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED=false` と `DEV_HISTORY_POST_NOOP12_READY=false` のまま、実runtime実行は準備段階に留める。

runtime activation gateがfalseの間は、通常の `ownership_wave=none` とbaseline／history-only経路を維持する。
developmentでruntime activation gateがtrueの場合、`none` はpreflightで認証前STOPする。独立history post-noop selectorや
exceptional modeからruntime tfvarsを省略する経路も使わず、部分採用の確認は明示的な `ownership_wave=noop` を使う。
従来のbaseline／history readerと`prevent_destroy`はfail-closedであり、このrouting gapを実削除が起きた証拠とは扱わない。

将来のactivationと必要な運用承認後も、`noop`のprivate v2 packetは次をすべて満たす必要がある:

1. `adopted` と `selected` は同じ空でない有効selection。poolのみ（baseline12＋1）、providerまで（＋2）、
   一部grant、全required grant、採用済みAPIを保持し、新規import／resource、selectionの縮小／拡張を許可しない。
2. authentic `history12` root receiptと、pool→provider→個別grant→APIのexact ordered adoption receiptsを保持する。
   `noop` receiptをadoption chainの1件に追加しない。欠落、重複、順序不一致、scope／state commitment不一致はSTOP。
3. SHA／期限／nonce／backend／history metadataを結び、認証前に0600／exclusiveのprivate stagingとGitHub provenance検証を行う。
   両jobのpacket／tfvars、fresh complete-state／metadata、最後の採用receiptのstate commitmentを一致させる。
4. full-rootのpre／postはimport0／全resource no-op。既存saved-plan sealとfresh state／metadata／lock／workspace検証を維持し、
   `noop`ではfull stateのraw bytesと完全なS3 HEAD応答を不変として検証する。APIの新規採用は引き続き全required grantの先行採用を要求する。

dummyで各段階のpacket→provenance→fresh state→plan／seal→post→public receipt／consumerを検証しても、実採用の証明にはならない。
既存のfresh history no-opは元importのhistorical root receiptの代替ではない。元root receiptが欠落／取得不能なら
この経路もSTOPし、recovery-rootの代替契約を追加せず、元importの再実行やauthenticated dispatchを行わない。

## 2026-10-13 current-state bootstrap（別契約、source-only）

この変更は `320270f1e2830195a66702b9bff46eaf623c07c5` をbaseとし、ownerが2026-10-13 15:32 UTCに承認した保証差を実装する。変更先は専用統合branch `feature/gcp-terraform-iac`。sourceレビュー・通常CIの後も、実発行、private binding/Secrets変更、protected dispatch、runtime plan/apply、mergeはそれぞれ既存の承認境界を守る。このsource PRでは実AWS/GCP認証・backend実行を行わない。

### 失われた歴史証拠と現在の確認

original apply [38326967769](https://github.com/kani3camp/youtube-study-space/actions/runs/38326967769)、source `f409439324f489b08fe9ccda3695d65828cd173a` はhistory import1とpost-noop12まで成功したが、final receiptはstate unknownで停止した。publishはskipped、artifact0、private capturesはcleanup済み。独立protected plan [38352702468](https://github.com/kani3camp/youtube-study-space/actions/runs/38352702468) は現在のimport0/no-op12/other0、stable state bytes/HEAD/table metadata、lock/workspace absenceを確認した。

限定した歴史候補比較では旧11 instances不変、history1追加、serial+1、outputs不変、既存10 checks不変と追加7 variable validation PASSが見えた。ただし候補はoriginal capturesへimmutableに結び付いておらず、削除済みversionも除外できない。当時の完全なdeltaは証明できない。historical comparatorを緩めず、lost receiptを復元・遡及発行せず、再importしない。

新契約は「現在の正規12件を新しい起点として受け入れる」保証であり、original importの連続した歴史を保証しない。`historical_delta_attested=false` は必須で、v1 history receiptへの変換は不可。

### 新しい発行・受理契約

[`terraform_current_state_attestation.py`](../../../.github/scripts/terraform_current_state_attestation.py) はstrict schema1 / type `dev-history12-current-state-v1`。operationは`protected-plan`、phaseは`current-state`、job/Environmentはplan/`terraform-dev-plan`、dev/default root12、Terraform1.16.4、exact import0/no-op12/other0、観測・admission時刻、成功outcomes、全checksとopaque commitmentsだけを公開する。original ownership receipt v1 / packet v2のapply/import-before1/post0検査はそのまま維持する。

既存post-noop verifierのHEAD→bounded GET→HEAD、before/after raw bytesとHEADの完全一致、exact state12/canonical table/schema/deletion protection、stable metadata、native lock/workspace absence、init/plan/validation成功、plan exit0、fresh admissionを再利用する。expiry・plan/state失敗でもbounded after safety readsを独立に試し、partial PASSからrecordを作らない。新recordはstrict summary envelope/version/int countsも検査する。

private bindingは既存secretによる `{schema_version:1, attestation_type:"dev-history12-current-state-v1", receipt_binding}`。scopeは新base contract名とprivate backend/stable metadataを結ぶ。decoded full-stateの既存HMACに加え、別domain `ownership-state-observation-v1` でrawbytes SHA256とprivate HEAD responseを結ぶ。nonceはprivate scope bindingであって独立の署名鍵ではない。rawstate hash、VersionId、ETag、nonce、backend、private logsをpublicへ出さない。

専用artifactは `current-state-attestation-v1-<run>-1-<source>`、canonical JSON+改行の`current-state-attestation.json`一件のみ。payload16 KiB/ZIP64 KiB、retention90日、pinned upload、glob/directory/hidden/private uploadなし。record、state/metadata、public staging、credentials、saved planとlogsをalways cleanupする。consumerはpublisher成功だけでは受理せず、run/job/cleanup成功も要求する。

別の[`current-state-attestation-sources.json`](../../../.github/terraform/current-state-attestation-sources.json) はschema1で、下記closureが成功issuer Sだけをpinする。review済みexact SHA、actual workflow ID、唯一のplan job名とapply job名、plan Environment ID/name、owner-reviewed reviewer IDs、ordered critical steps、唯一のpublisher、固定purposeをpinする。実IDを推測しない。GET-only authenticatorはlegacy bounded transport/redirect policyを使い、exact run/attempt1/repository/ref/caller/reusable、successful protected plan、Environment approval、executed apply不在、unique artifact/digest/expiry/timing/strict ZIPを確認し、run/artifact/listを再読する。single-page成功時はAPI GET10（うちZIP1）、signed redirect GETは別。logs fallbackはない。

### packet v3と初回state join

`terraform_ownership_root_basis` / reusable `ownership_root_basis` のdefaultは`original-history-import`。これにはpacket v2とoriginal `artifact-v1` history rootが必要。`current-state-bootstrap` を選んだ場合はstrict packet v3、`history_receipt`の代わりに`current_state_attestation`、別transport `current-state-artifact-v1`が必要。選択/version/type/transportの混合は認証前STOP、自動fallbackなし。v1 packetはoffline専用のまま。

v3は同じinventory/adopted/selected/freshness/bindingとordered adoption receiptsを使う。初回はpool/provider=false、grants/apis空、pool一件のみの追加。root digest、新scope、state commitmentをchildのprior fieldsに連結する。最初のprotected planとapplyのbefore state読取ではdecoded-state HMACとrawbytes/HEAD observation commitmentを両方照合する。JSONとして同じでもbytesやversion/ETagが違えばSTOP。後続childのschema1は維持し、scope生成だけが明示されたroot versionに従う。旧scope/別nonce/backend/metadata/member、skip/batch/削減は受理しない。

owner-reviewed admissionのissued→expiresは最大24時間。初回の未採用consumerはfresh admission期限内で検査を完了する。後続chainは最初のpool protected planとapply開始がroot完了後・expiry前だったことを再確認し、その後も各packetの24時間freshnessとfresh state照合を維持する。artifact expiry/deletionはいつでもSTOP。未採用で期限が切れたら新しいprotected current-state no-opを取り直し、保存延長・定期再bootstrapは別scope。

### 閉じたゲートと後続承認

current-state発行のprivate入力は、認証前の `stage-binding` で一度だけ読み取る。
既存 `terraform-dev-plan` Environment の `GCP_RUNTIME_OWNERSHIP_PACKET_JSON` が
厳密な旧 `{schema_version: 1, receipt_binding}` envelopeなら、`attestation_type`
だけを追加する。現形式は `{schema_version: 1, attestation_type:
"dev-history12-current-state-v1", receipt_binding}` のexact schemaとして検査する。
重複key、非有限JSON、不正version/type、余分/不足field、public receipt、runtime packetは
STOP。nonce/backend/完全なhistory metadataは変更・生成・置換せず、認証前に既存backendとの
一致を検査する。旧history receipt/v2 runtime契約へのfallbackや履歴証拠への昇格はない。

検査済み現形式だけをrepository外のowned `RUNNER_TEMP/current-state-private-binding.json`
へ0600・exclusive/no-followで保存する。fresh metadata照合、post-noop verifier、emitterは
この一つのprivate fileを再検査し、Secretを再読しない。metadataのstable設定・未知fieldの
完全比較、rawbytes/HEAD不変、no-op12、lock/workspace不存在、期限、provenance、
`historical_delta_attested=false` を維持する。private fileは出力・artifact・cacheへ渡さず、
既存always cleanupで失敗時も削除する。Secret/API/IAM/Environmentの変更やMac原稿取得は不要。
Secretが欠落・別形式・backend不一致・stable metadata変更ならfail-closedで止める。

このsource変更を実行する承認は、旧sourceに固定された承認とは別に扱う。
ダミー検証と独立レビュー後、新exact sourceとEnvironment review、24時間以内のadmission、
plan1/apply0/import0を改めて束ねる。旧sourceの承認期限を新sourceへ引き継がない。

`terraform_current_state_attestation` / reusable `current_state_attestation` はdefault false。dev / mode=plan / history_post_noop=true / ownership_wave=none / history_import_approval空 / initial attempt1 / default root selectorだけ許す。miscombinationとclosed emission gateはcredentials前に拒否する。

初期sourceでは `DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED=false`、両issuer catalogsは空だった。Sではemissionのみを開いて下記の発行が成功した。closureでも `DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY=false`、`DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED=false`、旧 `DEV_HISTORY_POST_NOOP12_READY=false` を維持する。実binding、Secrets、権限・Environment、dev/main/prodは変更しない。

発行にはownerがexact source S、actual Environment protection/reviewer/bypass policy、既存principalの境界、private nonce/backend/metadata、same-SHA/max4MiB/24時間/quiet-writer admission、費用と実行scopeをまとめて別承認する。review済みSでemissionのみを開き、protected plan1/apply0/import0を実行する。成功run/steps/cleanup/unique artifactを確認したclosure source CがSを別catalogへpinする。catalog受理だけではnew-root ready gateを有効化しない。Sを自己参照pinせず、bootstrap issuerをruntime apply issuerへ昇格しない。runtime planは別CI execution approval、runtime applyはさらに別承認が必要。

accountingはobserved Terraform outcomesとexplicit S3 reads10/canonical metadata GET2のみ。provider内部・workflow全体requests・actual costはunknown。後の通常planはnative S3 lockのPutObject/DeleteObjectやversion retentionを伴い得るため、全S3 writes0とは約束しない。

public dummy testsとPR専用fixture artifactは実cloud credentials/OIDC/Environmentを使わず、実データを含めない。fixtureはtrust catalogへ追加せず、実attestation/root-ready/実行承認の証拠にはしない。source revertとnew gate/catalog closureでconsumerを停止できる。既存history managed flagをfalseへ戻したり、destroy/rollback applyを行ったりしない。

### 2026-10-14 current-state issuer closure（データ参照準備のみ）

以下は初回 issuer の履歴記録。現在の fresh 参照は後続の fresh current-state issuer closure に記載する。
旧 receipt の初回消費期限は新 issuer の登録によって延長されない。

source S `94786489c90666889dd4efe5f303d1444bc8c430`（tree `9801dc0f4b35511a737ab5192a271ffc9d222d71`）の
[run 38528566934](https://github.com/kani3camp/youtube-study-space/actions/runs/38528566934) attempt1、
[plan job 115734708760](https://github.com/kani3camp/youtube-study-space/actions/runs/38528566934/job/115734708760) は成功した。
workflow ID `325439161`、Environment `terraform-dev-plan` / ID `23410074023`、owner reviewer ID `54093651`、admin bypass false。
ordered `REQUIRED_STEPS`、publisher、cleanupが成功し、applyはskipped。CI全体は2026-10-14 07:31:32 UTCにSUCCESS。

closure CはSのみをcurrent-state専用catalogへpinする。C自身をpinせず、
`runtime-receipt-sources.json` は空のまま、Sをruntime apply issuerへ昇格しない。
root-ready、旧historical readiness、runtime plan、production gatesはfalseを維持し、実行権限を増やさない。
この変更の受理と後続のlive plan/apply承認は別である。

非秘密のreceipt参照は次の通り。private inventory、nonce、backend、metadataは含めない。

```json
{
  "run_id": 38528566934,
  "attempt": 1,
  "job_id": 115734708760,
  "source_sha": "94786489c90666889dd4efe5f303d1444bc8c430",
  "receipt_sha256": "5cf14275459ae11f384b05ae5269280d88110bb9f4ca92848989ede96752b5fa",
  "transport": "current-state-artifact-v1",
  "artifact_id": 11876852839,
  "artifact_sha256": "1f376b8f2e075c1a9dbe86f5dd9ad9885f5b018405e472ec38365863897b681c"
}
```

sanitized ZIPは893 bytes。独立検証でZIP / strict schema / source / guardsはPASS、
no-op12 / change0 / import0 / drift0、state bytes/HEADとmetadata不変、lock/workspace不存在、cleanup PASS。
観測は `2026-10-14T07:30:03Z`、初回消費期限は `2026-10-15T03:04:34Z`。
artifactの90日保存は初回消費期限を延長しない。`historical_delta_attested=false`、実費と全request数はunknown。

後続の新しい実行計画はpool一件を先行させ、`schema_version=3` / `current-state-bootstrap`、
`adopted={pool:false,provider:false,grants:[],apis:[]}`、
`selected={pool:true,provider:false,grants:[],apis:[]}`、`adoption_receipts=[]` とする。
exact consumer source、private packet/bindingとfresh admission、Environment review、root-ready/runtime gateの別承認が必要。
このclosureではpacket作成・Secret読取更新・gate有効化・protected dispatch・live plan/applyを行わない。
未採用のまま期限が切れた場合は新しいprotected current-state no-opと別issuer closureが必要で、古いreceiptへfallbackしない。

### 2026-10-14 fresh current-state issuer closure（現在の参照）

fresh issuer S `18d41d08bd8221e11db51d5f9f093d19784327e3` の
[run 38617630530](https://github.com/kani3camp/youtube-study-space/actions/runs/38617630530)（CI #799）attempt1 と
[plan job 116036746682](https://github.com/kani3camp/youtube-study-space/actions/runs/38617630530/job/116036746682) は SUCCESS。
workflow ID `325439161`、Environment `terraform-dev-plan` / ID `23410074023`、owner reviewer ID `54093651`。
actual GitHub API / ordered required steps / Environment approval / publisher / cleanup と strict canonical ZIP の独立検証は PASS、
apply は skipped。authenticated full-root plan の fresh 証拠として記録する。

```json
{
  "run_id": 38617630530,
  "attempt": 1,
  "job_id": 116036746682,
  "source_sha": "18d41d08bd8221e11db51d5f9f093d19784327e3",
  "receipt_sha256": "fb2a64a9b43b4ba2fd38f63c9eff696c2f63636c73e8a8867dad08a9647edfa3",
  "transport": "current-state-artifact-v1",
  "artifact_id": 11933325159,
  "artifact_sha256": "1b3039aa6bb7cbdac9bc6d990fd604a5aa7372df62b25b9b7352f82957baaa3a"
}
```

sanitized ZIP は888 bytes。`import0 / no-op12 / drift0 / change0`、State bytes / full HEAD と stable metadata 不変、
native lock / workspace 不存在、cleanup を確認した。観測は `2026-10-14T22:18:59Z`、
fresh receipt の初回消費期限は `2026-10-15T18:43:51Z`。artifact retention はこの期限を延長しない。
`historical_delta_attested=false` を維持し、失われた original history final receipt を復元したとは扱わない。

closure は S18 だけを current-state catalog に追加する。旧 S947 entry は履歴として保持し、
旧 receipt の `2026-10-15T03:04:34Z` を更新・新期限へ読み替えない。closure 自身や runtime apply issuer は追加しない。
current-state root-ready / original-history readiness / runtime-plan / metadata-read / development apply /
production plan・apply gates は false のまま。既存の emission 設定は変更しない。
private nonce / backend / metadata / inventory は公開せず、API/IAM/Secret/State ownership はこの closure で変更しない。
初回 Pool wave は fresh packet / exact consumer SHA / private handoff と各 GitHub Environment の本人承認を確認して進める。

### Fresh current-state の再発行から初回 Pool wave まで

最新 Current Canon の包括承認内では Chat の再承認は不要。以下の各 protected job は
GitHub Environment の Required reviewer によるユーザー本人の承認を受ける。
過去の source-only PR に記した未実行や別承認の説明は、現行包括承認の取り消しを意味しない。

1. 現 integration HEAD と実 Environment protection / reviewer / bypass 設定、同 State の writer が静止していること、
   fresh State HEAD の size を確認する。root-ready / runtime-plan / apply はまだ false とする。
2. `ci.yml` を integration ref から fresh initial attempt で dispatch する。入力は
   `terraform_authenticated=true`, `terraform_target=dev`, `terraform_mode=plan`,
   `terraform_history_post_noop=true`, `terraform_current_state_attestation=true`,
   `terraform_ownership_root_basis=original-history-import`, `terraform_ownership_wave=none`。
   `terraform_plan_cost_evidence` / `terraform_history_import_approval` は空、schema audit は false とする。
   `terraform_history_plan_safety_evidence` は exact schema
   `{schema_version:1, git_sha:<exact HEAD>, issued_utc:<fresh UTC>, expires_utc:<24時間以内>,
   max_state_bytes:<fresh State size以上かつ4194304以下>, state_writers_quiescent:true}`。
   size / quiet-writer は実測・確認した値だけを記載する。
3. `terraform-dev-plan` の承認待ちへ進める。同 Environment の既存
   `GCP_RUNTIME_OWNERSHIP_PACKET_JSON` は legacy binding envelope または typed current-state envelope として再利用する。
   `stage-binding` が同 nonce / backend / history metadata を認証前に private file へ保持する。
   format 不一致、metadata 変更、no-op12 不成立は STOP。Secret を公開・平文取得する経路は追加しない。
4. 成功後、exact run / attempt1 / plan job / ordered critical steps / cleanup、apply skipped、
   Environment の実承認、唯一の artifact / digest / canonical attestation / expiry を GET-only で独立検証する。
   `historical_delta_attested=false` を維持し、公開証跡は counts / outcomes / opaque commitments と参照のみとする。
5. issuer source SHA を actual workflow ID / job names / Environment ID / reviewer IDs / required steps / purpose とともに
   `current-state-attestation-sources.json` へ別 closure PR で登録する。closure 自身は issuer として登録しない。
   runbook / README の現在参照と初回消費期限を更新し、root-ready / runtime-plan / apply は false のまま保持する。
6. 初回 Pool 用 strict v3 packet は fresh complete inventory と同一 `receipt_binding` が必要。
   `adopted={pool:false,provider:false,grants:[],apis:[]}`、
   `selected={pool:true,provider:false,grants:[],apis:[]}`、`adoption_receipts=[]` とし、
   new current-state reference / exact consumer SHA / 24時間以内の packet freshness を結ぶ。
   nonce は既存 private envelope から保持する。GitHub encrypted Secret の GET は平文回収手段ではなく、
   nonce の再生成や旧 scope の補完はできない。runner 内で既存 envelope を保持する下記 adapter は
   default-off。利用できる private handoff がなければ Pool wave だけを保留し、不足を記録する。
7. full packet と exact-read / least-privilege 境界の独立 review 後、別小 PR で current-state root-ready / runtime-plan を有効化する。
   apply gate は false を維持して protected Pool plan へ進む。Pool 一件でも safety metadata は
   Pool / Provider / attestation / SA policy / enabled APIs / project の全 inventory を再読するため、Pool GET だけでは足りない。
   runtime-plan が true の間は `ownership_wave=none` を拒否するので、current-state 再発行と closure を先に完了する。

#### D1 初回 Pool import の最小 private handoff

protected current-state attestation の成功・別 issuer closure・freshness に加え、必要な private 入力は次の3点。

1. 既存 `receipt_binding` の同一 nonce / backend / canonical history metadata。
   暗号化された GitHub Secret の GET や新 nonce 生成で代替せず、既存 private envelope を保持する。
   既存 Secret が envelope だけの場合は、下記 runner 内 join で binding を外へ取り出さずに再利用できる。
2. exact development project / Pool / Provider / 空で完了した attestation response / runtime SA 全 IAM policy /
   全 page 完了した enabled services の fresh inventory。既存 CI plan/apply identity の各 exact read 成功を
   operator read の成功と分けて検証する。Trust / unrelated bindings / enabled 状態は変更しない。
3. 既存 AWS Lambda6 caller と ECS task-role caller の audience / runtime SA / actual role metadata を join して
   重複排除した required grants の review map。実 member / condition と public aliases を固定する。
   最初の Pool apply receipt は required-grants 全 map を scope に結ぶため、空 map で採用してから
   後続 wave で grant を追加する方法は使用しない。7 caller が7 grants になるとは仮定しない。

全 enabled API を `Investigate` で分類し、`selected.apis=[]` のまま Pool を開始できる。
先に全 API を `Own` と判定する必要はなく、explicit consumer evidence による採用判断は D2 に分離する。
各 API の classification / reason / dependency はレビュー入力であり、enabled 件数や候補数から選択を補完しない。

#### 既存 binding を保持する runner 内 private handoff（default-off）

既存 `GCP_RUNTIME_OWNERSHIP_PACKET_JSON` の full packet 単独経路は維持する。
同 Secret が strict legacy `{schema_version:1, receipt_binding}` または typed
`{schema_version:1, attestation_type:"dev-history12-current-state-v1", receipt_binding}` の場合、
別の `GCP_RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON` Environment Secret に binding を除いた
reviewed v3 packet を渡せる。後者の exact keys は `schema_version`、`git_sha`、`wave`、
`reviewed_utc`、`expires_utc`、`inventory`、`adopted`、`selected`、
`current_state_attestation`、`adoption_receipts`。`receipt_binding` は null を含め記載しない。

source gate `DEV_RUNTIME_OWNERSHIP_SPLIT_INPUT_ENABLED=false` を維持し、full private packet と
既存 plan/apply 各 identity の read 境界を独立 review してから別 source で開く。
追加 Secret は caller/reusable の明示 name mapping と各 protected staging step だけへ渡す。
gate=false で追加入力がある場合、full packet と追加入力の併用、未知 field／重複 JSON key／
別 type／binding override は認証前 STOP。repository-wide secret や `secrets: inherit` へ変更しない。

adapter は runner 内で既存 backend context を照合し、同一 binding を in-memory で join するだけ。
各入力と合成後 bytes は4MiB以内とし、既存 strict v3 validator と provenance／freshness／
complete inventory／required-grants scope／same-SHA plan/apply commitment をそのまま適用する。
新 nonce、metadata の補完、grant map の生成、Secret 平文の回収や export は行わない。
staging は既存固定名2件の0600／exclusive fileだけで、成功・失敗とも既存 always cleanup 対象。
private 値を outputs／summary／artifact／diagnostic へ出さない。この source-only adapter は
inventory 成功、issuer closure、runtime gate activation、Environment 承認、Pool import の証明ではない。

### Private handoff 不足中の protected metadata-only read

[`terraform_current_state_read.py`](../../../.github/scripts/terraform_current_state_read.py) は
専用 `current-state-read` mode / `current_state_read` job の source helper。
workflow 配線・限定 gate activation・GitHub Environment approval は別途必要で、この追加自体は実行結果ではない。
既存 `terraform-dev-plan` identity / trust / reviewer を使い、production は許可しない。
専用 `DEV_CURRENT_STATE_READ_ENABLED` gate は source で default false とし、実行後は再び閉鎖する。

creds 前の `preflight` は既存 exact development plan SA、repo / immutable IDs / integration caller-ref / initial attempt / dev backend と
repository 外の owned non-symlink temp を検査する。既存 AWS OIDC 認証後の `identity` は
exact same-SHA OIDC claims と STS account / immutable RoleId を既存 helper で確認する。
既存 GCP plan SA の token を渡す `read` は exact project GET と canonical history `tables.get` を前後に取得し、
fixed 4 MiB ceiling を HEAD で先に検査してから State HEAD → GET → HEAD を前後に実行する。
history12 State envelope / addresses / table identity / canonical8 schema と bytes / full HEAD の前後一致、
stable table configuration / full project metadata の前後一致、lock / workspace prefix の成功 LIST による不存在を確認する。
文書化された table output-only metrics は既存 stable-metadata 契約の範囲で変動を許す。

private State / metadata の fixed files は exclusive0600、失敗時を含め cleanup 必須。
public Summary は fixed PASS/STOP と全検査成功時の managed12 / canonical8 のみ。artifact0、
Terraform init / plan / apply / State write / native lock creation0。
metadata の unknown stable field は前後比較から除外せず、schema の unknown field / legacy timestamp は STOP。
before / after の lock 不存在は期間全体の writer quiescence を証明しない。
この route は full-root no-op / drift0 / issuer / historical delta / admission receipt を発行せず、
後続 plan admission の `state_writers_quiescent=true` へ自動変換しない。

### CI各主体の設定読取の初回window（実測後閉鎖）

当該sourceは設定を読む専用modeだけを、UTC `2026-10-15T00:58:09Z` から `2026-10-15T08:58:09Z` のwindowで有効化した。
既存 `terraform-dev-plan` / `terraform-dev-apply` は本人の各Environment承認後に実測を完了した。
State・Terraform・IAM変更・ownership採用のgateは開かず、既存のcurrent-state証跡も消費しなかった。
初回実測後にread専用gate=false、発行・失効UTCは空へ閉鎖した。後続の独立windowは次節を参照する。
古いwindowの再利用や延長で追加実測を代替しない。結果は上の初回実測記録を参照する。


### 有限診断付きmetadata再検証の独立window

#1312で初回windowを閉鎖し、固定alias/statusと有限reasonをstdoutにも残す修正を統合した。
初回runの4項目STOPはHTTP403等へ読み替えず保存する。今回の別source変更は
UTC `2026-10-15T01:38:10Z` から `2026-10-15T09:38:10Z` の新規8時間windowで、
`runtime-metadata-read` のみ有効化する。古いwindowの延長・過去runの再実行は使わない。

独立review・required CI・integration統合後のexact SHAから新規attempt1をdispatchし、
既存 `terraform-dev-plan` と `terraform-dev-apply` を本人がそれぞれ承認してから観測する。
対象は既存development projectのPool/同Pool attestation/Provider/Runtime SA policy/project/enabled APIsの
metadataとproject範囲のpermission検査だけ。HTTP403は実応答時だけ記録し、
HTTP200のshape拒否・通信失敗等を区別する。全条件未達はSTOPを維持し、自動grantは行わない。

Terraform・State・lock・runtime mutation・artifact・ownership receiptには接続しない。
既存current-state issuerを消費せず、read成功もPool取込やInfrastructure Readyへ読み替えない。
この実測後も別review済source変更でgate=false、発行・失効UTCを空へ閉鎖する。


### 診断付き再検証の実測結果とwindow閉鎖

[run38667310894](https://github.com/kani3camp/youtube-study-space/actions/runs/38667310894) は
integration `5bd2d63c9486560d639c1c99df649263fb85392c` / workflow_dispatch / 新規attempt1。
本人 `kani3camp` の既存dev-plan/dev-apply両Environment承認を実APIで確認した。
[plan job116202140398](https://github.com/kani3camp/youtube-study-space/actions/runs/38667310894/job/116202140398) と
[apply job116202140258](https://github.com/kani3camp/youtube-study-space/actions/runs/38667310894/job/116202140258) は
exact Environment/same-SHA OIDC・既存GCP短期認証がSUCCESS、metadata readはSTOP、実cleanup stepとpost-auth cleanupはSUCCESSで完了した。

supported job-log APIから固定24行blockを各jobちょうど1件として抽出し、全key/有限enumを検証した。
両identityの実測は以下のとおり。前後片方の成功による補完や、旧runへの理由の遡及は行わない。

| Fixed alias | Plan before / after | Apply before / after | Result |
| --- | --- | --- | --- |
| `runtime-pool` | HTTP403 / HTTP403 | HTTP403 / HTTP403 | STOP |
| `runtime-attestation` | HTTP403 / HTTP403 | HTTP403 / HTTP403 | STOP |
| `runtime-provider` | HTTP403 / HTTP403 | HTTP403 / HTTP403 | STOP |
| `runtime-sa-policy` | HTTP403 / HTTP403 | HTTP403 / HTTP403 | STOP |
| `runtime-project` | PASS / PASS | PASS / PASS | PASS |
| `runtime-enabled-services` | PASS / PASS | PASS / PASS | PASS |

`runtime-project-permissions` / `runtime-window` / `runtime-cleanup` は両identityでPASS。
`runtime-stable` は未実行、全体はSTOP。project範囲のpermission観測を全resource上の権限否定へ拡張しない。
HTTP403は当該readが拒否された証拠であり、特定のAllow欠落・Deny・condition等の原因を単独で確定しない。
旧run38663394483の理由は引き続き未判別であり、今回の403を転記しない。

通常Terraform plan/apply、current-state metadata、identity probe、schema routeはskipped。
Terraform/State/lock、IAM/resource変更、artifact、ownership receiptは0、既存current-state issuerは未消費。
public logのAWS ARN/SA email/access-key/Bearer token/receipt bindingの5pattern検査は両jobで検出0
（全private値の完全検査や実runner disk検査ではない）。
使用済みの再検証gateをfalse、発行・失効UTCを空へ閉鎖する。追加実測には別reviewの新windowと本人の各Environment承認が必要。

### Exact readの最小権限候補（未適用・未実証）

[allow policy対応resource一覧](https://docs.cloud.google.com/iam/docs/resource-types-with-policies) はWorkload Identity Pool自身を含み、
[GA Pool binding CLI](https://docs.cloud.google.com/sdk/gcloud/reference/iam/workload-identity-pools/add-iam-policy-binding) はcustom roleを明示的に許可する。
[custom-role permission support](https://docs.cloud.google.com/iam/docs/custom-roles-permissions-support) では
`iam.workloadIdentityPools.get` / `iam.workloadIdentityPools.getAttestationRules` /
`iam.workloadIdentityPoolProviders.get` はSUPPORTED。role定義をprojectに置くことと、principalへのproject-wide grantは同一ではない。
したがって、この3readだけのcustom roleをRuntime Pool自身へbindする案は仕様上の候補であり、
AWS Providerを含むという理由だけでPool policy不可と断定しない。実対象でのpolicy受理・実CI read成功は未実証である。

[親policyの子への継承](https://docs.cloud.google.com/iam/docs/resource-hierarchy-access-control) と
[Provider resourceの構造](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools.providers/get) により、
Poolに置くProvider GET grantは同Pool内のcurrent/future全Providerに及ぶ粒度としてreviewする。
既存aws-provider1件だけへのgrantとは扱わず、unsupportedなresource.name条件で制限できたとしない。
project-wide roleやOwner/Editorを追加する代替にしない。

次の最小調査は、既存の管理用identityによるexact Runtime PoolのgetIamPolicy（version3、full policy/etag/conditions保持）と、
[queryTestablePermissions](https://docs.cloud.google.com/iam/docs/reference/rest/v1/permissions/queryTestablePermissions) を同Pool/Providerに限定した全pageのprivate read。
query結果はcustom-role/testability仕様であり、callerの保有権限の証明ではない。
SA policy readとhistory table metadataは別resourceのscopeで検討する。既存のread-only collectorで自動grantは行わない。
将来の最小IAM変更も既存principal/binding保持、独立review、negative test、GitHub Environmentの本人承認を省略せず、
調査用の元AWS長期キーでは適用しない。必要なmetadata/caller/private joinが欠ける間はPool importへ進まない。

### 正規CI主体のIAM authority前提を読む追加source（本8時間窓の診断準備・actual未実行）

`runtime-metadata-read` の既存plan/apply各Environmentに、独立したauthority観測stepを用意する。
`DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED=true` とし、上記UTC `2026-10-15T04:32:00Z` から
`2026-10-15T12:32:00Z` の独立8時間窓でreview済み追加診断を準備する。actualは未実行。
metadata read gateとその既存windowだけを同時に有効化し、その他のgate・OIDC・role・Secret・既存read/negative条件は変更しない。
独立review・required CI・integration統合後の新規attempt1と本人の両Environment承認を経るまで実行しない。
実測後は別source変更で両read gate=false・発行/失効UTC空へ閉鎖する。
stepは `always()` / `!cancelled()` / 既存GCP認証success / 新gatetrueを全て要求し、既存cleanupの前に置く。
metadataや既存negative stepの失敗を成功へ書き換えず、元のrequest集合・projection・終了条件を保持する。

追加helperはまず固定dev projectをfresh GETし、ID/number/ACTIVEが既存selectorと一致した場合だけ、
同じ既存短期tokenで以下の3 POSTを各1回行う。各request前・応答直後・公開直前にcontext/windowを再確認する。

| Fixed scope | Read-only request | Requested permissions |
| --- | --- | --- |
| project roles | CRM v3 exact dev project `:testIamPermissions` | `iam.roles.create`, `iam.roles.get`, `iam.roles.update` |
| Pool policy | IAM v1 exact Runtime Pool `:testIamPermissions` | `iam.workloadIdentityPools.getIamPolicy`, `iam.workloadIdentityPools.setIamPolicy` |
| SA policy | IAM v1 exact runtime SA `:testIamPermissions` | `iam.serviceAccounts.getIamPolicy`, `iam.serviceAccounts.setIamPolicy` |

responseは `permissions` だけのobject、requested subsetのunique文字列listだけを受理する。
有効なHTTP200 `{}` / `{"permissions":[]}` の未返却項目は `NOT_RETURNED`、返却された項目は `HELD` とする。
NOT_RETURNEDはresource不存在、globalなpermission不在、Deny原因の証明ではない。
非200・未知field・未知permission・重複・型違いは当該scope全体を `UNKNOWN` とし、他scopeで補完しない。
応答中にwindow/contextが失効した場合は当該scopeを受理せず `ADMISSION_STOP`、後続requestは実行しない。
公開時だけ失効した場合、既に受理した時点観測は保持するがwindow/全体はSTOPにする。
公開projectionは別schema/purpose `prerequisites-only`、固定alias/status/有限reasonだけで、両公開sinkでallowlist検証する。
IAM変更許可・writer/bootstrap/metadata/migration readyは全てfalse固定とする。

project上のroles.get/update返却は、将来の特定roleに対する実操作authorityの証明ではない。
HELDも当該scope/時刻のpermission観測であり、API受理、OrgPolicy/Deny、binding受理、Provider継承、
将来時点の成功、full7 metadataのfreshnessやroot admissionを証明しない。
ownerが読んだPool policy `{}` / etagなしの安全な初回writeも証明しない。
role CRUD・特定role GET・setIamPolicy・IAM bootstrap・State/lock/Terraform・artifact/receiptは実行しない。
既存metadataの4read403、期限切れfull7、閉じたruntime ownership/root/apply/production gatesは独立したSTOP条件として保持する。
