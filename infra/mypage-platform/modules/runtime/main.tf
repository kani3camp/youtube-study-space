terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }
}

variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "environment" {
  type = string
  validation {
    condition     = contains(["development", "production"], var.environment)
    error_message = "Invalid MyPage environment."
  }
}

# nullのときは、Terraform外の前提（番号固定のSecret値と環境別Registryのimage）が
# まだ無いことを表す。その間はAPI・maintenance Jobなど実行系の資源を宣言しない。
variable "runtime_settings" {
  type = object({
    image_digest                = string
    project_number              = string
    web_app_id                  = string
    public_origin               = string
    oauth_client_id_version     = string
    oauth_client_secret_version = string
    youtube_api_key_version     = string
    image_source_sha            = optional(string, "")
    hosting_backend_host        = optional(string, "")
    previous_image_digest       = optional(string, "")
    read_metrics_enabled        = optional(bool, false)
  })
  default = null
}

# オーナーが追加した allUsers invoker を State へ取り込むときだけ true にする。
# CIはIAM setter権限を持たないため、Terraformがこのメンバーを新規作成する経路は無い。
variable "public_invoker" {
  type    = bool
  default = false
  validation {
    condition     = !var.public_invoker || var.runtime_settings != null
    error_message = "The public invoker requires the runtime Service."
  }
}

locals {
  # 実行系（Cloud Run API・maintenance Jobとその専用IAM/WIF・Firestore権限）は、
  # Terraform外の前提をそろえてruntime_settingsを渡したときだけ宣言する。
  deploy_api                     = var.runtime_settings != null
  public_api                     = local.deploy_api && var.public_invoker
  runtime_email                  = "${google_service_account.runtime.account_id}@${var.project_id}.iam.gserviceaccount.com"
  runtime_principal              = "serviceAccount:${local.runtime_email}"
  max_instances                  = var.environment == "development" ? 1 : 3
  maintenance_image_release      = local.deploy_api
  maintenance_ref                = var.environment == "production" ? "refs/heads/main" : "refs/heads/integration/mypage-canon-20261006"
  maintenance_project_number     = var.environment == "production" ? "906336399194" : "48101442817"
  maintenance_github_environment = "mypage-maintenance-${var.environment}"
  maintenance_operator_email     = "mypage-maintenance-job@${var.project_id}.iam.gserviceaccount.com"
  maintenance_invoker_email      = "mypage-maintenance-invoker@${var.project_id}.iam.gserviceaccount.com"
  foundation_plan_email          = "mypage-foundation-plan@${var.project_id}.iam.gserviceaccount.com"
  foundation_apply_email         = "mypage-foundation-apply@${var.project_id}.iam.gserviceaccount.com"
  operator_deployer_email        = "mypage-operator-deploy@${var.project_id}.iam.gserviceaccount.com"
  backend_deployer_email         = "mypage-backend-deploy@${var.project_id}.iam.gserviceaccount.com"
  hosting_deployer_email         = "mypage-hosting-deploy@${var.project_id}.iam.gserviceaccount.com"

  # 本番のアプリ配備用本人情報（Backend / Operator / Hosting）。DEVの同名の本人情報は
  # オーナーの手動スクリプトが作成済みでこのStateの外にあるため、ここでは本番だけを宣言する。
  # Cloud Run Service（mypage-api）上の配備者バインディングはCIにService IAMの変更権限を
  # 渡さないためTerraform外（オーナー付与）とし、ここでは宣言しない。
  release_identities = local.deploy_api && var.environment == "production"
  release_workflows  = "kani3camp/youtube-study-space/.github/workflows/"
  release_files = {
    backend  = ["deploy-mypage-api.yml", "mypage-backend-authenticated.yml"]
    operator = ["deploy-mypage-maintenance.yml", "mypage-operator-authenticated.yml"]
    hosting  = ["deploy-mypage-frontend.yml", "mypage-hosting-authenticated.yml"]
  }
  # infra/mypage-platform/scripts/foundation_contract.py の release_condition と同じ文字列を作る
  # （Plan契約が完全一致を検査する）。クレーム（claim）の順序も同じにする。
  release_claims = {
    for component, files in local.release_files : component => [
      ["repository_id", "340900071"],
      ["repository_owner_id", "54093651"],
      ["actor_id", "54093651"],
      ["actor", "kani3camp"],
      ["repository", "kani3camp/youtube-study-space"],
      ["ref", local.maintenance_ref],
      ["workflow_ref", "${local.release_workflows}${files[0]}@${local.maintenance_ref}"],
      ["job_workflow_ref", "${local.release_workflows}${files[1]}@${local.maintenance_ref}"],
      ["event_name", "workflow_dispatch"],
      ["run_attempt", "1"],
      ["environment", "mypage-prod-${component}"],
    ]
  }
  release_conditions = {
    for component, claims in local.release_claims : component => format(
      "(%s && assertion.sub == \"%s\" && assertion.workflow_sha == assertion.job_workflow_sha)",
      join(" && ", [for claim in claims : "assertion.${claim[0]} == \"${claim[1]}\""]),
      join(":", [
        for key in ["repository_id", "repository_owner_id", "environment", "ref", "workflow_ref", "job_workflow_ref", "event_name"] :
        "${key}:${replace(one([for claim in claims : claim[1] if claim[0] == key]), ":", "%3A")}"
      ])
    )
  }
  release_subject = "assertion.repository_id+\":\"+assertion.environment+\":\"+assertion.actor_id"
}

