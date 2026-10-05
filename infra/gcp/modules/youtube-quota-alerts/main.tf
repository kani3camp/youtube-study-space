terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }
}

locals {
  alert_types = {
    minute_80 = { period = "minute", label = "Queries per minute - high", prompts = [] }
    day_80    = { period = "day", label = "Queries per day - high", prompts = [] }
    day_60    = { period = "day", label = "Queries per day - warning", prompts = ["OPENED", "CLOSED"] }
  }
}

resource "google_monitoring_alert_policy" "quota" {
  # Opt-in is separate from the migration CI gate. Enabling this module still
  # produces create actions that the global import-only policy must reject.
  for_each = var.manage_policies ? local.alert_types : {}

  project = var.project_id
  display_name = lookup(var.legacy_display_names, each.key,
  "[${var.environment}] YouTube quota ${each.value.label}")
  combiner              = "OR"
  enabled               = var.policy_settings[each.key].enabled
  severity              = var.policy_settings[each.key].severity
  notification_channels = var.notification_channels
  # Provider read-back returns an empty map; persist the same representation.
  user_labels = {}

  conditions {
    display_name = "Quota usage reached defined threshold"
    condition_monitoring_query_language {
      query = trimspace(templatefile("${path.module}/queries/${each.value.period}.mql.tftpl", {
        project_id = var.project_id
        threshold  = var.policy_settings[each.key].threshold
      }))
      duration = var.policy_settings[each.key].duration
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "[${var.environment}] Review YouTube quota usage: https://console.cloud.google.com/iam-admin/quotas?service=$${resource.label.service}&metric=$${metric.label.quota_metric}&limit=$${metric.label.limit_name}&project=${var.project_id}&fromNotifications=1"
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close           = var.auto_close
    notification_prompts = each.value.prompts
  }

  lifecycle {
    prevent_destroy = true
    precondition {
      condition = length(var.notification_channels) > 0 && alltrue([
        for channel in var.notification_channels :
        startswith(channel, "projects/${var.project_id}/notificationChannels/")
      ])
      error_message = "Policy ownership requires verified notification channels in the target project. Do not copy the dangling production channel."
    }
  }
}
