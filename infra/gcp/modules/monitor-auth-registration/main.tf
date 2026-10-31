variable "project_id" { type = string }
variable "region" { type = string }
variable "runtime_service_account_email" {
  type = string
  validation {
    condition     = endswith(var.runtime_service_account_email, "@${var.project_id}.iam.gserviceaccount.com")
    error_message = "実行用SAは対象プロジェクトの専用SAを指定してください。"
  }
}
variable "build_service_account_email" {
  type = string
  validation {
    condition     = endswith(var.build_service_account_email, "@${var.project_id}.iam.gserviceaccount.com") && var.build_service_account_email != var.runtime_service_account_email
    error_message = "ビルド用SAは対象プロジェクト内で実行用SAと分離してください。"
  }
}
variable "preserved_before_sign_in_uri" {
  type    = string
  default = ""
  validation {
    condition     = var.preserved_before_sign_in_uri == "" || startswith(var.preserved_before_sign_in_uri, "https://")
    error_message = "既存beforeSignInのHTTPS URLを維持してください。"
  }
}

locals {
  source_path = "${path.module}/function/source.zip"
  source_sha  = fileexists(local.source_path) ? filesha256(local.source_path) : ""
  triggers = merge(
    {},
    var.preserved_before_sign_in_uri == "" ? {} : { beforeSignIn = var.preserved_before_sign_in_uri }
  )
}

resource "google_storage_bucket" "source" {
  project                     = var.project_id
  name                        = "${var.project_id}-monitor-auth-source"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false
  lifecycle {
    prevent_destroy = true
    precondition {
      condition     = fileexists(local.source_path)
      error_message = "配備前にpackage_monitor_auth.pyで認証用ZIPを生成してください。"
    }
  }
}

resource "google_artifact_registry_repository" "build" {
  project       = var.project_id
  location      = var.region
  repository_id = "monitor-auth"
  format        = "DOCKER"
  description   = "認証用登録制限関数のビルド専用"
  lifecycle { prevent_destroy = true }
}

# Artifact Registryはresource.name条件に対応しないため、専用リポジトリへ直接付与する。
resource "google_artifact_registry_repository_iam_member" "builder" {
  project    = var.project_id
  location   = var.region
  repository = google_artifact_registry_repository.build.repository_id
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${var.build_service_account_email}"
}

# 撤去中だけ、専用ソースバケット上のオブジェクト削除を通常Apply主体へ許可する。
# 関数・オブジェクトの撤去後、空のバケットとこのgrantを一緒に削除する。
resource "google_storage_bucket_iam_member" "source_retirement" {
  bucket = google_storage_bucket.source.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:terraform-dev-normal-apply@${var.project_id}.iam.gserviceaccount.com"
}

resource "google_storage_bucket_object" "source" {
  bucket       = google_storage_bucket.source.name
  name         = "registration-guard/${local.source_sha}.zip"
  source       = local.source_path
  content_type = "application/zip"
  # トリガー解除後の撤去準備。専用ソースの削除権限を先に設定する。
  depends_on      = [google_storage_bucket_iam_member.source_retirement]
  deletion_policy = "DELETE"
}

resource "google_cloudfunctions2_function" "guard" {
  project         = var.project_id
  location        = var.region
  name            = "monitor-password-signup-guard"
  description     = "一般利用者によるメール・パスワードの新規登録のみを拒否するbeforeCreate関数"
  deletion_policy = "DELETE"
  depends_on      = [google_artifact_registry_repository_iam_member.builder]

  build_config {
    runtime           = "nodejs24"
    entry_point       = "blockPasswordSignup"
    service_account   = "projects/${var.project_id}/serviceAccounts/${var.build_service_account_email}"
    docker_repository = google_artifact_registry_repository.build.id
    source {
      storage_source {
        bucket     = google_storage_bucket.source.name
        object     = google_storage_bucket_object.source.name
        generation = google_storage_bucket_object.source.generation
      }
    }
  }
  service_config {
    service_account_email = var.runtime_service_account_email
    min_instance_count    = 0
    max_instance_count    = 2
    available_memory      = "256Mi"
    timeout_seconds       = 7
    ingress_settings      = "ALLOW_ALL"
    # Googleが付与する実行IDログの既定値を維持し、毎回の削除・再付与差分を避ける。
    environment_variables = {
      GCLOUD_PROJECT   = var.project_id
      LOG_EXECUTION_ID = "true"
    }
  }
  lifecycle { prevent_destroy = true }
}

# Identity PlatformのHTTP呼び出しを受け付け、イベント署名はFirebase公式SDKで検証する。
resource "google_cloud_run_v2_service_iam_member" "invoker" {
  project  = var.project_id
  location = var.region
  name     = basename(google_cloudfunctions2_function.guard.service_config[0].service)
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_identity_platform_config" "auth" {
  project = var.project_id
  sign_in {
    email {
      enabled           = true
      password_required = true
    }
  }
  dynamic "blocking_functions" {
    for_each = length(local.triggers) == 0 ? [] : [local.triggers]
    content {
      dynamic "triggers" {
        for_each = blocking_functions.value
        content {
          event_type   = triggers.key
          function_uri = triggers.value
        }
      }
    }
  }
  depends_on = [google_cloud_run_v2_service_iam_member.invoker]

  lifecycle {
    prevent_destroy = true
    # 管理するのはメール/パスワードと登録トリガー。既存のGoogle IdP、許可ドメイン、
    # MFA、App Checkなどをこの変更で作り直したり、既定値へ戻したりしない。
    ignore_changes = [
      authorized_domains, autodelete_anonymous_users, client, mfa, monitoring,
      multi_tenant, quota, sms_region_config,
      sign_in[0].allow_duplicate_emails, sign_in[0].anonymous, sign_in[0].phone_number,
      blocking_functions[0].forward_inbound_credentials,
    ]
  }
}