# 管理対象は新しいMyPageメタデータであり、既存のBot/Firestore資源は含めない。
# Service Account・Secretの器・Registryは、Secret値の投入とimage pushより前に必要なので常に宣言する。
resource "google_service_account" "runtime" {
  project      = var.project_id
  account_id   = "mypage-api"
  display_name = "YouTube Study Space MyPage API"
  description  = "Keyless Cloud Run BFF runtime; no service account JSON key."
  lifecycle {
    prevent_destroy = true
  }
}

# 環境専用のイメージ保存先を、ダイジェスト生成前に構築する。
# 既存Functionsのリポジトリや、他のArtifact Registry所有者は変更しない。
resource "google_artifact_registry_repository" "mypage" {
  project                = var.project_id
  location               = var.region
  repository_id          = "mypage"
  description            = "YouTube Study Space MyPage ${var.environment} images"
  format                 = "DOCKER"
  mode                   = "STANDARD_REPOSITORY"
  cleanup_policy_dry_run = true
  docker_config {
    immutable_tags = true
  }
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret" "oauth_client_id" {
  project             = var.project_id
  secret_id           = "mypage-oauth-client-id"
  deletion_protection = true
  replication {
    auto {}
  }
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret" "oauth_client_secret" {
  project             = var.project_id
  secret_id           = "mypage-oauth-client-secret"
  deletion_protection = true
  replication {
    auto {}
  }
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_secret_manager_secret" "youtube_api_key" {
  project             = var.project_id
  secret_id           = "mypage-youtube-api-key"
  deletion_protection = true
  replication {
    auto {}
  }
  lifecycle {
    prevent_destroy = true
  }
}

# IAMメンバーは追加だけを行い、プロジェクトのIAMポリシーやバインディング全体を置き換えない。
resource "google_secret_manager_secret_iam_member" "oauth_client_id_runtime" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.oauth_client_id.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.runtime_principal
}

resource "google_secret_manager_secret_iam_member" "oauth_client_secret_runtime" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.oauth_client_secret.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.runtime_principal
}

resource "google_secret_manager_secret_iam_member" "youtube_api_key_runtime" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.youtube_api_key.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.runtime_principal
}

# Firebase Custom Tokenの署名にはIAM signBlobを使う。
# 専用ランタイムサービスアカウントだけに付与し、プロジェクト内の全サービスアカウントへ広げない。
resource "google_service_account_iam_member" "runtime_signer" {
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.runtime_email}"
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = local.runtime_principal
}

