terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }
}
variable "project_id" { type = string }
variable "service_keys" {
  description = "Explicitly reviewed services only; do not feed the entire enabled inventory here."
  type        = set(string)
  default     = []
}
variable "classification" {
  sensitive = true
  type = map(object({
    classification       = string
    state                = string
    dependency_addresses = set(string)
    reason               = string
  }))
  default = {}
  validation {
    condition = alltrue([for service in var.service_keys : try(
      can(regex("^[a-z][a-z0-9-]*\\.googleapis\\.com$", service)) &&
      var.classification[service].classification == "Own" &&
      var.classification[service].state == "ENABLED" &&
      length(var.classification[service].dependency_addresses) > 0 &&
      length(trimspace(var.classification[service].reason)) > 0, false
    )])
    error_message = "Only existing enabled Own APIs with explicit reviewed resource dependency evidence may be adopted."
  }
}

resource "google_project_service" "owned" {
  for_each                   = var.service_keys
  project                    = var.project_id
  service                    = each.key
  disable_on_destroy         = false
  disable_dependent_services = false
  # Leave the pinned provider's local deletion_policy default unchanged on import.
  lifecycle { prevent_destroy = true }
}
