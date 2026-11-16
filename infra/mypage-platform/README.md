# MyPage Platform Terraform

MyPage の Cloud Run API 基盤・実行権限を管理するための Terraform ルートです。既存の GCP 資源取り込み専用ルート（`infra/gcp`）とは別の State で管理します。

> [!NOTE]
> - 最新の適用・配備・試験状況は [統合 PR #1236](https://github.com/kani3camp/youtube-study-space/pull/1236) を参照してください。
> - 2026-10-21 JST に Development 環境の専用 SA と 3 つの Secret メタデータを作成して専用 State に保存済みですが、以下の構築記録は各段階の観測であり、過去の未配備記述から現在状態を判断しないでください（公開前条件は [公開条件](../../docs/mypage/release/README.md) で確認します）。

- **アーキテクチャ / API 設計**: [Notion 07｜技術アーキテクチャ](https://app.notion.com/p/3ef357a8d0ce81b5892ac1dc2c99e587) / [Notion 08｜API・デプロイ設計](https://app.notion.com/p/3ef357a8d0ce81cbbce9c2ae948ebfb3)
- **関連資料**: [GCP Terraform](../gcp/README.md) / [Issue #1191](https://github.com/kani3camp/youtube-study-space/issues/1191) / [Go サーバー入口](../../system/cmd/mypage-server/main.go) / [OAuth TTL 専用ルート](../mypage-oauth-ttl/README.md) / [実構築記録](docs/development-foundation.md)

---

## State を分離する理由と環境構成

既存の `infra/gcp/environments/{dev,prod}` は Dev 31 件・Prod 30 件の基準資源を `import-only / no-op` 保護のもとで管理しているため、新規リソース追加のために既存の IAM や apply 保護を緩めません。MyPage 用の新規リソースは、State キーと実行用本人情報を分離して管理します（OAuth TTL 用の小さな専用 State もそのまま維持し、ここへは統合しません。既存資源との二重管理を見つけた場合は停止します）。

| ルート | ターゲット（Project / Region） | 独立 State キー（本番は未構築） |
| --- | --- | --- |
| `environments/dev` | `test-youtube-study-space` / `asia-southeast2` | `youtube-study-space/mypage-platform/dev/terraform.tfstate` |
| `environments/prod` | `youtube-study-space` / `asia-northeast2` | `youtube-study-space/mypage-platform/prod/terraform.tfstate` |

- **ツールバージョン**: Terraform CLI `1.16.4`、Google provider `8.5.0`、S3 native lock および既存 multi-platform provider lock を使用します（非公開 `backend.hcl` と認証情報はコミットしないでください）。

---

## 通常の desired state と内部 stage

通常運用では `Terraform`（`terraform.yml`）の `environment` / `operation=plan|apply` / `component=all|mypage` を使用します。MyPage はレビュー済みの `environments/{dev,prod}/desired-stage.txt`（現在 DEV=`runtime_public`、本番=`disabled`）を解決して固定で Plan/Apply するため、利用者が内部 stage を手動選択する必要はありません。

- **PR Plan（自動）**: オーナーの同一リポジトリ PR で MyPage ルートが変更されると、DEV の desired state を対象に `-lock=false` で PR Plan を実行します（手動 Plan と同じ `mypage-foundation-authenticated.yml` の `foundation_plan` と `mypage-dev-plan` 環境を使用し、Apply は手動 dispatch のみ）。
- **手動 Plan の Summary**: Plan 契約・所有権・State バージョンの検証に成功すると、Job Summary に操作別件数と `Resource type / Actions / Count` 表を表示します（削除を含む場合は Terraform アドレスと削除件数も表示。クラウド側の名前・ID・設定値・outputs・commitment は非公開）。なお、`drift` は観測件数であり適用予定の変更件数ではありません。

### 内部 stage 一覧（初回構築・契約制御用の内部定義）

ソースの既定値 `stage = "disabled"` はモジュール件数 `0` を意味します（実 State の状態と混同しないでください）。本番が `disabled` の間は認証情報不要で Core GCP のみを運用できます。

| Stage | Terraform が管理する範囲 | 意図的に含めないもの |
| --- | --- | --- |
| `disabled` | 新規リソースなし | State 変更 |
| `foundation_metadata` | 専用 SA と 3 つの Secret メタデータのみ | IAM メンバー変更・Secret 値・Cloud Run |
| `foundation` | 上記 + Secret ごとの Accessor IAM、SA 自身の Custom Token 用 signer IAM | Secret バージョン/値、Firestore 権限、Cloud Run、公開 Invoker |
| `registry` | 上記 8 資源 + 専用 Docker リポジトリ `mypage`（不変タグ、cleanup dry-run、`prevent_destroy`） | イメージ push、Cloud Run、Firestore 権限、公開 Invoker |
| `maintenance_operator` | 上記 + 固定 digest の単一 maintenance Job、専用 runtime/invoker SA、Firestore カスタムロール、環境別 WIF | Cloud Run API 作成/更新、制御文書の暗黙作成、一般 Invoker、GitHub runner の Firestore 権限 |
| `maintenance_upgrade` | 既存 `runtime_public` を no-op に保ちつつ、maintenance Job と専用 IAM/WIF を追加 | 既存 Cloud Run API・Firestore メンバー・公開 Invoker の変更 |
| `runtime_private` | 上記 + `roles/datastore.user` メンバー + Cloud Run v2 API（`mypage-api`）本体 | `allUsers` invoker、Hosting 公開、scheduler / alert |
| `runtime_public` | 上記 + 当該 Cloud Run サービスのみへの `roles/run.invoker`（`allUsers`） | ログイン必須 API の認証免除、一般告知 |

- **Cloud Run の設定**: 専用 SA / ADC、`min_instance_count=0`、`max_instance_count=1`（DEV）/ `3`（PROD）、CPU 1 / RAM 512Mi、port `8080`、30 秒 timeout。
- **イメージ管理の分担**: **初回作成後の API Service と maintenance Job のコンテナイメージは CI/CD が所有し、Terraform は `lifecycle.ignore_changes` で変更しません。** API は [Backend CI 手順](../../docs/mypage/release/backend-ci.md)、Job は `Deploy Maintenance Operator` がそれぞれ独立して更新します。

### Secret と IAM の境界

1. **Secret はコンテナのみ管理**: Terraform は `mypage-oauth-client-id` / `mypage-oauth-client-secret` / `mypage-youtube-api-key` の器のみを管理し、値は保存しません。Cloud Run には非公開手順で投入した番号固定バージョンを参照させます（`latest` 禁止）。
2. **サーバー環境変数**: `GOOGLE_CLOUD_PROJECT` および `MYPAGE_ENVIRONMENT` / `REGION` / `PROJECT_NUMBER` / `WEB_APP_ID` / `PUBLIC_ORIGIN` / `SIGNER_EMAIL` / `PRIVACY_VERSION` / `TERMS_VERSION` を渡します（OAuth トークン等は Terraform に流しません）。
3. **Firestore IAM の範囲**: `roles/datastore.user` はプロジェクト単位でありコレクション単位に絞れないため、他データにも権限が及びます（より限定的なカスタムロールの採否は実審査で評価）。
4. **公開境界**: `INGRESS_TRAFFIC_ALL` は Hosting 転送用であり、`runtime_public` まで Invoker IAM は付与しません（公開時もアプリ側の Auth / App Check / SAC / Origin 検証を維持）。また `MYPAGE_INFRASTRUCTURE_READY=true` は起動時の要求フラグであり、Ready Gate 完了の証明ではありません。

※本ルートは Google APIs の所有権、Firebase Auth / App Check / Hosting site、OAuth コンソールクライアント、Artifact Registry の既存資源、Firestore DB/インデックス、Cloud Scheduler、監視通知先などは作成しません。

---

## 認証情報を使わないローカル検査

GitHub PR では必須 `CI Gate` 配下の `CI GCP Terraform` が MyPage 基盤の検査も実行します。ローカルでは以下を実行できます（`mock_provider` を使用するため実認証情報や apply は行いません）。

```bash
bash infra/mypage-platform/scripts/check-local.sh
```

---

## 初回 bootstrap・公開準備の流れと停止条件

日常運用では `Terraform` の `operation=plan` で差分を確認し、承認した差分だけを `operation=apply` します。以下は空環境から構築する際の段階と停止条件です。

1. **Live inventory**: DEV / PROD の既存 SA・Cloud Run・Secret・Firestore・Hosting・Google APIs・IAM と他の State 所有者の有無を読み取り専用で確認します（既存作成物があれば上書きしません）。
2. **Backend / 権限準備**: 専用 S3 キーと `.tflock`、AWS / GCP WIF の ref・workflow・target・principal を設定します（本番経路は `mypage-prod-plan/apply` の対象ブランチを `main` のみに固定。設定と実 Plan/Apply は未実施）。
3. **Foundation Dev**: 非公開 `backend.hcl` で空 State を初期化し、`stage="foundation"` の Plan 確認後に保存済み Plan を適用して、Secret 値を Terraform 外から投入・読み戻します。
4. **Maintenance 準備 Dev**: `maintenance_operator` または `maintenance_upgrade` で単一 Job と専用 IAM/WIF を作成し、`enabled=false` で `config/mypage` を初期化したうえで Backend と Frontend を配備します（[運用手順](../../docs/mypage/maintenance-mode.md)）。
5. **Private runtime Dev**: digest 固定イメージ・設定・番号固定 Secret バージョンを一致確認して `runtime_private` へ適用し、稼働とヘルスを確認します。
6. **Public runtime Dev**: Hosting `/api/**` 転送・OAuth コールバック・CSP / Cookie / Origin / rate limiting を実機検証し、`runtime_public` で公開 Invoker を明示承認します。
7. **Production**: DEV 確認後、独立したターゲット・State・権限・ロールバック承認を得て同じ順序で実施します。

> [!CAUTION]
> 異常時はまず後続 apply や告知を停止してください。Cloud Run には `deletion_protection` と `prevent_destroy` があるため安易に `destroy` せず、Secret 値や TTL・既存資源の巻き戻しも行いません。前世代互換を保ちつつ既知リビジョンへの forward-fix または明示ロールバックで対処します。

### Runtime の診断と既存イメージの出所

- `image_source_sha`（既定は空文字）は、不変タグを作成した 40 桁コミット SHA を指定する任意設定です（空なら実行元 SHA を要求）。配信イメージは CI/CD で独立更新されるため `system/` ツリーとの完全一致は求めませんが、指定 SHA が現 HEAD の祖先であり、かつ同じ不変タグが bootstrap digest を指すことを検証します（インフラ診断だけの変更でリビジョン更新を不要にするため）。
- Maintenance Operator のイメージは `Deploy Maintenance Operator` が所有します。DEV 初回 Job 作成時のみ、分離前ビルドの固定 source SHA の `mypage-api` 不変タグを Plan 時に読み取り専用解決して一時入力に使います（旧 `maintenance_operator_*` 3 キーは移行互換のため読み捨て）。

---

## Plan guard の責任分担と運用実績

MyPage の削除を含む Apply は、起動時の `allow_mypage_deletions` と Plan 生成後の GitHub Environment 承認が必要です（手順は [CI/CD ワークフロー](../../docs/development/ci-cd-workflows.md#32-terraform-plan--apply) を参照）。削除は MyPage State 内の単独 `delete` に限定し、置き換え・削除保護付き資源・計測切替との混在は拒否します。

- **Python 契約と native Plan の分担**: リソーススキーマ・computed 属性・`etag`・空 map/list 補完・Job 実行件数などは Terraform / provider に任せ、Python 側に全フィールドの allowlist は持ちません。refresh は既知資源・planned no-op・planned after 一致のみに限定し、native Plan 全体と drift は HMAC に含めて Apply へ結び付けます。
- **公開 Invoker の取り込み方式**: Cloud Run サービス上の条件付き IAM 制約により、承認済みの `mypage-api` / `roles/run.invoker` / `allUsers`（1 メンバーのみ）はオーナーが `etag` 保護で追加し、public pipeline がポリシーを検証したうえで `import` ブロックにより State へ取り込みます（CI に IAM setter 権限は付与しません）。
- **Hosting 転送ホストの固定**: `MYPAGE_HOSTING_BACKEND_HOST` に実メタデータの Cloud Run ホスト名を固定し、一致する backend Host・単一の `X-Forwarded-Host`・`X-Forwarded-Proto: https` を要求します（2026-10-21 の確認により追加）。
- **S04 実測後の DEV リクエスト URL 保存抑止**: 2026-10-21 に合成コールバックで `code`/`state` クエリが Cloud Run リクエストログに残ることを確認したため、オーナーが `_Default` sink に callback URL・クエリ付き API URL を除外する exclusion（`mypage-sensitive-request-url`）を手動適用しました（Terraform State 外、DEV のみ。全層の S04 完了を示すものではありません）。
