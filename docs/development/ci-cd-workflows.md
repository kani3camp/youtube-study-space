# CI / Terraform / Deploy / Operation workflow のユースケース設計

GitHub Actions のワークフローを「人間が実行したいユースケース」単位で整理する設計ドキュメントです。
[Issue #1500](https://github.com/kani3camp/youtube-study-space/issues/1500) の棚卸し・目標設計・移行順序をここで管理し、移行 PR では本書の「[5. 移行状況](#5-移行状況)」表を合わせて更新します。

> [!NOTE]
> - 本書は **設計と移行の現在地** を記録します。実クラウドでの成否は対象 Run の実行結果で確認してください。
> - 本書の記述と実際のコードが異なる場合は、`integration/mypage-canon-20261006` 上の現行コード・ワークフロー・GitHub / GCP / AWS の実設定を優先します。
> - AWS CDK の手動デプロイは対象外とし、挙動を変更しません。

---

## 1. 現行の trust model（移行計画の前提）

資格情報を使う経路はすべて OIDC トークンの `workflow_ref`（最上位ワークフロー）と `job_workflow_ref`（reusable workflow）に固定されています。
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

- Production Terraform Apply の承認未設定: `terraform-prod-apply` に required reviewers がありません（branch policy `main` のみ）。UC3 完了にはオーナーによる reviewer 設定が必要です。
- PR Plan 用の trust 不足（解消済み）: 棚卸し時点ではどの trust も `workflow_dispatch` 固定でしたが、#5 で PR Plan を追加しオーナー作業で接続しました。
- Maintenance WIF provider の `google.subject` 長さ上限超過（コード修正済み・DEV Apply 待ち）: リポジトリのカスタム `sub`（DEV 約 449 バイト / PROD 約 361 バイト）は Google STS の上限 127 バイトを超えるため、このままではトークン交換が拒否されます。2026-10-31 に他 identity と同じ `repository_id:environment:actor_id`（約 50 バイト）へ変更し、condition に `job_workflow_ref` を追加する遷移（`reviewed_provider_update`）を実装済みです（DEV への適用には Apply 時に一時的な `iam.workloadIdentityPoolProviders.update` 権限が必要）。
- Maintenance mode の認証未成功: 唯一の実行（2026-10-29、run 40879914039）は Environment `mypage-maintenance-development` に `MYPAGE_MAINTENANCE_WIF_PROVIDER` が未設定で停止しました（`mypage-maintenance-production` も未作成）。DEV の資源・Operator イメージは 2026-10-31 に配備済みのため、残る作業は (1) 上記 `google.subject` 修正の DEV Apply、(2) Environment の設定、(3) 実 Run 検証です。
- `gcp-user-activity-schema-audit` 撤去: branch policy が `dev` のみで統合ブランチから到達不能かつ無効化済みだったため、#8 で撤去しました。

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

- 内部用 reusable workflow（`gcp-terraform-authenticated.yml` 等）には `workflow_dispatch` を持たせません。
- Production の Frontend / API / Operator デプロイは、公開 Gate（[#1133](https://github.com/kani3camp/youtube-study-space/issues/1133)）を満たすまで入力選択肢に出しません。また、PR や `push` を契機とした自動デプロイは行いません。
- 各 Deploy ワークフローは資格情報を取得する前に配備対象ドメインの認証なし検査を同じ Run 内で実行します（API / Operator は `ci-system.yml`、Frontend は `ci-frontend.yml` の `mypage`）。Ruleset が要求する PR ごとの `CI Gate` は統合後のツリーそのものを検査しないため、対象ドメインの検査で担保します。

### 3.2 Terraform Plan / Apply

#### PR Plan（UC2 自動）

- `pull_request` で起動し、変更パスから component を自動判定します（`infra/gcp/**` → core、`infra/mypage-platform/**` → mypage、共有の CI スクリプトやワークフロー変更 → 両方）。対象は DEV のみです（PROD の Plan Environment は `main` 限定のため手動 Plan）。
- fork 由来の PR では資格情報を取得しません。`pull_request` は fork に OIDC や secret を渡さないうえ、ジョブ条件 `head.repo.full_name == github.repository` を満たさない場合はスキップ（false-green）にせず **失敗として** 理由を表示します。
- 2026-10-31 のオーナー判断により、DEV の手動 Plan と同じ reusable・Environment（`terraform-dev-plan` / `mypage-dev-plan`）・identity を共用します（当初は分離していましたが PR 専用の identity / Environment / workflow は持たず、Apply 用は手動 dispatch のみにします）。
  - オーナーの PR で動くコードは、オーナーが任意のブランチから手動 dispatch するコードと信頼の前提が同じです。DEV Environment の Secret は資格情報ではない設定値だけで、資格情報の境界はクラウド trust の actor 検査にあります。
  - WIF condition と AWS trust は手動 dispatch 条件に PR 用条件を OR で加えます（`event_name == 'pull_request'`、`ref` が `refs/pull/<n>/merge` の形式、`workflow_ref == terraform.yml@` + `assertion.ref`、`job_workflow_ref` が同じ reusable の `@` + `assertion.ref`、Plan Environment 限定、repository / owner ID / actor は共通、GCP WIF のみ手動は `run_attempt == 1` で PR 節は正整数の attempt を許可）。
  - オーナーが最初に起動した DEV PR Plan に限り、GitHub が再実行を許可する write 権限保持者による **Re-run jobs / Re-run failed jobs** を許可します。初回の不変 `actor_id` は維持して PR 経路では `GITHUB_TRIGGERING_ACTOR` を制限しませんが、他者が最初に起動した Run はオーナーが再実行しても通りません。OIDC の `run_attempt` は今回の実行値と一致させ、コミット追加による `synchronize`（別 Run・attempt 1）と同一コミットの再実行（attempt 2 以降）を区別します。
  - `-lock=false` で実行するため、手動 Plan/Apply を待たせません（Apply は必ず fresh Plan を作成します）。
- 同一リポジトリの PR ではワークフロー定義自体が PR 作成者の管理下にあるため、trust で実行コードを固定できません。そこで実行できる actor をオーナー（`actor_id == 54093651`）に限定し、write 権限を持つ協力者の PR では PR Plan を失敗させてオーナーが手動 Plan を実行します（協力者のジョブが DEV Environment の Secret を読めても、クラウド資格情報は actor 検査で取得できません）。AI エージェントなどの actor を追加する場合は、追加する claim と理由を PR で明示してから trust を広げます。
- 結果は Job Summary と check に値を含まない要約（resource type・操作・件数）のみ出し、plan ファイル・JSON・artifact・PR コメントは残しません（State 由来の値を PR 作成者の管理下にあるコードの外へ持ち出さないためです。詳細確認にはオーナーが手動 Plan を実行します）。
- クラウド trust の `actor` は初回起動者であり再実行者を示さないため、この運用は信頼する write 権限保持者に「オーナー起動の DEV Plan の再実行」を委ねる前提です。write 権限の誤付与やアカウント侵害時には同じ SHA/ref のコードを再実行して DEV State や構成を参照され得るほか、外部依存・private inputs・クラウド状態は初回から変わり得ます。協力者自身が起動した PR Run の許可や手動 Plan/Apply・PROD の緩和は含みません。

#### 手動 Plan / Apply（UC2 手動 / UC3）

- 「plan ジョブ → apply ジョブ内で独立して再 Plan → 保存した Plan とソース・State の HMAC digest 一致時のみ `terraform apply <plan-file>`」の方式を維持します（PR の plan ファイルは後日 Apply に使いません）。
- `source SHA`・`environment`・`component` は dispatch 入力と `github.sha` で固定し、plan / apply ジョブ間で一致を検査します。
- Apply は `workflow_dispatch` のみとし、Apply identity は既存のものを使って PR Plan identity とは共有しません。Production Apply は `terraform-prod-apply`（将来の `mypage-prod-apply` を含む）に required reviewers を設定して承認必須とします。

#### MyPage の削除を含む Plan

1. `Terraform` を `operation=plan`、`component=mypage` で実行し、Job Summary に出る対象環境・Terraform アドレス・削除件数を確認します（単独の `delete` のみ許可し、置き換えや削除保護付き資源は拒否します）。
2. 適用時は `operation=apply` に加えて `allow_mypage_deletions` をオンにします（既定はオフのため削除を含む Plan で停止します）。`component=all` で Core にも変更がある場合は Core Apply が MyPage の承認待ちより先に進むため、MyPage の削除だけを承認・適用するときは `component=mypage` を使います。
3. 同じ Run の **Review MyPage Terraform deletion Plan** が承認待ちになったら、依存する MyPage Plan の要約を確認して Actions の **Review deployments** で承認します。Apply は Plan 作成から 30 分以内に再生成し、State・コミット・Plan の HMAC 照合が一致した保存済み Plan のみ適用します（却下・未承認・期限切れ・照合不一致時は停止し、再試行は新しい Run を手動起動します）。

初回実行前にリポジトリオーナーが GitHub Settings → Environments で `mypage-dev-delete-approval`（本番有効化時は `mypage-prod-delete-approval`）を作成し、Required reviewers 登録・管理者バイパス無効化・Deployment branches 固定（DEV: `integration/mypage-canon-20261006`、PROD: `main`）を設定してください。1 人運用の場合は自分を reviewer にして Prevent self-review をオフにします。Secrets やクラウド権限は置かず承認記録専用として使い、ワークフローが GitHub API でレビュールールと承認記録を確認します（設定不備や API 障害時は停止します）。

### 3.3 API と Maintenance Operator のイメージ分離

| 項目 | API | Maintenance Operator |
| --- | --- | --- |
| Dockerfile | `system/Dockerfile.mypage-api`（`./cmd/mypage-server` のみ） | `system/Dockerfile.mypage-maintenance-operator`（`./cmd/mypage-maintenance-operator` のみ） |
| Artifact Registry | `mypage/mypage-api@sha256:...` | `mypage/mypage-maintenance-operator@sha256:...`（同じ `mypage` リポジトリの別パッケージ。既存の repository 単位の push 権限で対応可） |
| Deploy 対象 | Cloud Run Service `mypage-api` のみ | Cloud Run Job `mypage-maintenance-operator` のみ |
| Deployer identity | `mypage-backend-deploy`（Job 権限・maintenance SA `actAs` を削除） | 新設 `mypage-operator-deploy`（Job `run.jobs.get/update` と maintenance runtime SA `actAs` のみ。SA/WIF/Environment は [`operator_identity.py`](../../infra/mypage-platform/scripts/operator_identity.py)、Job IAM は Terraform で作成） |
| Release record | `mypage-backend-release-record`（schema 4、API digest のみ） | `mypage-operator-release-record`（schema 1、Operator digest のみ） |
| Terraform | Service イメージは `ignore_changes` | Job イメージは `ignore_changes`（初回作成用 `maintenance_bootstrap_image_digest` のみ旧 `mypage-api` 固定ビルド） |

- 「API の更新で Job を変更しない」ことは、コード分岐だけでなく API deployer の IAM から Job 権限を外すことで強制しています。
- `core/mypage` の `MaintenanceControl` や Firestore スキーマを変える PR では、後方・前方互換を保つか API・Operator 両方のデプロイが必要であることを PR 説明に明記してください（平常時に同一 digest へは同期しません）。
- 旧 Backend release と Terraform 契約が Job イメージに `mypage-api` を要求していたため、#2 で「Backend release から Job 同期を外し Service のみ更新にする」「Terraform 契約と Operator 配備スクリプトで現在値として `mypage-api` と `mypage-maintenance-operator` の両方を許容する（初回作成用 `maintenance_bootstrap_image_digest` は固定の旧 API イメージのまま）」の 2 点を済ませたうえで初回 Operator deploy を実施しました。新しい環境では Job を固定の旧 API イメージで初回作成するため、旧パッケージの許容を外すと最初の Operator deploy が拒否されます。
- API イメージから operator バイナリを外しても Job は配備済みの旧 digest（operator バイナリ入り）を参照し続けるため安全であり、#2 で同時に分離しました。旧 `mypage-backend-deploy` の Job binding と `actAs` は初回 Operator deploy 確認後に削除専用遷移で外しました（DEV は 2026-11-01 に完了し遷移コードは削除済み。今後の単独削除は 3.2 の手順で扱います）。

### 3.4 Rollback

- API / Operator は各 Deploy ワークフローの `mode=deploy-existing` で検証済み不変 digest と `source SHA` を指定します（パッケージ・Registry 存在・source SHA タグと digest の一致・現 HEAD の祖先チェックを通過した対象リソースのみ更新し、従来の `backend_release.py` の rollback 検査を流用しています）。
- Frontend は現行の Hosting release record に直前リリース・バージョンが保持されており、復旧方式（過去バージョンの再公開または過去コミットからの再ビルド）を Frontend 移行 PR で決定します。

### 3.5 Concurrency（同時実行制御）

GitHub Actions の concurrency group は 1 ジョブ 1 つのため、「Terraform は API・Operator 両方と排他、API と Operator は互いに独立」を group だけでは表せません。リソース所有権と競合の実害から、以下の最小限の排他を設定しています。

| Group | 対象操作 | 排他の理由 |
| --- | --- | --- |
| `gcp-terraform-authenticated-<target>` | Core Terraform plan / apply（reusable 内） | 同一 State・同一資源の保護 |
| `mypage-<env>-cloud-run` | 手動 `Terraform` Run 全体（Core / MyPage）、API deploy、Operator deploy | Terraform の refresh + PATCH が途中のイメージ更新を巻き戻すのを防ぐため（workflow ごとに 1 つの制限により Core のみの Run も含めて環境単位のロックとし、全操作の global lock にはしない） |
| `mypage-<env>-hosting` | Frontend deploy（`mypage-hosting-authenticated.yml`） | Hosting サイトのみ変更 |
| `mypage-maintenance-<env>` | Maintenance mode 切替 | Firestore 制御文書のみ変更（Job 実行は開始時点定義を使うため Operator deploy と排他しない） |
| `terraform-pr-plan-<pr>`<br>（reusable 内 `gcp-terraform-pr-plan-<pr>` / `mypage-pr-plan-<pr>`） | PR Plan（`cancel-in-progress: true`） | `-lock=false` の助言用であり、手動 Plan/Apply の group に入れず待たせない |

移行中は旧グループ `mypage-dev-runtime-change` に全参加者を入れていましたが、新経路へ揃った #7 で `mypage-dev-cloud-run`（本番は `mypage-prod-cloud-run`、旧 `gcp-infrastructure-production`）と `mypage-dev-hosting`（旧 `mypage-dev-hosting-release`）へ改名しました。新旧グループは互いに排他しないため、最終 PR 取り込み前に旧グループに実行中・待機中 Run がないことを確認します（4.2）。各デプロイスクリプトでも `resourceVersion` / `etag` / 再読み取りによる楽観的排他を維持し、競合時は自動再試行せず停止します。

### 3.6 Actions の実行保護（オーナー設定）

GitHub Settings → Actions → Policies（2026-10-31 オーナー設定）で、全ブランチのワークフローに対して起動者とイベントを制限しています。`gh api repos/kani3camp/youtube-study-space/actions/policies` で一覧（ID と名前）を取得し、ID ごとに `gh api repos/kani3camp/youtube-study-space/actions/policies/<policy_id>` で対象パス（`conditions`）と許可ルール（`rules`）を確認できます。

| Policy | 対象ワークフロー | 許可する actor / event |
| --- | --- | --- |
| Allow base image report `pull_request_target` | `base-image-update-report.yml` | owner と `dependabot[bot]` / `pull_request_target` |
| Restrict Terraform execution | `terraform.yml` | owner / `pull_request`・`workflow_dispatch` |
| Restrict manual MyPage operations | `deploy-mypage-api.yml`・`deploy-mypage-frontend.yml`・`deploy-mypage-maintenance.yml`・`mypage-maintenance.yml` | owner / `workflow_dispatch` |
| Protect authenticated reusable workflows | 資格情報を扱う reusable（`gcp-terraform-authenticated.yml`・`mypage-*-authenticated.yml`） | owner / `workflow_call` |
| Block unregistered workflows | 除外リストにない全ワークフロー | なし（起動不可） |

> [!IMPORTANT]
> ワークフローを追加・改名する際は同じ PR の作業として「Block unregistered workflows」の除外リストと該当ポリシーを必ず更新し、削除時は各リストから外してください（未登録だと `startup_failure` で起動しません）。

これにより協力者や外部 PR が新しいワークフローを追加して起動することや、制限対象をオーナー以外が起動することを防げます。既存 `ci.yml` などの書き換え自体は防げませんが、その場合も DEV Environment の Secret に資格情報はなく、クラウド資格情報は trust の actor 検査で取得できません（3.2）。なお参照されていても呼ばれない reusable（`main` の `ci.yml` が参照する撤去済み `gcp-user-activity-schema-audit.yml`）は起動の妨げになりません。

---

## 4. 移行計画と実施記録

作業用統合ブランチ `integration/ci-usecase-redesign-20261030`（base: `integration/mypage-canon-20261006`）に PR #1〜#8 を積み、最終 PR #9 で `integration/mypage-canon-20261006` へ統合しました（`dev` へは直接マージしません）。統合ブランチの ref はどの trust（GCP WIF / AWS role / GitHub Environment）にも含まれないため、実環境での有効化は最終 PR 取り込みとオーナーの trust 更新後になります。本番 Core Terraform は `main` から実行するため影響を受けません。

### 4.1 PR の積み順とオーナー作業一覧

| # | PR 内容 | 主なコード変更 | 取り込み後のオーナー作業（リポジトリ外） |
| --- | --- | --- | --- |
| 1 | 棚卸し・目標設計（本書） | — | なし |
| 2 | API / Operator イメージ分離 | Dockerfile 分離、Backend から Job 同期撤去、`deploy-mypage-maintenance.yml` 追加、両パッケージ許容 | `operator_identity.py --apply`、Terraform Apply、foundation WIF へ `deploy-mypage-maintenance.yml` 追加、初回 Operator deploy |
| 3 | API / Frontend Deploy ワークフロー | `deploy-mypage-api.yml` / `deploy-mypage-frontend.yml` 新設、router と push 自動 CD 撤去 | Backend / Hosting / foundation image-build の WIF condition 更新、Backend から `push` 削除 |
| 4 | `terraform.yml` 統合 | 手動 Plan/Apply と `component` 入力統合、旧操作/infra ワークフロー撤去（`migration` / `e2e` mode は後続 #1521 で撤去） | 4.3 の trust 更新、`terraform-prod-apply` へ required reviewers 設定 |
| 5 | PR Plan | `terraform.yml` の `pull_request` Plan（DEV 手動 Plan と identity 共通化、当初の専用ワークフローと identity は統合・撤去） | 4.4 の trust 更新と旧 PR Plan 用資源の撤去 |
| 6 | Maintenance mode の reusable 化 | 認証ジョブを `mypage-maintenance-authenticated.yml` へ分離、`checkout` SHA 固定、認証前の actor / ref / caller 検査追加 | Environment `mypage-maintenance-development` へ `MYPAGE_MAINTENANCE_WIF_PROVIDER` 設定（2.4 の `google.subject` 修正適用が先。production は Environment 作成から） |
| 7 | `ci.yml` 最終整理 | concurrency group 改名、共有ロックの契約テスト、文書整合 | 移行・検証済み DEV identity から旧 `ci.yml` claim を削除 |
| 8 | schema audit 撤去（オーナー判断） | `gcp-user-activity-schema-audit.yml`・`ci-operations.yml` 等を撤去（履歴 adoption テストは `test_user_activity_history_preparation.py` へ分離） | Environment `gcp-dev-user-activity-schema-audit` と secret の削除（任意） |
| 9 | 作業用統合ブランチ → `integration/mypage-canon-20261006` | — | 4.2 の順序で適用 |

※各最上位ワークフローは [`test_ci_workflows.py`](../../.github/scripts/test_ci_workflows.py)・[`test_ci_deploy_workflows.py`](../../.github/scripts/test_ci_deploy_workflows.py) および actionlint の対象に登録し、資格情報ジョブが reusable 外にないこと（#6 で例外ゼロ化）や `!cancelled()` 条件（`always()` 禁止）をテストで検証しています。また PR 説明には「旧経路が有効か」「新経路がまだ無効か」を明記します。

### 4.2 最終 PR 取り込み後の実施順序（オーナー）

1. 取り込み前に、旧 concurrency group（`mypage-dev-runtime-change`、`mypage-dev-hosting-release`、`gcp-infrastructure-production`）に実行中・待機中の Run がないことを確認します。
2. 各 PR の「取り込み後のオーナー作業」を実施し、新しい `workflow_ref` / `job_workflow_ref` を trust に追加します。
3. 新経路を実 Run で 1 回ずつ検証します（Terraform 手動 Plan → API deploy → Frontend deploy → Operator deploy → Maintenance mode `enabled=false`）。
4. 初回 Operator deploy で Job が `mypage-maintenance-operator` パッケージへ移ったことを確認したあと、旧 `mypage-backend-deploy` の Job binding と maintenance runtime SA の `actAs` を削除専用の契約遷移で外します。DEV は 2026-11-01 に実施済み（#1533）で、[run 41260474728](https://github.com/kani3camp/youtube-study-space/actions/runs/41260474728) にて 2 つの binding を削除し、Job の `mypageMaintenanceImageRelease` と maintenance runtime SA の `roles/iam.serviceAccountUser` がどちらも `mypage-operator-deploy` のみになったことを読み戻し確認しました（Job 実行用の `mypage-maintenance-invoker` の binding は別に残ります）。なお MyPage の Apply SA は通常読み取り権限のみのため、Apply 中だけオーナーが 1 時間で切れる一時 custom role（Job と SA の `getIamPolicy` / `setIamPolicy`）を付与し、Apply 後に binding とロールを削除して 0 件を読み戻しています。旧 `mypage-api` パッケージの現在値としての許容は初回作成要件（3.3）のため維持します。
5. 移行・検証済みの DEV identity の trust から旧 `ci.yml` claim を削除します（2026-11-01 実施済み）。Core（GCP `terraform-normal`、AWS `terraform-dev-plan` / `-apply`）、MyPage backend / hosting（GCP）、AWS `mypage-dev-apply` から `ci.yml` を外して読み戻し確認を完了し、本番 Core も v4.34.0 の後に `terraform.yml` だけへ限定しました（4.3）。旧移行用の `terraform-ci` pool の provider は DEV・本番とも無効化し、旧 Secret `GCP_TERRAFORM_WIF_PROVIDER` / `GCP_TERRAFORM_SERVICE_ACCOUNT` を 4 つの Environment から削除しました（pool と旧移行用 SA `terraform-*-plan` / `-apply` の削除は無効化後に別途実施。schema audit のクラウド側 trust は監査後に撤回済み）。
6. Maintenance WIF provider（Terraform 管理）を、provider 更新だけを許す専用遷移で修正し、`google.subject` を 127 バイト以内の値へ変更すること（必須、2.4 参照）と condition への `job_workflow_ref == mypage-maintenance-authenticated.yml@<ref>` 追加（hardening）を同時に適用します（コードは修正済み。一時的な provider 更新権限での DEV Apply 完了まで Set Maintenance Mode は認証できません）。
7. #1236 を `dev` に取り込んだ後、MyPage の DEV 固定 ref 4 箇所（`infra/mypage-platform/scripts/foundation_contract.py` の `development` `ref`、`infra/mypage-platform/scripts/hosting_release.py` の `REF`、`.github/workflows/mypage-maintenance-authenticated.yml` の事前検査 `ref`、`infra/mypage-platform/modules/runtime/main.tf` の `maintenance_ref`）と関連 trust（foundation / backend / hosting / operator の GCP WIF condition と AWS role）を 1 つの PR と作業でまとめて `refs/heads/dev` へ移します。移行完了までは `dev` から `Terraform` を実行しても `select` が実行 ref と固定 ref を比較して停止するため、`component=all` は MyPage をスキップして Core のみ実行し、`component=mypage` は理由を表示して停止します。

### 4.3 Terraform の trust 更新（#4）

`ci.yml` から `terraform.yml` へ caller を置き換える手順の一覧です。

| 対象 | 方法 |
| --- | --- |
| Core DEV GCP WIF（`terraform-normal` pool） | [`bootstrap_normal_ops.py`](../../infra/gcp/scripts/bootstrap_normal_ops.py) を再実行（既存 provider を `update-oidc` で更新し readback） |
| Core DEV AWS State role（`youtube-study-space-terraform-dev-plan` / `-apply`） | trust policy の `StringLike` `sub` の `workflow_ref` を置き換え、[`enable_normal_integration.py`](../../infra/gcp/scripts/enable_normal_integration.py) `--verify-aws` で readback |
| MyPage DEV foundation GCP WIF（`mypage-foundation` pool） | `foundation_contract.py provider-condition` の出力で condition を置き換え |
| MyPage DEV AWS State role | `foundation_contract.trust_policy()` の出力（`sub` の caller が `terraform.yml`）で置き換え |
| Core PROD（`main` 取り込み時） | 本番の無停止移行として、(1) `ci.yml` と `terraform.yml` の両方を許可する dual 設定 → (2) `main` 取込 → (3) `main` から本番 Plan 確認 → (4) [`bootstrap_production_normal_ops.py`](../../infra/gcp/scripts/bootstrap_production_normal_ops.py) と [`production_backend_packet.py`](../../infra/gcp/scripts/production_backend_packet.py) で `terraform.yml` だけへ限定（AWS の `aud`・`actor_id` は保持）。v4.34.0 / #1545 で 2026-11-01 に実施済み（dual で [run 41283573872](https://github.com/kani3camp/youtube-study-space/actions/runs/41283573872)、限定後に [run 41283884680](https://github.com/kani3camp/youtube-study-space/actions/runs/41283884680) のどちらも差分なしで成功） |

### 4.4 PR Plan の統合（#5、2026-10-31 状況）

PR Plan を DEV 手動 Plan に統合する #1527 の取り込み後、以下の順で更新します（順序を逆にすると PR Plan が通らないまま制限だけが外れます）。撤去以外の 5 行は実施・readback 済み（DEV の 7 Environment は `No restriction`、PROD は変更なし）で、新経路の PR Plan は [run 41205098180](https://github.com/kani3camp/youtube-study-space/actions/runs/41205098180)（#1528）にて MyPage が plan contract（26 資源・作成 0）まで成功しました。Core は認証・State 読み取り・native Plan まで成功後に `plan-validation` の `resource-contract` で停止しましたが、DEV State に #1467 の Auth 資源が適用済みで統合ブランチの HCL になかったためであり（その後 #1532 で解消し run 41226105756 で差分なし）、経路の問題ではありません。旧 PR Plan 用資源（`terraform-*-pr-plan` / `mypage-dev-pr-plan`、SA、WIF pool/provider `terraform-pr-plan`・`mypage-foundation/github-pr-plan`、AWS role）は対象一覧と現物を照合し、残存がないことを確認して [Issue #1562](https://github.com/kani3camp/youtube-study-space/issues/1562) に記録してください。

| 対象 | 方法 |
| --- | --- |
| Core DEV GCP WIF（`terraform-normal` pool） | `bootstrap_normal_ops.CONDITION` で `update-oidc` を実行し、`enable_normal_integration.py --verify-gcp` で readback（2026-11-01 に移行用 `ci.yml` を削除済み、監査も `ci.yml` を拒否） |
| Core DEV AWS State role（`youtube-study-space-terraform-dev-plan`） | trust の `StringLike` `sub` に `enable_normal_integration.custom_subject('plan', 'refs/pull/*/merge', event_name='pull_request')` を追加し、`--verify-aws` で readback（Apply role は無変更） |
| MyPage DEV foundation GCP WIF（`mypage-foundation` / `github-actions`） | `foundation_contract.py provider-condition` の出力で condition を置換 |
| MyPage DEV AWS Plan role | `foundation_contract.py trust-policy <account> plan` の出力で trust を置換 |
| DEV Environment | `enable_normal_integration.py --apply`（Core 2 環境）と GitHub Settings（`mypage-dev-*`）で branch 制限を `No restriction` に変更（PROD は無変更） |
| 撤去 | 旧 PR Plan 用 Environment・SA・WIF pool / provider・AWS role を削除し現物照合 |

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

更新後、オーナーが起動した PR の Terraform Run を write 権限保持者が再実行し、Core・MyPage 両方で OIDC 検証と native Plan が通ることを確認してください（既存 Run の再実行は以前の SHA の workflow/controller を使うため、修正前に作られた Run は対象外です。更新前の WIF は再実行を拒否するため、コードやテストの成功だけで実環境の有効化済みとは扱いません。PR の結果は引き続き助言とし、Apply では別途 fresh Plan を作ります）。

---

## 5. 移行状況

「実装済み」は作業用統合ブランチ上のコードを指します。実環境での有効性は 4.2 のオーナー作業と実 Run で確認してください。

| UC | 状態 | 経路 |
| --- | --- | --- |
| UC1 CI | 実装済み（手動 operation なし、`workflow_dispatch` は再検査のみ） | `ci.yml` |
| UC2 PR Plan | DEV で有効（run 41205098180 で MyPage 成功、#1532 + run 41226105756 で Core 差分なし）。旧 PR Plan identity 撤去は [Issue #1562](https://github.com/kani3camp/youtube-study-space/issues/1562) で現物照合 | `terraform.yml`（`pull_request`）→ `gcp-terraform-authenticated.yml`（`pr-plan`）/ `mypage-foundation-authenticated.yml` |
| UC2 手動 Plan / UC3 Apply | DEV で有効（対象 commit を 1 つ確定し隔離コンテナで検査後に同じ SHA で実行。DEV Core のみ対象ブランチ・SHA 指定可、#1530 / #1534 統合。手動 Plan: run 41226781485 で Core/MyPage 差分なし成功、MyPage Apply: run 41260474728 で #1533 の削除 2 件成功、Core Apply 未実施）。PROD Core は v4.34.0（`main`）で本番 Plan 差分なし・trust `terraform.yml` のみで有効（PROD Apply reviewer はオーナー設定待ち）。旧ルーター経路とオーケストレーター撤去済み | `terraform.yml` |
| UC4 Frontend | DEV で有効（[run 41147610986](https://github.com/kani3camp/youtube-study-space/actions/runs/41147610986)、trust は `deploy-mypage-frontend.yml` のみ、旧ルーター撤去済み） | `deploy-mypage-frontend.yml` |
| UC5 API | DEV で有効（[run 41146478688](https://github.com/kani3camp/youtube-study-space/actions/runs/41146478688)、Service のみ、trust は `deploy-mypage-api.yml` のみ、旧ルーターと自動 CD 撤去済み） | `deploy-mypage-api.yml` |
| UC6 Operator | 実装済み・DEV で成功（run 41178451321 で build・Job 更新・読み戻し・受領記録まで確認） | `deploy-mypage-maintenance.yml` |
| UC7 Maintenance mode | 実装済み（認証ジョブを reusable 化）。`google.subject` 修正の DEV 未適用と `MYPAGE_MAINTENANCE_WIF_PROVIDER` 未設定のため未成功 | `mypage-maintenance.yml` → `mypage-maintenance-authenticated.yml` |

---

## 6. DEV Plan の構成参照ロール

Core・MyPage の DEV Plan は共通の静的マニフェストを使用します（同じ SA を使う手動 DEV Plan にも参照範囲が及びますが、再実行の緩和は PR のみで、Apply との共有 read ロール本文は拡張せず Plan SA のバインディングのみ変更します）。手順は [plan-read-access.md](../../infra/gcp/docs/plan-read-access.md) に従い、通常の bootstrap の丸ごと実行で代用しないでください。
