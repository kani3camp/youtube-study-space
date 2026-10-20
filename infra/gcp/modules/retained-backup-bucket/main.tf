terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }
}

variable "project_id" { type = string }
variable "bucket_name" { type = string }
variable "location" { type = string }
variable "storage_class" { type = string }
variable "retention_policy" {
  type    = object({ seconds = number, locked = bool })
  default = null
}
variable "soft_delete_seconds" { type = number }

resource "google_storage_bucket" "retained" {
  project                     = var.project_id
  name                        = var.bucket_name
  location                    = var.location
  storage_class               = var.storage_class
  force_destroy               = false
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  default_event_based_hold    = false
  requester_pays              = false

  soft_delete_policy {
    retention_duration_seconds = var.soft_delete_seconds
  }
  dynamic "retention_policy" {
    for_each = var.retention_policy == null ? [] : [var.retention_policy]
    content {
      retention_period = retention_policy.value.seconds
      is_locked        = retention_policy.value.locked
    }
  }
  lifecycle {
    prevent_destroy = true
  }
}