# 現在のBFFはStudy Spaceの履歴を読み、自身のアカウントとOAuthメタデータだけを書き込む。
# Firestore IAMはコレクション単位で制限できないため、プロジェクト全体のdatastore.user権限は
# APIを作成するときだけ付与する。
resource "google_project_iam_member" "firestore_user" {
  count   = local.deploy_api ? 1 : 0
  project = var.project_id
  role    = "roles/datastore.user"
  member  = local.runtime_principal
}

resource "google_cloud_run_v2_service" "api" {
  count                = local.deploy_api ? 1 : 0
  project              = var.project_id
  name                 = "mypage-api"
  location             = var.region
  ingress              = "INGRESS_TRAFFIC_ALL"
  deletion_policy      = "PREVENT"
  deletion_protection  = true
  invoker_iam_disabled = false
  launch_stage         = "GA"

  scaling {
    min_instance_count = 0
    max_instance_count = local.max_instances
    scaling_mode       = "AUTOMATIC"
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  template {
    service_account                  = local.runtime_email
    timeout                          = "30s"
    max_instance_request_concurrency = 40
    execution_environment            = "EXECUTION_ENVIRONMENT_GEN2"

    scaling {
      min_instance_count = 0
      max_instance_count = local.max_instances
    }

    containers {
      image = var.runtime_settings.image_digest

      ports {
        container_port = 8080
        name           = "http1"
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle          = true
        startup_cpu_boost = false
      }

      startup_probe {
        initial_delay_seconds = 0
        timeout_seconds       = 1
        period_seconds        = 10
        failure_threshold     = 3
        tcp_socket {
          port = 8080
        }
      }

      env {
        name  = "GOOGLE_CLOUD_PROJECT"
        value = var.project_id
      }
      env {
        name  = "MYPAGE_ENVIRONMENT"
        value = var.environment
      }
      env {
        name  = "MYPAGE_REGION"
        value = var.region
      }
      env {
        name  = "MYPAGE_PROJECT_NUMBER"
        value = var.runtime_settings.project_number
      }
      env {
        name  = "MYPAGE_WEB_APP_ID"
        value = var.runtime_settings.web_app_id
      }
      env {
        name  = "MYPAGE_PUBLIC_ORIGIN"
        value = var.runtime_settings.public_origin
      }
      env {
        name  = "MYPAGE_SIGNER_EMAIL"
        value = local.runtime_email
      }
      # 明示的に有効化した場合だけ使う。モジュールを直接呼んでも本番では無効にする。
      # この実装だけでは配信リビジョンの更新を承認しない。
      dynamic "env" {
        for_each = var.environment == "development" && var.runtime_settings.read_metrics_enabled ? [true] : []
        content {
          name  = "MYPAGE_READ_METRICS"
          value = "true"
        }
      }
      env {
        name  = "MYPAGE_INFRASTRUCTURE_READY"
        value = "true"
      }
      env {
        name = "MYPAGE_OAUTH_CLIENT_ID"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.oauth_client_id.secret_id
            version = var.runtime_settings.oauth_client_id_version
          }
        }
      }
      env {
        name = "MYPAGE_OAUTH_CLIENT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.oauth_client_secret.secret_id
            version = var.runtime_settings.oauth_client_secret_version
          }
        }
      }
      env {
        name = "MYPAGE_YOUTUBE_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.youtube_api_key.secret_id
            version = var.runtime_settings.youtube_api_key_version
          }
        }
      }
      dynamic "env" {
        for_each = var.runtime_settings.hosting_backend_host == "" ? [] : [var.runtime_settings.hosting_backend_host]
        content {
          name  = "MYPAGE_HOSTING_BACKEND_HOST"
          value = env.value
        }
      }
    }
  }

  depends_on = [
    google_secret_manager_secret_iam_member.oauth_client_id_runtime,
    google_secret_manager_secret_iam_member.oauth_client_secret_runtime,
    google_secret_manager_secret_iam_member.youtube_api_key_runtime,
    google_service_account_iam_member.runtime_signer,
    google_project_iam_member.firestore_user,
  ]

  lifecycle {
    prevent_destroy = true
    # Application CI/CD owns only the container image after initial creation.
    # All IAM, env, secrets, scale and networking fields remain Terraform-owned.
    ignore_changes = [template[0].containers[0].image]
  }
}

