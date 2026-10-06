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

variable "field_order" {
  description = "Existing canonical field order observed from live table metadata after legacy schema repair."
  type        = list(string)

  validation {
    condition = length(var.field_order) == 8 && toset(var.field_order) == toset([
      "seat_id",
      "taken_at",
      "user_id",
      "activity_type",
      "is_member_seat",
      "__key__",
      "__error__",
      "__has_error__",
    ])
    error_message = "Preserve exactly the eight canonical retained user-activity-history fields; legacy timestamp is not allowed."
  }
}

locals {
  fields = { for field in jsondecode(file("${path.module}/schema.json")) : field.name => field }
}

resource "google_bigquery_table" "retained" {
  project             = var.project_id
  dataset_id          = var.dataset_id
  table_id            = "user-activity-history"
  schema              = jsonencode([for name in var.field_order : local.fields[name]])
  deletion_protection = true

  lifecycle {
    prevent_destroy = true
  }
}
