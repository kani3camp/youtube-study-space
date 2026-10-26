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

variable "stage" {
  type = string
  validation {
    condition     = contains(["foundation_metadata", "foundation", "registry", "maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"], var.stage)
    error_message = "Invalid MyPage module stage."
  }
}

variable "runtime_settings" {
  type = object({
    image_digest                          = string
    project_number                        = string
    web_app_id                            = string
    public_origin                         = string
    privacy_version                       = string
    terms_version                         = string
    oauth_client_id_version               = string
    oauth_client_secret_version           = string
    youtube_api_key_version               = string
    history_coverage_json                 = optional(string, "")
    image_source_sha                      = optional(string, "")
    hosting_backend_host                  = optional(string, "")
    previous_image_digest                 = optional(string, "")
    read_metrics_enabled                  = optional(bool, false)
    maintenance_operator_enabled          = optional(bool, false)
    maintenance_operator_image_digest     = optional(string, "")
    maintenance_operator_image_source_sha = optional(string, "")
  })
}

locals {
  deploy_api                     = contains(["maintenance_upgrade", "runtime_private", "runtime_public"], var.stage)
  public_api                     = contains(["maintenance_upgrade", "runtime_public"], var.stage)
  runtime_email                  = "${google_service_account.runtime.account_id}@${var.project_id}.iam.gserviceaccount.com"
  runtime_principal              = "serviceAccount:${local.runtime_email}"
  max_instances                  = var.environment == "development" ? 1 : 3
  maintenance_operator           = contains(["maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"], var.stage) && var.runtime_settings.maintenance_operator_enabled
  maintenance_ref                = var.environment == "production" ? "refs/heads/main" : "refs/heads/integration/mypage-canon-20261006"
  maintenance_project_number     = var.environment == "production" ? "906336399194" : "48101442817"
  maintenance_github_environment = "mypage-maintenance-${var.environment}"
  maintenance_operator_email     = "mypage-maintenance-job@${var.project_id}.iam.gserviceaccount.com"
  maintenance_invoker_email      = "mypage-maintenance-invoker@${var.project_id}.iam.gserviceaccount.com"
  maintenance_bootstrap_email    = "mypage-maintenance-bootstrap@${var.project_id}.iam.gserviceaccount.com"
  foundation_plan_email          = "mypage-foundation-plan@${var.project_id}.iam.gserviceaccount.com"
  foundation_apply_email         = "mypage-foundation-apply@${var.project_id}.iam.gserviceaccount.com"
  backend_deployer_email         = "mypage-backend-deploy@${var.project_id}.iam.gserviceaccount.com"
}

# These resources are created only when the caller leaves the default disabled
# stage. They own new MyPage metadata, never existing Bot/Firestore resources.
resource "google_service_account" "runtime" {
  project      = var.project_id
  account_id   = "mypage-api"
  display_name = "YouTube Study Space MyPage API"
  description  = "Keyless Cloud Run BFF runtime; no service account JSON key."
  lifecycle {
    prevent_destroy = true
  }
}

# DEV image storage is provisioned before building a digest for Cloud Run.
# Existing Functions repositories and production ownership are untouched.
resource "google_artifact_registry_repository" "mypage" {
  count                  = var.environment == "development" && contains(["registry", "maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"], var.stage) ? 1 : 0
  project                = var.project_id
  location               = var.region
  repository_id          = "mypage"
  description            = "YouTube Study Space MyPage development images"
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

# Additive IAM membership; never replace a project's IAM policy or bindings.
resource "google_secret_manager_secret_iam_member" "oauth_client_id_runtime" {
  count     = var.stage == "foundation_metadata" ? 0 : 1
  project   = var.project_id
  secret_id = google_secret_manager_secret.oauth_client_id.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.runtime_principal
}

resource "google_secret_manager_secret_iam_member" "oauth_client_secret_runtime" {
  count     = var.stage == "foundation_metadata" ? 0 : 1
  project   = var.project_id
  secret_id = google_secret_manager_secret.oauth_client_secret.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.runtime_principal
}

resource "google_secret_manager_secret_iam_member" "youtube_api_key_runtime" {
  count     = var.stage == "foundation_metadata" ? 0 : 1
  project   = var.project_id
  secret_id = google_secret_manager_secret.youtube_api_key.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.runtime_principal
}

# Firebase Custom Token signing uses IAM signBlob; bind only the dedicated
# runtime service account resource, never all service accounts in the project.
resource "google_service_account_iam_member" "runtime_signer" {
  count              = var.stage == "foundation_metadata" ? 0 : 1
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.runtime_email}"
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = local.runtime_principal
}

