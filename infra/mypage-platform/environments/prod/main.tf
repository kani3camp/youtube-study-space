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
  project = "youtube-study-space"
  region  = "asia-northeast2"
}

variable "stage" {
  description = "disabled -> foundation -> runtime_private -> runtime_public; default makes no cloud resources."
  type        = string
  default     = "disabled"
  validation {
    condition     = contains(["disabled", "foundation_metadata", "foundation", "runtime_private", "runtime_public"], var.stage)
    error_message = "Invalid MyPage stage."
  }
}

variable "runtime_settings" {
  description = "Reviewed public identifiers and pinned secret VERSION NUMBERS; never raw secret values."
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
    history_coverage_json       = optional(string, "")
    image_source_sha            = optional(string, "")
    hosting_backend_host        = optional(string, "")
    previous_image_digest       = optional(string, "")
    read_metrics_enabled        = optional(bool, false)
    read_metrics_from_revision  = optional(string, "")
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

  # Reject even in disabled/foundation stages; do not silently accept a DEV flag.
  validation {
    condition     = !var.runtime_settings.read_metrics_enabled && var.runtime_settings.read_metrics_from_revision == ""
    error_message = "Snapshot read metrics and their serving transitions are development-only."
  }

  validation {
    condition = var.runtime_settings.history_coverage_json == "" || (
      length(var.runtime_settings.history_coverage_json) <= 16384 &&
      can(tolist(jsondecode(var.runtime_settings.history_coverage_json)))
    )
    error_message = "History coverage must be absent or an at-most-16KiB JSON array of reviewed intervals."
  }

  validation {
    condition = !contains(["runtime_private", "runtime_public"], var.stage) || (
      can(regex("^[-a-z0-9.]+-docker\\.pkg\\.dev/[a-z0-9-]+/[a-z0-9_-]+/[A-Za-z0-9._/-]+@sha256:[a-f0-9]{64}$", var.runtime_settings.image_digest)) &&
      can(regex("^[0-9]{6,20}$", var.runtime_settings.project_number)) &&
      can(regex("^1:[0-9]+:web:[A-Za-z0-9]+$", var.runtime_settings.web_app_id)) &&
      can(regex("^https://[A-Za-z0-9.-]+$", var.runtime_settings.public_origin)) &&
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
  project_id       = "youtube-study-space"
  region           = "asia-northeast2"
  environment      = "production"
  stage            = var.stage
  runtime_settings = var.runtime_settings
}

output "environment" {
  value = "production"
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
