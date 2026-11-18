terraform {
  required_version = "= 1.16.4"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }

  # MyPage専用の独立Stateを使い、取り込み済みinfra/gcpルートのStateは使わない。
  backend "s3" {}
}

provider "google" {
  project = "youtube-study-space"
  region  = "asia-northeast2"
}

variable "runtime_settings" {
  description = "Reviewed public identifiers and pinned secret VERSION NUMBERS; never raw secret values."
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
    read_metrics_from_revision  = optional(string, "")
  })
  # null: Secret値と環境別imageがまだ無い初回構築。実行系の資源を宣言しない。
  default = null

  validation {
    condition     = var.runtime_settings == null || try(!var.runtime_settings.read_metrics_enabled && var.runtime_settings.read_metrics_from_revision == "", false)
    error_message = "Snapshot read metrics and their serving transitions are development-only."
  }

  validation {
    condition = var.runtime_settings == null || try(
      can(regex("^[-a-z0-9.]+-docker\\.pkg\\.dev/[a-z0-9-]+/[a-z0-9_-]+/[A-Za-z0-9._/-]+@sha256:[a-f0-9]{64}$", var.runtime_settings.image_digest)) &&
      var.runtime_settings.project_number == "906336399194" &&
      can(regex("^1:906336399194:web:[A-Za-z0-9]+$", var.runtime_settings.web_app_id)) &&
      var.runtime_settings.public_origin == "https://moku.work" &&
      alltrue([
        for version in [
          var.runtime_settings.oauth_client_id_version,
          var.runtime_settings.oauth_client_secret_version,
          var.runtime_settings.youtube_api_key_version,
        ] : can(regex("^[1-9][0-9]*$", version))
      ]),
      false
    )
    error_message = "Runtime activation requires a pinned Artifact Registry image digest, reviewed Firebase origin/IDs and numeric secret versions."
  }
}

variable "public_invoker" {
  description = "True only after the owner's allUsers invoker binding exists; CI then imports it. Terraform never creates it."
  type        = bool
  default     = false
  validation {
    condition     = !var.public_invoker || var.runtime_settings != null
    error_message = "The public invoker requires the runtime Service."
  }
}

module "platform" {
  source           = "../../modules/runtime"
  project_id       = "youtube-study-space"
  region           = "asia-northeast2"
  environment      = "production"
  runtime_settings = var.runtime_settings
  public_invoker   = var.public_invoker
}

# 段階変数の撤去でmoduleのcountを外した。既存Stateのアドレスをそのまま移す（no-op）。
moved {
  from = module.platform[0]
  to   = module.platform
}

output "environment" {
  value = "production"
}

output "runtime_enabled" {
  value = module.platform.runtime_enabled
}

output "public_invoker_enabled" {
  value = module.platform.public_invoker_enabled
}

output "service_url" {
  value = module.platform.service_url
}

output "maintenance_workload_identity_provider" {
  value = module.platform.maintenance_workload_identity_provider
}

output "maintenance_invoker_service_account" {
  value = module.platform.maintenance_invoker_service_account
}
