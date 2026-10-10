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
variable "execution_service_account_email" {
  type     = string
  nullable = false
}

# NOTE: Adopt the existing Gen1 deployment without rebuilding its source. The
# reserved deployment-tool label and Google-managed artifacts remain external.
resource "google_cloudfunctions_function" "export" {
  project               = var.project_id
  region                = var.region
  name                  = "firestoreCollectionsExport"
  runtime               = "nodejs22"
  entry_point           = "scheduledFirestoreExport"
  available_memory_mb   = 256
  timeout               = 60
  min_instances         = 0
  max_instances         = 1
  service_account_email = var.execution_service_account_email
  ingress_settings      = "ALLOW_ALL"
  description           = null

  environment_variables = {
    YSS_EXPORT_ENVIRONMENT = "development"
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