# 未認証の外部アクセスは、オーナーが追加したメンバーを取り込む場合だけ宣言する。
# 各アプリケーションエンドポイントは引き続きAuth/App Check/Originを検証する。
resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  count    = local.public_api ? 1 : 0
  project  = var.project_id
  location = google_cloud_run_v2_service.api[0].location
  name     = google_cloud_run_v2_service.api[0].name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# メンテナンスは独立した認証・権限の系統を使う。GitHubはこのJobだけを実行できる。
# Firestore権限は持たず、Terraform配備用の本人情報も再利用できない。
resource "google_service_account" "maintenance_operator" {
  count           = local.deploy_api ? 1 : 0
  project         = var.project_id
  account_id      = "mypage-maintenance-job"
  display_name    = "MyPage maintenance state operator"
  description     = "Keyless Cloud Run Job identity for the fixed maintenance operator."
  disabled        = false
  deletion_policy = "PREVENT"
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_service_account" "maintenance_invoker" {
  count           = local.deploy_api ? 1 : 0
  project         = var.project_id
  account_id      = "mypage-maintenance-invoker"
  display_name    = "MyPage maintenance GitHub invoker"
  description     = "Federated identity limited to invoking the maintenance Job."
  disabled        = false
  deletion_policy = "PREVENT"
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_project_iam_custom_role" "maintenance_firestore" {
  count           = local.deploy_api ? 1 : 0
  project         = var.project_id
  role_id         = "mypageMaintenanceOperator"
  title           = "MyPage maintenance control writer"
  description     = "Minimum Firestore entity permissions for the maintenance control document."
  deletion_policy = "PREVENT"
  permissions = [
    "datastore.databases.get",
    "datastore.entities.create",
    "datastore.entities.get",
    "datastore.entities.update",
  ]
  stage = "GA"
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_project_iam_member" "maintenance_firestore" {
  count   = local.deploy_api ? 1 : 0
  project = var.project_id
  role    = "projects/${var.project_id}/roles/mypageMaintenanceOperator"
  member  = "serviceAccount:${local.maintenance_operator_email}"

  depends_on = [
    google_project_iam_custom_role.maintenance_firestore,
    google_service_account.maintenance_operator,
  ]
}

resource "google_iam_workload_identity_pool" "maintenance_github" {
  count                     = local.deploy_api ? 1 : 0
  project                   = var.project_id
  workload_identity_pool_id = "mypage-maintenance"
  display_name              = "MyPage maintenance GitHub"
  description               = "Environment-scoped GitHub Actions trust for maintenance Job invocation."
  disabled                  = false
  mode                      = "FEDERATION_ONLY"
  deletion_policy           = "PREVENT"
}

