terraform {
  required_providers {
    google = { source = "hashicorp/google", version = "= 8.5.0" }
  }
}
variable "project_id" { type = string }
variable "environment" {
  type = string
  validation {
    condition     = contains(["development", "production"], var.environment)
    error_message = "Use the explicit target environment."
  }
}
variable "manage_primary_email" {
  type    = bool
  default = false
}
variable "email_address" {
  type      = string
  sensitive = true
  default   = null
}
resource "google_monitoring_notification_channel" "primary_email" {
  count        = var.manage_primary_email ? 1 : 0
  project      = var.project_id
  display_name = "[${var.environment}] Study Space primary email"
  type         = "email"
  labels       = { email_address = var.email_address }
  enabled      = true
  force_delete = false
  lifecycle {
    prevent_destroy = true
    precondition {
      condition     = var.email_address == null ? false : can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.email_address))
      error_message = "Enabled primary Email ownership requires a private valid address."
    }
  }
}
output "primary_email_name" {
  value     = try(google_monitoring_notification_channel.primary_email[0].name, null)
  sensitive = true
}
