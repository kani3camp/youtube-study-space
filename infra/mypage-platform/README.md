# MyPage Platform Terraform

DEVへの配備・適用・試験の最新状態は[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)を参照する。以下の構築記録は各段階の観測であり、過去の失敗・未配備の記述から現在状態を判断しない。公開前の未検証項目は[公開条件](../../docs/mypage/release/README.md)で個別に確認する。

マイページの新しいCloud Run API基盤と実行権限を、既存GCP Terraformの既存資源取り込み専用運用とは別のStateで管理する。2026-10-21 JSTにDevelopmentの専用SAと3個のSecretメタデータを実作成し、専用Stateに保存した。段階別の適用結果は[実構築記録](docs/development-foundation.md)と[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)の現在の記録を参照する。Cloud Run・Hosting・Secret値・本番・公開準備の完了を意味しない。

- プロダクト / 環境配置: [Notion 07｜技術アーキテクチャ](https://app.notion.com/p/3ef357a8d0ce81b5892ac1dc2c99e587)
- API・OAuth cookie・release設計: [Notion 08｜API・デプロイ設計](https://app.notion.com/p/3ef357a8d0ce81cbbce9c2ae948ebfb3)
- Terraform既存baseline: [GCP Terraform](../gcp/README.md)、[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)
- MyPage entrypoint: [Go server](../../system/cmd/mypage-server/main.go)
- Hosting候補/公開準備: [MyPage release](../../docs/mypage/release/README.md)
- OAuth transaction TTL: [独立Terraform root](../mypage-oauth-ttl/README.md)

## なぜ分離するか

既存の`infra/gcp/environments/{dev,prod}`はDev31件・Prod30件を管理する固定の基準構成であり、認証ワークフローはimport-only/no-opで動く。新規リソースを作るために、既存のIAMやapply保護を緩めない。MyPageの新規作成には通常変更用の別の運用仕様が必要なため、このルートはStateキーと実行用本人情報を分ける。OAuth TTLの小さな専用Stateは既存のplan仕様を維持し、ここには統合・移管しない。既存GCP資源やCloud Run/Firebase管理対象の二重管理を発見したら停止する。

| Root | Cloud target | 独立State key（Productionは未構築） |
| --- | --- | --- |
| `environments/dev` | `test-youtube-study-space` / `asia-southeast2` | `youtube-study-space/mypage-platform/dev/terraform.tfstate` |
| `environments/prod` | `youtube-study-space` / `asia-northeast2` | `youtube-study-space/mypage-platform/prod/terraform.tfstate` |

Terraform CLI `1.16.4`、Google provider `8.5.0`、S3 native lockと既存multi-platform provider lockを使用する。Development専用のAWS backend Plan/Apply role、GCP WIF/構築SA、GitHub Environmentを接続し、metadata段階の実Plan/ApplyとState readbackが成功した。[Development Foundation構築](docs/development-foundation.md)を参照し、既存 `infra/gcp` 用のState roleを流用しない。Private `backend.hcl` と認証情報をcommitしない。

## 通常desired stateとbootstrap内部stage

通常運用では`Terraform`（`terraform.yml`）の`environment` / `operation=plan|apply` / `component=all|mypage`だけを使う。MyPageは`terraform.yml`がreview済みの`desired-stage.txt`を解決し、最終desired state（現在は`runtime_public`相当）に固定してPlan/Applyする。利用者はstageを選ばない。Core GCPとはStateを分けたまま、同じ`Terraform`実行内で両方を扱える。

ownerの同一repository PRでMyPage rootが変わると、`Terraform` がDEVのdesired stateを **PR Plan** する（[`terraform-pr-plan.yml`](../../.github/workflows/terraform-pr-plan.yml)）。PR Plan identityは手動Plan / Applyと別で、`mypage-foundation-pr-plan` SA（`mypage-foundation-plan`と同じ読み取り権限）、`mypage-foundation` poolの`github-pr-plan` provider（conditionは`python3 infra/mypage-platform/scripts/foundation_contract.py pr-plan-provider-condition`）、State読み取りだけのAWS role（`pr-plan-aws-policies <account> <bucket>`）、Environment `mypage-dev-pr-plan`（手動と別の`MYPAGE_PLAN_HMAC_KEY`）を使う。`-lock=false`で実行し、plan contractの判定だけを表示してcommitmentを残さない。

自動PR PlanはCoreのDEV/PRODとMyPage DEVの専用identity・Environment secretを準備してから、repository variable `TERRAFORM_PR_PLAN_ENABLED=true` で一括有効化する。未設定または`false`では認証jobとPR Plan Gateをスキップし、Summaryに未有効化とnative Plan未実行を明示する。credentialless CIは継続する。有効化後の認証不備・Plan失敗は失敗のままとし、手動Plan/Apply用identityへフォールバックしない。

以下のstageは初回構築と既存の拒否条件を維持するための内部実装であり、通常の手動UIには出さない。ソースの`stage = "disabled"`は未構築ルートの既定値で、Module count=0・新GCP資源=0を意味する。初回構築時だけ段階を明示し、既定値を実Stateの状態と混同しない。

通常経路が使うstageは、`environments/{dev,prod}/desired-stage.txt`をレビューして決める。現在はDEV=`runtime_public`、本番=`disabled`である。本番が`disabled`の間はMyPageのクラウド認証情報を求めず、Core GCPだけを通常運用できる。本番のMyPage公開準備を進める場合は、この変更自体をPRでレビューし、必要なEnvironment・IAM・Stateの準備と同じ変更単位で扱う。

| Stage | Terraformが管理するもの | 意図的に含めないもの |
| --- | --- | --- |
| `disabled` | 新resourceなし | 運用中のTerraform State変更 |
| `foundation_metadata` | 新専用SAと3個のSecret metadataのみ | IAM member変更・Secret値・Cloud Run |
| `foundation` | 新専用 user-managed SA `mypage-api`、Secret Managerの3個の**secret metadata**、SA単位のFirebase Custom Token用signer IAM、各secret単位のAccessor IAM | secret version/value、Firestore権限、Cloud Run、公開Invoker |
| `registry` | 上記8資源 + 対象環境regionの専用Docker repository `mypage`。immutable tags、cleanup dry-run、prevent_destroy | image push、Cloud Run、Firestore権限、公開Invoker |
| `maintenance_operator` | 上記 + 固定 digest の単一 maintenance Job、専用 runtime/invoker SA、Firestore custom role、環境別 WIF | Cloud Run API の作成・更新、制御文書の暗黙作成、一般 Invoker、GitHub runner の Firestore 権限 |
| `maintenance_upgrade` | 既存 `runtime_public` を no-op に保ち、maintenance Job と専用 IAM/WIF を追加 | 既存 Cloud Run API・Firestore member・public Invoker の作成・更新・削除・置換 |
| `runtime_private` | 上記 + `roles/datastore.user` の新SAに対する**additive project IAM member** + Cloud Run v2 API本体 | Cloud Runの `allUsers` invoker、Hosting公開、scheduler/alert |
| `runtime_public` | 上記 + 当該Cloud Runサービスだけに `roles/run.invoker` / `allUsers` memberを追加 | ログイン必須APIのID token/App Check免除、一般告知 |

Cloud Runは `mypage-api`、専用SA/ADC、`min_instance_count=0`、`max_instance_count=1`（Dev）/ `3`（Production）、CPU 1 / RAM 512Mi、port 8080、30秒timeout。

リビジョンのイメージはArtifact Registryのダイジェスト（`@sha256:...`）で固定し、タグやlatestで不用意に差し替えない。 **初回Cloud Run作成後のAPI Serviceとmaintenance JobのコンテナimageはCI/CDが所有し、Terraformは両方のimage属性を`ignore_changes`で管理しない。** API ServiceはBackend release、Jobは独立した`Deploy Maintenance Operator`が更新し、同じdigestへ同期しない。 maintenance専用digest/source SHAをruntime settingsへ追加しない。既存サービスのimage更新は[Backend CI手順](../../docs/mypage/release/backend-ci.md)のBuild→Registry digest→直接image配備→検証で行う。通常releaseごとにTerraform Plan/Applyやdigest変更PRを実行しない。実費は起動回数・Secret Manager・ログ量・API呼び出しなどで変わるため、未計測で月額1ドル未満と断定しない。<https://cloud.google.com/run/pricing>と<https://cloud.google.com/secret-manager/pricing>を参考に、実対象と試算を別途レビューする。

作業履歴の完全性は実データの確認済み区間だけで宣言する。`runtime_settings.history_coverage_json` は**既定が空文字の任意設定**で、空文字ならCloud Runに`MYPAGE_HISTORY_COVERAGE`は存在しない。指定する場合は最大16KiBのJSON配列を渡し、Goの[厳格parser](../../system/cmd/mypage-server/coverage.go)がUTC時刻・順序・重複・上限を起動前に検証する。`[]`や未指定は完全な履歴を表さない。実際のwriterと履歴のRead-only inventoryに基づく区間以外は指定せず、実Deploy/rollbackの設定値と欠損の扱いを照合する。

### Secret / IAM境界

1. Terraformは `mypage-oauth-client-id`、`mypage-oauth-client-secret`、`mypage-youtube-api-key` の**コンテナだけ**を管理する。値および `google_secret_manager_secret_version` を作成・保存しない。Secret versionはowner-approved private手順で投入し、Cloud Runには**番号で固定**して環境変数へ参照する。`latest`は禁止。
2. Server envには `GOOGLE_CLOUD_PROJECT`、`MYPAGE_ENVIRONMENT/REGION/PROJECT_NUMBER/WEB_APP_ID/PUBLIC_ORIGIN/SIGNER_EMAIL/PRIVACY_VERSION/TERMS_VERSION` を渡す。実OAuth token、access/refresh token、Firebase ID tokenをTerraformに流さない。
3. Signer権限は専用SA自身の `roles/iam.serviceAccountTokenCreator`、Secret Accessorは各secret、Firestoreは `roles/datastore.user` を現状project単位で付与する。**Firestore IAMはcollection単位に絞れず、当該権限は他のFirestoreデータにも及ぶ**。対象・権限・D01削除運用を実審査し、より限定的なcustom roleの採否も評価する。運用SAが必要な他権限（Cloud Run deployerの`actAs`、Artifact Registry reader等）はowner inventory確認まで勝手に付けない。
4. `INGRESS_TRAFFIC_ALL` はHosting rewrite/callbackに備えたネットワーク設定であり、**Invoker IAMはruntime_publicまで未付与**。公開時でもAPIのAuth/App Check/ServiceAccessControl/Origin gateを維持する。意図しない直URLからのアクセス、Host/callback、Cookie `__session` の実経路試験は別Gate。Public化はアクセス拡張なので単なるHCL validateでは承認しない。
5. `MYPAGE_INFRASTRUCTURE_READY=true` はGo起動設定の要求値であり、**Ready Gateを技術的に証明したことを意味しない**。人間判断 / 実設定のreadback / 実E2Eのゲートは別に閉じる。

本sourceはGoogle APIsの所有権やFirebase Auth / App Check / Hosting site、IAM deployer、OAuth console client、Artifact Registry repository、Firestore database/index、Cloud Scheduler/metadata cleanup runner、Monitoring receiver、Privacy実削除を作らない。既存repoに宣言済みのAPI `google_project_service` を**二重に管理しない**。それらは最新inventoryを取得し、必要な最小差分を別PRで設計する。

## 認証情報を使わない検査

GitHub PRでは必須の`CI Gate`配下の`CI GCP Terraform`が、MyPage基盤の認証情報不要の検証も実行する。S3/GCPへのOIDC・シークレット・実State・plan/apply・配備は使わない。独立した`mypage-platform.yml`ワークフローは廃止し、GCP IaC検証を1つの必須経路に集約する。

```bash
bash infra/mypage-platform/scripts/check-local.sh
```

このスクリプトは`terraform fmt -check`、両ルートの`init -backend=false -lockfile=readonly`、`validate`、`mock_provider`による`terraform test`を実行する。安全な合成プロジェクト番号・appID・ダイジェストはテスト内だけで使う。モックPlanの成功は、実GCP Plan・予算・E2Eの承認ではない。

## 初回bootstrap・公開準備の作業と停止条件

通常の変更ではこの段階一覧を手動選択せず、`Terraform`の`operation=plan` で両Stateの差分を確認し、承認した差分だけ `operation=apply` する。以下は空環境からのbootstrapや公開準備の履歴・停止条件であり、日常の操作UIではない。

1. **Live inventory**：Dev/ProdのSA名、Cloud Runサービス名、Secret ID、Firestore、Firebase Hosting、Google APIsとIAM既存値をread-only収集。既存作成物があれば「新規create」で上書きしない。特に他のTerraform State ownerの有無を確認する。
2. **Backend/権限準備**：MyPage専用S3 keyと `.tflock`、AWS identity、GCP Workload Identityの**正確なref / workflow / target / principal**を確認。GitHub public PRや任意branchには認証を渡さない。[Development Foundation経路](docs/development-foundation.md)は8資源だけを許可する専用workflowとguardを持つ。専用AWS/GCP/GitHub接続とmetadata段階の実適用は成功済み。同じ境界をtarget-awareに拡張したProduction source経路は、protected Environment進入前のcontext検査に加え、`mypage-prod-plan/apply` Environmentのdeployment branchを`main`だけに固定する。exact SHA・prod専用State/AWS role/GCP SAへ分離し、registry plan/apply、image-build、maintenance operator/upgrade plan/applyだけを許可する。ProductionのEnvironment/secret/state設定と実Plan/Applyは未実施である。
3. **Foundation Dev**：private `backend.hcl` で正確な空Stateを初期化し、レビュー済み `stage="foundation"` の実plan・plan jsonをprivateに保全、差分type/action/IAM/コストを確認。独立reviewと実行承認後にのみ保存済みplanをapply。secret versionはTerraform外でprivate投入し、metadataとIAMをreadback。
4. **Maintenance 準備 Dev**：`maintenance_operator` または既存 runtime を保持する `maintenance_upgrade` で単一 Job と専用 IAM/WIF を作成する。固定名の既存資源を事前照会し、同名資源が State 外にあれば停止する。workflow の `enabled=false` で `config/mypage` を初期化し、Job の readback を確認してから Backend と Frontend を配備する。[運用手順](../../docs/mypage/maintenance-mode.md)を参照。
5. **Private runtime Dev**：digestを持つ実Goイメージ、project number、Web App ID、origin、規約version、**固定secret version番号**を一致確認。 `runtime_private` へ段階適用。Service起動/keyless signer/Firestore/App Check/Auth/ヘルスと本番同等経路を観測。条件不成立・権限拡張・unknown/driftは停止し、差分をplan修正して再レビュー。
6. **Public runtime Dev**：Hosting `/api/**` rewrite、OAuth callback、CSP/cookie/Originとrate limiting等を実機で検証し、`runtime_public` でInvoker公開を明示承認。公開URLが機密アクセス制御の代替ではない。Frontend HostingはTerraformとは独立した配備操作・release recordだが、DEVでは[release手順](../../docs/mypage/release/README.md)の専用Hosting jobによりCI内でbuild/apply/readbackを完結させる。過去のowner配備記録を現行のCI構成と混同しない。
7. **Production**：Dev の確認後、独立した target / state / permission / rollback 承認で同じ順序を実施する。immutable repository と review 済み image digest を用意し、maintenance operator の plan / apply、初期化、runtime 配備を行う。実環境操作は別途承認を要する。

異常時は先に後続apply/一般告知を停止する。Cloud Runは `deletion_protection` と `prevent_destroy` があるため、事故対応を安易な `destroy` としない。新規Secretの値を消したり、TTLや既存GCP管理資源を巻き戻したりしない。Release対（Frontend commit/Hosting release、Backend image digest/Run revision、同意版、検証証拠）と**前世代互換**を記録し、必要時は前のreview済みimage/revisionへforward-fixまたは明示rollbackする。公開権限の取り消しは別途明示的に実施できる。

DevelopmentのFoundation metadataは実作成・State readback済み。Secret version、Cloud Run/Hosting、OAuth、release gateは未完了。各段階の実施記録は[Development Foundation構築](docs/development-foundation.md)と統合PRへ残す。

### Runtimeの診断と既存イメージの出所

runtime設定の任意 `image_source_sha`（既定は空文字）は、既存の不変tagを作成した40桁commit SHAを示す。空文字なら従来どおり実行元SHAのtagを要求する。別SHAの場合はそのcommitが現行HEADの祖先であることと、同SHAの不変tagがbootstrap digestを指すことを確認する。配信imageはCI/CDで別途更新されるため、実行元の`system` treeとの完全一致を要求しない。Docker contextは `system/` 全体であり、Dockerfile、依存lock、ignore設定とGoソースを含む。tagの作り替え、出所不明digest、Terraformから配信imageを巻き戻す変更を拒否する。これはinfra診断だけの変更で既存revisionを更新する必要をなくすためで、Cloud Runのupdate/delete/replaceは引き続き拒否する。

maintenance operatorはAPIと別の`mypage/mypage-maintenance-operator` packageを使い、Job作成後のimageは`Deploy Maintenance Operator`が所有する（API releaseとは同じdigestへ同期しない）。Terraformはimage属性だけ`ignore_changes`する。専用のdigest/source SHAはruntime settingsへ持たない。DEV初回Job作成時だけ、image分離前にビルドされoperator binaryを含む固定source SHAの`mypage-api` immutable tagをPlan時にread-only解決し、そのdigestをprivateな一時Plan入力として使用する。旧Environment secretに残る`maintenance_operator_*` 3キーは移行互換のため読み捨て、private tfvarsやPlanへは渡さない。

## Plan guardの責任分担

Terraformのresource schema・computed metadata・etag・空map/list補完・Job実行件数/リビジョン観測値は、Terraform/providerに任せる。Pythonにprovider field全体のallowlistを持たない。refreshは既知の管理資源・Google provider・planned no-op・planned after一致に限定し、destroy/replace/update/importの許可は従来の目的別境界を維持する。

アプリの計測設定を含むCloud Run security設定のdrift拒否、固定project/region/principal・IAM broadening拒否、Secretの参照のみ、image digest・環境変数・capacityなどの重要設定検査は残す。アプリimageのリリースはCI/CDが担当し、ビルドごとのprevious digest宣言PRやTerraform Applyは不要。後日のPlanでは正規のimage-only refreshを検査し、構成値の変更や不正なイメージ参照を拒否する。実配備のReady/traffic/設定不変はbackend release側が検証する。

native Plan全体とdriftは引き続きHMACへ含め、source・State versionと保存PlanをApplyへ結び付ける。二重管理検出・state owner・DEV/PROD State分離・Production approval・saved plan apply・重要設定readbackを維持する。診断はdrift資源数とplanned write数のみで、Plan値や未知field名を公開しない。provider upgrade時はTerraformのlock/ネイティブテストを更新し、Pythonへcomputed field名を追記しない。

### 公開Invokerをownerが追加し、CIは固定memberだけを取り込む

実Apply run 39572740715 は `run.services.setIamPolicy` のauthorization拒否で停止した。IAM条件の `modifiedGrantsByRole` を持つbindingの配置先はproject/folder/organizationに限られ、Cloud Runサービス上に置く方法は適用できなかった。実readではversion省略でも条件が保持され、providerによる条件消失という仮説は否定された。診断用一時bindingは全て解除済み。

CIの権限を広げず、承認済みの `mypage-api` / `roles/run.invoker` / `allUsers` 1 memberだけをownerがetag保護で追加する。public pipelineはversion3のpolicyを読み、その無条件memberと、その他のpublic broadeningがないことを確認する。private runner workspaceへ固定target/idのimport blockを生成し、Plan/Applyで同じ内容を再現する。public invokerはno-opだけを認め、importがある場合もこのexact idだけに限る。他resource/import id/role/principal/condition、Cloud Runの更新・削除・置換は拒否する。native Plan/HMAC/State version/readback/cleanupは維持。CIにIAM setterを付与しない。既存Stateに取り込み済みの場合、import blockはno-opとなる。

### Hosting転送の固定ホストとowner image更新

以下は初期接続時のowner操作記録。以後のDEV通常image更新は [Backend CI手順](../../docs/mypage/release/backend-ci.md) で専用deployerへ分離する。既存Terraform SAへRun updateを追加せず、source-pinned digest・resourceVersion・設定不変を検証する。State refreshは次回のインフラPlan/Apply時にだけ行い、通常リリース時の必須操作にしない。CI経路の実接続・配備結果は、対象sourceのActionsとrelease recordで確認する。

2026-10-21にownerのpublic invokerをrun 39587793777で取り込み、12資源・create/updateなしでState readbackを確認。Hosting release `1792580103818000` を公開し、8ファイルの実配信内容一致を確認した。画面は表示できたが、Hosting経由のAPIはOrigin境界で400となったため、OAuth実試験の前に転送先ホストの明示設定を追加する。S01/S04の合格を意味しない。

`MYPAGE_HOSTING_BACKEND_HOST` は実metadataから得たCloud Run hostnameを固定する。未設定時は従来どおり公開Hostのみを認める。設定時も一致するbackend Host、単一の公開 `X-Forwarded-Host`、単一の `X-Forwarded-Proto: https` を要求する。転送情報は認証根拠とせず、Auth/App Check/Origin/cookie検証とIP判定を維持する。

`environments/dev/runtime-release.json`の4値は旧方式の履歴であり、現在のCI/CD・Terraform入力ではない。アプリ更新はコードPRのレビュー後、同じActions runでbuildしたdigestをCloud Runへ直接渡す。Secret参照番号・同意版・project等の設定はimage更新で変更できない。更新対象イメージはimmutable tag・Registry digestで厳格に検証する。

以前はownerが手動でimageを更新していたが、今後は専用CIのbackend deployerがv1 resourceVersion付きでimageだけを変更する。初回のHosting接続時だけ固定host env 1個を追加し、以後のimage更新では全envをそのまま保持する。通常のアプリ配備ではTerraform applyをしない。Cloud Runの前後で同一service・env/Secret/scale/IAM不変・Ready/最新revision100%・generation+1・更新時刻前進を検証する。後日のTerraform Planはimage-only（複数世代も可）と許可されたcomputed metadataだけをState refreshとして受理し、その他のdriftを拒否する。

### S04の実測後のDEV request URL保存抑止

2026-10-21 11:16 UTCの合成callbackをCloud Run request logで観測し、code/state queryが保存されることを確認した（実OAuthは未実施）。`environments/dev/request-log-exclusion.json` はowner管理の `_Default` sink exclusionのreview済み入力で、Terraform Stateの追加資源ではない。ownerが既存sink inventoryとfilter previewを確認して作成し、readbackと別合成markerで再確認する。CIのIAMや既存State管理範囲を増やさない。

filterはtest projectのmypage-apiの標準request logのうちcallback URLまたはquery付きAPI URLだけに限定する。通常のqueryなしAPI request、application/security log、監査・system log、他service/projectは対象外。既存の合成記録は削除せず、保持期間中は既存の閲覧境界内に残る。今後別sink/ancestor routingを増やす場合は同じprivacy境界を検証する。Hosting/CDNや実OAuth部分を含む全層のS04完了をこの局所修正だけでは宣言しない。実Auth、実データ削除、Production、先行ログ基盤追加は対象外。

2026-10-21のowner更新後、実serviceは `mypage-api-00002-qcs` Readyとなり、Hosting GET/POSTのApp Check欠落403、不正Origin400、method405を確認した。run 39592888606のPlanはplanned no-opだが `runtime-owner-refresh-ready` で変更前停止。owner API metadataのtemplate.revisionは更新前後とも空で、実revisionはlatestReady/latestCreatedにだけ現れた。自動命名時の空template.revisionは前後不変の場合だけ認め、固定serviceのlatest Ready/Created一致、成功condition、最新100% traffic、generation+1の検証は維持する。非空の旧revision・他service・未知値は拒否する。

### 手動TerraformのPlan回数とSummary

`operation=plan`は選択したrootごとにPlanを1回実行する。`operation=apply`は外側のPlanをスキップし、認証なしの検証後、各rootのreusable workflow内でPlan権限によるPlanとApply権限による独立Planを生成する。一致した場合だけApply job内の保存Planを適用する（各rootの適用前Planは計2回）。Coreには適用後のno-op検証Planもあるため、成功時のPlan総実行回数は従来4回から3回になる。`component=all`ではCore Applyが成功してからMyPageの内部Plan/Applyを開始するため、MyPageが失敗しても成功済みのCore変更は残る。root間の一括ロールバックはない。

MyPageの通常Plan（Apply内のPlan権限jobを含む）はPlan契約・所有権・State versionの検証成功後、Job Summaryに操作別件数と資源種別・操作別件数を出す。Coreと同じ`Resource type / Actions / Count`形式を使い、名前・ID・設定値・outputs・commitmentは公開しない。`drift`は観測件数で、適用予定の変更件数ではない。拒否されたPlanの値や診断は引き続き非公開とする。