resource "google_iam_workload_identity_pool_provider" "maintenance_github" {
  count                              = local.deploy_api ? 1 : 0
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.maintenance_github[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "MyPage maintenance workflow"
  description                        = "GitHub OIDC provider restricted to the reviewed maintenance workflow."
  disabled                           = false
  deletion_policy                    = "PREVENT"
  attribute_mapping = {
    # Google STS limits google.subject to 127 bytes; the custom OIDC sub is far longer.
    "google.subject"                = "assertion.repository_id+\":\"+assertion.environment+\":\"+assertion.actor_id"
    "attribute.actor"               = "assertion.actor"
    "attribute.repository_id"       = "assertion.repository_id"
    "attribute.repository_owner_id" = "assertion.repository_owner_id"
    "attribute.actor_id"            = "assertion.actor_id"
    "attribute.run_attempt"         = "assertion.run_attempt"
    "attribute.ref"                 = "assertion.ref"
    "attribute.sha"                 = "assertion.sha"
    "attribute.workflow_ref"        = "assertion.workflow_ref"
    "attribute.workflow_sha"        = "assertion.workflow_sha"
    "attribute.environment"         = "assertion.environment"
    "attribute.event_name"          = "assertion.event_name"
  }
  attribute_condition = "assertion.repository_id == '340900071' && assertion.repository_owner_id == '54093651' && assertion.actor_id == '54093651' && assertion.actor == 'kani3camp' && assertion.run_attempt == '1' && assertion.event_name == 'workflow_dispatch' && assertion.ref == '${local.maintenance_ref}' && assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@${local.maintenance_ref}' && assertion.job_workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/mypage-maintenance-authenticated.yml@${local.maintenance_ref}' && assertion.environment == '${local.maintenance_github_environment}' && assertion.workflow_sha == assertion.sha"
  oidc {
    issuer_uri        = "https://token.actions.githubusercontent.com"
    allowed_audiences = []
  }
}

resource "google_service_account_iam_member" "maintenance_workload_identity" {
  count              = local.deploy_api ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.maintenance_invoker_email}"
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${local.maintenance_project_number}/locations/global/workloadIdentityPools/mypage-maintenance/attribute.workflow_ref/kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@${local.maintenance_ref}"

  depends_on = [
    google_service_account.maintenance_invoker,
    google_iam_workload_identity_pool_provider.maintenance_github,
  ]
}

resource "google_cloud_run_v2_job" "maintenance_operator" {
  count               = local.deploy_api ? 1 : 0
  project             = var.project_id
  name                = "mypage-maintenance-operator"
  location            = var.region
  deletion_policy     = "PREVENT"
  deletion_protection = true
  launch_stage        = "GA"

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account       = local.maintenance_operator_email
      max_retries           = 0
      timeout               = "60s"
      execution_environment = "EXECUTION_ENVIRONMENT_GEN2"
      containers {
        # Only the first creation uses the reviewed API bootstrap digest. The
        # Deploy Maintenance Operator workflow then owns the Job image.
        image   = var.runtime_settings.image_digest
        command = ["/app/mypage-maintenance-operator"]
        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }
        env {
          name  = "GOOGLE_CLOUD_PROJECT"
          value = var.project_id
        }
        env {
          name  = "MYPAGE_ENVIRONMENT"
          value = var.environment
        }
      }
    }
  }

  depends_on = [google_project_iam_member.maintenance_firestore]
  lifecycle {
    prevent_destroy = true
    # The Deploy Maintenance Operator workflow owns the Job image, separately
    # from the API Service image. Terraform retains every non-image field.
    ignore_changes = [template[0].template[0].containers[0].image]
  }
}

# The DEV role keeps its original description because the runtime contract
# admits only create/no-op for it; Production creates it with the accurate one.
resource "google_project_iam_custom_role" "maintenance_image_release" {
  count           = local.maintenance_image_release ? 1 : 0
  project         = var.project_id
  role_id         = "mypageMaintenanceImageRelease"
  title           = "MyPage maintenance image release"
  description     = var.environment == "development" ? "Allows the existing backend deployer to read and update only the maintenance Cloud Run Job." : "Allows the Operator deployer to read and update only the maintenance Cloud Run Job."
  deletion_policy = "PREVENT"
  permissions     = ["run.jobs.get", "run.jobs.update"]
  stage           = "GA"

  lifecycle {
    prevent_destroy = true
  }
}

# The dedicated operator deployer is the only Job image releaser. DEV's
# deployer SA is owner-created; Production's is declared below.
resource "google_service_account_iam_member" "operator_image_release_act_as" {
  count              = local.maintenance_image_release ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.maintenance_operator_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.operator_deployer_email}"

  depends_on = [
    google_service_account.maintenance_operator,
    google_service_account.operator_deployer,
  ]
}

resource "google_cloud_run_v2_job_iam_member" "operator_image_release" {
  count    = local.maintenance_image_release ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.maintenance_operator[0].name
  role     = "projects/${var.project_id}/roles/mypageMaintenanceImageRelease"
  member   = "serviceAccount:${local.operator_deployer_email}"

  depends_on = [
    google_project_iam_custom_role.maintenance_image_release,
    google_cloud_run_v2_job.maintenance_operator,
    google_service_account.operator_deployer,
  ]
}

