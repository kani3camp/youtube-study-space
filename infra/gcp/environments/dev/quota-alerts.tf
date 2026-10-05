module "youtube_quota_alerts" {
  source = "../../modules/youtube-quota-alerts"

  project_id            = var.project_id
  environment           = "development"
  manage_policies       = var.manage_youtube_quota_alerts
  notification_channels = var.manage_primary_email ? [module.notification_channels.primary_email_name] : var.youtube_quota_notification_channels
  policy_settings       = var.youtube_quota_policy_settings
  legacy_display_names  = var.youtube_quota_legacy_display_names
}

variable "manage_youtube_quota_alerts" {
  description = "Remains false during import-only migration. Requires verified channels and approved normal-change apply."
  type        = bool
  default     = false
}

variable "youtube_quota_notification_channels" {
  sensitive = true
  type      = list(string)
  default   = []
}

variable "youtube_quota_legacy_display_names" {
  type    = map(string)
  default = {}
}

variable "youtube_quota_policy_settings" {
  type = map(object({
    threshold = number
    duration  = string
    enabled   = optional(bool, true)
    severity  = optional(string)
  }))
  default = {
    minute_80 = { threshold = 0.8, duration = "60s" }
    day_80    = { threshold = 0.8, duration = "60s" }
    day_60    = { threshold = 0.6, duration = "60s" }
  }
}
