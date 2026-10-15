# Runtime 設定を読むツール（管理権限でログイン済みの端末向け）

Codex Cloud の調査用アカウントには、移行対象の認証設定と履歴テーブルの設定を読む権限がありません。
管理権限でログイン済みの端末で [`collect_runtime_metadata.py`](../scripts/collect_runtime_metadata.py) を実行すると、
必要な7種類の最新設定を、その端末の非公開ファイルに保存できます。
クラウド設定・権限・データは変更せず、TerraformやStateにも触れません。

## 開発環境の GCP 設定

その端末で `gcloud` にログイン中のアカウントを使います。短期認証の通常更新は行いますが、
新しいログイン・秘密鍵の発行・別アカウントへの切り替えは行いません。対象を読めない場合は停止します。

対象を指定する既存の非公開ファイル `runtime-selectors.json` を、自分だけが読み書きできるフォルダー（0700）へ、
自分だけが読める権限（0600）で配置します。結果の `runtime-metadata-capture.json` も同じフォルダーへ0600で新規作成します。
既存 output / symlink / checkout 内の出力、selector の未知キー・重複キー・不正な型・別環境は認証前に拒否します。

selector は strict schema1 です。実値をこの文書・Issue・Actions inputs/log/artifact に貼りません。

| 項目 | 非公開の元記録との照合 |
| --- | --- |
| `schema_version` | integer `1` |
| `project_id`, `project_number` | development の exact project ID / number |
| `runtime_pool_resource`, `runtime_provider_resource` | 既存 `aws-runtime / aws-provider` の 完全なリソース名 |
| `runtime_service_account_email`, `runtime_iam_policy_resource` | 現行 Runtime の default App Engine SA と exact project-qualified resource |
| `runtime_audience` | 現行 caller env と一致する canonical audience |
| `lambda6_source_match`, `ecs_source_match` | それぞれ source と fresh actual caller metadata の private review 結果。前者は `true` 必須、後者は boolean |

元記録との照合フラグは入力された確認結果であり、設定読取ツール自体が AWS caller を検査した証拠ではありません。
ECS 未照合のまま metadata を取得できても、7 caller の join / grants review は未完了です。

```sh
umask 077
# 非公開の対象指定ファイルを置いた、自分だけが使える0700のフォルダーを指定する。
# RUNTIME_METADATA_PRIVATE_DIR は checkout 外の既存 path。値・本文を出力しない。
python3 infra/gcp/scripts/collect_runtime_metadata.py \
  --private-root "$RUNTIME_METADATA_PRIVATE_DIR" \
  --selectors-file "$RUNTIME_METADATA_PRIVATE_DIR/runtime-selectors.json"
```

成功時は固定 PASS だけを表示します。token / metadata / dependency error / URL実値は表示しません。
HTTP403を含む失敗、redirect、未知の attestation/list response、非空 attestation、未完了・重複・loopした
pagination、4 MiBを超えるresponse/output、legacy history schemaは STOP し、新規 outputを削除します。
取得済みファイルは token を含みませんが、IAM member / CEL / private identity metadataを含みます。
結果は既存の非公開の引き継ぎ手段でのみ渡し、所定の保管・削除手順に従って削除します。

| 読む設定 | 対象と読取方法 | 必要な読取権限 |
| --- | --- | --- |
| Project | dev project GET | `resourcemanager.projects.get` |
| Pool | exact Runtime Pool GET | `iam.workloadIdentityPools.get` |
| Provider | exact Runtime Provider GET | `iam.workloadIdentityPoolProviders.get` |
| Attestation | same Pool `:listAttestationRules` GET | `iam.workloadIdentityPools.getAttestationRules` |
| SA policy | exact Runtime SA `:getIamPolicy?options.requestedPolicyVersion=3`, empty-body POST | `iam.serviceAccounts.getIamPolicy` |
| Enabled APIs | dev project enabled-services list、最大16 page、全page完了 | `serviceusage.services.list` |
| History | exact retained history `tables.get` GET、canonical8 metadata | `bigquery.tables.get` |

Pool/Provider/SA/key list、project IAM policy、BQ row query / tabledata / jobs は取得しません。
outputは時刻・7種類のcomplete metadataを保存し、全 unrelated SA bindings / CEL / enabled APIs / table fieldsを保持します。
これは一回の収集期間の記録であり、atomic snapshot / drift0 / writer quiescence / admission receipt ではありません。
`ci_identity_read_verified=false` / `migration_ready=false` を維持します。API分類、caller join、grants選択、
private binding/nonce、既存 plan/apply CI 各 principal の exact reads、protected approval は別に確認します。