# gcloud validates argument overrides by reading the Job, then polls the
# Execution for --wait. jobsExecutorWithOverrides does not include these reads.
resource "google_project_iam_custom_role" "maintenance_execution_read" {
  count           = local.deploy_api ? 1 : 0
  project         = var.project_id
  role_id         = "mypageMaintenanceExecutionRead"
  title           = "MyPage maintenance execution reader"
  description     = "Allows the maintenance invoker to read the fixed Job and wait for its execution."
  deletion_policy = "PREVENT"
  permissions     = ["run.executions.get", "run.jobs.get"]
  stage           = "GA"

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_cloud_run_v2_job_iam_member" "maintenance_reader" {
  count    = local.deploy_api ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.maintenance_operator[0].name
  role     = "projects/${var.project_id}/roles/mypageMaintenanceExecutionRead"
  member   = "serviceAccount:${local.maintenance_invoker_email}"

  depends_on = [
    google_project_iam_custom_role.maintenance_execution_read,
    google_service_account.maintenance_invoker,
    google_cloud_run_v2_job.maintenance_operator,
  ]
}

resource "google_cloud_run_v2_job_iam_member" "maintenance_invoker" {
  count    = local.deploy_api ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.maintenance_operator[0].name
  role     = "roles/run.jobsExecutorWithOverrides"
  member   = "serviceAccount:${local.maintenance_invoker_email}"

  depends_on = [
    google_service_account.maintenance_invoker,
    google_cloud_run_v2_job.maintenance_operator,
  ]
}

# Production application deployers. Each deploy unit has its own keyless SA,
# WIF pool/provider and GitHub Environment. The provider admits only the
# owner's first manual run of that unit's Deploy workflow from main, and the SA
# trusts only that Environment's subject.

resource "google_service_account" "backend_deployer" {
  count           = local.release_identities ? 1 : 0
  project         = var.project_id
  account_id      = "mypage-backend-deploy"
  display_name    = "MyPage Backend release"
  description     = "Keyless GitHub deployer that updates only the mypage-api Service image."
  disabled        = false
  deletion_policy = "PREVENT"
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_service_account" "operator_deployer" {
  count           = local.release_identities ? 1 : 0
  project         = var.project_id
  account_id      = "mypage-operator-deploy"
  display_name    = "MyPage Operator release"
  description     = "Keyless GitHub deployer that updates only the maintenance Job image."
  disabled        = false
  deletion_policy = "PREVENT"
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_service_account" "hosting_deployer" {
  count           = local.release_identities ? 1 : 0
  project         = var.project_id
  account_id      = "mypage-hosting-deploy"
  display_name    = "MyPage Hosting release"
  description     = "Keyless GitHub deployer that publishes only the MyPage Hosting site."
  disabled        = false
  deletion_policy = "PREVENT"
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_iam_workload_identity_pool" "backend_release" {
  count                     = local.release_identities ? 1 : 0
  project                   = var.project_id
  workload_identity_pool_id = "mypage-backend"
  display_name              = "MyPage Backend release"
  description               = "Environment-scoped GitHub Actions trust for the MyPage Backend deploy."
  disabled                  = false
  mode                      = "FEDERATION_ONLY"
  deletion_policy           = "PREVENT"
}

resource "google_iam_workload_identity_pool_provider" "backend_release" {
  count                              = local.release_identities ? 1 : 0
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.backend_release[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "github-actions"
  display_name                       = "MyPage Backend workflow"
  description                        = "GitHub OIDC provider restricted to the reviewed Backend deploy."
  disabled                           = false
  deletion_policy                    = "PREVENT"
  attribute_mapping = {
    "google.subject" = local.release_subject
  }
  attribute_condition = local.release_conditions["backend"]
  oidc {
    issuer_uri        = "https://token.actions.githubusercontent.com"
    allowed_audiences = []
  }
}

resource "google_service_account_iam_member" "backend_release_federation" {
  count              = local.release_identities ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.backend_deployer_email}"
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/projects/${local.maintenance_project_number}/locations/global/workloadIdentityPools/mypage-backend/subject/340900071:mypage-prod-backend:54093651"

  depends_on = [
    google_service_account.backend_deployer,
    google_iam_workload_identity_pool_provider.backend_release,
  ]
}