# The current BFF reads Study Space history and writes only its own account /
# OAuth metadata. Firestore IAM cannot restrict access by collection; review
# the project-wide datastore.user impact before runtime_private activation.
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
  deletion_protection  = true
  invoker_iam_disabled = false

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
      env {
        name  = "MYPAGE_PRIVACY_VERSION"
        value = var.runtime_settings.privacy_version
      }
      env {
        name  = "MYPAGE_TERMS_VERSION"
        value = var.runtime_settings.terms_version
      }
      # No historical completeness is assumed without an inventory-backed
      # explicit interval list. The server validates the intervals at startup.
      dynamic "env" {
        for_each = var.runtime_settings.history_coverage_json == "" ? [] : [var.runtime_settings.history_coverage_json]
        content {
          name  = "MYPAGE_HISTORY_COVERAGE"
          value = env.value
        }
      }
      # Opt-in only. Keep production OFF even when the module is called directly.
      # This source capability does not authorize a serving-revision update.
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
  }
}

# Only the explicit runtime_public stage grants an unauthenticated edge.
# Every application endpoint still performs its own Auth/App Check/Origin gate.
resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  count    = local.public_api ? 1 : 0
  project  = var.project_id
  location = google_cloud_run_v2_service.api[0].location
  name     = google_cloud_run_v2_service.api[0].name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# Maintenance has a separate trust chain. GitHub can invoke exactly this Job,
# but has no Firestore role and cannot reuse the Terraform deployment identity.
resource "google_service_account" "maintenance_operator" {
  count        = local.maintenance_operator ? 1 : 0
  project      = var.project_id
  account_id   = "mypage-maintenance-job"
  display_name = "MyPage maintenance state operator"
  description  = "Keyless Cloud Run Job identity for the fixed maintenance CAS binary."
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_service_account" "maintenance_invoker" {
  count        = local.maintenance_operator ? 1 : 0
  project      = var.project_id
  account_id   = "mypage-maintenance-invoker"
  display_name = "MyPage maintenance GitHub invoker"
  description  = "Federated identity limited to invoking the maintenance Job."
  lifecycle {
    prevent_destroy = true
  }
}

# This identity is deliberately read-only. Foundation and release identities
# can invoke its fixed Job but cannot pass arguments or read Firestore directly.
resource "google_service_account" "maintenance_bootstrap" {
  count        = local.maintenance_operator ? 1 : 0
  project      = var.project_id
  account_id   = "mypage-maintenance-bootstrap"
  display_name = "MyPage maintenance bootstrap reader"
  description  = "Read-only identity for the fixed pre-deployment initialized-control check."
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_project_iam_custom_role" "maintenance_firestore" {
  count       = local.maintenance_operator ? 1 : 0
  project     = var.project_id
  role_id     = "mypageMaintenanceOperator"
  title       = "MyPage maintenance control writer"
  description = "Minimum Firestore entity permissions for the maintenance control and audit transaction."
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
  count   = local.maintenance_operator ? 1 : 0
  project = var.project_id
  role    = "projects/${var.project_id}/roles/mypageMaintenanceOperator"
  member  = "serviceAccount:${local.maintenance_operator_email}"

  depends_on = [google_project_iam_custom_role.maintenance_firestore]
}

resource "google_project_iam_custom_role" "maintenance_bootstrap_reader" {
  count       = local.maintenance_operator ? 1 : 0
  project     = var.project_id
  role_id     = "mypageMaintenanceBootstrapReader"
  title       = "MyPage maintenance bootstrap reader"
  description = "Read-only Firestore entity permissions for the fixed initialized-control deployment barrier."
  permissions = [
    "datastore.databases.get",
    "datastore.entities.get",
  ]
  stage = "GA"
  lifecycle {
    prevent_destroy = true
  }
}

resource "google_project_iam_member" "maintenance_bootstrap_reader" {
  count   = local.maintenance_operator ? 1 : 0
  project = var.project_id
  role    = "projects/${var.project_id}/roles/mypageMaintenanceBootstrapReader"
  member  = "serviceAccount:${local.maintenance_bootstrap_email}"

  depends_on = [google_project_iam_custom_role.maintenance_bootstrap_reader]
}

resource "google_iam_workload_identity_pool" "maintenance_github" {
  count                     = local.maintenance_operator ? 1 : 0
  project                   = var.project_id
  workload_identity_pool_id = "mypage-maintenance"
  display_name              = "MyPage maintenance GitHub"
  description               = "Environment-scoped GitHub Actions trust for maintenance Job invocation."
  disabled                  = false
}

