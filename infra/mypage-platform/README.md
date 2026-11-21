# MyPage Platform Terraform

MyPage の Cloud Run API 基盤・実行権限を管理するための Terraform ルートです。既存の GCP 資源取り込み専用ルート（`infra/gcp`）とは別の State で管理します。

> [!NOTE]
> - 最新の適用・配備・試験状況は [統合 PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236) を参照してください。
> - Development 環境は全構成（API・maintenance Job・公開 Invoker まで）を専用 State で管理しています。以下の構築記録は当時の観測であり、過去の未配備記述から現在状態を判断しないでください（公開前条件は [公開条件](../../docs/mypage/release/README.md) で確認します）。

- アーキテクチャ / API 設計: [Notion 07｜技術アーキテクチャ](https://app.notion.com/p/3ef357a8d0ce81b5892ac1dc2c99e587) / [Notion 08｜API・デプロイ設計](https://app.notion.com/p/3ef357a8d0ce81cbbce9c2ae948ebfb3)
- 関連資料: [GCP Terraform](../gcp/README.md) / [Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191) / [Go サーバー入口](../../system/cmd/mypage-server/main.go) / [OAuth TTL 専用ルート](../mypage-oauth-ttl/README.md) / [実構築記録](docs/development-foundation.md)

---

## State を分離する理由と環境構成

既存の `infra/gcp/environments/{dev,prod}` は Dev 31 件・Prod 30 件の基準資源を `import-only / no-op` 保護のもとで管理しているため、新規リソース追加のために既存の IAM や apply 保護を緩めません。MyPage 用の新規リソースは State キーと実行用本人情報を分離して管理します（OAuth TTL 用の小さな専用 State も既存の plan 仕様のまま維持して統合・移管せず、既存資源との二重管理を見つけた場合は停止します）。

| ルート | ターゲット（Project / Region） | 独立 State キー（本番は未構築） |
| --- | --- | --- |
| `environments/dev` | `test-youtube-study-space` / `asia-southeast2` | `youtube-study-space/mypage-platform/dev/terraform.tfstate` |
| `environments/prod` | `youtube-study-space` / `asia-northeast2` | `youtube-study-space/mypage-platform/prod/terraform.tfstate` |

ツールは Terraform CLI `1.16.4`、Google provider `8.5.0`、S3 native lock と既存 multi-platform provider lock を使用します。Development 専用の AWS backend Plan/Apply role、GCP WIF/構築 SA、GitHub Environment を接続して Plan/Apply と State readback を確認済みです（[Development Foundation 構築](docs/development-foundation.md)を参照し、既存 `infra/gcp` 用 State role は流用しません。非公開 `backend.hcl` と認証情報はコミットしないでください）。

---

## 通常運用と宣言範囲（段階の選択はありません）

通常運用では `Terraform`（`terraform.yml`）の `environment` / `operation=plan|apply` / `component=all|mypage` を使用します。DEV と本番は同じ module を同じ構成で宣言し、段階を選ぶ変数やファイルはありません（旧 `stage` 変数と `desired-stage.txt` は撤去済みで、既存 State のアドレスは `moved` ブロックで no-op のまま移しています）。MyPage は信頼設定が固定する ref（DEV は統合ブランチ、本番は `main`）からだけ実行でき、それ以外の ref から `component=all` を選ぶと Core だけを実行します。

- オーナーの同一リポジトリ PR で MyPage ルートが変更されると DEV を対象に `-lock=false` で PR Plan を実行します。手動 Plan と同じ `mypage-foundation-authenticated.yml` の `foundation_plan` と `mypage-dev-plan` 環境（`mypage-foundation-plan` SA・`mypage-foundation` pool・Plan 用 AWS role）を共用し、Apply は手動 dispatch のみに限定します（DEV trust は 2026-10-31 更新済み、plan contract の判定のみ表示して commitment は残しません）。
- 手動実行の MyPage Plan（Apply 内の Plan 権限ジョブを含む）は Plan 契約・所有権・State バージョンの検証に成功すると、Job Summary に宣言範囲（Runtime / Public invoker）、操作別件数と `Resource type / Actions / Count` 表を表示します。削除を含む場合は受入済み Terraform アドレスと削除件数も出しますが、クラウド側の名前・ID・設定値・outputs・commitment は非公開です（なお `drift` は観測件数であり適用予定の変更件数ではありません。PR Plan は従来の件数表示を維持します）。

### 宣言範囲を決めるのは Terraform 外の事実だけ

Terraform が作れない前提（番号固定の Secret 値、環境別 Registry の image、オーナーが付与する公開 Invoker）がある資源だけは、その前提が実在するときに宣言します。CI はこれを Plan のたびに読み取り専用で判定し、Terraform 変数 `runtime_settings` / `public_invoker` として渡します（利用者が選ぶ入力ではありません）。

| 宣言の条件 | Terraform が管理する範囲 | 判定と停止条件 |
| --- | --- | --- |
| 常に | 専用 SA、3 つの Secret の器と Accessor IAM、SA 自身の Custom Token 用 signer IAM、専用 Docker リポジトリ `mypage`（不変タグ、cleanup dry-run、`prevent_destroy`）の 9 資源 | Secret 値・`google_secret_manager_secret_version` は作らない |
| GitHub Environment の `MYPAGE_RUNTIME_SETTINGS_JSON` がある | 上記 + `roles/datastore.user` メンバー、Cloud Run v2 API（`mypage-api`）、maintenance Job（`mypage-maintenance-operator`）と専用 runtime/invoker SA・Firestore カスタムロール・環境別 WIF・Job IAM・Operator deployer 用の Job IAM。本番はさらに Backend / Operator / Hosting の配備者 SA・WIF pool/provider・権限（21 資源。DEV の同じ本人情報はオーナーのスクリプトが作成済みで State 外） | Plan 前に image の不変タグと 3 つの Secret 版が `ENABLED` であることを確認する。固定名の API が既に実在するのに設定が無い場合は `runtime-settings-required` で停止し、実行系を消す Plan は作らない |
| 固定名の API が実在する | 上記 + 当該 Cloud Run サービスのみへの `roles/run.invoker`（`allUsers`）を固定 ID の `import` で State へ取り込む | オーナーの付与が無い・条件付き・`allAuthenticatedUsers` を含む場合は `runtime-owner-public-invoker` で停止する。Terraform がこのメンバーを新規作成する経路は無い |

- Cloud Run は `mypage-api`・専用 SA / ADC で、`min_instance_count=0`、`max_instance_count=1`（DEV）/ `3`（PROD）、CPU 1 / RAM 512Mi、port `8080`、30 秒 timeout に設定します。
- イメージは Artifact Registry のダイジェスト（`@sha256:...`）で固定し、タグや `latest` で不用意に差し替えません。**初回作成後の API Service と maintenance Job のコンテナイメージは CI/CD が所有し、Terraform は `lifecycle.ignore_changes` で変更しません。** 初回作成時は Service も Job も `runtime_settings.image_digest`（API の bootstrap image）で作られ、以降 API は [Backend CI 手順](../../docs/mypage/release/backend-ci.md)、Job は `Deploy Maintenance Operator` がそれぞれ独立して更新し、同じ digest へ同期しません（通常リリースごとに Terraform Plan/Apply や digest 変更 PR は実行しません）。なお実費は起動回数・Secret Manager・ログ量・API 呼び出しなどで変わるため未計測で月額 1 ドル未満とは断定せず、[Cloud Run 料金](https://cloud.google.com/run/pricing)と [Secret Manager 料金](https://cloud.google.com/secret-manager/pricing)を参考に別途試算・レビューします。

### Secret と IAM の境界

1. Terraform は `mypage-oauth-client-id` / `mypage-oauth-client-secret` / `mypage-youtube-api-key` の器のみを管理し、値や `google_secret_manager_secret_version` は作成しません。Secret 値は非公開手順で投入した番号固定バージョンを環境変数へ参照させます（`latest` 禁止）。
2. サーバー環境変数には `GOOGLE_CLOUD_PROJECT` および `MYPAGE_ENVIRONMENT` / `REGION` / `PROJECT_NUMBER` / `WEB_APP_ID` / `PUBLIC_ORIGIN` / `SIGNER_EMAIL` を渡し、実 OAuth トークンや Firebase ID トークンは Terraform に流しません。同意版はソースの定数（[`policy.go`](../../system/core/mypage/policy.go)）で管理し、環境変数では渡しません。
3. Signer 権限は専用 SA 自身の `roles/iam.serviceAccountTokenCreator`、Secret Accessor は各 secret、Firestore は `roles/datastore.user` をプロジェクト単位で付与します。現状 Firestore IAM はコレクション単位に絞れないため他データにも権限が及びます（D01 削除運用と合わせて実審査し、より限定的なカスタムロールの採否を評価します。運用 SA の他権限もオーナー確認まで勝手に付けません）。
4. `INGRESS_TRAFFIC_ALL` は Hosting 転送用です。公開 Invoker はオーナーの付与を取り込むだけで、公開時もアプリ側の Auth / App Check / SAC / Origin 検証を維持し、直 URL・Host/callback・Cookie `__session` の実経路試験は別条件とします。また `MYPAGE_INFRASTRUCTURE_READY=true` は起動時の要求フラグであり、公開準備の完了証明ではありません。

※本ルートは Google APIs の所有権、Firebase Auth / App Check / Hosting site、DEV の配備者（Backend / Operator / Hosting）の SA・WIF、Cloud Run Service 上の配備者バインディング（両環境）、OAuth コンソールクライアント、Artifact Registry の既存資源、Firestore DB/インデックス、Cloud Scheduler/cleanup runner、監視通知先、Privacy 実削除は作成せず、既存 `google_project_service` と二重管理しません。

---

## 認証情報を使わないローカル検査

GitHub PR では必須 `CI Gate` 配下の `CI GCP Terraform` が MyPage 基盤の認証情報不要の検査も実行します（独立 `mypage-platform.yml` は廃止して 1 経路に集約しました）。ローカルでは以下を実行でき、安全な合成値・`mock_provider` を用いて `terraform fmt -check`・両ルートの `init -backend=false -lockfile=readonly`・`validate`・`terraform test` を行います（実 GCP Plan・予算・E2E の承認ではありません）。

```bash
bash infra/mypage-platform/scripts/check-local.sh
```

---

## 本番の初回構築（`main` から）

本番も DEV と同じ宣言を `main` から Plan/Apply します。コード変更や dev→main の往復を挟まず、オーナーが Terraform 外の前提をそろえて同じ `Terraform` ワークフローを再実行するだけで宣言範囲が広がります。各 Apply の前には必ず `operation=plan` の Job Summary（宣言範囲・作成件数・資源種別）を確認してください。

### 0. Terraform を動かす前にオーナーが用意するもの

1. Live inventory 確認: 本番プロジェクトの既存 SA・Cloud Run・Secret・Artifact Registry・Firestore・Hosting・Google APIs・IAM と他の Terraform State 所有者を読み取り専用で確認します（固定名の同名資源が State 外にあれば Plan の所有権検査が停止します。上書きのための import はしません）。
2. State と実行用本人情報: 専用 S3 キー（`youtube-study-space/mypage-platform/prod/terraform.tfstate`）、Plan/Apply 用 AWS role（`youtube-study-space-mypage-prod-plan` / `-apply`）、GCP WIF pool `mypage-foundation` / provider `github-actions` と `mypage-foundation-plan` / `mypage-foundation-apply` SA を [Development Foundation 構築](docs/development-foundation.md) と同じ形で本番に作ります。信頼条件は `MYPAGE_TARGET=production python3 infra/mypage-platform/scripts/foundation_contract.py provider-condition` と `... trust-policy <AWS アカウント ID> plan|apply` が出力する `main` 固定の値を使います（本番は PR Plan を受け付けません）。
3. GitHub Environments: `mypage-prod-plan` / `mypage-prod-apply`（対象ブランチ `main`、Apply は required reviewers 必須）と、削除を承認する場合だけ使う `mypage-prod-delete-approval`（reviewer 必須・管理者バイパス無効）を作り、`MYPAGE_STATE_BUCKET` / `MYPAGE_STATE_ACCOUNT` / `MYPAGE_BACKEND_ROLE_ARN` / `MYPAGE_BACKEND_ROLE_ID` / `MYPAGE_GCP_WIF_PROVIDER` / `MYPAGE_GCP_SERVICE_ACCOUNT` / `MYPAGE_PLAN_HMAC_KEY` を設定します。この時点では `MYPAGE_RUNTIME_SETTINGS_JSON` を**設定しません**。`mypage-prod-apply` は本番 image build（`Deploy MyPage API` / `Deploy Maintenance Operator`）でも使うため、`mypage-foundation-apply` SA に本番 Registry `mypage` への push 権限を与えます。
4. 配備用 GitHub Environments: `mypage-prod-backend` / `mypage-prod-operator` / `mypage-prod-hosting` を作ります（対象ブランチ `main` のみ・タグ不可、承認者なし・管理者バイパス無効。Secret は置きません。本番 image build は承認者必須の `mypage-prod-apply` で動くため、ビルドを伴う配備では承認が 1 回入ります）。変数は手順 4 で設定します。
5. 実行用本人情報の権限: 手順 3 の 2 回目の Apply は配備者の SA・WIF pool/provider・カスタムロール・IAM メンバーを作るため、`mypage-foundation-apply` にそれらの作成権限と、作成した SA・Registry `mypage`・runtime / maintenance SA・maintenance Job・プロジェクトの IAM ポリシー編集権限を DEV の構築時と同じ形で（作成用は一時的に）付与します。Plan SA には各資源と IAM ポリシーの読み取り権限を与えます。Cloud Run Service の IAM 変更権限は CI に付与しません。
6. Google APIs: MyPage Terraform は API を有効化しません。本番プロジェクトで `secretmanager.googleapis.com`（Secret の器の作成に必要）、`firebaseappcheck.googleapis.com`・`recaptchaenterprise.googleapis.com`（App Check）、`apikeys.googleapis.com`（YouTube API キーの発行・制限）を有効にします。DEV で有効なものと突き合わせ、ほかの API は既存の Core Terraform の所有範囲を変えないよう個別に追加しません。
7. Firebase / OAuth: 本番 Firebase Web アプリ（`project_number=906336399194`、`public_origin=https://moku.work`）、Auth / App Check、Hosting サイト、Google OAuth クライアント（コールバック URL）と YouTube API キーを用意します（Terraform 管理外）。

> [!IMPORTANT]
> 本番の MyPage Environment を用意するまでは、本番の `Terraform` を `component=core` で実行してください。`component=all` は `main` から MyPage Plan も実行するため、未設定の間は MyPage Plan が失敗して Core Apply にも進みません。

### 1. 1 回目の Apply（基盤 9 資源）

`Terraform` を `environment=production` / `component=mypage` で `main` から `operation=plan` → `operation=apply` します。runtime settings が無いため、専用 SA・3 つの Secret の器・Accessor/signer IAM・Artifact Registry `mypage` の 9 資源だけを作成します。

### 2. Terraform 外の前提をそろえる

1. 3 つの Secret（`mypage-oauth-client-id` / `mypage-oauth-client-secret` / `mypage-youtube-api-key`）に値を非公開手順で追加し、各版番号を控えます（値は Terraform・GitHub・ログに置きません）。
2. `Deploy MyPage API` を `main` から `environment=production`・`mode=build-only` で実行し、本番 Artifact Registry `mypage/mypage-api` に API の不変タグ（40 桁のソース SHA）付き image を push して digest を控えます（Service がまだ無いため配備ジョブは起動しません）。
3. `mypage-prod-plan` と `mypage-prod-apply` の両方に `MYPAGE_RUNTIME_SETTINGS_JSON`（`image_digest`・`image_source_sha`・`project_number`・`web_app_id`・`public_origin`・3 つの Secret 版番号）を同じ内容で設定します（同意版は設定しません。旧 `privacy_version` / `terms_version` が残っていても読み込み時に除外され、それ以外の未知のキーは拒否されます）。`hosting_backend_host` は Service の作成後に決まるため、この時点では設定しません（手順 4）。

### 3. 2 回目の Apply（実行系）

同じ `Terraform` を再実行します。CI は image タグと Secret 版を読み取り確認したうえで、`roles/datastore.user`、Cloud Run API、maintenance Job（初回は API image で作成。API image には Operator のバイナリが無いため、手順 5 の Operator 配備までは実行できません）、maintenance 用 SA・カスタムロール・WIF・Job IAM、Backend / Operator / Hosting の配備者の SA・WIF pool/provider（`main` 固定の条件）・権限を作成します。API はまだ存在しなかったため、公開 Invoker は宣言しません。

### 4. ホスト名の固定・Service 上のバインディング・配備用変数と 3 回目の Apply

1. Hosting 転送ホストの固定: 作成された `mypage-api` の `uri` から `*.a.run.app` ホスト名を読み取り、オーナーが Service に環境変数 `MYPAGE_HOSTING_BACKEND_HOST=<ホスト名>` だけを追加します（image・他の環境変数・Secret 参照・権限は変えない新しいリビジョン）。同じ値を `MYPAGE_RUNTIME_SETTINGS_JSON` の `hosting_backend_host` に（`mypage-prod-plan` / `mypage-prod-apply` の両方へ）追加します。次の Plan はこの 1 変数の追加だけを State の refresh として受け入れ、更新は計画しません（他の差分が混ざれば停止します）。Backend / Hosting の配備スクリプトは、この環境変数と Service の `uri` の一致を要求します。
2. オーナーが `mypage-api` サービスに次の 3 つ（いずれも条件なし）を `etag` 保護で追加します。CI に Service の IAM 変更権限を渡さないため、どれも Terraform では作りません。
   - `roles/run.invoker` / `allUsers`（1 メンバーのみ）
   - `projects/youtube-study-space/roles/mypageBackendRelease` / `serviceAccount:mypage-backend-deploy@youtube-study-space.iam.gserviceaccount.com`
   - `roles/run.viewer` / `serviceAccount:mypage-hosting-deploy@youtube-study-space.iam.gserviceaccount.com`
3. 配備用 Environment に変数を設定します（いずれも公開値で Secret ではありません。`MYPAGE_HOSTING_BACKEND_HOST` は手順 4-1 と同じ値）。
   - `mypage-prod-backend`: `MYPAGE_BACKEND_WIF_PROVIDER`（`projects/906336399194/locations/global/workloadIdentityPools/mypage-backend/providers/github-actions`）、`MYPAGE_HOSTING_BACKEND_HOST`
   - `mypage-prod-operator`: `MYPAGE_OPERATOR_WIF_PROVIDER`（`.../workloadIdentityPools/mypage-operator/providers/github-actions`）
   - `mypage-prod-hosting`: `MYPAGE_HOSTING_WIF_PROVIDER`（`.../workloadIdentityPools/mypage-hosting/providers/github-actions`）、`MYPAGE_HOSTING_BACKEND_HOST`、本番の `MYPAGE_HOSTING_PUBLIC_CONFIG_JSON`
4. 同じ `Terraform` を再実行して公開 Invoker を State へ取り込みます（import と手順 4-1 の refresh のみで、ほかの変更は 0 件）。API の作成後は、公開 Invoker の付与が無い Plan は停止します。

### 5. アプリケーションの初期化と配備

各配備は `main` から `environment=production` で実行し、Run の記録（`*-release-record` / `*-verified-record`）を残します。失敗した Run でも配備済みの場合があるため、記録と実資源を読み戻してから次へ進みます（[Backend CI の本番の配備](../../docs/mypage/release/backend-ci.md#本番の配備)）。

1. Operator: `Deploy Maintenance Operator` で maintenance Job を専用 Operator image へ更新し、generation 前進と設定不変を読み戻します。
2. `config/mypage` 初期化: `Set Maintenance Mode`（`mypage-maintenance.yml`）を `environment=production`・`enabled=false` で実行します（[運用手順](../../docs/mypage/maintenance-mode.md)。GitHub Environment `mypage-maintenance-production` の作成が前提です）。
3. Frontend: `Deploy MyPage Frontend` で初回の Hosting 公開を行います。前提として、`mypage/src/public-policy-config.ts` の `POLICY_EFFECTIVE_DATE` が設定済みであること（本番の公開設定検査が必須とする）と、Hosting サイト `study-space` への `moku.work` の接続と TLS 証明書の発行が完了していること（公開後の確認は `https://moku.work` に対して行う）を確かめます。Backend の配備後確認は Hosting 経由の配信を前提とし、未公開のサイトでは書き込み前に停止するため、初回だけは Terraform が bootstrap image で作成した API に対して Frontend を先に公開します。
4. Backend: `main` が手順 2-2 と同じ SHA のままなら、`Deploy MyPage API` を `mode=deploy-existing` で、手順 2-2 の digest と source SHA を指定して実行します（同じ SHA の不変タグは作り直せないため `build-and-deploy` は `image-tag-already-exists` で停止します。Service は既にその image のため書き込みはせず、読み戻しと匿名 smoke だけを行います）。`main` が進んでいれば `mode=build-and-deploy` を使います。2 回目以降のリリースは Backend → Frontend の順です（Hosting `/api/**` 転送・OAuth コールバック・CSP / Cookie / Origin / rate limiting の実機確認と公開判断は [公開条件](../../docs/mypage/release/README.md) に従います）。

> [!CAUTION]
> 異常時はまず後続 apply や告知を停止してください。Cloud Run には `deletion_protection` と `prevent_destroy` があるため安易に `destroy` せず、Secret 値や TTL・既存資源の巻き戻しも行いません。リリース対（Frontend commit・Backend digest・同意版・検証証拠）と前世代互換を記録し、既知リビジョンへの forward-fix または明示ロールバックで対処します。

### Runtime の診断と既存イメージの出所

- `image_source_sha`（既定は空文字）は、不変タグを作成した 40 桁コミット SHA を指定する任意設定です（空なら実行元 SHA を要求）。配信イメージは CI/CD で独立更新されるため `system/` ツリーとの完全一致は求めませんが、Docker コンテキストは `system/` 全体であり、指定 SHA が現 HEAD の祖先かつ同じ不変タグが bootstrap digest を指すことを検証します（インフラ診断だけの変更でリビジョン更新を不要にするため、tag の作り替えや Cloud Run の更新・置換は拒否します）。
- Maintenance Operator のイメージは `Deploy Maintenance Operator` が所有し、API リリースとは同じ digest へ同期しません。Job を初めて作るときだけ `runtime_settings.image_digest` を使います（旧 `maintenance_operator_*` 3 キーは移行互換のため読み捨て、private tfvars や Plan へは渡しません）。

---

## Plan guard の責任分担と運用実績

MyPage の削除を含む Apply は、起動時の `allow_mypage_deletions` と Plan 生成後の GitHub Environment 承認が必要です（手順は [CI/CD ワークフロー](../../docs/development/ci-cd-workflows.md#32-terraform-plan--apply) を参照）。削除は MyPage State 内の単独 `delete` に限定し、置き換え・削除保護付き資源・計測切替との混在は拒否します。

Terraform のリソーススキーマ・computed 属性・`etag`・空 map/list 補完・Job 実行件数・リビジョン観測値は Terraform / provider に任せ、Python 側に全フィールドの allowlist は持ちません（provider 更新時は Terraform の lock とネイティブテストを更新し、Python へ computed フィールド名を追記しません）。refresh は既知資源・planned no-op・planned after 一致のみに限定し、計測設定を含むセキュリティ設定や固定属性の drift 拒否を維持しつつ native Plan 全体と drift を HMAC に含めて Apply へ結び付けます。診断では drift 資源数と planned write 数のみ表示し、Plan 値や未知フィールド名は公開しません。

### 公開 Invoker の取り込み方式

実 Apply [run 39572740715](https://github.com/kani3camp/youtube-study-space/actions/runs/39572740715) が `run.services.setIamPolicy` の認可拒否で停止し、条件付き IAM 制約（`modifiedGrantsByRole`）もプロジェクト階層限定で Cloud Run サービス上に置けないことが判明しました（実読み取りでは version 省略でも条件が保持され、一時診断用バインディングは解除済み）。

そこで CI に IAM setter 権限を付与せず、承認済みの `mypage-api` / `roles/run.invoker` / `allUsers`（1 メンバーのみ）をオーナーが `etag` 保護で追加し、public pipeline が version 3 ポリシーを検証したうえで固定 ID の `import` ブロックをランナー上に生成して State へ取り込みます（import 以外の更新・削除・置換・他メンバーは拒否し、取り込み後は no-op となります）。

### Hosting 転送ホストの固定とオーナー更新履歴

初期接続時に 2026-10-21 の [run 39587793777](https://github.com/kani3camp/youtube-study-space/actions/runs/39587793777) で公開 Invoker を State へ取り込んで 12 資源（create/update なし）を確認し、Hosting release `1792580103818000` を公開して 8 ファイルの一致を確認しました。画面表示後に Hosting 経由 API が Origin 境界で `400` となったため、`MYPAGE_HOSTING_BACKEND_HOST` で実メタデータの Cloud Run ホスト名を固定し、一致する backend Host・単一の `X-Forwarded-Host`・単一の `X-Forwarded-Proto: https` を要求するよう追加しました（転送情報は認証根拠とせず、Auth / App Check / Origin / Cookie 検証を維持）。

なお 2026-10 の旧方式で使っていた 4 値のリリース設定ファイルは削除済み（Git 履歴のみ保持）であり、以降の通常アプリ更新は [Backend CI 手順](../../docs/mypage/release/backend-ci.md) の専用 deployer が v1 `resourceVersion` 付き PUT でイメージのみ更新します（初回接続時のみ固定ホスト環境変数 1 個を追加し、以降は環境変数・Secret・IAM 等を不変のまま維持して Terraform apply を行いません）。オーナー更新後に `mypage-api-00002-qcs` の Ready と App Check 欠落 `403`・不正 Origin `400`・method `405` を確認した際、[run 39592888606](https://github.com/kani3camp/youtube-study-space/actions/runs/39592888606) の Plan は自動命名時の空 `template.revision` により `runtime-owner-refresh-ready` で停止しましたが、更新前後とも空の場合のみ許可し、固定サービスの最新 Ready/Created 一致・成功 condition・最新 100% トラフィック・`generation+1` の検証は維持しています。

### S04 実測後の DEV リクエスト URL 保存抑止

2026-10-21 11:16 UTC に合成コールバックで `code`/`state` クエリが Cloud Run リクエストログに残ることを確認したため（実 OAuth は未実施）、オーナーが既存 sink 一覧とフィルタ確認のうえ `_Default` sink に `mypage-api` の callback URL とクエリ付き API URL のみを除外する exclusion（`mypage-sensitive-request-url`）を手動適用し、読み戻しと別合成マーカーで再確認しました（Terraform State 外・JSON 非コード管理、DEV のみ）。

クエリなしの通常 API リクエストやアプリ/セキュリティ/監査/システムログ、他サービス/プロジェクトは対象外であり、既存の合成記録も保持期間中は既存閲覧境界内に残ります。この局所修正だけで Hosting/CDN や実 OAuth を含む全層の S04 完了とは扱いません（実 Auth・実データ削除・本番は対象外）。