resource "google_iam_workload_identity_pool" "operator_release" {
  count                     = local.release_identities ? 1 : 0
  project                   = var.project_id
  workload_identity_pool_id = "mypage-operator"
  display_name              = "MyPage Operator release"
  description               = "Environment-scoped GitHub Actions trust for the MyPage Operator deploy."
  disabled                  = false
  mode                      = "FEDERATION_ONLY"
  deletion_policy           = "PREVENT"
}

resource "google_iam_workload_identity_pool_provider" "operator_release" {
  count                              = local.release_identities ? 1 : 0
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.operator_release[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "github-actions"
  display_name                       = "MyPage Operator workflow"
  description                        = "GitHub OIDC provider restricted to the reviewed Operator deploy."
  disabled                           = false
  deletion_policy                    = "PREVENT"
  attribute_mapping = {
    "google.subject" = local.release_subject
  }
  attribute_condition = local.release_conditions["operator"]
  oidc {
    issuer_uri        = "https://token.actions.githubusercontent.com"
    allowed_audiences = []
  }
}

resource "google_service_account_iam_member" "operator_release_federation" {
  count              = local.release_identities ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.operator_deployer_email}"
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/projects/${local.maintenance_project_number}/locations/global/workloadIdentityPools/mypage-operator/subject/340900071:mypage-prod-operator:54093651"

  depends_on = [
    google_service_account.operator_deployer,
    google_iam_workload_identity_pool_provider.operator_release,
  ]
}

resource "google_iam_workload_identity_pool" "hosting_release" {
  count                     = local.release_identities ? 1 : 0
  project                   = var.project_id
  workload_identity_pool_id = "mypage-hosting"
  display_name              = "MyPage Hosting release"
  description               = "Environment-scoped GitHub Actions trust for the MyPage Hosting deploy."
  disabled                  = false
  mode                      = "FEDERATION_ONLY"
  deletion_policy           = "PREVENT"
}

resource "google_iam_workload_identity_pool_provider" "hosting_release" {
  count                              = local.release_identities ? 1 : 0
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.hosting_release[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "github-actions"
  display_name                       = "MyPage Hosting workflow"
  description                        = "GitHub OIDC provider restricted to the reviewed Hosting deploy."
  disabled                           = false
  deletion_policy                    = "PREVENT"
  attribute_mapping = {
    "google.subject" = local.release_subject
  }
  attribute_condition = local.release_conditions["hosting"]
  oidc {
    issuer_uri        = "https://token.actions.githubusercontent.com"
    allowed_audiences = []
  }
}

resource "google_service_account_iam_member" "hosting_release_federation" {
  count              = local.release_identities ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.hosting_deployer_email}"
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/projects/${local.maintenance_project_number}/locations/global/workloadIdentityPools/mypage-hosting/subject/340900071:mypage-prod-hosting:54093651"

  depends_on = [
    google_service_account.hosting_deployer,
    google_iam_workload_identity_pool_provider.hosting_release,
  ]
}

# The owner binds this role on the mypage-api Service (outside Terraform).
resource "google_project_iam_custom_role" "backend_release" {
  count           = local.release_identities ? 1 : 0
  project         = var.project_id
  role_id         = "mypageBackendRelease"
  title           = "MyPage Backend release"
  description     = "Allows the Backend deployer to read and update the mypage-api Service it is bound on."
  deletion_policy = "PREVENT"
  permissions     = ["run.services.get", "run.services.update"]
  stage           = "GA"

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_service_account_iam_member" "backend_release_act_as" {
  count              = local.release_identities ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.runtime_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.backend_deployer_email}"

  depends_on = [google_service_account.backend_deployer]
}

