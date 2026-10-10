variable "project_id" {
  type = string
}

variable "environment" {
  type = string
  validation {
    condition     = contains(["development", "production"], var.environment)
    error_message = "Use an explicit development or production environment."
  }
}

variable "manage_policies" {
  description = "Opt in only after notification-channel verification and approval of a normal-change apply."
  type        = bool
  default     = false
}

variable "notification_channels" {
  sensitive   = true
  description = "Existing, verified target-project channels. This module never creates channels."
  type        = list(string)
  default     = []
}

variable "legacy_display_names" {
  description = "Optional exact legacy names for a future no-op production import; rename in a separate approved change."
  type        = map(string)
  default     = {}
}

variable "auto_close" {
  type    = string
  default = "604800s"
}

variable "policy_settings" {
  description = "Environment-specific usage ratios, durations, enabled states, and optional severity."
  type = map(object({
    threshold = number
    duration  = string
    enabled   = optional(bool, true)
    severity  = optional(string)
  }))
  validation {
    condition = toset(keys(var.policy_settings)) == toset(["minute_80", "day_80", "day_60"]) && alltrue([
      for setting in values(var.policy_settings) : setting.threshold > 0 && setting.threshold <= 1
    ])
    error_message = "Configure exactly minute_80, day_80, day_60 with ratios in (0, 1]."
  }
}
