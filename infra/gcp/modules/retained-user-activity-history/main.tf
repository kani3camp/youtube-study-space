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

variable "field_descriptions" {
  description = "Private existing column descriptions keyed by canonical field path (including __key__.child)."
  type        = map(string)
  sensitive   = true
  nullable    = false
  default     = {}

  validation {
    condition = alltrue([
      for path, description in var.field_descriptions :
      contains(local.description_paths, path) && description != null && description != ""
    ])
    error_message = "Descriptions must be nonempty strings for existing canonical field paths only."
  }
}

locals {
  fields = { for field in jsondecode(file("${path.module}/schema.json")) : field.name => field }
  description_paths = flatten([
    for field in local.fields : concat([field.name], [
      for child in try(field.fields, []) : "${field.name}.${child.name}"
    ])
  ])
  described_fields = {
    for name, field in local.fields : name => merge(field,
      try({ description = var.field_descriptions[name] }, {}),
      contains(keys(field), "fields") ? {
        fields = [for child in field.fields : merge(child,
          try({ description = var.field_descriptions["${name}.${child.name}"] }, {})
        )]
      } : {}
    )
  }
}

resource "google_bigquery_table" "retained" {
  project    = var.project_id
  dataset_id = var.dataset_id
  table_id   = "user-activity-history"
  # Keep the imported provider attribute's sensitivity unchanged: marking only
  # schema sensitive creates an import+update even when every value is equal.
  # Raw Terraform plan/state/logs must stay private; public output is value-free.
  schema              = nonsensitive(jsonencode([for name in var.field_order : local.described_fields[name]]))
  deletion_protection = true

  lifecycle {
    prevent_destroy = true
  }
}