resource "google_artifact_registry_repository_iam_member" "backend_release_reader" {
  count      = local.release_identities ? 1 : 0
  project    = var.project_id
  location   = var.region
  repository = "mypage"
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${local.backend_deployer_email}"

  depends_on = [
    google_artifact_registry_repository.mypage,
    google_service_account.backend_deployer,
  ]
}

resource "google_artifact_registry_repository_iam_member" "operator_release_reader" {
  count      = local.release_identities ? 1 : 0
  project    = var.project_id
  location   = var.region
  repository = "mypage"
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${local.operator_deployer_email}"

  depends_on = [
    google_artifact_registry_repository.mypage,
    google_service_account.operator_deployer,
  ]
}

resource "google_project_iam_member" "backend_release_hosting_viewer" {
  count   = local.release_identities ? 1 : 0
  project = var.project_id
  role    = "roles/firebasehosting.viewer"
  member  = "serviceAccount:${local.backend_deployer_email}"

  depends_on = [google_service_account.backend_deployer]
}

resource "google_project_iam_member" "backend_release_quota" {
  count   = local.release_identities ? 1 : 0
  project = var.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = "serviceAccount:${local.backend_deployer_email}"

  depends_on = [google_service_account.backend_deployer]
}

resource "google_project_iam_member" "operator_release_quota" {
  count   = local.release_identities ? 1 : 0
  project = var.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = "serviceAccount:${local.operator_deployer_email}"

  depends_on = [google_service_account.operator_deployer]
}

resource "google_project_iam_member" "hosting_release_admin" {
  count   = local.release_identities ? 1 : 0
  project = var.project_id
  role    = "roles/firebasehosting.admin"
  member  = "serviceAccount:${local.hosting_deployer_email}"

  depends_on = [google_service_account.hosting_deployer]
}

resource "google_project_iam_member" "hosting_release_quota" {
  count   = local.release_identities ? 1 : 0
  project = var.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = "serviceAccount:${local.hosting_deployer_email}"

  depends_on = [google_service_account.hosting_deployer]
}

output "runtime_enabled" {
  value = local.deploy_api
}

output "public_invoker_enabled" {
  value = local.public_api
}

output "service_name" {
  value = local.deploy_api ? google_cloud_run_v2_service.api[0].name : null
}

output "service_url" {
  value = local.deploy_api ? google_cloud_run_v2_service.api[0].uri : null
}

output "maintenance_workload_identity_provider" {
  value = local.deploy_api ? google_iam_workload_identity_pool_provider.maintenance_github[0].name : null
}

output "maintenance_invoker_service_account" {
  value = local.deploy_api ? local.maintenance_invoker_email : null
}

output "maintenance_job_name" {
  value = local.deploy_api ? google_cloud_run_v2_job.maintenance_operator[0].name : null
}

# Security Current Canonで定めた環境別の容量上限を適用する。
output "max_instances" {
  value = local.max_instances
}

# 認証情報を使わないモックテストでは、この固定かつ個人を識別しない環境変数の値だけを調べる。
output "read_metrics_env_values" {
  value = local.deploy_api ? [
    for item in google_cloud_run_v2_service.api[0].template[0].containers[0].env : item.value
    if item.name == "MYPAGE_READ_METRICS"
  ] : []
}

# 段階変数の撤去で常設になった資源の、旧count付きアドレスからの移動（no-op）。
moved {
  from = google_artifact_registry_repository.mypage[0]
  to   = google_artifact_registry_repository.mypage
}

moved {
  from = google_secret_manager_secret_iam_member.oauth_client_id_runtime[0]
  to   = google_secret_manager_secret_iam_member.oauth_client_id_runtime
}

moved {
  from = google_secret_manager_secret_iam_member.oauth_client_secret_runtime[0]
  to   = google_secret_manager_secret_iam_member.oauth_client_secret_runtime
}

moved {
  from = google_secret_manager_secret_iam_member.youtube_api_key_runtime[0]
  to   = google_secret_manager_secret_iam_member.youtube_api_key_runtime
}

moved {
  from = google_service_account_iam_member.runtime_signer[0]
  to   = google_service_account_iam_member.runtime_signer
}
