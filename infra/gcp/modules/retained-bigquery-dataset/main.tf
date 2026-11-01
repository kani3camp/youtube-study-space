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

variable "dataset_id" {
  type = string
}

variable "location" {
  type = string
}

resource "google_bigquery_dataset" "retained" {
  project                    = var.project_id
  dataset_id                 = var.dataset_id
  location                   = var.location
  max_time_travel_hours      = "168"
  delete_contents_on_destroy = false

  # NOTE: Access remains externally owned. Omit inline access and IAM/access
  # resources so importing metadata cannot acquire or replace existing grants.
  # The provider's optional/computed access is retained in state on import.
  lifecycle {
    prevent_destroy = true
    ignore_changes  = [access]
  }
}

output "dataset_id" {
  value = google_bigquery_dataset.retained.dataset_id
}
