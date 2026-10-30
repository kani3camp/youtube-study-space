# CI / Terraform / Deploy / Operation workflow のユースケース設計

GitHub Actions を「人間が実行したいユースケース」単位で整理するための正本。
[Issue #1500](https://github.com/kani3camp/youtube-study-space/issues/1500) の棚卸し・目標設計・移行順序をここで管理し、
各移行PRは本書の「移行状況」表を同じPR内で更新する。

- 本書は **設計と移行の現在地** を記録する。実クラウドで成功したかは対象runの記録で確認する。
- 本書の記述より、`integration/mypage-canon-20261006` の現行tree・workflow・GitHub/GCP/AWS の実設定を優先する。
- AWS CDK の手動deployは対象外で、挙動を変えない。

## 1. 現行のtrust model（移行計画の前提）

現行の資格情報付き経路は、すべて **OIDC token の `workflow_ref`（最上位workflow）と `job_workflow_ref`（reusable workflow）** に固定されている。
workflowファイルの改名・新設は、コードだけでは完結せず、対応するクラウド側trustの更新を伴う。

| 境界 | 現行の固定値 | 定義場所 |
| --- | --- | --- |
| Repository OIDC `sub` テンプレート | `repository_id`, `repository_owner_id`, `environment`, `ref`, `workflow_ref`, `job_workflow_ref`, `event_name`（repository単位設定） | GitHub `actions/oidc/customization/sub` |
| Core Terraform GCP WIF | `workflow_ref == ci.yml@ref`、`job_workflow_ref == gcp-terraform-authenticated.yml@ref`、`workflow_dispatch`、owner actor、`run_attempt == 1` | [`bootstrap_normal_ops.py`](../../infra/gcp/scripts/bootstrap_normal_ops.py)、[`terraform_normal_ops.py`](../../infra/gcp/scripts/terraform_normal_ops.py) |
| Core Terraform AWS State role | 上記 `sub` の完全一致 | [`terraform-authentication-boundary.md`](../../infra/gcp/docs/terraform-authentication-boundary.md) |
| MyPage foundation GCP WIF / AWS State role | `ci.yml@integration ref` + `mypage-foundation-authenticated.yml@integration ref`。AWS State は `workflow_dispatch` のみ、GCP image-build は `push` も許可 | [`foundation_contract.py`](../../infra/mypage-platform/scripts/foundation_contract.py) |
| MyPage Backend deployer WIF | `ci.yml` + `mypage-backend-authenticated.yml`、`workflow_dispatch` または `push`、Environment `mypage-dev-backend` | [`backend_identity.py`](../../infra/mypage-platform/scripts/backend_identity.py) |
| MyPage Hosting deployer WIF | `ci.yml` + `mypage-hosting-authenticated.yml`、Environment `mypage-dev-hosting` | [MyPage配備手順](../mypage/release/README.md) |
| Maintenance invoker WIF | `workflow_ref == mypage-maintenance.yml@<env ref>`（最上位workflow直接）、Environment `mypage-maintenance-<env>` | [`modules/runtime/main.tf`](../../infra/mypage-platform/modules/runtime/main.tf) |

したがって、新しい最上位workflowへ移すたびに、次の順序を守る。

1. 新workflowを追加する（旧経路は有効なまま）。
2. ownerが対象identityのtrust（GCP WIF provider condition / AWS role trust policy / GitHub Environment）へ **新workflowのclaimを追加** する。
3. 新経路を実runで1回検証する。
4. 旧経路をコードから撤去し、ownerがtrustから **旧claimを削除** する。

資格情報を取得する job は、必ず reusable workflow の中に置く（最上位workflowの job で直接認証しない）。repository の OIDC `sub` テンプレートは `job_workflow_ref` を含むが、GitHub の仕様はこの claim を「reusable workflow を使う job」のものとしてしか定義しておらず、AWS State role は `sub` の完全一致で照合するためである。reusable workflow を使わない job でこの claim と `sub` がどうなるかは仕様に書かれておらず、このリポジトリでも実証されていない。そのため、最上位workflowで直接認証している現行の `mypage-maintenance.yml` も例外にせず、認証 job を reusable workflow へ移す。

2と4はリポジトリ外の実状態変更であり、各PRの説明に「必要なowner作業」と「実施済みか」を明記する。
trustを広げる際は、追加するclaimと脅威境界を同じPRで説明する。

## 2. 現行workflow棚卸し

### 2.1 分類

| Workflow | 現在のユースケース | 分類 | 移行先 |
| --- | --- | --- | --- |
| [`ci.yml`](../../.github/workflows/ci.yml) | PR CI、`workflow_dispatch` の操作router（`gcp-plan` / `gcp-apply` / `gcp-user-activity-schema-audit` / `mypage-image-build` / `mypage-hosting-release` / `mypage-backend-release`）、`system/**` push時のMyPage DEV自動CD | **残す（縮小）** | UC1 通常CIだけにする。dispatch入力・push自動CD・`operations` jobを撤去 |
| [`ci-operations.yml`](../../.github/workflows/ci-operations.yml) | 操作router本体。CI結果の前提検査と下位workflow呼び出し | **廃止** | 各ユースケースworkflowへ分解 |
| `ci-system.yml` / `ci-frontend.yml` / `ci-tools.yml` / `ci-infra.yml` / `ci-repository.yml` | UC1 credentialless検査 | **残す** | 変更なし（`ci-repository.yml` の契約テスト対象だけ追随） |
| [`ci-gcp-terraform.yml`](../../.github/workflows/ci-gcp-terraform.yml) | UC1 Terraform fmt / validate / mock・native credentialless test | **残す** | UC1のまま。実state planはUC2へ分離 |
| [`gcp-infrastructure-authenticated.yml`](../../.github/workflows/gcp-infrastructure-authenticated.yml) | Core + MyPage の plan / apply 合成（componentは常に両方） | **統合→廃止** | `terraform.yml`（UC2/UC3）へ統合 |
| [`gcp-terraform-authenticated.yml`](../../.github/workflows/gcp-terraform-authenticated.yml) | Core root の credentialed plan / apply | **残す（縮小）** | `terraform.yml` から呼ぶ core reusable。到達不能modeを撤去 |
| [`mypage-infrastructure-authenticated.yml`](../../.github/workflows/mypage-infrastructure-authenticated.yml) | MyPage root の desired-stage 解決 | **統合→廃止** | desired-stage解決を `terraform.yml` の mypage component へ移す |
| [`mypage-foundation-authenticated.yml`](../../.github/workflows/mypage-foundation-authenticated.yml) | MyPage root の credentialed plan / apply と image build（2責務が同居） | **分割** | Terraform部分は mypage reusable として残す。image buildは各Deploy workflowへ移す |
| [`mypage-hosting-authenticated.yml`](../../.github/workflows/mypage-hosting-authenticated.yml) | UC4 DEV Hosting build / deploy / smoke | **残す（最上位化）** | `deploy-mypage-frontend.yml` |
| [`mypage-backend-authenticated.yml`](../../.github/workflows/mypage-backend-authenticated.yml) | API Service と maintenance Job の同一digest同期deploy | **分割** | `deploy-mypage-api.yml`（Serviceのみ）と `deploy-mypage-maintenance.yml`（Jobのみ） |
| [`mypage-maintenance.yml`](../../.github/workflows/mypage-maintenance.yml) | UC7 maintenance mode ON/OFF | **残す（認証jobを分離）** | 最上位workflowの名称は維持する。認証 job を reusable workflow へ移し、WIF condition に `job_workflow_ref` を追加する |
| `gcp-user-activity-schema-audit.yml` | read-only schema audit（`DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED: "false"` で無効） | **保留（owner判断）** | 6ユースケース外。router撤去時に「独立dispatch化」か「撤去」を決める |
| `codeql-*`、`deploy-docs.yml`、`base-image-update-report.yml`、`formal-spec-report.yml` | 本Issue対象外 | 対象外 | 変更なし |

### 2.2 資格情報付きworkflowの詳細

| Workflow | Trigger / caller | GitHub Environment（現設定） | GCP / AWS identity | Write対象 | Concurrency | Source SHA trust | Artifact / record |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `gcp-infrastructure-authenticated.yml` | `ci.yml` dispatch → `ci-operations.yml` | なし（下位jobが取得） | 下位に委譲 | 下位に委譲 | dev: `mypage-dev-runtime-change`、prod: `gcp-infrastructure-production` | `expected_sha == github.sha` | なし |
| `gcp-terraform-authenticated.yml` | 上記から `normal-plan` / `normal-apply` | `terraform-<dev\|prod>-plan` / `-apply`。**reviewer設定なし**。branch policy: dev=`dev`,integration、prod=`main` | GCP: `terraform-<target>-normal-plan` / `-apply` SA（Environment別Secret `GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT`）。AWS: 末尾 `-plan` / `-apply` の別role（`AWS_TERRAFORM_BACKEND_ROLE_ARN`）。Plan identityに書き込み権限がないことを `terraform_normal_ops.py` が検査する | Core GCP資源・Core state | `gcp-terraform-authenticated-<target>` | preflightで actor / ref / `workflow_ref` / SHA を検査、OIDC claim再検査 | HMAC digestをjob output比較（値はログに出さない） |
| `mypage-infrastructure-authenticated.yml` → `mypage-foundation-authenticated.yml` | 上記から `desired-plan` / `desired-apply` | `mypage-dev-plan` / `mypage-dev-apply`（integrationのみ、reviewerなし）。prod用Environmentは未作成 | GCP: `mypage-foundation-plan` / `-apply` SA（`mypage-foundation` pool）。AWS: `MYPAGE_BACKEND_ROLE_ARN` | MyPage GCP資源・MyPage state | `mypage-<target>-foundation` | `foundation_contract.context` | commitment比較 |
| `mypage-foundation-authenticated.yml`（image-build） | `ci.yml` dispatch / integration push | `mypage-dev-apply` | GCP: `mypage-foundation-apply` SA（AWS不使用） | Artifact Registry `mypage/mypage-api:<sha>` | `mypage-<target>-foundation` | `image_ops.preflight` | `image_digest` job output |
| `mypage-backend-authenticated.yml` | `ci.yml` dispatch / integration push自動CD | `mypage-dev-backend` | `mypage-backend-deploy` SA（`run.services.get/update`。棚卸し時点では Job `run.jobs.get/update` と maintenance runtime SAへのactAsも保持） | Cloud Run Service `mypage-api` の image（棚卸し時点では Job `mypage-maintenance-operator` の image も） | `mypage-dev-runtime-change` | `backend_release.context`、rollbackは祖先commitのtagのみ | `mypage-backend-release-record`（棚卸し時点は schema 3 で Job digest の同一性を検査。#2 で schema 4 へ） |
| `mypage-hosting-authenticated.yml` | `ci.yml` dispatch | `mypage-dev-hosting` | `mypage-hosting-deploy` SA（DEV `firebasehosting.admin` ほか） | Firebase Hosting DEV site | `mypage-dev-hosting-release` | `hosting_release.context` | `mypage-dev-hosting-release-record` |
| `mypage-maintenance.yml` | 直接 `workflow_dispatch` | `mypage-maintenance-<env>`（developmentのみ存在、branch policy・reviewerなし。境界はWIF conditionの `ref`） | `mypage-maintenance-invoker` SA | Cloud Run Job execution（Firestore maintenance control） | `mypage-maintenance-<env>` | WIF condition（`workflow_sha == sha`、固定ref） | Job実行ログのみ |

### 2.3 撤去・縮小できるlegacy / migration-only要素

| 要素 | 根拠 | 扱い |
| --- | --- | --- |
| `gcp-terraform-authenticated.yml` の `migration-plan` / `migration-apply` job と `terraform_migration.py` の credentialed経路 | どのcallerからも渡されない（`gcp-infrastructure-authenticated.yml` は `normal-plan` / `normal-apply` のみ） | `terraform.yml` 移行PRで撤去。過去の移行証跡（docs・test fixture）は根拠なく削除しない |
| 同 `normal-e2e` mode | callerなし | 同上 |
| `mypage-foundation-authenticated.yml` の `foundation-*` / `foundation-metadata-*` / `registry-*` / `maintenance-*` / `runtime-*` mode | callerは `desired-plan` / `desired-apply` / `image-build` のみ | 同上。`desired-stage.txt` によるstage選択は維持 |
| `ci.yml` の push自動CD（`mypage-auto-cd`）と `backend_cd.py` | 本設計では application deploy を自動化しない | API deploy移行PRで撤去し、Backend WIF conditionから `push` を削除 |
| Backend releaseの Service/Job 同一digest同期（record schema 3 の `maintenanceJob*`） | API と Operator を独立deploy unitにする | **撤去済み（#2）**。record は schema 4（API のみ） |
| `mypage-maintenance.yml` の未pin `actions/checkout@v7` | 他workflowはSHA pin | 該当workflowに触れるPRでpinする |

### 2.4 既知のギャップ

- **Production Terraform Apply の承認が未設定**: `terraform-prod-apply` に required reviewers がない（branch policy `main` のみ）。UC3の完了条件を満たすには、ownerがGitHub Environmentへ reviewer を設定する。
- **PR Plan用のtrustがない**: 手動 Plan の identity は既に Apply と分離されている（Core: `terraform-<target>-normal-plan` SA と `-plan` AWS role、MyPage: `mypage-foundation-plan` SA）。ただしどの trust も `workflow_dispatch`・owner actor・`ci.yml` 固定であり、`pull_request` からは認証できない。
- **Maintenance mode の認証が一度も成功していない**: 唯一の run（2026-10-29、run 40879914039）は、Environment `mypage-maintenance-development` に `MYPAGE_MAINTENANCE_WIF_PROVIDER` が設定されておらず、認証の前段で停止した。`mypage-maintenance-production` Environment も未作成である。また、この workflow は reusable workflow を使わずに直接認証しており、`job_workflow_ref` を含む `sub` テンプレートの下で認証できるかは未検証である。
- **PR時のnative planがない**: 現在のnative planは owner の手動dispatchのみ。PRでは credentialless validate / mock test だけが走る。
- `gcp-user-activity-schema-audit` は Environment branch policy が `dev` のみで、integration branch からは到達しない（かつ無効化フラグがfalse）。

## 3. 目標構成

### 3.1 利用者に見せるworkflow

| UC | Workflow（予定名） | Trigger | 入力 | 対象環境 |
| --- | --- | --- | --- | --- |
| UC1 | `ci.yml`（名称 `CI`） | `pull_request`（+ 必要なら `push`、deployなし） | なし | なし |
| UC2/UC3 | `terraform.yml`（名称 `Terraform`） | `pull_request`（Planのみ）/ `workflow_dispatch` | `environment=development\|production`、`operation=plan\|apply`、`component=all\|core\|mypage` | dev / prod |
| UC4 | `deploy-mypage-frontend.yml`（`Deploy MyPage Frontend`） | `workflow_dispatch` | なし（developmentのみ） | dev |
| UC5 | `deploy-mypage-api.yml`（`Deploy MyPage API`） | `workflow_dispatch` | `mode=build-and-deploy\|deploy-existing`、`image_digest`、`image_source_sha` | dev |
| UC6 | `deploy-mypage-maintenance.yml`（`Deploy Maintenance Operator`） | `workflow_dispatch` | 同上 | dev |
| UC7 | `mypage-maintenance.yml`（`Set Maintenance Mode`） | `workflow_dispatch` | `environment`、`enabled` | dev / prod |

- 内部の reusable workflow（`gcp-terraform-authenticated.yml`、MyPage Terraform reusable、Hosting / API / Operator の deploy job）は維持してよいが、`workflow_dispatch` を持たせない。
- Production の Frontend / API / Operator deploy は、公開Gate（[#1133](https://github.com/kani3camp/youtube-study-space/issues/1133)）を満たすまで **入力の選択肢として出さない**。
- 自動 application deploy は行わない。PR / push を契機に実環境へwriteするのは無し。
- 各 Deploy workflow は、配備する domain の credentialless 検査を同じ run の中で、資格情報の取得より前に通す（API / Operator: `ci-system.yml`、Frontend: Hosting build job の audit / check / typecheck / test）。統合ブランチの ruleset は PR ごとの `CI Gate` を要求するが、統合後の tree そのものは検査しないため。旧 router の「全 domain の CI 成功」は、配備対象の domain に限った検査へ置き換える。

### 3.2 Terraform Plan / Apply

**PR Plan（UC2 自動）**

- `pull_request` で起動し、変更パスから component を判定する（`infra/gcp/**` → core、`infra/mypage-platform/**` → mypage、共有の CI script / workflow 変更 → 両方）。dev / prod の両方を plan する。
- fork 由来PRでは資格情報を取得しない。`pull_request` は fork に OIDC / secret を渡さないことに加え、job条件で `head.repo.full_name == github.repository` を要求し、満たさない場合は **失敗として** 理由を表示する（false-green skip にしない）。
- PR Plan identity は、Apply identity とも、手動 Plan の identity とも分離して新設する。手動 Plan の identity は owner が dispatch した既知のコードだけを実行する前提で権限（state lock の書き込みなど）を持つ。PR Plan は PR 作成者が変更できるコードを実行するため、同じ identity を共有しない。
  - GCP: 環境ごとの read-only SA（`get` / `list` / `getIamPolicy` 系のみ。既存 [`bootstrap_normal_ops.py`](../../infra/gcp/scripts/bootstrap_normal_ops.py) の READ 集合を基準にする）。
  - AWS State: state object の `GetObject` と bucket の対象prefix `ListBucket` のみ。lock file への書き込み権限は与えず、PR Plan は `-lock=false` で実行する（PR Plan は助言であり、Apply は必ず fresh plan を作るため）。
  - WIF condition: `event_name == 'pull_request'`、`workflow_ref == terraform.yml@refs/pull/<n>/merge`（= `assertion.ref`）、`job_workflow_ref` 固定、repository / owner ID 固定、専用Environment `terraform-<env>-pr-plan`。
- **脅威境界**: 同一repositoryのPRでは workflow 定義そのものがPR作成者の管理下にあるため、trust は「どのコードが実行されるか」を固定できない。よって PR Plan identity の権限（state read と GCP read）そのものが漏えい時の影響上限になる。state には sensitive 値が含まれ得るため、PR Plan を許可する actor は初期状態で owner（`actor_id == 54093651`）に限定する。AIエージェント等の actor を追加する場合は、追加する claim（`actor_id` 等）と state read を委ねてよい理由を PR で明示してから trust を広げる。owner 以外の actor による Terraform 変更PRでは PR Plan job を失敗させ、owner が手動 Plan を実行する。
- 結果は Job Summary と check に要約し、詳細 plan は sensitive 値を除いた JSON / text を短期 artifact に保存する。PRコメントは投稿しない。

**手動 Plan / Apply（UC2 手動 / UC3）**

- 既存の「plan job → apply job で独立に再plan → HMAC digest一致時のみ apply job 自身のplan fileを `terraform apply <plan-file>`」方式を維持する。PRで作った plan file を後日 apply しない。
- source SHA・environment・component は dispatch 入力と `github.sha` で固定し、plan / apply job 間で一致を検査する。
- Apply は `workflow_dispatch` のみ。Production Apply は `terraform-prod-apply`（および将来の `mypage-prod-apply`）に required reviewers を設定して承認必須とする。
- Apply identity は既存のものを使い、PR Plan identity とは共有しない。

### 3.3 API と Maintenance Operator の image 分離

| 項目 | API | Maintenance Operator |
| --- | --- | --- |
| Dockerfile | `system/Dockerfile.mypage-api`（`./cmd/mypage-server` のみ） | `system/Dockerfile.mypage-maintenance-operator`（`./cmd/mypage-maintenance-operator` のみ） |
| Artifact Registry | `mypage/mypage-api@sha256:...` | `mypage/mypage-maintenance-operator@sha256:...`（同じ `mypage` repository の別package。repository単位の既存push権限で足りる） |
| Deploy対象 | Cloud Run Service `mypage-api` のみ | Cloud Run Job `mypage-maintenance-operator` のみ |
| Deployer identity | `mypage-backend-deploy`（Job権限・maintenance SA actAs を **削除**） | 新設 `mypage-operator-deploy`（Job `run.jobs.get/update` と maintenance runtime SA actAs のみ）。SA・WIF・Environment は [`operator_identity.py`](../../infra/mypage-platform/scripts/operator_identity.py)、Job IAM は Terraform が作る |
| Release record | API digest のみを所有（`mypage-backend-release-record` schema 4） | Operator digest のみを所有（`mypage-operator-release-record` schema 1） |
| Terraform | Service image は `ignore_changes` のまま | Job image は `ignore_changes` のまま。`maintenance_bootstrap_image_digest`（Job が存在しない初回作成だけで使う固定の過去build）は `mypage-api` package のまま |

- 「APIの更新でJobを変更しない」は、コードの分岐だけでなく API deployer の IAM から Job 権限を外すことで強制する。
- `core/mypage` の MaintenanceControl / Firestore schema を変える PR は、後方・前方互換を保つか、API と Operator の両方の deploy が必要であることをPR説明に明記する。平常時に同一digestへ同期はしない。
- 現行の旧 Backend release（`backend_release.py` の `job_baseline`）と Terraform 契約（`runtime_contract.py` の maintenance Job image 検査）は、どちらも Job image が `mypage-api` package であることを要求する。そのため、Job を operator package へ切り替える前に、次の2つを済ませておく必要がある。
  1. Backend release から Job 同期をなくす（Service だけを更新する）。
  2. Terraform 契約・`maintenance_bootstrap_image_digest` の検査で、`mypage-api` と `mypage-maintenance-operator` の両 package を受け入れる。
  そのうえで最初の Operator deploy を行い、切り替わったことを確認してから旧 package の許容を撤去する。
- #2 で上の1と2を実施した。API image から operator binary を外しても、Job は配備済みの旧 digest（operator binary 入り）を参照し続けるため安全であり、同じ #2 で分離した。
- MyPage runtime の Terraform 契約は create / no-op だけを許可する。そのため #2 は `mypage-operator-deploy` の binding を **追加** するだけにし、旧 `mypage-backend-deploy` の Job binding と actAs の削除は、初回 Operator deploy の確認後に削除専用の遷移として別に行う（4.2）。

### 3.4 Rollback

- API / Operator: 各 deploy workflow の `mode=deploy-existing` で、検証済み immutable digest と source SHA を指定する。対象 package・Registry 上の存在・source SHA tag と digest の一致・source SHA が現HEADの祖先であることを検査し、対象 resource だけを更新する（現 `backend_release.py` の rollback 検査を流用）。
- Frontend: 現行の Hosting release record は直前 release / version を保持している。Hosting の rollback 方式（過去 version の再公開か、過去 commit からの再build）は Frontend 移行PRで既存能力を確認して決める。

### 3.5 Concurrency

GitHub Actions の concurrency group は job ごとに1つしか持てないため、「Terraform は API / Operator の両方と排他、API と Operator は互いに独立」という関係は group だけでは表せない。resource ownership と race の実害から、次の最小排他とする。

| Group | 参加する操作 | 理由 |
| --- | --- | --- |
| `terraform-core-<env>` | Core Terraform plan / apply（手動） | 同一 state・同一資源 |
| `mypage-<env>-cloud-run` | MyPage Terraform plan / apply（手動）、API deploy、Operator deploy | Terraform は refresh 後に Service / Job 全体を PATCH するため、その間の image 更新を巻き戻し得る。API / Operator 同士は相互に直列化されるが、同一環境の Cloud Run 変更に限定した lock で、全操作の global lock にはしない |
| `mypage-<env>-hosting` | Frontend deploy | Hosting site だけを変更する |
| `mypage-maintenance-<env>` | Maintenance mode 切替 | Firestore の maintenance control を変更する。Job 実行は開始時点の Job 定義を使うため、Operator deploy とは排他にしない |
| `terraform-pr-plan-<pr>`（`cancel-in-progress: true`） | PR Plan | read-only・`-lock=false`。apply を待たせない |

**移行期間中の扱い**: 現行の旧Terraform（dev）と旧Backend releaseは `mypage-dev-runtime-change` を使っている。新しい API / Operator deploy と新しい `terraform.yml` の mypage component も、移行中はこの既存 group に参加する。`mypage-<env>-cloud-run` への改名は、全参加者が新経路へ移った後（移行計画 #7）に1つの PR でまとめて行う。最終PRを取り込む前に、旧 group の実行中・待機中の run がないことを確認する（4.2）。

加えて、deploy script は既存の resourceVersion / etag / 再読取による楽観的排他を維持し、競合時は自動 retry せず停止する。

## 4. 移行計画

作業用統合ブランチ `integration/ci-usecase-redesign-20261030`（base: `integration/mypage-canon-20261006`）へ PR を積み、最後に統合ブランチから `integration/mypage-canon-20261006` へ1本のPRで戻す。`dev` へはマージしない。

作業用統合ブランチの ref は、どの trust（GCP WIF condition・AWS role・GitHub Environment の branch policy）にも含まれていない。したがって、各 PR の新経路は作業用統合ブランチ上では認証できず、**実環境で有効になるのは最終PRを `integration/mypage-canon-20261006` へ取り込み、owner が trust を更新した後** になる。旧経路も最終PRの取り込みと同時にコードから消えるため、取り込みから trust 更新までの間、DEV の Deploy / Terraform 操作は認証段階で停止する（fail closed）。本番の Core Terraform は `main` から実行するため、この統合では影響を受けない。

### 4.1 PR の積み順

| # | PR | 内容 | 必要なowner作業（最終PR取り込み後、リポジトリ外） |
| --- | --- | --- | --- |
| 1 | 棚卸し・目標設計（本書） | — | なし |
| 2 | API / Operator の image 分離と Operator deploy | `Dockerfile.mypage-api` / `Dockerfile.mypage-maintenance-operator`、Backend release から Job 同期を撤去、`deploy-mypage-maintenance.yml` + `mypage-operator-authenticated.yml`、`mypage-operator-deploy` の Job IAM（Terraform、追加のみ）、Terraform 契約の両 package 許容 | `operator_identity.py --apply`、MyPage Terraform Apply、foundation WIF condition へ `deploy-mypage-maintenance.yml` を追加、初回 Operator deploy |
| 3 | API / Frontend の Deploy workflow | `deploy-mypage-api.yml`（build-and-deploy / deploy-existing）、`deploy-mypage-frontend.yml`。router から `mypage-image-build` / `mypage-backend-release` / `mypage-hosting-release` と push 自動CD を撤去 | Backend / Hosting / foundation image-build の WIF condition を新 `workflow_ref` へ更新し、Backend から `push` を削除 |
| 4 | `terraform.yml`（手動 plan / apply、component 入力） | `gcp-plan` / `gcp-apply`、`gcp-infrastructure-authenticated.yml` / `mypage-infrastructure-authenticated.yml` を撤去。到達不能 mode を整理 | Core / MyPage の GCP WIF condition と AWS State role trust を新 `workflow_ref` へ更新。`terraform-prod-apply` に required reviewers |
| 5 | PR Plan | `terraform.yml` の `pull_request` Plan と PR Plan identity の bootstrap | dev / prod の read-only GCP SA・WIF provider、AWS State read role、Environment `terraform-<env>-pr-plan` |
| 6 | Maintenance mode の reusable 化 | 認証 job を reusable workflow へ移し、`actions/checkout` を SHA pin | Maintenance WIF condition（Terraform 管理）への `job_workflow_ref` 追加の Apply、`MYPAGE_MAINTENANCE_WIF_PROVIDER` 設定 |
| 7 | `ci.yml` を純粋なCIへ縮小 | `ci-operations.yml` 削除、schema audit の扱いを確定、concurrency group の改名、docs・契約テストの最終整合 | 旧 claim を全 trust から削除 |
| 8 | 作業用統合ブランチ → `integration/mypage-canon-20261006` | — | 4.2 の順で実施 |

- 各PRは、追加した最上位 workflow を `ci-repository.yml` の workflow 契約テスト（[`test_ci_workflows.py`](../../.github/scripts/test_ci_workflows.py)、[`test_ci_deploy_workflows.py`](../../.github/scripts/test_ci_deploy_workflows.py) ほか）と actionlint の対象に登録する。
- 資格情報を取得する job が reusable workflow の外にないことを `test_ci_deploy_workflows.py` が検査する。既知の例外（`mypage-maintenance.yml`）は #6 で空にする。
- PR 説明では「旧経路が有効か」「新経路がまだ無効か」を必ず書く。

### 4.2 最終PR取り込み後の順序（owner）

1. 取り込み前に、`mypage-dev-runtime-change` などの旧 concurrency group に実行中・待機中の run がないことを確認する。
2. 各PRの「必要なowner作業」を実施し、新しい `workflow_ref` / `job_workflow_ref` を trust に追加する。
3. 新経路を実runで1回ずつ検証する（Terraform 手動 Plan → API deploy → Frontend deploy → Operator deploy → Maintenance mode `enabled=false`）。
4. 初回 Operator deploy で Job が `mypage-maintenance-operator` package へ移ったことを確認してから、次を1つのPRで行う。
   - Terraform 契約・Operator 配備スクリプトから旧 `mypage-api` package の許容を外す。
   - 旧 `mypage-backend-deploy` の Job binding と maintenance runtime SA actAs を削除する（削除専用の契約遷移）。
5. 旧 claim（`ci.yml` を caller とするもの、Backend の `push`）を全 trust から削除する。

## 5. 移行状況

「実装済み」は作業用統合ブランチ上のコードを指す。実環境で有効かどうかは 4.2 の owner 作業と実runで確認する。

| UC | 状態 | 経路 |
| --- | --- | --- |
| UC1 CI | 現行（router 同居） | `ci.yml` |
| UC2 PR Plan | 未実装 | なし（credentialless validate のみ） |
| UC2 手動 Plan / UC3 Apply | 現行 | `ci.yml` dispatch `gcp-plan` / `gcp-apply` |
| UC4 Frontend | 現行 | `ci.yml` dispatch `mypage-hosting-release` |
| UC5 API | 現行（Service のみ。Job 同期は撤去済み） | `ci.yml` dispatch `mypage-backend-release` / `mypage-image-build`、integration push 自動CD |
| UC6 Operator | 実装済み（未有効化） | `deploy-mypage-maintenance.yml` |
| UC7 Maintenance mode | 現行 | `mypage-maintenance.yml`（development のみ Environment 作成済み） |
