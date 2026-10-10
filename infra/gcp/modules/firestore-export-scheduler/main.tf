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

# NOTE: Preserve the observed development Pub/Sub job. No HTTP/App Engine target,
# IAM, API, generated delivery subscription or manual execution is owned here.
resource "google_cloud_scheduler_job" "export" {
  project   = var.project_id
  region    = var.region
  name      = "scheduledFirestoreCollectionsExport"
  schedule  = "0 0 * * *"
  time_zone = "Asia/Tokyo"
  paused    = false

  # Both fields are absent in the live Pub/Sub job. Do not introduce a deadline
  # or description just to fill a provider field. This resource has no labels.
  description      = null
  attempt_deadline = null

  retry_config {
    retry_count          = 3
    max_retry_duration   = "0s"
    min_backoff_duration = "5s"
    max_backoff_duration = "3600s"
    max_doublings        = 5
  }

  pubsub_target {
    topic_name = var.topic_id
    data       = "c3RhcnQgZXhwb3J0"
  }

  lifecycle {
    prevent_destroy = true
  }
}
