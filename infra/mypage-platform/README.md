# MyPage Platform Terraform

マイページの新規Cloud Run API基盤と実行権限を、既存GCP Terraformの**既存資源import-only運用とは別State**で管理する。2026-10-21 JSTにDevelopmentの専用SAと3 Secret metadataを実作成し、専用Stateへ保存した。段階別の適用結果は[実構築記録](docs/development-foundation.md)と[統合PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236)の現行記録を参照する。**Cloud Run/Hosting・Secret値・Production・公開Readyの完了を意味しない**。

- プロダクト / 環境配置: [Notion 07｜技術アーキテクチャ](https://app.notion.com/p/3ef357a8d0ce81b5892ac1dc2c99e587)
- API・OAuth cookie・release設計: [Notion 08｜API・デプロイ設計](https://app.notion.com/p/3ef357a8d0ce81cbbce9c2ae948ebfb3)
- Terraform既存baseline: [GCP Terraform](../gcp/README.md)、[Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191)
- MyPage entrypoint: [Go server](../../system/cmd/mypage-server/main.go)
- Hosting候補/公開準備: [MyPage release](../../docs/mypage/release/README.md)
- OAuth transaction TTL: [独立Terraform root](../mypage-oauth-ttl/README.md)

## なぜ分離するか

既存 `infra/gcp/environments/{dev,prod}` はDev31/Prod30件を管理する固定baselineで、認証workflowは **import-only/no-op**。そこで新規リソースを作れるようにIAMやapply guardを緩めない。MyPage新規作成は別の**normal-change運用契約**を要求するため、このrootはState keyも実行identityも分離する。OAuth TTLの小さな専用Stateは既存のplan contractを維持し、ここへ統合・移管しない。既存GCP所有資源・Cloud Run/Firebase管理対象との二重ownershipを発見したら停止する。

| Root | Cloud target | 独立State key（Productionは未構築） |
| --- | --- | --- |
| `environments/dev` | `test-youtube-study-space` / `asia-southeast2` | `youtube-study-space/mypage-platform/dev/terraform.tfstate` |
| `environments/prod` | `youtube-study-space` / `asia-northeast2` | `youtube-study-space/mypage-platform/prod/terraform.tfstate` |

Terraform CLI `1.16.4`、Google provider `8.5.0`、S3 native lockと既存multi-platform provider lockを使用する。Development専用のAWS backend Plan/Apply role、GCP WIF/構築SA、GitHub Environmentを接続し、metadata段階の実Plan/ApplyとState readbackが成功した。[Development Foundation構築](docs/development-foundation.md)を参照し、既存 `infra/gcp` 用のState roleを流用しない。Private `backend.hcl` と認証情報をcommitしない。

## 段階ごとの責務

ソースの `stage = "disabled"` は未構築rootの既定で、**Module count=0／新GCP resource=0**。構築済みDevelopmentでは実行workflowが当該段階を明示する。既定値を実Stateの状態と混同せず、次の段階へ進むには対象・plan・権限を独立にレビューする。

| Stage | Terraformが管理するもの | 意図的に含めないもの |
| --- | --- | --- |
| `disabled` | 新resourceなし | 運用中のTerraform State変更 |
| `foundation_metadata` | 新専用SAと3個のSecret metadataのみ | IAM member変更・Secret値・Cloud Run |
| `foundation` | 新専用 user-managed SA `mypage-api`、Secret Managerの3個の**secret metadata**、SA単位のFirebase Custom Token用signer IAM、各secret単位のAccessor IAM | secret version/value、Firestore権限、Cloud Run、公開Invoker |
| `registry`（Devのみ） | 上記8資源 + `asia-southeast2`の専用Docker repository `mypage`。immutable tags、cleanup dry-run、prevent_destroy | image push、Cloud Run、Firestore権限、公開Invoker、Production |
| `runtime_private` | 上記 + `roles/datastore.user` の新SAに対する**additive project IAM member** + Cloud Run v2 API本体 | Cloud Runの `allUsers` invoker、Hosting公開、scheduler/alert |
| `runtime_public` | 上記 + 当該Cloud Runサービスだけに `roles/run.invoker` / `allUsers` memberを追加 | ログイン必須APIのID token/App Check免除、一般告知 |

Cloud Runは `mypage-api`、専用SA/ADC、`min_instance_count=0`、`max_instance_count=1`（Dev）/ `3`（Production）、CPU 1 / RAM 512Mi、port 8080、30秒timeout。

Revision imageは**Artifact Registry digest固定**（`@sha256:...`）、tagやlatestで不用意に差し替えない。実費は起動回数、Secret Manager、ログ量、API呼出等で変わるため、**未計測で月額1ドル未満と断定しない**。<https://cloud.google.com/run/pricing> と <https://cloud.google.com/secret-manager/pricing> を参考に実target/estimateを別途レビューする。

作業履歴の完全性は実データの確認済み区間だけで宣言する。`runtime_settings.history_coverage_json` は**既定が空文字の任意設定**で、空文字ならCloud Runに`MYPAGE_HISTORY_COVERAGE`は存在しない。指定する場合は最大16KiBのJSON配列を渡し、Goの[厳格parser](../../system/cmd/mypage-server/coverage.go)がUTC時刻・順序・重複・上限を起動前に検証する。`[]`や未指定は完全な履歴を表さない。実際のwriterと履歴のRead-only inventoryに基づく区間以外は指定せず、実Deploy/rollbackの設定値と欠損の扱いを照合する。

### Secret / IAM境界

1. Terraformは `mypage-oauth-client-id`、`mypage-oauth-client-secret`、`mypage-youtube-api-key` の**コンテナだけ**を管理する。値および `google_secret_manager_secret_version` を作成・保存しない。Secret versionはowner-approved private手順で投入し、Cloud Runには**番号で固定**して環境変数へ参照する。`latest`は禁止。
2. Server envには `GOOGLE_CLOUD_PROJECT`、`MYPAGE_ENVIRONMENT/REGION/PROJECT_NUMBER/WEB_APP_ID/PUBLIC_ORIGIN/SIGNER_EMAIL/PRIVACY_VERSION/TERMS_VERSION` を渡す。実OAuth token、access/refresh token、Firebase ID tokenをTerraformに流さない。
3. Signer権限は専用SA自身の `roles/iam.serviceAccountTokenCreator`、Secret Accessorは各secret、Firestoreは `roles/datastore.user` を現状project単位で付与する。**Firestore IAMはcollection単位に絞れず、当該権限は他のFirestoreデータにも及ぶ**。対象・権限・D01削除運用を実審査し、より限定的なcustom roleの採否も評価する。運用SAが必要な他権限（Cloud Run deployerの`actAs`、Artifact Registry reader等）はowner inventory確認まで勝手に付けない。
4. `INGRESS_TRAFFIC_ALL` はHosting rewrite/callbackに備えたネットワーク設定であり、**Invoker IAMはruntime_publicまで未付与**。公開時でもAPIのAuth/App Check/ServiceAccessControl/Origin gateを維持する。意図しない直URLからのアクセス、Host/callback、Cookie `__session` の実経路試験は別Gate。Public化はアクセス拡張なので単なるHCL validateでは承認しない。
5. `MYPAGE_INFRASTRUCTURE_READY=true` はGo起動設定の要求値であり、**Ready Gateを技術的に証明したことを意味しない**。人間判断 / 実設定のreadback / 実E2Eのゲートは別に閉じる。

本sourceはGoogle APIsの所有権やFirebase Auth / App Check / Hosting site、IAM deployer、OAuth console client、Artifact Registry repository、Firestore database/index、Cloud Scheduler/metadata cleanup runner、Monitoring receiver、Privacy実削除を作らない。既存repoに宣言済みのAPI `google_project_service` を**二重に管理しない**。それらは最新inventoryを取得し、必要な最小差分を別PRで設計する。

## Credentiallessチェック

GitHub PRでは専用 `mypage-platform.yml` が**読み取り権限だけ**で実行する。S3/GCPへのOIDC、secret、実State、plan/apply、deployは一切使わない。

```bash
bash infra/mypage-platform/scripts/check-local.sh
```

このスクリプトは `terraform fmt -check`、両rootの `init -backend=false -lockfile=readonly`、`validate`、`mock_provider` を使った `terraform test` を行う。安全な合成project番号・appID・digestはtests内だけ。mock planの成功は実GCP planやbudget / E2Eの承認ではない。

## 実適用までの作業と停止条件

1. **Live inventory**：Dev/ProdのSA名、Cloud Runサービス名、Secret ID、Firestore、Firebase Hosting、Google APIsとIAM既存値をread-only収集。既存作成物があれば「新規create」で上書きしない。特に他のTerraform State ownerの有無を確認する。
2. **Backend/権限準備**：MyPage専用S3 keyと `.tflock`、AWS identity、GCP Workload Identityの**正確なref / workflow / target / principal**を確認。GitHub public PRや任意branchには認証を渡さない。[Development Foundation経路](docs/development-foundation.md)は8資源だけを許可する専用workflowとguardを持つ。専用AWS/GCP/GitHub接続とmetadata段階の実適用は成功済み。RuntimeとProductionは引き続き別経路の実装・reviewが必要。
3. **Foundation Dev**：private `backend.hcl` で正確な空Stateを初期化し、レビュー済み `stage="foundation"` の実plan・plan jsonをprivateに保全、差分type/action/IAM/コストを確認。独立reviewと実行承認後にのみ保存済みplanをapply。secret versionはTerraform外でprivate投入し、metadataとIAMをreadback。
4. **Private runtime Dev**：digestを持つ実Goイメージ、project number、Web App ID、origin、規約version、**固定secret version番号**を一致確認。 `runtime_private` へ段階適用。Service起動/keyless signer/Firestore/App Check/Auth/ヘルスと本番同等経路を観測。条件不成立・権限拡張・unknown/driftは停止し、差分をplan修正して再レビュー。
5. **Public runtime Dev**：Hosting `/api/**` rewrite、OAuth callback、CSP/cookie/Originとrate limiting等を実機で検証し、`runtime_public` でInvoker公開を明示承認。公開URLが機密アクセス制御の代替ではない。Frontend Hostingは別デプロイ操作・別release record。
6. **Production**：Dev成立・プライバシー/削除/運用Gateを閉じてから、独立target/state/permission/rollback承認で同順序。**今回Productionへの操作0**。

異常時は先に後続apply/一般告知を停止する。Cloud Runは `deletion_protection` と `prevent_destroy` があるため、事故対応を安易な `destroy` としない。新規Secretの値を消したり、TTLや既存GCP管理資源を巻き戻したりしない。Release対（Frontend commit/Hosting release、Backend image digest/Run revision、同意版、検証証拠）と**前世代互換**を記録し、必要時は前のreview済みimage/revisionへforward-fixまたは明示rollbackする。公開権限の取り消しは別途明示的に実施できる。

DevelopmentのFoundation metadataは実作成・State readback済み。Secret version、Cloud Run/Hosting、OAuth、release gateは未完了。各段階の実施記録は[Development Foundation構築](docs/development-foundation.md)と統合PRへ残す。

### Runtimeの診断と既存イメージの出所

runtime設定の任意 `image_source_sha`（既定は空文字）は、既存の不変tagを作成した40桁commit SHAを示す。空文字なら従来どおり実行元SHAのtagを要求する。別SHAの場合はpublic repositoryからそのcommitを取得し、実行元とbuild元の `system` tree全体が一致することを、Plan/Applyの双方で確認する。Docker contextは `system/` 全体であり、Dockerfile、依存lock、ignore設定とGoソースを含む。tagの作り替え、出所不明digest、アプリ変更後の旧image流用は拒否する。これはinfra診断だけの変更で既存revisionを更新する必要をなくすためで、Cloud Runのupdate/delete/replaceは引き続き拒否する。

Cloud Runのdrift診断は、固定allowlistに含まれる変更field名、その他fieldの件数、planned no-opの真偽だけを公開する。値、未知のfield名、State/Plan本文は公開せず、診断を理由にadmissionを緩めない。private runtimeの初回適用はrun 39566961739で成功した。続くpublic Planは `runtime-drift-address` で変更前に停止し、付与済みの一時権限3件は全て解除した。公開接続とHosting releaseは未実施。

### 初回Cloud Run readの空値補完

実Plan run 39571661136 で、planned no-opのCloud Runに対し、root `annotations` / `labels`、revision templateの同2 fieldがnull→{}、root `custom_audiences`、唯一のcontainerの `args` / `command` / `depends_on` がnull→[]となる8 pathだけを観測した。この8 pathだけを、known・同じschema・同じ固定service boundary・planned afterがrefresh afterと同一の場合に受理する。8 pathを補完したbeforeがafterと完全一致することを要求し、追加field、非空値、逆方向変化、アプリ/認証/secret/scale/traffic変更、unknown、update/delete/replace/importは受理しない。native driftを消さず、HMACには従来どおり含める。

### 公開Invokerをownerが追加し、CIは固定memberだけを取り込む

実Apply run 39572740715 は `run.services.setIamPolicy` のauthorization拒否で停止した。IAM条件の `modifiedGrantsByRole` を持つbindingの配置先はproject/folder/organizationに限られ、Cloud Runサービス上に置く方法は適用できなかった。実readではversion省略でも条件が保持され、providerによる条件消失という仮説は否定された。診断用一時bindingは全て解除済み。

CIの権限を広げず、承認済みの `mypage-api` / `roles/run.invoker` / `allUsers` 1 memberだけをownerがetag保護で追加する。public pipelineはversion3のpolicyを読み、その無条件memberと、その他のpublic broadeningがないことを確認する。private runner workspaceへ固定target/idのimport blockを生成し、Plan/Applyで同じ内容を再現する。public invokerはno-opだけを認め、importがある場合もこのexact idだけに限る。他resource/import id/role/principal/condition、Cloud Runの更新・削除・置換は拒否する。native Plan/HMAC/State version/readback/cleanupは維持。CIにIAM setterを付与しない。既存Stateに取り込み済みの場合、import blockはno-opとなる。

### Hosting転送の固定ホストとowner image更新

2026-10-21にownerのpublic invokerをrun 39587793777で取り込み、12資源・create/updateなしでState readbackを確認。Hosting release `1792580103818000` を公開し、8ファイルの実配信内容一致を確認した。画面は表示できたが、Hosting経由のAPIはOrigin境界で400となったため、OAuth実試験の前に転送先ホストの明示設定を追加する。S01/S04の合格を意味しない。

`MYPAGE_HOSTING_BACKEND_HOST` は実metadataから得たCloud Run hostnameを固定する。未設定時は従来どおり公開Hostのみを認める。設定時も一致するbackend Host、単一の公開 `X-Forwarded-Host`、単一の `X-Forwarded-Proto: https` を要求する。転送情報は認証根拠とせず、Auth/App Check/Origin/cookie検証とIP判定を維持する。

public releaseの4値だけを `environments/dev/runtime-release.json` に記録できる（image_digest/image_source_sha/hosting_backend_host/previous_image_digest）。Secret参照番号・同意版・project等は上書きできない。先にコードをreview/mergeしてそのSHAでimage-buildし、次のsource-only PRでdigestとbuild SHAを固定する。full system tree一致とimmutable digest検証を維持する。

CIに `run.services.update` を与えず、ownerがetag付きで既存serviceのcontainer imageと固定host env 1個だけを更新する。その後のpublic applyはplanned no-opのみを認める。旧digestの明示、同一service境界、env追加1個、Ready/最新revision100%、generation+1、増加したupdate_timeを照合し、観測済みcomputed metadata以外の差を拒否する。native driftはHMACへ含めて独立Applyが再現し、更新ではなくState refreshだけを実行する。
