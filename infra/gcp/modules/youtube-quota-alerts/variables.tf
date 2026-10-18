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

variable "import_existing_policies" {
  description = "Own only existing policies using exact private metadata; never combine with policy creation."
  type        = bool
  default     = false
  validation {
    condition     = !var.import_existing_policies || !var.manage_policies
    error_message = "Existing-policy import and policy creation are separate operations."
  }
  validation {
    condition = !var.import_existing_policies || alltrue([
      for key in var.policy_keys : contains(keys(var.legacy_display_names), key) && contains(keys(var.legacy_notification_channels), key) && contains(keys(var.legacy_documentation), key)
    ])
    error_message = "Every selected existing policy requires its exact observed name, references and documentation."
  }
}

variable "policy_keys" {
  description = "Cumulative existing policy aliases to import; creation behavior is unchanged."
  type        = set(string)
  default     = ["minute_80", "day_80", "day_60"]
  validation {
    condition     = length(setsubtract(var.policy_keys, toset(["minute_80", "day_80", "day_60"]))) == 0
    error_message = "Select only the three supported quota policy aliases."
  }
}

variable "legacy_notification_channels" {
  description = "Exact existing per-policy references for import, including dangling references; this does not verify or repair a channel."
  type        = map(list(string))
  sensitive   = true
  default     = {}
}

variable "legacy_documentation" {
  description = "Exact existing Markdown content, required for selected imports and ignored by the established creation path."
  type        = map(string)
  sensitive   = true
  default     = {}
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
