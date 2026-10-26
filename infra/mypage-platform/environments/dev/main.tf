terraform {
  required_version = "= 1.16.4"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }

  # Independent MyPage state, never the imported infra/gcp root state.
  backend "s3" {}
}

provider "google" {
  project = "test-youtube-study-space"
  region  = "asia-southeast2"
}

variable "stage" {
  description = "disabled -> foundation -> registry -> maintenance_operator or maintenance_upgrade -> runtime_private -> runtime_public; default makes no cloud resources."
  type        = string
  default     = "disabled"
  validation {
    condition     = contains(["disabled", "foundation_metadata", "foundation", "registry", "maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"], var.stage)
    error_message = "Invalid MyPage stage."
  }
}

variable "runtime_settings" {
  description = "Reviewed public identifiers and pinned secret VERSION NUMBERS; never raw secret values."
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
    read_metrics_from_revision            = optional(string, "")
  })
  default = {
    image_digest                = ""
    project_number              = ""
    web_app_id                  = ""
    public_origin               = ""
    privacy_version             = ""
    terms_version               = ""
    oauth_client_id_version     = ""
    oauth_client_secret_version = ""
    youtube_api_key_version     = ""
  }

  # Explicit one-shot admission for a metrics-only serving update. It is not
  # forwarded as an environment variable and does not permit an image rollout.
  validation {
    condition = !contains(["maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"], var.stage) || (
      var.runtime_settings.maintenance_operator_enabled &&
      can(regex("^asia-southeast2-docker\\.pkg\\.dev/test-youtube-study-space/mypage/mypage-api@sha256:[a-f0-9]{64}$", var.runtime_settings.maintenance_operator_image_digest)) &&
      can(regex("^[a-f0-9]{40}$", var.runtime_settings.maintenance_operator_image_source_sha))
    )
    error_message = "Maintenance operator and runtime stages require a separately pinned operator digest and reviewed source SHA."
  }

  validation {
    condition = var.runtime_settings.read_metrics_from_revision == "" || (
      contains(["runtime_private", "runtime_public"], var.stage) &&
      can(regex("^mypage-api-[a-z0-9-]{1,50}$", var.runtime_settings.read_metrics_from_revision))
    )
    error_message = "A metrics transition requires a runtime stage and an exact prior ready revision."
  }

  validation {
    condition = var.runtime_settings.history_coverage_json == "" || (
      length(var.runtime_settings.history_coverage_json) <= 16384 &&
      can(tolist(jsondecode(var.runtime_settings.history_coverage_json)))
    )
    error_message = "History coverage must be absent or an at-most-16KiB JSON array of reviewed intervals."
  }

  validation {
    condition = !contains(["maintenance_upgrade", "runtime_private", "runtime_public"], var.stage) || (
      can(regex("^[-a-z0-9.]+-docker\\.pkg\\.dev/[a-z0-9-]+/[a-z0-9_-]+/[A-Za-z0-9._/-]+@sha256:[a-f0-9]{64}$", var.runtime_settings.image_digest)) &&
      var.runtime_settings.project_number == "48101442817" &&
      can(regex("^1:48101442817:web:[A-Za-z0-9]+$", var.runtime_settings.web_app_id)) &&
      var.runtime_settings.public_origin == "https://test--study-space.web.app" &&
      can(regex("^[A-Za-z0-9._-]{1,64}$", var.runtime_settings.privacy_version)) &&
      can(regex("^[A-Za-z0-9._-]{1,64}$", var.runtime_settings.terms_version)) &&
      alltrue([
        for version in [
          var.runtime_settings.oauth_client_id_version,
          var.runtime_settings.oauth_client_secret_version,
          var.runtime_settings.youtube_api_key_version,
        ] : can(regex("^[1-9][0-9]*$", version))
      ])
    )
    error_message = "Runtime activation requires a pinned Artifact Registry image digest, reviewed Firebase origin/IDs and numeric secret versions."
  }
}

module "platform" {
  count            = var.stage == "disabled" ? 0 : 1
  source           = "../../modules/runtime"
  project_id       = "test-youtube-study-space"
  region           = "asia-southeast2"
  environment      = "development"
  stage            = var.stage
  runtime_settings = var.runtime_settings
}

output "environment" {
  value = "development"
}

output "stage" {
  value = var.stage
}

output "runtime_enabled" {
  value = var.stage == "disabled" ? false : module.platform[0].runtime_enabled
}

output "public_invoker_enabled" {
  value = var.stage == "disabled" ? false : module.platform[0].public_invoker_enabled
}

output "service_url" {
  value = var.stage == "disabled" ? null : module.platform[0].service_url
}

output "maintenance_workload_identity_provider" {
  value = var.stage == "disabled" ? null : module.platform[0].maintenance_workload_identity_provider
}

output "maintenance_invoker_service_account" {
  value = var.stage == "disabled" ? null : module.platform[0].maintenance_invoker_service_account
}

output "maintenance_inspect_service_account" {
  value = var.stage == "disabled" ? null : module.platform[0].maintenance_inspect_service_account
}

output "maintenance_inspect_service_url" {
  value = var.stage == "disabled" ? null : module.platform[0].maintenance_inspect_service_url
}