resource "google_iam_workload_identity_pool_provider" "maintenance_github" {
  count                              = local.maintenance_operator ? 1 : 0
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.maintenance_github[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "MyPage maintenance workflow"
  disabled                           = false
  attribute_mapping = {
    "google.subject"                = "assertion.sub"
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
  attribute_condition = "assertion.repository_id == '340900071' && assertion.repository_owner_id == '54093651' && assertion.actor_id == '54093651' && assertion.actor == 'kani3camp' && assertion.run_attempt == '1' && assertion.event_name == 'workflow_dispatch' && assertion.ref == '${local.maintenance_ref}' && assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@${local.maintenance_ref}' && assertion.environment == '${local.maintenance_github_environment}' && assertion.workflow_sha == assertion.sha"
  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "maintenance_workload_identity" {
  count              = local.maintenance_operator ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.maintenance_invoker_email}"
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${local.maintenance_project_number}/locations/global/workloadIdentityPools/mypage-maintenance/attribute.repository_id/340900071"

  depends_on = [google_service_account.maintenance_invoker]
}

resource "google_cloud_run_v2_job" "maintenance_operator" {
  count               = local.maintenance_operator ? 1 : 0
  project             = var.project_id
  name                = "mypage-maintenance-operator"
  location            = var.region
  deletion_protection = true

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = local.maintenance_operator_email
      max_retries     = 0
      timeout         = "60s"
      containers {
        image   = var.runtime_settings.maintenance_operator_image_digest
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
  }
}

resource "google_cloud_run_v2_job_iam_member" "maintenance_invoker" {
  count    = local.maintenance_operator ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.maintenance_operator[0].name
  role     = "roles/run.jobsExecutorWithOverrides"
  member   = "serviceAccount:${local.maintenance_invoker_email}"
}

# Break-glass uses a different fixed command and Job. No human principal is
# permanently bound here; the runbook grants short-lived, resource-level
# execution authority to this Job only and relies on Cloud Audit Logs.
resource "google_cloud_run_v2_job" "maintenance_breakglass" {
  count               = local.maintenance_operator ? 1 : 0
  project             = var.project_id
  name                = "mypage-maintenance-breakglass"
  location            = var.region
  deletion_protection = true

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = local.maintenance_operator_email
      max_retries     = 0
      timeout         = "60s"
      containers {
        image   = var.runtime_settings.maintenance_operator_image_digest
        command = ["/app/mypage-maintenance-breakglass"]
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
  }
}

resource "google_cloud_run_v2_job" "maintenance_bootstrap" {
  count               = local.maintenance_operator ? 1 : 0
  project             = var.project_id
  name                = "mypage-maintenance-bootstrap-check"
  location            = var.region
  deletion_protection = true

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = local.maintenance_bootstrap_email
      max_retries     = 0
      timeout         = "30s"
      containers {
        image   = var.runtime_settings.maintenance_operator_image_digest
        command = ["/app/mypage-maintenance-bootstrap-check"]
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

  depends_on = [google_project_iam_member.maintenance_bootstrap_reader]
  lifecycle {
    prevent_destroy = true
  }
}

# Exact Job-level invocation only; roles/run.invoker cannot override its
# command, arguments, environment, service account or task configuration.
resource "google_cloud_run_v2_job_iam_member" "maintenance_bootstrap_foundation_plan" {
  count    = local.maintenance_operator ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.maintenance_bootstrap[0].name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${local.foundation_plan_email}"
}

resource "google_cloud_run_v2_job_iam_member" "maintenance_bootstrap_foundation_apply" {
  count    = local.maintenance_operator ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.maintenance_bootstrap[0].name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${local.foundation_apply_email}"
}

resource "google_cloud_run_v2_job_iam_member" "maintenance_bootstrap_backend_deployer" {
  count    = local.maintenance_operator ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.maintenance_bootstrap[0].name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${local.backend_deployer_email}"
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

output "maintenance_operator_enabled" {
  value = local.maintenance_operator
}

output "maintenance_workload_identity_provider" {
  value = local.maintenance_operator ? google_iam_workload_identity_pool_provider.maintenance_github[0].name : null
}

output "maintenance_invoker_service_account" {
  value = local.maintenance_operator ? local.maintenance_invoker_email : null
}

output "maintenance_job_name" {
  value = local.maintenance_operator ? google_cloud_run_v2_job.maintenance_operator[0].name : null
}

# Enforce the environment-specific capacity bounds from Security Current Canon.
output "max_instances" {
  value = local.max_instances
}

# Expose only a boolean-like count of the actual planned Cloud Run env entry.
output "history_coverage_env_count" {
  value = local.deploy_api ? length([
    for item in google_cloud_run_v2_service.api[0].template[0].containers[0].env : item
    if item.name == "MYPAGE_HISTORY_COVERAGE"
  ]) : 0
}

# Inspect only this fixed, non-identifying env value in credentialless mock tests.
output "read_metrics_env_values" {
  value = local.deploy_api ? [
    for item in google_cloud_run_v2_service.api[0].template[0].containers[0].env : item.value
    if item.name == "MYPAGE_READ_METRICS"
  ] : []
}
