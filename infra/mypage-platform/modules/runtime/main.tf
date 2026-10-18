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
}

variable "stage" {
  type = string
  validation {
    condition     = contains(["foundation", "runtime_private", "runtime_public"], var.stage)
    error_message = "Module stages are foundation, runtime_private or runtime_public."
  }
}

variable "runtime_settings" {
  type = object({
    image_digest                = string
    project_number              = string
    web_app_id                  = string
    public_origin               = string
    privacy_version             = string
    terms_version               = string
    oauth_client_id_version     = string
    oauth_client_secret_version = string
    youtube_api_key_version     = string
  })
}

locals {
  deploy_api = contains(["runtime_private", "runtime_public"], var.stage)
  public_api = var.stage == "runtime_public"
  runtime_principal = "serviceAccount:${google_service_account.runtime.email}"
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

resource "google_secret_manager_secret" "oauth_client_id" {
  project           = var.project_id
  secret_id         = "mypage-oauth-client-id"
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

# Firebase Custom Token signing uses IAM signBlob; bind only the dedicated
# runtime service account resource, never all service accounts in the project.
resource "google_service_account_iam_member" "runtime_signer" {
  service_account_id = google_service_account.runtime.name
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
  count               = local.deploy_api ? 1 : 0
  project             = var.project_id
  name                = "mypage-api"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = true

  template {
    service_account                 = google_service_account.runtime.email
    timeout                         = "30s"
    max_instance_request_concurrency = 40

    scaling {
      min_instance_count = 0
      max_instance_count = 2
    }

    containers {
      image = var.runtime_settings.image_digest

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle = true
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
        value = google_service_account.runtime.email
      }
      env {
        name  = "MYPAGE_PRIVACY_VERSION"
        value = var.runtime_settings.privacy_version
      }
      env {
        name  = "MYPAGE_TERMS_VERSION"
        value = var.runtime_settings.terms_version
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
