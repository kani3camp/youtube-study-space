# CI / Terraform / Deploy / Operation workflow のユースケース設計

GitHub Actions のワークフローを「人間が実行したいユースケース」単位で整理する設計ドキュメントです。
[Issue #1500](https://github.com/kani3camp/youtube-study-space/issues/1500) の棚卸し・目標設計・移行順序をここで管理し、移行 PR では本書の「[5. 移行状況](#5-移行状況)」表を合わせて更新します。

> [!NOTE]
> - 本書は **設計と移行の現在地** を記録します。実クラウドでの成否は対象 Run の実行結果で確認してください。
> - 本書の記述と実際のコードが異なる場合は、`integration/mypage-canon-20261006` 上の現行コード・ワークフロー・GitHub / GCP / AWS の実設定を優先します。
> - AWS CDK の手動デプロイは対象外とし、挙動を変更しません。

---

## 1. 現行の trust model（移行計画の前提）

資格情報を使う経路は、すべて **OIDC トークンの `workflow_ref`（最上位ワークフロー）と `job_workflow_ref`（reusable workflow）** に固定されています。
そのため、ワークフローファイルの改名や新設はコード変更だけで完結せず、クラウド側の trust 更新を伴います。

以下は 2026-11-01 時点の固定値です（DEV・本番ともに `ci.yml` からの呼び出しは削除済み）。

| 境界 | 現行の固定値 | 定義場所 |
| --- | --- | --- |
| Repository OIDC `sub` テンプレート | `repository_id`, `repository_owner_id`, `environment`, `ref`, `workflow_ref`, `job_workflow_ref`, `event_name`（リポジトリ単位設定） | GitHub `actions/oidc/customization/sub` |
| Core Terraform GCP WIF | `workflow_ref == terraform.yml@ref`、`job_workflow_ref == gcp-terraform-authenticated.yml@ref`、`workflow_dispatch`（DEV は owner の PR Plan も許可）、owner actor、手動時は `run_attempt == 1`、DEV PR Plan は正整数の attempt | [`bootstrap_normal_ops.py`](../../infra/gcp/scripts/bootstrap_normal_ops.py) / [`terraform_normal_ops.py`](../../infra/gcp/scripts/terraform_normal_ops.py) |
| Core Terraform AWS State role | 上記 `sub` との完全一致 | [`terraform-authentication-boundary.md`](../../infra/gcp/docs/terraform-authentication-boundary.md) |
| MyPage foundation GCP WIF / AWS State role | `terraform.yml@integration ref`（image-build は `deploy-mypage-api.yml` / `deploy-mypage-maintenance.yml`）+ `mypage-foundation-authenticated.yml@integration ref`、`workflow_dispatch`（DEV Plan は owner の PR Plan も許可） | [`foundation_contract.py`](../../infra/mypage-platform/scripts/foundation_contract.py) |
| MyPage Backend deployer WIF | `deploy-mypage-api.yml` + `mypage-backend-authenticated.yml`、`workflow_dispatch`、Environment `mypage-dev-backend` | [`backend_identity.py`](../../infra/mypage-platform/scripts/backend_identity.py) |
| MyPage Hosting deployer WIF | `deploy-mypage-frontend.yml` + `mypage-hosting-authenticated.yml`、`workflow_dispatch`、Environment `mypage-dev-hosting` | [MyPage 配備手順](../mypage/release/README.md) |
| Maintenance invoker WIF | `workflow_ref == mypage-maintenance.yml@<env ref>`（最上位ワークフロー直接）、Environment `mypage-maintenance-<env>` | [`modules/runtime/main.tf`](../../infra/mypage-platform/modules/runtime/main.tf) |

### 新ワークフローへ移行する際の必須手順

1. **新ワークフローを追加する**（旧経路は有効なまま残す）。
2. **クラウド側の trust へ新 claim を追加する**（オーナーが GCP WIF provider condition / AWS role trust policy / GitHub Environment を更新）。
3. **新経路を実 Run で 1 回検証する**。
4. **旧経路をコードから撤去し、クラウド側の trust から旧 claim を削除する**。

> [!IMPORTANT]
> 資格情報を取得するジョブは、必ず **reusable workflow の中** に配置してください（最上位ワークフローで直接認証しない）。
> GitHub の仕様上、`job_workflow_ref` は「reusable workflow を使うジョブ」に対してのみ定義されており、AWS State role はこの値を含む `sub` の完全一致で照合するためです。最上位ワークフローで直接認証している現行の `mypage-maintenance.yml` も、認証ジョブを reusable workflow へ移します。

※手順 2 と 4 はリポジトリ外の操作であるため、各 PR の説明に「必要なオーナー作業」と「実施済みかどうか」を明記してください（trust を広げる場合は、追加する claim と脅威境界も同じ PR で説明します）。

---

## 2. 現行 workflow 棚卸し

> [!NOTE]
> この節は **移行前（#1501 時点）の棚卸し** です。「移行先」列には当初計画を残し、実際の結果を `(#n)` で追記しています。現在の構成は「[3. 目標構成](#3-目標構成)」と「[5. 移行状況](#5-移行状況)」を参照してください。

### 2.1 分類

| Workflow | 移行前のユースケース | 方針 | 移行先・現在の扱い |
| --- | --- | --- | --- |
| [`ci.yml`](../../.github/workflows/ci.yml) | PR CI、`workflow_dispatch` の操作ルーター、および `system/**` push 時の MyPage DEV 自動 CD | **残す（縮小済み）** | UC1 通常 CI へ整理済み。push 自動 CD と全操作ルーターは撤去（#3、#4、schema audit は #8）。`workflow_dispatch` は入力なしの再検査のみ |
| `ci-operations.yml` | 操作ルーター本体（CI 結果の前提検査と下位ワークフロー呼び出し） | **廃止（#8 で削除）** | 各ユースケースワークフローへ分解し、最後に残った schema audit とともに削除 |
| `ci-system.yml` / `ci-frontend.yml` / `ci-tools.yml` / `ci-infra.yml` / `ci-repository.yml` | UC1 認証なし検査 | **残す** | 変更なし（`ci-repository.yml` の契約テスト対象のみ追随） |
| [`ci-gcp-terraform.yml`](../../.github/workflows/ci-gcp-terraform.yml) | UC1 Terraform fmt / validate / mock・native 認証なしテスト | **残す** | UC1 のまま維持（実 State の Plan は UC2 へ分離） |
| `gcp-infrastructure-authenticated.yml` | Core + MyPage の Plan / Apply 合成（常に両方実行） | **統合→廃止（#4 で削除）** | `terraform.yml`（UC2/UC3）へ統合 |
| [`gcp-terraform-authenticated.yml`](../../.github/workflows/gcp-terraform-authenticated.yml) | Core ルートの認証付き Plan / Apply | **残す（縮小）** | `terraform.yml` から呼ぶ Core 用 reusable workflow（到達不能 mode は #4 時点では維持、後に #1521 で整理） |
| `mypage-infrastructure-authenticated.yml` | MyPage ルートの `desired-stage` 解決 | **統合→廃止（#4 で削除）** | `desired-stage` 解決を `terraform.yml` の `mypage` component へ移動 |
| [`mypage-foundation-authenticated.yml`](../../.github/workflows/mypage-foundation-authenticated.yml) | MyPage ルートの認証付き Plan / Apply と image build（2 責務同居） | **残す（同居のまま）** | trust 変更を最小にするため `image-build` mode（`image_component` 入力付き）として維持し、各 Deploy ワークフローから呼び出し（group `mypage-<target>-foundation` で Terraform と直列化） |
| [`mypage-hosting-authenticated.yml`](../../.github/workflows/mypage-hosting-authenticated.yml) | UC4 DEV Hosting の build / deploy / smoke | **残す（最上位化）** | `deploy-mypage-frontend.yml` から呼び出し |
| [`mypage-backend-authenticated.yml`](../../.github/workflows/mypage-backend-authenticated.yml) | API Service と maintenance Job の同一 digest 同期デプロイ | **分割** | `deploy-mypage-api.yml`（Service のみ）と `deploy-mypage-maintenance.yml`（Job のみ）へ分割 |
| [`mypage-maintenance.yml`](../../.github/workflows/mypage-maintenance.yml) | UC7 メンテナンスモードの ON / OFF | **残す（認証分離）** | 最上位ワークフロー名は維持し、認証ジョブを reusable workflow へ分離して WIF condition に `job_workflow_ref` を追加 |
| `gcp-user-activity-schema-audit.yml` | 読み取り専用 schema audit（`false` で無効化済み） | **撤去（#8）** | 2026 年の一回限りの監査（PR #1238）用だったため撤去。結果は [`user-activity-history-canonicalization.md`](../../infra/gcp/docs/user-activity-history-canonicalization.md) に保存 |
| `codeql-*`、`deploy-docs.yml`、`base-image-update-report.yml`、`formal-spec-report.yml` | 本 Issue の対象外 | 対象外 | 変更なし |

### 2.2 資格情報付き workflow の棚卸し詳細

| Workflow | Trigger / caller | GitHub Environment（棚卸し時点） | GCP / AWS identity | Write 対象 | Concurrency | Source SHA trust | 記録 / 成果物 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `gcp-infrastructure-authenticated.yml` | `ci.yml` dispatch → `ci-operations.yml` | なし（下位ジョブが取得） | 下位に委譲 | 下位に委譲 | dev: `mypage-dev-runtime-change`、prod: `gcp-infrastructure-production` | `expected_sha == github.sha` | なし |
| `gcp-terraform-authenticated.yml` | 上記から `normal-plan` / `normal-apply` | `terraform-<dev\|prod>-plan` / `-apply`（reviewer なし。policy: dev=`dev`,integration、prod=`main`） | GCP: `terraform-<target>-normal-plan` / `-apply` SA<br>AWS: 末尾 `-plan` / `-apply` role | Core GCP 資源・Core State | `gcp-terraform-authenticated-<target>` | preflight で actor / ref / `workflow_ref` / SHA 検査、OIDC 再検査 | HMAC digest をジョブ出力比較（値は非出力） |
| `mypage-infrastructure-authenticated.yml` → `mypage-foundation-authenticated.yml` | 上記から `desired-plan` / `desired-apply` | `mypage-dev-plan` / `-apply`（integration のみ、reviewer なし。prod 未作成） | GCP: `mypage-foundation-plan` / `-apply` SA<br>AWS: `MYPAGE_BACKEND_ROLE_ARN` | MyPage GCP 資源・MyPage State | `mypage-<target>-foundation` | `foundation_contract.context` | commitment 比較 |
| `mypage-foundation-authenticated.yml`（image-build） | `ci.yml` dispatch / integration push | `mypage-dev-apply` | GCP: `mypage-foundation-apply` SA（AWS 不使用） | Artifact Registry `mypage/mypage-api:<sha>` | `mypage-<target>-foundation` | `image_ops.preflight` | `image_digest` ジョブ出力 |
| `mypage-backend-authenticated.yml` | `ci.yml` dispatch / integration push 自動 CD | `mypage-dev-backend` | `mypage-backend-deploy` SA | Cloud Run Service `mypage-api`（当時は Job も） | `mypage-dev-runtime-change` | `backend_release.context`（ロールバックは祖先のみ） | `mypage-backend-release-record`（#2 で schema 4 へ） |
| `mypage-hosting-authenticated.yml` | `ci.yml` dispatch | `mypage-dev-hosting` | `mypage-hosting-deploy` SA | Firebase Hosting DEV サイト | `mypage-dev-hosting-release` | `hosting_release.context` | `mypage-dev-hosting-release-record` |
| `mypage-maintenance.yml` | 直接 `workflow_dispatch` | `mypage-maintenance-<env>`（DEV のみ、policy/reviewer なし） | `mypage-maintenance-invoker` SA | Cloud Run Job 実行（メンテナンス制御） | `mypage-maintenance-<env>` | WIF condition（`workflow_sha == sha`、固定 ref） | Job 実行ログのみ |

※`gcp-terraform-authenticated.yml` では Plan identity に書き込み権限がないことを `terraform_normal_ops.py` で検査しています。

### 2.3 撤去・縮小できる legacy / migration-only 要素

| 要素 | 撤去・縮小の根拠 | 現在の扱い |
| --- | --- | --- |
| `gcp-terraform-authenticated.yml` の `migration-plan` / `migration-apply` と `terraform_migration.py` 認証経路 | どの caller からも渡されない | **撤去済み**（dev の #1521、統合ブランチへは #1526 で取り込み）。`terraform.yml` は `normal-plan` / `normal-apply` のみ呼ぶ |
| 同 `normal-e2e` mode | caller なし | **撤去済み**（同上） |
| `mypage-foundation-authenticated.yml` の `foundation-*` / `registry-*` / `maintenance-*` / `runtime-*` 等 | caller は `desired-plan` / `desired-apply` / `image-build` のみ | **維持（到達不能）**。初回構築の内部契約としてのみ保持し、`terraform.yml` からは呼ばない |
| `ci.yml` の push 自動 CD（`mypage-auto-cd`）と `backend_cd.py` | アプリケーションデプロイを自動化しない方針 | **撤去済み（#3）**。Backend WIF condition からも `push` を削除（オーナー作業） |
| Backend release での Service / Job 同一 digest 同期（schema 3） | API と Operator を独立配備単位にするため | **撤去済み（#2）**。record は schema 4（API のみ） |
| `mypage-maintenance.yml` の未固定 `actions/checkout@v7` | 他ワークフローは SHA でピン留め済み | **SHA 固定済み（#6）** |

### 2.4 棚卸し時点の既知ギャップと対応状況

- **Production Terraform Apply の承認未設定**: `terraform-prod-apply` に required reviewers がありません（branch policy `main` のみ）。UC3 完了にはオーナーによる reviewer 設定が必要です。
- **PR Plan 用の trust 不足（解消済み）**: 棚卸し時点ではどの trust も `workflow_dispatch` 固定でしたが、#5 で PR Plan を追加しオーナー作業で接続しました。
- **Maintenance WIF provider の `google.subject` 長さ上限超過（コード修正済み・DEV Apply 待ち）**: リポジトリのカスタム `sub`（DEV 約 449 バイト / PROD 約 361 バイト）は Google STS の上限 127 バイトを超えるため、このままではトークン交換が拒否されます。2026-10-31 に他 identity と同じ `repository_id:environment:actor_id`（約 50 バイト）へ変更し、condition に `job_workflow_ref` を追加する遷移（`reviewed_provider_update`）を実装済みです（DEV への適用には Apply 時に一時的な `iam.workloadIdentityPoolProviders.update` 権限が必要）。
- **Maintenance mode の認証未成功**: 唯一の実行（2026-10-29、run 40879914039）は Environment `mypage-maintenance-development` に `MYPAGE_MAINTENANCE_WIF_PROVIDER` が未設定で停止しました（`mypage-maintenance-production` も未作成）。DEV の資源・Operator イメージは 2026-10-31 に配備済みのため、残る作業は (1) 上記 `google.subject` 修正の DEV Apply、(2) Environment の設定、(3) 実 Run 検証です。
- **`gcp-user-activity-schema-audit` 撤去**: branch policy が `dev` のみで統合ブランチから到達不能かつ無効化済みだったため、#8 で撤去しました。

---

## 3. 目標構成

### 3.1 利用者に見せる workflow

| UC | Workflow | Trigger | 主な入力 | 対象環境 |
| --- | --- | --- | --- | --- |
| **UC1** | `ci.yml`（`CI`） | `pull_request` / `workflow_dispatch`（再検査のみ） | なし | なし |
| **UC2 / UC3** | `terraform.yml`（`Terraform`） | `pull_request`（Plan のみ） / `workflow_dispatch` | `environment=development\|production`、`operation=plan\|apply`、`component=all\|core\|mypage` | dev / prod |
| **UC4** | `deploy-mypage-frontend.yml`（`Deploy MyPage Frontend`） | `workflow_dispatch` | なし（development のみ） | dev |
| **UC5** | `deploy-mypage-api.yml`（`Deploy MyPage API`） | `workflow_dispatch` | `mode=build-and-deploy\|deploy-existing`、`image_digest`、`image_source_sha` | dev |
| **UC6** | `deploy-mypage-maintenance.yml`（`Deploy Maintenance Operator`） | `workflow_dispatch` | 同上 | dev |
| **UC7** | `mypage-maintenance.yml`（`Set Maintenance Mode`） | `workflow_dispatch` | `environment`、`enabled` | dev / prod |

- **内部用 reusable workflow の扱い**: `gcp-terraform-authenticated.yml` 等には `workflow_dispatch` を持たせません。
- **本番 Deploy 選択肢の制限**: Production の Frontend / API / Operator デプロイは、公開 Gate（[#1133](https://github.com/kani3camp/youtube-study-space/issues/1133)）を満たすまで入力選択肢に出しません。また、PR や `push` を契機とした自動デプロイは行いません。
- **デプロイ前の必須検査**: 各 Deploy ワークフローは、資格情報を取得する前に配備対象ドメインの認証なし検査を同じ Run 内で実行します（API / Operator は `ci-system.yml`、Frontend は `ci-frontend.yml` の `mypage`）。Ruleset が要求する PR ごとの `CI Gate` は統合後のツリーそのものを検査しないため、対象ドメインの検査で担保します。

### 3.2 Terraform Plan / Apply

#### PR Plan（UC2 自動）

- **起動と対象判定**: `pull_request` で起動し、変更パスから component を自動判定します（`infra/gcp/**` → core、`infra/mypage-platform/**` → mypage、共有の CI スクリプト/ワークフロー変更 → 両方）。対象は DEV のみです（PROD は `main` 限定のため手動 Plan）。
- **fork PR の拒否**: fork 由来 PR に OIDC / secret を渡さないことに加え、ジョブ条件 `head.repo.full_name == github.repository` を満たさない場合はスキップ（false-green）にせず **失敗として理由を表示** します。
- **手動 Plan との identity 共通化**: 2026-10-31 のオーナー判断により、DEV の手動 Plan と同じ reusable・Environment（`terraform-dev-plan` / `mypage-dev-plan`）・identity を使います（PR 専用 identity は持たず、`Apply` 用 Environment/identity は手動 dispatch のみ）。
  - WIF / AWS trust は手動 dispatch 条件に PR 用条件を OR で追加しています（`event_name == 'pull_request'`、`refs/pull/<n>/merge`、Plan Environment 限定、共通の repository/owner/actor、PR 節のみ正整数の `run_attempt` を許可）。
  - オーナーが最初に起動した DEV PR Plan に限り、GitHub の write 権限保持者による **Re-run jobs / Re-run failed jobs** を許可します（PR 経路では `GITHUB_TRIGGERING_ACTOR` を制限せず、初回 `actor_id` と署名済み `run_attempt` を検証。他者起動 Run の再実行や、新規コミットによる `synchronize`（attempt 1）とは区別）。
  - `-lock=false` で実行するため、手動 Plan/Apply を待たせません（Apply は必ず fresh Plan を作成）。
- **脅威境界と出力制限**: PR ではワークフロー定義が PR 作成者の管理下にあるため、実行できる actor をオーナー（`actor_id == 54093651`）に限定しています。協力者の PR では PR Plan を失敗させ、オーナーが手動 Plan を実行します。結果は Job Summary と check に値を含まない要約（resource type・操作・件数）のみ出し、plan ファイル・JSON・PR コメントは残しません。
- **残るリスク**: クラウド trust の `actor` は初回起動者を示すため、この運用は信頼する write 権限保持者に「オーナー起動の DEV Plan の再実行」を委ねる前提です（協力者自身の PR 起動や手動 Plan/Apply、PROD には適用しません）。

#### 手動 Plan / Apply（UC2 手動 / UC3）

- **二段階の整合検証**: 「plan ジョブ → apply ジョブ内で独立して再 Plan → 保存した Plan とソース・State の HMAC digest 一致時のみ `terraform apply <plan-file>`」の方式を維持します（PR の plan ファイルは使いません）。
- `source SHA`・`environment`・`component` は dispatch 入力と `github.sha` で固定し、Production Apply では `terraform-prod-apply`（将来は `mypage-prod-apply`）に required reviewers を設定して承認必須とします。

#### MyPage の削除を含む Plan

1. `Terraform` を `operation=plan`、`component=mypage` で実行し、Job Summary に出る対象環境・Terraform アドレス・削除件数を確認します（単独の `delete` のみ許可し、置き換えや削除保護付き資源は拒否）。
2. 適用時は `operation=apply` に加えて `allow_mypage_deletions` をオンにします（既定オフ時は削除を含む Plan で停止。`component=all` で Core に変更がある場合は Core Apply が先に進むため、MyPage の削除だけを行うときは `component=mypage` を使用）。
3. 同じ Run の **Review MyPage Terraform deletion Plan** が承認待ちになったら、MyPage Plan の要約を確認して Actions の **Review deployments** で承認します（30 分以内に再生成した Plan と HMAC 一致した保存済み Plan のみ適用。却下・期限切れ・不一致時は停止）。

※初回実行前に GitHub Settings → Environments で `mypage-dev-delete-approval`（本番有効化時は `mypage-prod-delete-approval`）を作成し、Required reviewers 登録・管理者バイパス無効化・Deployment branches 固定（DEV: `integration/mypage-canon-20261006`、PROD: `main`）を設定してください（1 人運用の場合は自分を reviewer にして Prevent self-review をオフ。Secrets やクラウド権限は置かず、承認記録専用として使います）。

### 3.3 API と Maintenance Operator のイメージ分離

| 項目 | API | Maintenance Operator |
| --- | --- | --- |
| **Dockerfile** | `system/Dockerfile.mypage-api`（`./cmd/mypage-server` のみ） | `system/Dockerfile.mypage-maintenance-operator`（`./cmd/mypage-maintenance-operator` のみ） |
| **Artifact Registry** | `mypage/mypage-api@sha256:...` | `mypage/mypage-maintenance-operator@sha256:...`（同じ `mypage` リポジトリの別パッケージ） |
| **Deploy 対象** | Cloud Run Service `mypage-api` のみ | Cloud Run Job `mypage-maintenance-operator` のみ |
| **Deployer identity** | `mypage-backend-deploy`（Job 権限・maintenance SA `actAs` は削除済み） | `mypage-operator-deploy`（Job `run.jobs.get/update` と maintenance runtime SA `actAs` のみ） |
| **Release record** | `mypage-backend-release-record`（schema 4、API digest のみ） | `mypage-operator-release-record`（schema 1、Operator digest のみ） |
| **Terraform** | Service イメージは `ignore_changes` | Job イメージは `ignore_changes`（初回作成用 `maintenance_bootstrap_image_digest` のみ旧 `mypage-api` 固定ビルド） |

- 「API の更新で Job を変更しない」ことは、API deployer の IAM から Job 権限を外すことで強制しています（DEV は 2026-11-01 に削除完了。旧 `mypage-api` パッケージを新規環境の初期値として許容する仕様のみ維持）。
- `core/mypage` の `MaintenanceControl` や Firestore スキーマを変える PR では、互換性を保つか両方のデプロイが必要であることを PR 説明に明記してください（平常時は同一 digest へ同期しません）。

### 3.4 Rollback

- **API / Operator**: 各 Deploy ワークフローの `mode=deploy-existing` で検証済み不変 digest と `source SHA` を指定します（パッケージ・Registry 存在・タグと digest 一致・現 HEAD の祖先チェックを通過した対象リソースのみ更新）。
- **Frontend**: 現行の Hosting release record に保持された直前リリース/バージョン情報をもとに、Frontend 移行 PR で決定する方式で復旧します。

### 3.5 Concurrency（同時実行制御）

GitHub Actions の concurrency group は 1 ジョブ 1 つのため、競合リスクに応じて以下のように最小限の排他を設定しています。

| Group | 対象操作 | 排他の理由 |
| --- | --- | --- |
| `gcp-terraform-authenticated-<target>` | Core Terraform plan / apply（reusable 内） | 同一 State・同一資源の保護 |
| `mypage-<env>-cloud-run` | 手動 `Terraform` Run 全体、API deploy、Operator deploy | Terraform の refresh + PATCH が途中のイメージ更新を巻き戻すのを防ぐため（環境単位のロック） |
| `mypage-<env>-hosting` | Frontend deploy（`mypage-hosting-authenticated.yml`） | Hosting サイトのみ変更 |
| `mypage-maintenance-<env>` | Maintenance mode 切替 | Firestore 制御文書のみ変更（Job 実行は開始時点定義を使うため Operator deploy と排他しない） |
| `terraform-pr-plan-<pr>`<br>（reusable 内: `gcp-terraform-pr-plan-<pr>` / `mypage-pr-plan-<pr>`） | PR Plan（`cancel-in-progress: true`） | `-lock=false` の助言用であり、手動 Plan/Apply を待たせない |

※移行完了後の #7 で旧グループ（`mypage-dev-runtime-change`、`mypage-dev-hosting-release`、`gcp-infrastructure-production`）から上記へ改名済みです。また、各デプロイスクリプトでも `resourceVersion` / `etag` / 再読み取りによる楽観的ロックを維持しています。

### 3.6 Actions の実行保護（オーナー設定）

GitHub Settings → Actions → Policies（2026-10-31 設定）で、ワークフローの起動者・イベントを制限しています（確認コマンド: `gh api repos/kani3camp/youtube-study-space/actions/policies`）。

| Policy | 対象ワークフロー | 許可する actor / event |
| --- | --- | --- |
| Allow base image report `pull_request_target` | `base-image-update-report.yml` | owner と `dependabot[bot]` / `pull_request_target` |
| Restrict Terraform execution | `terraform.yml` | owner / `pull_request`・`workflow_dispatch` |
| Restrict manual MyPage operations | `deploy-mypage-api.yml`・`deploy-mypage-frontend.yml`・`deploy-mypage-maintenance.yml`・`mypage-maintenance.yml` | owner / `workflow_dispatch` |
| Protect authenticated reusable workflows | 資格情報を扱う reusable（`gcp-terraform-authenticated.yml`・`mypage-*-authenticated.yml`） | owner / `workflow_call` |
| Block unregistered workflows | 除外リストにない全ワークフロー | なし（起動不可） |

> [!IMPORTANT]
> ワークフローを **追加・改名・削除** する際は、「Block unregistered workflows」の除外リストと該当ポリシーを必ず更新してください（未登録だと `startup_failure` で起動しません）。

---

## 4. 移行計画と実施記録

作業用統合ブランチ `integration/ci-usecase-redesign-20261030`（base: `integration/mypage-canon-20261006`）に PR #1〜#8 を積み、最終 PR #9 で `integration/mypage-canon-20261006` へ統合しました（`dev` へは直接マージしません）。本番 Core Terraform は `main` から実行するためこの統合の影響を受けません。

### 4.1 PR の積み順とオーナー作業一覧

| # | PR 内容 | 主なコード変更 | 取り込み後のオーナー作業（リポジトリ外） |
| --- | --- | --- | --- |
| 1 | 棚卸し・目標設計（本書） | — | なし |
| 2 | API / Operator イメージ分離 | Dockerfile 分離、Backend から Job 同期撤去、`deploy-mypage-maintenance.yml` 追加、両パッケージ許容 | `operator_identity.py --apply`、Terraform Apply、foundation WIF へ `deploy-mypage-maintenance.yml` 追加、初回 Operator deploy |
| 3 | API / Frontend Deploy ワークフロー | `deploy-mypage-api.yml` / `deploy-mypage-frontend.yml` 新設、router と push 自動 CD 撤去 | Backend / Hosting / foundation image-build の WIF condition 更新、Backend から `push` 削除 |
| 4 | `terraform.yml` 統合 | 手動 Plan/Apply と `component` 入力統合、旧操作/infra ワークフロー撤去 | 4.3 の trust 更新、`terraform-prod-apply` へ required reviewers 設定 |
| 5 | PR Plan | `terraform.yml` の `pull_request` Plan（DEV 手動 Plan と identity 共通化） | 4.4 の trust 更新と旧 PR Plan 用資源の撤去 |
| 6 | Maintenance mode の reusable 化 | 認証ジョブを `mypage-maintenance-authenticated.yml` へ分離、`checkout` SHA 固定 | Environment `mypage-maintenance-development` へ `MYPAGE_MAINTENANCE_WIF_PROVIDER` 設定（2.4 の `google.subject` 修正適用が先） |
| 7 | `ci.yml` 最終整理 | concurrency group 改名、共有ロックテスト、文書整合 | 移行済み DEV identity から旧 `ci.yml` claim を削除 |
| 8 | schema audit 撤去（オーナー判断） | `gcp-user-activity-schema-audit.yml`・`ci-operations.yml` 撤去、`ci.yml` 操作入力削除 | Environment `gcp-dev-user-activity-schema-audit` と secret 削除（任意） |
| 9 | 作業用統合ブランチ → `integration/mypage-canon-20261006` | — | 4.2 の順序で適用 |

※各最上位ワークフローは [`test_ci_workflows.py`](../../.github/scripts/test_ci_workflows.py)・[`test_ci_deploy_workflows.py`](../../.github/scripts/test_ci_deploy_workflows.py) および actionlint の対象に登録し、資格情報ジョブが reusable 外にないことや `!cancelled()` 条件をテストで検証しています。

### 4.2 最終 PR 取り込み後の実施順序（オーナー）

1. 取り込み前に旧 concurrency group に実行中・待機中 Run がないことを確認します。
2. 新しい `workflow_ref` / `job_workflow_ref` を trust に追加し、各新経路を実 Run で 1 回ずつ検証します。
3. 初回 Operator deploy 後に旧 `mypage-backend-deploy` の Job binding と `actAs` を削除します（**DEV は 2026-11-01 実施済み**：#1533 / [run 41260474728](https://github.com/kani3camp/youtube-study-space/actions/runs/41260474728)）。
4. 移行済み **DEV** identity の trust から旧 `ci.yml` claim を削除します（**2026-11-01 実施済み**：Core・MyPage ともに削除して読み戻し確認済み。本番 Core も v4.34.0 後に `terraform.yml` のみに限定し、旧 `terraform-ci` pool の provider と旧 Secret を削除済み）。
5. Maintenance WIF provider を専用遷移で修正・DEV Apply します（**コード修正済み・DEV Apply 待ち**）。
6. #1236 を `dev` に取り込んだ後、MyPage の DEV 固定 ref（`foundation_contract.py`、`hosting_release.py`、`mypage-maintenance-authenticated.yml`、`modules/runtime/main.tf` の 4 箇所）と関連 trust を `refs/heads/dev` へ移します（移すまでは `component=all` で MyPage をスキップし、`component=mypage` は停止します）。

### 4.3 Terraform の trust 更新（#4）

`ci.yml` から `terraform.yml` へ置き換える手順の一覧です。

| 対象 | 方法 |
| --- | --- |
| Core DEV GCP WIF（`terraform-normal` pool） | [`bootstrap_normal_ops.py`](../../infra/gcp/scripts/bootstrap_normal_ops.py) を再実行（`update-oidc` と readback） |
| Core DEV AWS State role（`youtube-study-space-terraform-dev-plan` / `-apply`） | trust policy の `workflow_ref` を置き換え、[`enable_normal_integration.py`](../../infra/gcp/scripts/enable_normal_integration.py) `--verify-aws` で readback |
| MyPage DEV foundation GCP WIF（`mypage-foundation` pool） | `foundation_contract.py provider-condition` の出力で condition を更新 |
| MyPage DEV AWS State role | `foundation_contract.trust_policy()` の出力で trust を更新 |
| Core PROD（`main` 取り込み時） | 無停止移行のため dual 許可 → `main` 取込 → 本番 Plan 確認 → [`bootstrap_production_normal_ops.py`](../../infra/gcp/scripts/bootstrap_production_normal_ops.py) と [`production_backend_packet.py`](../../infra/gcp/scripts/production_backend_packet.py) で `terraform.yml` に限定（**v4.34.0 / #1545 で実施済み**：2026-11-01 [run 41283573872](https://github.com/kani3camp/youtube-study-space/actions/runs/41283573872)・[run 41283884680](https://github.com/kani3camp/youtube-study-space/actions/runs/41283884680)） |

### 4.4 PR Plan の統合（#5、2026-10-31 状況）

PR Plan 統合（#1527）後の 5 項目（Core / MyPage の GCP WIF と AWS Plan role の PR 用 subject 追加、および DEV Environment の `No restriction` 化）は実施・readback 済みで、[run 41205098180](https://github.com/kani3camp/youtube-study-space/actions/runs/41205098180)（#1528）と #1532 で動作を確認しています。旧 PR Plan 用 Environment（`terraform-*-pr-plan` / `mypage-dev-pr-plan`）と旧 identity の撤去状況は [Issue #1562](https://github.com/kani3camp/youtube-study-space/issues/1562) で現物照合してください。

### 4.5 PR Plan の再実行を有効にする（owner）

コード変更だけでは既存 GCP WIF の初回限定条件は変わりません。統合ブランチ取り込み後、認証済み管理端末から DEV の 2 つの provider condition を更新してください（AWS trust・権限・SA・PROD は変更せず、手動 dispatch の初回限定条件も維持します）。

```bash
# リポジトリルートで実行: 現在のソースから condition を生成し、更新後に完全一致を検証
python3 - <<'PY'
import os
import subprocess
import sys

project = 'test-youtube-study-space'
sources = [
    ('terraform-normal', ['-c', "import sys; sys.path.insert(0, 'infra/gcp/scripts'); import bootstrap_normal_ops as b; print(b.CONDITION)"]),
    ('mypage-foundation', ['infra/mypage-platform/scripts/foundation_contract.py', 'provider-condition']),
]
for pool, args in sources:
    condition = subprocess.check_output([sys.executable, *args], text=True,
        env=os.environ | {'MYPAGE_TARGET': 'development'}).strip()
    scope = ['github-actions', '--project=' + project, '--location=global', '--workload-identity-pool=' + pool]
    command = ['gcloud', 'iam', 'workload-identity-pools', 'providers']
    subprocess.run([*command, 'update-oidc', *scope, '--attribute-condition=' + condition, '--quiet'], check=True)
    actual = subprocess.check_output([*command, 'describe', *scope, '--format=value(attributeCondition)'], text=True).strip()
    if actual != condition:
        raise SystemExit('WIF condition readback mismatch: ' + pool)
    print(pool + ': condition readback PASS')
PY
```

更新後、オーナーが起動した PR の Terraform Run を write 権限保持者が再実行し、Core・MyPage 両方で OIDC 検証と native Plan が通ることを確認してください（更新前に作成された既存 Run は対象外）。

---

## 5. 移行状況

「実装済み」は統合ブランチ上のコードを示します。実環境での有効性は 4.2 のオーナー作業と実 Run で確認してください。

| UC | 状態 | 経路 |
| --- | --- | --- |
| **UC1 CI** | 実装済み（`workflow_dispatch` は再検査のみ） | `ci.yml` |
| **UC2 PR Plan** | DEV で有効（run 41205098180 で MyPage 成功、#1532 + run 41226105756 で Core 差分なし）。旧 PR Plan identity 撤去は [Issue #1562](https://github.com/kani3camp/youtube-study-space/issues/1562) で確認 | `terraform.yml`（`pull_request`）→ `gcp-terraform-authenticated.yml`（`pr-plan`）/ `mypage-foundation-authenticated.yml` |
| **UC2 手動 Plan / UC3 Apply** | DEV で有効（手動 Plan: run 41226781485 で Core/MyPage 成功、MyPage Apply: run 41260474728 で削除 2 件成功、Core Apply 未実施）。PROD Core は v4.34.0 で有効（PROD Apply の reviewer はオーナー設定待ち）。旧ルーター撤去済み | `terraform.yml` |
| **UC4 Frontend** | DEV で有効（[run 41147610986](https://github.com/kani3camp/youtube-study-space/actions/runs/41147610986)、旧 `ci.yml` claim 削除・旧ルーター撤去済み） | `deploy-mypage-frontend.yml` |
| **UC5 API** | DEV で有効（[run 41146478688](https://github.com/kani3camp/youtube-study-space/actions/runs/41146478688)、Service のみ、旧 `ci.yml` claim と自動 CD 撤去済み） | `deploy-mypage-api.yml` |
| **UC6 Operator** | 実装済み・DEV で成功（run 41178451321 で build・Job 更新・読み戻し・受領記録まで確認） | `deploy-mypage-maintenance.yml` |
| **UC7 Maintenance mode** | 実装済み（認証ジョブを reusable 化）。`google.subject` 修正の DEV 未適用と `MYPAGE_MAINTENANCE_WIF_PROVIDER` 未設定のため未成功 | `mypage-maintenance.yml` → `mypage-maintenance-authenticated.yml` |

---

## 6. DEV Plan の構成参照ロール

Core・MyPage の DEV Plan は共通の静的マニフェストを使用します（同じ SA を使う手動 DEV Plan にも参照範囲が及びますが、再実行の緩和は PR のみ。Apply との共有 read ロールは拡張せず、Plan SA のバインディングのみ変更します）。手順は [plan-read-access.md](../../infra/gcp/docs/plan-read-access.md) を参照してください。
