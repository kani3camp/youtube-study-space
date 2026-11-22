# Production never creates a channel. Selecting the primary Email adopts the
# existing operator-created channel by import, without changing its values.
variable "manage_primary_email" {
  type    = bool
  default = false
  validation {
    condition     = !var.manage_primary_email || can(regex("^projects/youtube-study-space/notificationChannels/[0-9]+$", var.primary_email_channel_name))
    error_message = "Production adopts only the existing channel; provide its private channel name for import."
  }
}
variable "primary_email_address" {
  type      = string
  sensitive = true
  default   = null
}
variable "primary_email_channel_name" {
  type      = string
  sensitive = true
  default   = null
  validation {
    condition     = var.primary_email_channel_name == null || var.primary_email_channel_name == "" ? true : startswith(var.primary_email_channel_name, "projects/youtube-study-space/notificationChannels/")
    error_message = "Use a private channel name from this environment's project."
  }
}
module "notification_channels" {
  source               = "../../modules/monitoring-notification-channels"
  project_id           = var.project_id
  environment          = "production"
  manage_primary_email = var.manage_primary_email
  email_address        = var.primary_email_address
  # Observed labels of the existing channel. Import must stay value-preserving;
  # relabel only as a separate normal change after ownership is established.
  user_labels = { environment = "production", managed_by = "manual" }
}

import {
  for_each = var.manage_primary_email ? { primary = true } : {}
  to       = module.notification_channels.google_monitoring_notification_channel.primary_email[0]
  id       = var.primary_email_channel_name
}
