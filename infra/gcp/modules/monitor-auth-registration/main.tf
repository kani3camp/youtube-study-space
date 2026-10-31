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



# 撤去中だけ、専用ソースバケット上のオブジェクト削除を通常Apply主体へ許可する。
# 関数・オブジェクトの撤去後、空のバケットとこのgrantを一緒に削除する。
resource "google_storage_bucket_iam_member" "source_retirement" {
  bucket = google_storage_bucket.source.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:terraform-dev-normal-apply@${var.project_id}.iam.gserviceaccount.com"
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
