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
  description = "Existing environment field order; do not reorder live tables during import."
  type        = list(string)
  validation {
    condition = length(var.field_order) == 8 && toset(var.field_order) == toset([
      "seat_id", "ordered_at", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"
    ])
    error_message = "Preserve exactly the eight retained order-history fields."
  }
}

locals {
  fields = { for field in jsondecode(file("${path.module}/schema.json")) : field.name => field }
}

resource "google_bigquery_table" "retained" {
  project             = var.project_id
  dataset_id          = var.dataset_id
  table_id            = "order-history"
  schema              = jsonencode([for name in var.field_order : local.fields[name]])
  deletion_protection = true

  lifecycle {
    prevent_destroy = true
  }
}
