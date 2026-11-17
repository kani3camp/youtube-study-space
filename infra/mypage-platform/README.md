# MyPage Platform Terraform

MyPage の Cloud Run API 基盤・実行権限を管理するための Terraform ルートです。既存の GCP 資源取り込み専用ルート（`infra/gcp`）とは別の State で管理します。

> [!NOTE]
> - 最新の適用・配備・試験状況は [統合 PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236) を参照してください。
> - 2026-10-21 JST に Development 環境の専用 SA と 3 つの Secret メタデータを作成して専用 State に保存済みですが、以下の構築記録は各段階の観測であり、過去の未配備記述から現在状態を判断しないでください（公開前条件は [公開条件](../../docs/mypage/release/README.md) で確認します）。

- アーキテクチャ / API 設計: [Notion 07｜技術アーキテクチャ](https://app.notion.com/p/3ef357a8d0ce81b5892ac1dc2c99e587) / [Notion 08｜API・デプロイ設計](https://app.notion.com/p/3ef357a8d0ce81cbbce9c2ae948ebfb3)
- 関連資料: [GCP Terraform](../gcp/README.md) / [Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191) / [Go サーバー入口](../../system/cmd/mypage-server/main.go) / [OAuth TTL 専用ルート](../mypage-oauth-ttl/README.md) / [実構築記録](docs/development-foundation.md)

---

## State を分離する理由と環境構成

既存の `infra/gcp/environments/{dev,prod}` は Dev 31 件・Prod 30 件の基準資源を `import-only / no-op` 保護のもとで管理しているため、新規リソース追加のために既存の IAM や apply 保護を緩めません。MyPage 用の新規リソースは State キーと実行用本人情報を分離して管理します（OAuth TTL 用の小さな専用 State も既存の plan 仕様のまま維持して統合・移管せず、既存資源との二重管理を見つけた場合は停止します）。

| ルート | ターゲット（Project / Region） | 独立 State キー（本番は未構築） |
| --- | --- | --- |
| `environments/dev` | `test-youtube-study-space` / `asia-southeast2` | `youtube-study-space/mypage-platform/dev/terraform.tfstate` |
| `environments/prod` | `youtube-study-space` / `asia-northeast2` | `youtube-study-space/mypage-platform/prod/terraform.tfstate` |

ツールは Terraform CLI `1.16.4`、Google provider `8.5.0`、S3 native lock と既存 multi-platform provider lock を使用します。Development 専用の AWS backend Plan/Apply role、GCP WIF/構築 SA、GitHub Environment を接続してメタデータ段階の Plan/Apply と State readback まで確認済みです（[Development Foundation 構築](docs/development-foundation.md)を参照し、既存 `infra/gcp` 用 State role は流用しません。非公開 `backend.hcl` と認証情報はコミットしないでください）。

---

## 通常の desired state と内部 stage

通常運用では `Terraform`（`terraform.yml`）の `environment` / `operation=plan|apply` / `component=all|mypage` を使用します。MyPage はレビュー済みの `environments/{dev,prod}/desired-stage.txt`（現在 DEV=`runtime_public`、本番=`disabled`）を解決して固定で Plan/Apply するため、利用者が内部 stage を手動選択する必要はありません。

- オーナーの同一リポジトリ PR で MyPage ルートが変更されると DEV の desired state を対象に `-lock=false` で PR Plan を実行します。手動 Plan と同じ `mypage-foundation-authenticated.yml` の `foundation_plan` と `mypage-dev-plan` 環境（`mypage-foundation-plan` SA・`mypage-foundation` pool・Plan 用 AWS role）を共用し、Apply は手動 dispatch のみに限定します（DEV trust は 2026-10-31 更新済み、plan contract の判定のみ表示して commitment は残しません）。
- 手動実行の MyPage Plan（Apply 内の Plan 権限ジョブを含む）は Plan 契約・所有権・State バージョンの検証に成功すると、Job Summary に操作別件数と `Resource type / Actions / Count` 表を表示します。削除を含む場合は受入済み Terraform アドレスと削除件数も出しますが、クラウド側の名前・ID・設定値・outputs・commitment は非公開です（なお `drift` は観測件数であり適用予定の変更件数ではありません。PR Plan は従来の件数表示を維持します）。

### 内部 stage 一覧（初回構築・契約制御用の内部定義）

ソースの既定値 `stage = "disabled"` はモジュール件数 `0`・新規 GCP 資源 `0` を意味します（実 State の状態と混同しないでください）。本番が `disabled` の間は認証情報不要で Core GCP のみを運用でき、本番 MyPage を進める際は `desired-stage.txt` の変更自体を PR でレビューして Environment・IAM・State の準備と同じ変更単位で扱います。

| Stage | Terraform が管理する範囲 | 意図的に含めないもの |
| --- | --- | --- |
| `disabled` | 新規リソースなし | State 変更 |
| `foundation_metadata` | 専用 SA と 3 つの Secret メタデータのみ | IAM メンバー変更・Secret 値・Cloud Run |
| `foundation` | 上記 + Secret ごとの Accessor IAM、SA 自身の Custom Token 用 signer IAM | Secret バージョン/値、Firestore 権限、Cloud Run、公開 Invoker |
| `registry` | 上記 8 資源 + 専用 Docker リポジトリ `mypage`（不変タグ、cleanup dry-run、`prevent_destroy`） | イメージ push、Cloud Run、Firestore 権限、公開 Invoker |
| `maintenance_operator` | 上記 + 固定 digest の単一 maintenance Job、専用 runtime/invoker SA、Firestore カスタムロール、環境別 WIF | Cloud Run API 作成/更新、制御文書の暗黙作成、一般 Invoker、GitHub runner の Firestore 権限 |
| `maintenance_upgrade` | 既存 `runtime_public` を no-op に保ちつつ、maintenance Job と専用 IAM/WIF を追加 | 既存 Cloud Run API・Firestore メンバー・公開 Invoker の作成・更新・削除・置換 |
| `runtime_private` | 上記 + `roles/datastore.user` メンバー + Cloud Run v2 API（`mypage-api`）本体 | `allUsers` invoker、Hosting 公開、scheduler / alert |
| `runtime_public` | 上記 + 当該 Cloud Run サービスのみへの `roles/run.invoker`（`allUsers`） | ログイン必須 API の認証免除、一般告知 |

- Cloud Run は `mypage-api`・専用 SA / ADC で、`min_instance_count=0`、`max_instance_count=1`（DEV）/ `3`（PROD）、CPU 1 / RAM 512Mi、port `8080`、30 秒 timeout に設定します。
- イメージは Artifact Registry のダイジェスト（`@sha256:...`）で固定し、タグや `latest` で不用意に差し替えません。**初回作成後の API Service と maintenance Job のコンテナイメージは CI/CD が所有し、Terraform は `lifecycle.ignore_changes` で変更しません。** API は [Backend CI 手順](../../docs/mypage/release/backend-ci.md)、Job は `Deploy Maintenance Operator` がそれぞれ独立して更新し、同じ digest へ同期しません（通常リリースごとに Terraform Plan/Apply や digest 変更 PR は実行しません）。なお実費は起動回数・Secret Manager・ログ量・API 呼び出しなどで変わるため未計測で月額 1 ドル未満とは断定せず、[Cloud Run 料金](https://cloud.google.com/run/pricing)と [Secret Manager 料金](https://cloud.google.com/secret-manager/pricing)を参考に別途試算・レビューします。

### Secret と IAM の境界

1. Terraform は `mypage-oauth-client-id` / `mypage-oauth-client-secret` / `mypage-youtube-api-key` の器のみを管理し、値や `google_secret_manager_secret_version` は作成しません。Secret 値は非公開手順で投入した番号固定バージョンを環境変数へ参照させます（`latest` 禁止）。
2. サーバー環境変数には `GOOGLE_CLOUD_PROJECT` および `MYPAGE_ENVIRONMENT` / `REGION` / `PROJECT_NUMBER` / `WEB_APP_ID` / `PUBLIC_ORIGIN` / `SIGNER_EMAIL` を渡し（同意版は API イメージ内の定数）、実 OAuth トークンや Firebase ID トークンは Terraform に流しません。
3. Signer 権限は専用 SA 自身の `roles/iam.serviceAccountTokenCreator`、Secret Accessor は各 secret、Firestore は `roles/datastore.user` をプロジェクト単位で付与します。現状 Firestore IAM はコレクション単位に絞れないため他データにも権限が及びます（D01 削除運用と合わせて実審査し、より限定的なカスタムロールの採否を評価します。運用 SA の他権限もオーナー確認まで勝手に付けません）。
4. `INGRESS_TRAFFIC_ALL` は Hosting 転送用であり、`runtime_public` まで Invoker IAM は付与しません（公開時もアプリ側の Auth / App Check / SAC / Origin 検証を維持し、直 URL・Host/callback・Cookie `__session` の実経路試験は別条件とします）。また `MYPAGE_INFRASTRUCTURE_READY=true` は起動時の要求フラグであり、公開準備の完了証明ではありません。

※本ルートは Google APIs の所有権、Firebase Auth / App Check / Hosting site、IAM deployer、OAuth コンソールクライアント、Artifact Registry の既存資源、Firestore DB/インデックス、Cloud Scheduler/cleanup runner、監視通知先、Privacy 実削除は作成せず、既存 `google_project_service` と二重管理しません。

---

## 認証情報を使わないローカル検査

GitHub PR では必須 `CI Gate` 配下の `CI GCP Terraform` が MyPage 基盤の認証情報不要の検査も実行します（独立 `mypage-platform.yml` は廃止して 1 経路に集約しました）。ローカルでは以下を実行でき、安全な合成値・`mock_provider` を用いて `terraform fmt -check`・両ルートの `init -backend=false -lockfile=readonly`・`validate`・`terraform test` を行います（実 GCP Plan・予算・E2E の承認ではありません）。

```bash
bash infra/mypage-platform/scripts/check-local.sh
```

---

## 初回 bootstrap・公開準備の流れと停止条件

日常運用では `Terraform` の `operation=plan` で両 State の差分を確認し、承認した差分だけを `operation=apply` します。以下は空環境から構築する際の段階と停止条件です。

1. Live inventory 確認: DEV / PROD の既存 SA・Cloud Run・Secret・Firestore・Hosting・Google APIs・IAM と他の Terraform State 所有者の有無を読み取り専用で確認します（既存作成物があれば新規作成で上書きしません）。
2. Backend / 権限準備: 専用 S3 キーと `.tflock`、AWS / GCP WIF の正確な ref・workflow・target・principal を確認します（[Development Foundation 経路](docs/development-foundation.md)は 8 資源のみ許可する専用検査を持ち、本番経路は `mypage-prod-plan/apply` の対象ブランチを `main` に固定して registry・image-build・maintenance operator/upgrade のみ許可しますが、本番の Environment・Secret・State 設定と実 Plan/Apply は未実施です）。
3. Foundation Dev: 非公開 `backend.hcl` で空 State を初期化し、`stage="foundation"` の実 Plan と JSON を非公開保全して差分・IAM・コストを確認したうえ、独立レビューと承認後に保存済み Plan を適用して Secret 値を Terraform 外から投入・読み戻します。
4. Maintenance 準備 Dev: `maintenance_operator` または既存 runtime を保持する `maintenance_upgrade` で単一 Job と専用 IAM/WIF を作成します（固定名の既存資源を事前照会し、State 外に同名資源があれば停止）。`enabled=false` で `config/mypage` を初期化し、Job の読み戻しを確認したうえで Backend と Frontend を配備します（[運用手順](../../docs/mypage/maintenance-mode.md)）。
5. Private runtime Dev: digest 固定イメージ・設定・番号固定 Secret バージョンを一致確認して `runtime_private` へ適用し、稼働・signer・Firestore・App Check・Auth と本番同等経路を観測します（条件不成立・権限拡張・drift は停止して修正・再レビュー）。
6. Public runtime Dev: Hosting `/api/**` 転送・OAuth コールバック・CSP / Cookie / Origin / rate limiting を実機検証し、`runtime_public` で公開 Invoker を明示承認します。Frontend Hosting は Terraform とは独立した配備ですが DEV では専用 Hosting ジョブで build/apply/readback を完結させます。
7. Production: DEV 確認後、独立したターゲット・State・権限・ロールバック承認を得て同じ順序で実施します。

> [!CAUTION]
> 異常時はまず後続 apply や告知を停止してください。Cloud Run には `deletion_protection` と `prevent_destroy` があるため安易に `destroy` せず、Secret 値や TTL・既存資源の巻き戻しも行いません。リリース対（Frontend commit・Backend digest・同意版・検証証拠）と前世代互換を記録し、既知リビジョンへの forward-fix または明示ロールバックで対処します。

### Runtime の診断と既存イメージの出所

- `image_source_sha`（既定は空文字）は、不変タグを作成した 40 桁コミット SHA を指定する任意設定です（空なら実行元 SHA を要求）。配信イメージは CI/CD で独立更新されるため `system/` ツリーとの完全一致は求めませんが、Docker コンテキストは `system/` 全体であり、指定 SHA が現 HEAD の祖先かつ同じ不変タグが bootstrap digest を指すことを検証します（インフラ診断だけの変更でリビジョン更新を不要にするため、tag の作り替えや Cloud Run の更新・置換は拒否します）。
- Maintenance Operator のイメージは `Deploy Maintenance Operator` が所有し、API リリースとは同じ digest へ同期しません。DEV 初回 Job 作成時のみ、分離前ビルドの固定 source SHA の `mypage-api` 不変タグを Plan 時に読み取り専用解決して一時入力に使います（旧 `maintenance_operator_*` 3 キーは移行互換のため読み捨て、private tfvars や Plan へは渡しません）。

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
