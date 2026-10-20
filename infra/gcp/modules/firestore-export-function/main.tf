terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }
}

variable "project_id" { type = string }
variable "region" { type = string }
variable "topic_id" { type = string }
variable "name" {
  description = "Existing Gen1 function's short name, observed from target-environment metadata."
  type        = string
  default     = "firestoreCollectionsExport"
  validation {
    condition     = can(regex("^[A-Za-z][A-Za-z0-9_-]*$", var.name))
    error_message = "Use the existing short function name, without a project or location path."
  }
}
variable "available_memory_mb" {
  description = "Existing function memory in MB; preserve the observed positive integer."
  type        = number
  default     = 256
  validation {
    condition     = var.available_memory_mb > 0 && floor(var.available_memory_mb) == var.available_memory_mb
    error_message = "Function memory must be a positive whole number of MB supported by Google."
  }
}
variable "timeout" {
  description = "Existing Gen1 execution timeout in seconds, at most 540 seconds."
  type        = number
  default     = 60
  validation {
    condition     = var.timeout > 0 && floor(var.timeout) == var.timeout && var.timeout <= 540
    error_message = "Use a positive whole-second Gen1 timeout of at most 540 seconds."
  }
}
variable "environment" {
  description = "Existing export runtime environment; preserve the value observed in function metadata."
  type        = string
  default     = "development"
  validation {
    condition     = contains(["development", "production"], var.environment)
    error_message = "Use the existing development or production export environment."
  }
}
variable "execution_service_account_email" {
  type     = string
  nullable = false
}

# NOTE: Adopt the existing Gen1 deployment without rebuilding its source. The
# reserved deployment-tool label and Google-managed artifacts remain external.
resource "google_cloudfunctions_function" "export" {
  project               = var.project_id
  region                = var.region
  name                  = var.name
  runtime               = "nodejs22"
  entry_point           = "scheduledFirestoreExport"
  available_memory_mb   = var.available_memory_mb
  timeout               = var.timeout
  min_instances         = 0
  max_instances         = 1
  service_account_email = var.execution_service_account_email
  ingress_settings      = "ALLOW_ALL"
  description           = null

  environment_variables = {
    YSS_EXPORT_ENVIRONMENT = var.environment
    YSS_EXPORT_PROJECT_ID  = var.project_id
  }

  event_trigger {
    event_type = "google.pubsub.topic.publish"
    resource   = var.topic_id
    failure_policy {
      retry = false
    }
  }

  # NOTE: Do not configure labels/default labels, source_archive_bucket/object,
  # source_repository, generated buckets, build artifacts or trigger subscription.
  # SourceUploadUrl is not a Terraform field and must never enter configuration.
  lifecycle {
    prevent_destroy = true
  }
}