## 読取権限の対象を限定するために残る確認

HTTP403 を解消するために project-wide の既存 custom roleへ上記5 permissionを追加すると、
新しいread対象は exact Runtime resourceだけに限定されません。
[IAM Conditions の公式属性説明](https://docs.cloud.google.com/iam/docs/conditions-attribute-reference#support_for_inherited_conditions)
は、IAM resourceが `resource.name` を提供しないと明記しています。project bindingへPool/Provider/SAの
`resource.name` 条件を足して exact scopeが成立すると扱いません。

[allow policy対応resource](https://docs.cloud.google.com/iam/docs/resource-types-with-policies) と
[Pool policy API](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools/getIamPolicy)
は Pool自身のpolicyを公開していますが、既存AWS Poolでのcustom role適用、Providerへの継承、
現在のcustom-role permission supportは未実証です。新bindingやpolicy mutationを設定読取ツールで試しません。
SA/tableのdirect resource bindingも、既存の管理用アカウントによるpolicy/etag/継承範囲の確認、独立レビューとGitHub Environmentの本人承認が必要です。
Current Canonの包括承認Aに含まれる範囲ではChatの再承認を求めません。
exact scopeが確認できない場合はSTOPし、project-wide role追加で埋めません。

仕様確認先：
[Provider GET](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.locations.workloadIdentityPools.providers/get)、
[SA policy read](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.serviceAccounts/getIamPolicy)、
[attestation audit permission](https://docs.cloud.google.com/iam/docs/audit-logging#ListAttestationRules)、
[BigQuery tables.get](https://docs.cloud.google.com/bigquery/docs/reference/rest/v2/tables/get)。

## AWS の残る ECS caller を確認する

AWS で管理権限を持つアカウントへログイン済みの端末では、
[`collect_aws_runtime_task_metadata.py`](../scripts/collect_aws_runtime_task_metadata.py) が
既存の認証設定を使って設定を読みます。新しいログインや AssumeRole は行いません。
Codex の元 AWS keys は State account 用なので、この端末へ持ち込まず、
対象の既存 workload account を読める通常の認証を使用します。

入力は、fresh な Lambda6 の3環境変数と、CloudFormation の output / current physical ID を
照合した private handoff から作成します。実 ARN / account / audience / SA を公開場所へ貼りません。

| Selector key | 必須内容 |
| --- | --- |
| `schema_version` | integer `1` |
| `environment` | `dev` または `prod`。取得先の別承認にはならない |
| `aws_account_id` | 対象環境の既存 workload account。手入力で推測しない |
| `aws_region`, `stack_name` | `ap-northeast-1`, `AwsCdkStack` |
| `task_definition_arn` | 現行 DailyBatch の exact revision ARN。family の latest 参照に置換しない |
| `expected_auth` | `GOOGLE_CLOUD_PROJECT` / `GCP_WIF_AUDIENCE` / `GCP_WIF_SERVICE_ACCOUNT_EMAIL` の3キー。fresh actual Lambda6 / stack parameter の一致値 |

`dev` / `prod` はそれぞれ既存の GCP project と default App Engine SA に固定します。
account と audience の exact 値は、既存 private source と環境の対応を事前にレビューしてください。

```sh
umask 077
# 既存 private handoff を置いた owned 0700 directory の実パスを指定する。
python3 infra/gcp/scripts/collect_aws_runtime_task_metadata.py \
  --selectors "$RUNTIME_METADATA_PRIVATE_DIR/aws-task-selectors.json" \
  --output "$RUNTIME_METADATA_PRIVATE_DIR/aws-task-metadata-capture.json"
```

input は0600・64 KiB以下、output は0600・exclusive作成です。symlinkを含むpath、checkout内、
既存output、unknown/duplicate key は認証前に拒否します。管理環境のrootに `.git` guard がある場合も
checkout外を証明できないpathとして拒否します。guardの削除や権限変更で回避しません。

実行するreadは次の4件だけです。

1. STS identity の account が入力と一致することを確認。
2. exact stack の `DailyBatchTaskDefinitionArn` output が入力と一致することを確認。
3. 同 stack の current `AWS::ECS::TaskDefinition` physical ID が入力と一致することを確認。
4. その exact revision を `ecs:DescribeTaskDefinition` で読み、ACTIVE / revision / `daily-batch` / 3環境変数を照合。

保存するのは取得時刻、task-definition、**task role**、execution role、3環境変数と照合結果です。
execution roleをGCP callerに取り違えません。AWS CLIはtask metadata全体を受信し、`--query` で必要な項目だけに絞ります。
このツールに渡す標準出力と保存対象には3環境変数だけを含め、他の環境変数やsecret参照は公開・保存しません。
secret payloadを取得するAPIは呼びません。
失敗時は固定STOPだけを表示し、新規outputを削除します。Lambda invoke / ECS run-task / IAM操作 / Stateアクセスはありません。
この結果をLambda6とjoinする前に7 caller完了とは扱わず、GCP trust/memberの照合、CI exact read、
migration admission、Environment承認も別に維持します。4readはatomic snapshotではありません。

Codex の既存ReadOnlyRoleでは、両環境のこのexact ECS readが `no identity-based policy allows` で拒否されました。
同roleのpolicy/boundary/SCPを読むAPIも拒否されたため、それらの制限が不存在とは判断しません。
[ECS Service Authorization Reference](https://docs.aws.amazon.com/service-authorization/latest/reference/list_ecs.html)
では `DescribeTaskDefinition` のresource types欄が空で、IAMは `Resource: "*"` を要求します。
exact task ARNだけのAllow追加では解消しません。region条件を付けた1 action追加でも対象account内の
task-definition metadata全体へreadが広がるため、このツールは既存権限での取得を優先し、IAM変更を実施しません。

## Poolの権限設定と、対象で調べられる権限を読む

[`collect_runtime_read_scope.py`](../scripts/collect_runtime_read_scope.py) は、前回のGCP設定取得と
同じCloud Shellで使う補助ツールです。確認済みの元ファイルのSHA256を必須引数として渡します。
実際のハッシュ・認証情報・設定値は、この公開文書やソースへ保存しません。

```sh
python3 infra/gcp/scripts/collect_runtime_read_scope.py \
  --capture-sha256 "$REVIEWED_CAPTURE_SHA256"
```

元ファイルは既存の `/tmp/yss-metadata-*/private/runtime-metadata-capture.json` から探し、
ハッシュが一致する1件だけを使います。前のCloud Shellが終了して一時ファイルが失われた場合は、
同じ取得済みJSONをCloud Shellへアップロードし、`--capture-file` でその絶対パスを指定できます。
明示ファイルは所有者が一致し、親フォルダーが他の人に書き込めず、symlinkを含まないものに限定します。
アップロード時の0644は、本文を読む前に0600へ狭めます。元の自動探索では親2段の0700とファイルの0600を要求します。
どちらもハッシュ一致・開発環境・schema・取得開始から1時間以内を認証前に確認し、
各読取前と保存前にも期限を確認します。
期限切れや一致しない場合は停止し、入力を推測・更新しません。

読むのは次の3対象だけです。いずれも設定を変更しないPOSTです。

- 既存Pool自身の権限設定：`getIamPolicy` の本文で `options.requestedPolicyVersion=3` を指定。
- 既存Poolで調べられる権限：`permissions:queryTestablePermissions` の全ページ。
- 既存Providerで調べられる権限：同APIの全ページ。

結果は新規 `/tmp/yss-gcp-read-scope-*/runtime-read-scope.json` に0700／0600で保存します。
Poolの権限設定は条件や無関係な設定も含めて保持します。権限一覧は `scopeTestable` として保存し、
3種類の必要な読取権限について、仕様上の対応と取得完了状態を記録します。
これは実行アカウントが権限を持つ証拠、権限設定の変更許可、Poolへの権限付与やProviderへの継承の
成功証明ではありません。非200・未完了・未知の応答は未確認のまま保存し、空の成功へ置き換えません。
project全体の権限設定、role一覧、鍵、State、業務データは取得しません。


### 確認済みの対象名だけを使い、新しく観測する別手順

元の取得から1時間を超えた場合、通常の手順は認証前に停止します。この制限は変わりません。
元のJSONのハッシュと対象名を別途確認した場合だけ、明示的な `--reviewed-targets-file` で、
同ファイルを「対象の住所録」として使えます。`--capture-file` とは同時に指定できません。

```sh
python3 infra/gcp/scripts/collect_runtime_read_scope.py \
  --capture-sha256 "$REVIEWED_CAPTURE_SHA256" \
  --reviewed-targets-file "$REVIEWED_TARGETS_FILE"
```

この別手順が元ファイルから使うのは、開発環境のproject ID／番号とPool／Provider名だけです。
古い状態・CEL・権限設定・attestation・API一覧・履歴テーブル・成功フラグは、新しい証拠に使いません。
まず固定の開発projectを新しくGETし、ID・番号・ACTIVEを照合します。一致するまでIAM APIを呼びません。
その後、上記3種類の権限設定・仕様を新しく読みます。追加するAPIはこのproject GET 1件だけです。

結果は別kindのschema2となり、`source_usage=reviewed-target-identifiers-only` と、元ファイルの取得時刻・
新しい観測開始時点での年齢を記録します。新しいproject応答と取得開始／完了時刻を保存し、
新しい読取開始から最大1時間の窓を認証前・各読取前・保存前に検査します。
元の7種類の設定や移行の証拠の期限を延長した結果にはなりません。新しい取得の完了判定にも古い成功を使いません。
権限保有・変更能力・Poolへの権限付与・Provider継承・CIでの読取・Runtime trust・移行準備は引き続き未証明です。

## 過去の ECS 設定と、新しく読んだ版の参照を分けて保存する

[`collect_aws_runtime_versioned_config.py`](../scripts/collect_aws_runtime_versioned_config.py) は、
確認済みの ECS 取得結果を繰り返し本人に取り直してもらう負担を減らすための、別形式の設定読取ツールです。
既存の `ecs:DescribeTaskDefinition` の HTTP403 は解消・再確認せず、権限も追加しません。
元の4readツールは変更しません。新しい形式を旧schema1の取得結果や移行packetへ変換する処理もありません。

ECS の task role・execution role・環境変数を変更するには、新しい task definition revision を作成します。
そこで確認済みの **同じ exact revision ARN** に限って、過去の設定投影と、新しく観測する版の参照を分離します。
元ファイル14キーの本文・取得時刻・成功フラグは `historical_capture.body` に保持し、
`historical_capture.sha256` は元ファイルの**生bytes**のSHA256です。入れ子の本文を再保存したbytesのhashではありません。
古い成功フラグや時刻を更新せず、新しい `captured_at` に置き換えません。

新しい private request は次の11キーだけを持ち、raw SHA256でレビュー対象を固定します。
実 account / role / session / hash を公開場所へ保存しません。

| キー | 内容 |
| --- | --- |
| `schema_version`, `kind` | integer `1`, `private-aws-runtime-versioned-config-request` |
| `environment`, `aws_region`, `stack_name` | `dev`, `ap-northeast-1`, `AwsCdkStack` に固定 |
| `aws_account_id` | 既存 private canonical と対応を確認した開発account |
| `readonly_role_arn`, `readonly_session_name` | 今回使う既存の正規ReadOnly roleと exact session名 |
| `owner_capture_sha256` | 既に独立確認した元AWS取得ファイルのraw SHA256 |
| `issued_utc`, `expires_utc` | 今回の観測窓。UTCで最大1時間 |

```sh
python3 infra/gcp/scripts/collect_aws_runtime_versioned_config.py \
  --request-file "$AWS_RUNTIME_PRIVATE_DIR/versioned-config-request.json" \
  --request-sha256 "$REVIEWED_REQUEST_SHA256" \
  --owner-capture-file "$AWS_RUNTIME_PRIVATE_DIR/aws-task-metadata-capture.json" \
  --output "$AWS_RUNTIME_PRIVATE_DIR/versioned-config-offline.json"
```

既定は offline 検査で、認証・AWS API は0件です。過去の取得が古くても、本文・対象・hashをレビューした
immutable設定投影として検査できますが、現在の状態や期限内の移行証拠にはなりません。
新しいreadを行う場合だけ `--read-current` を付け、未使用の出力名を使います。
実行前に exact source / request / 有限call scopeのレビューを終え、既存の正規ReadOnly sessionを選択します。
ツール自身はログイン、AssumeRole、元の長期keysの差し替えを行いません。
元の長期IAM userで直接設定を読もうとしても、最初の exact assumed-role identity照合で停止します。

現在のreadは次の4件に限定します。APIの自動retryは1 attemptに固定します。
外側の既存認証手順でSource STSとAssumeRoleを使う場合、その2操作は別に数え、合計は最大6 CLI操作です。

1. STS `GetCallerIdentity` 1件で、accountと exact ReadOnly role/session を照合します。これは読取側のidentityです。
2. exact stackの `DescribeStacks` 1件で、`DailyBatchTaskDefinitionArn` output 1件と共通GCP認証parameter 3件を読みます。
3. 過去の exact ARN から導いた full family に `ListTaskDefinitions` 1件を行います。`ACTIVE` / `DESC` / `maxResults=1` / `--no-paginate` 固定です。
4. `ListFunctions` を `MaxItems=50` / `--no-paginate` の単一pageで1回だけ呼びます。AWS CLIはserviceのpage全体をメモリで受け取り、`--query` でsource固定6名の function名・ARN・role・共通3環境変数・`Version` / `RevisionId` / `LastModified` だけを選びます。collectorへ渡すのはこの投影と `has_more` booleanだけで、他のfunction・他の環境変数・NextMarker本文を保存しません。

stack output と ACTIVE family先頭の**両方**が過去の exact revision ARN と一致しない場合は停止します。
family先頭1件は版を確かめるための点観測であり、family全件の一覧ではありません。
過去の取得時の mutable なACTIVE状態を継承せず、ACTIVEは新しい限定listの応答時点だけの観測です。
stack parameterと新しいLambda6の3環境変数も、過去のtask設定と一致することを確認します。
Lambdaの `$LATEST` 設定は可変なので immutable と扱わず、単一pageの観測窓と各functionのrevision情報を保持します。
page内にexact6名・ARNが重複なくすべて存在し、各roleのaccount・共通3環境変数・version等が一致することを必須とします。
順序は問いません。6件が揃えばNextMarkerがあっても選択対象の点観測として受け入れますが、
`lambda_function_inventory_complete=false` を維持し、全function一覧が完了したとは扱いません。
不足・重複・不明な対象・shape不正は停止し、追加page・`GetFunctionConfiguration` / `GetFunction`・再試行へ切り替えません。
入力窓は認証前・各read前後・保存前後に確認し、途中の期限切れ・逆行時刻・不一致・不明な応答は新規出力を削除して停止します。
異なるrevision、family、account、role、functionへ探索や再試行を広げません。

出力は別kind `private-aws-runtime-versioned-config-observations` のschema1です。
新しい `observations` と過去の `historical_capture` を混同せず、移行を許可するreceiptにはしません。
各APIは別時点の観測であり、atomic snapshot、将来の参照先、実行中taskや自然実行の保証ではありません。
revisionなしの `RunTask` は最新ACTIVEを使うためfamily先頭も確認しますが、実際のStep Functions定義や
実行時の task-role / environment override はこのツールでは取得しません。
新しい `DescribeTaskDefinition`、IAM roleの存在・RoleIdの継続、workload自身のSTS、自然実行、
fresh full7 / CI read / migration readiness はすべて未証明のままです。
元のDescribeTaskDefinitionを追加しても、それだけでIAM roleの存在や実行時overrideが証明されるわけではありません。

input / outputはowned0700のフォルダー内の0600ファイルに限定し、既存output・symlink・hardlink・実checkoutを拒否します。
managed環境の `/tmp/.git` / `/workspace/.git` が空の0555 guardである場合だけ、checkoutではないものとして扱います。
それ以外の `.git` markerは空でも拒否し、guardを削除・変更しません。
標準出力・エラーは固定文だけです。設定本文やAWSエラー本文を公開logへ出さず、IAM / State / workloadを変更しません。

過去の `2026-10-15T02:01:47Z` のLambda再照合は `ListFunctions` 1件を含む計6認証・設定CLI操作で、
Lambda6件の個別GETではありません。新しく導入した個別 `GetFunctionConfiguration` は既存ReadOnly sessionで
最初の対象がCLI rc254 / `AccessDeniedException`となったため、既存と同じListFunctions操作へsourceを修正しました。
過去のListFunctions成功を現在の許可の証拠にはせず、修正後の実読取は別の有限実測まで未確認です。

公式仕様：
[task definitionの更新](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/update-task-definition-console-v2.html)、
[ListTaskDefinitions](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListTaskDefinitions.html)、
[RunTaskのrevision選択](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html)、
[実行時TaskOverride](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_TaskOverride.html)、
[ListFunctions](https://docs.aws.amazon.com/lambda/latest/api/API_ListFunctions.html)。
