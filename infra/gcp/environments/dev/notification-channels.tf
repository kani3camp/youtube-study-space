variable "manage_primary_email" {
  type    = bool
  default = false
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
    condition     = var.primary_email_channel_name == null || var.primary_email_channel_name == "" ? true : startswith(var.primary_email_channel_name, "projects/test-youtube-study-space/notificationChannels/")
    error_message = "Use a private channel name from this environment's project."
  }
}
module "notification_channels" {
  source               = "../../modules/monitoring-notification-channels"
  project_id           = var.project_id
  environment          = "development"
  manage_primary_email = var.manage_primary_email
  email_address        = var.primary_email_address
}

import {
  for_each = var.manage_primary_email ? { primary = true } : {}
  to       = module.notification_channels.google_monitoring_notification_channel.primary_email[0]
  id       = var.primary_email_channel_name
}
