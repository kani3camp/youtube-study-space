# Source-only adoption definitions. Selection is cumulative and default-off;
# each newly selected alias must have a fresh private manifest/import review.
variable "existing_resources" {
  description = "Cumulative aliases of already-existing resources to import. This never enables a notification repair or a production execution gate."
  type        = set(string)
  default     = []
  validation {
    condition = length(setsubtract(var.existing_resources, toset([
      "backup_schedule", "backup_bucket", "dataset", "order_history", "user_activity_history",
      "export_topic", "export_scheduler", "export_function", "quota_minute_80", "quota_day_80", "quota_day_60"
    ]))) == 0
    error_message = "Select only supported existing resource aliases."
  }
  validation {
    condition     = alltrue([for key in var.existing_resources : try(local.existing_by_alias[key] != null, false)])
    error_message = "Every selected existing resource requires its private observed metadata."
  }
  validation {
    condition     = length(var.existing_resources) == 0 || (!var.manage_primary_email && !var.manage_youtube_quota_alerts)
    error_message = "Existing-resource import definitions cannot be mixed with legacy notification creation."
  }
  validation {
    condition = length(distinct([
      for alias in var.existing_resources : try(local.existing_by_alias[alias].id, null) if startswith(alias, "quota_")
    ])) == length([for alias in var.existing_resources : alias if startswith(alias, "quota_")])
    error_message = "Selected quota aliases must reference distinct existing policies; one remote resource cannot have multiple owners."
  }
}

variable "existing_resource_inputs" {
  description = "Private observed metadata only, never credentials or data contents. Import IDs and identities must not be committed."
  sensitive   = true
  nullable    = false
  type = object({
    backup_schedule = optional(object({ id = string, retention = string }))
    backup_bucket = optional(object({
      name             = string, location = string, storage_class = string,
      retention_policy = object({ seconds = number, locked = bool }), soft_delete_seconds = number
    }))
    dataset       = optional(object({ dataset_id = string, location = string }))
    order_history = optional(object({ dataset_id = string, field_order = list(string) }))
    user_activity_history = optional(object({
      dataset_id = string, field_order = list(string), field_descriptions = map(string)
    }))
    export_topic     = optional(object({ name = string }))
    export_scheduler = optional(object({ name = string, region = string, topic_id = string }))
    export_function = optional(object({
      name                = string, region = string, topic_id = string, execution_service_account_email = string,
      available_memory_mb = number, timeout = number
    }))
    quota_policies = optional(map(object({
      id        = string, display_name = string, notification_channels = list(string), documentation = string,
      threshold = number, duration = string, enabled = bool, severity = optional(string)
    })), {})
  })
  default = {}
  validation {
    condition = alltrue([
      for key in keys(var.existing_resource_inputs.quota_policies) : contains(["minute_80", "day_80", "day_60"], key)
    ])
    error_message = "Existing quota metadata accepts only the three supported aliases."
  }
  validation {
    condition = alltrue(concat(
      var.existing_resource_inputs.backup_schedule == null ? [] : [can(regex("^projects/${var.project_id}/databases/\\(default\\)/backupSchedules/[0-9a-f-]{36}$", var.existing_resource_inputs.backup_schedule.id))],
      [for policy in values(var.existing_resource_inputs.quota_policies) : can(regex("^projects/${var.project_id}/alertPolicies/[0-9]+$", policy.id))],
      [for policy in values(var.existing_resource_inputs.quota_policies) : length(policy.notification_channels) > 0 && alltrue([
        for channel in policy.notification_channels : startswith(channel, "projects/${var.project_id}/notificationChannels/")
      ])],
      [for item in [var.existing_resource_inputs.export_scheduler, var.existing_resource_inputs.export_function] : try(can(regex("^projects/${var.project_id}/topics/[A-Za-z][A-Za-z0-9._~+-]{2,254}$", item.topic_id)), true) if item != null]
    ))
    error_message = "Existing IDs and topic references must belong to the exact production project."
  }
  validation {
    condition = var.existing_resource_inputs.export_function == null ? true : (
      var.existing_resource_inputs.export_function.execution_service_account_email == "${var.project_id}@appspot.gserviceaccount.com" ||
      endswith(var.existing_resource_inputs.export_function.execution_service_account_email, "@${var.project_id}.iam.gserviceaccount.com")
    )
    error_message = "The existing Function execution identity must belong to the production project."
  }
}

locals {
  # Metadata must preserve the provider's imported sensitivity mask. It contains
  # no credential fields; raw plans/state/logs remain private in every workflow.
  existing_metadata = nonsensitive(var.existing_resource_inputs)
  existing_by_alias = merge({
    for key, value in local.existing_metadata : key => value if key != "quota_policies"
  }, { for key, value in local.existing_metadata.quota_policies : "quota_${key}" => value })
  existing_quota_keys = toset([
    for key in ["minute_80", "day_80", "day_60"] : key if contains(var.existing_resources, "quota_${key}")
  ])
}

resource "google_firestore_backup_schedule" "daily" {
  count     = contains(var.existing_resources, "backup_schedule") ? 1 : 0
  project   = var.project_id
  database  = "(default)"
  retention = try(local.existing_metadata.backup_schedule.retention, "1728000s")
  daily_recurrence {}
  lifecycle { prevent_destroy = true }
}
import {
  for_each = contains(var.existing_resources, "backup_schedule") ? toset(["existing"]) : toset([])
  to       = google_firestore_backup_schedule.daily[0]
  id       = local.existing_metadata.backup_schedule.id
}

module "backup_bucket" {
  count               = contains(var.existing_resources, "backup_bucket") ? 1 : 0
  source              = "../../modules/retained-backup-bucket"
  project_id          = var.project_id
  bucket_name         = try(local.existing_metadata.backup_bucket.name, "missing-private-metadata")
  location            = try(local.existing_metadata.backup_bucket.location, var.region)
  storage_class       = try(local.existing_metadata.backup_bucket.storage_class, "COLDLINE")
  retention_policy    = try(local.existing_metadata.backup_bucket.retention_policy, null)
  soft_delete_seconds = try(local.existing_metadata.backup_bucket.soft_delete_seconds, 604800)
}
import {
  for_each = contains(var.existing_resources, "backup_bucket") ? toset(["existing"]) : toset([])
  to       = module.backup_bucket[0].google_storage_bucket.retained
  id       = local.existing_metadata.backup_bucket.name
}

module "retained_dataset" {
  count      = contains(var.existing_resources, "dataset") ? 1 : 0
  source     = "../../modules/retained-bigquery-dataset"
  project_id = var.project_id
  dataset_id = try(local.existing_metadata.dataset.dataset_id, "missing_private_metadata")
  location   = try(local.existing_metadata.dataset.location, var.region)
}
import {
  for_each = contains(var.existing_resources, "dataset") ? toset(["existing"]) : toset([])
  to       = module.retained_dataset[0].google_bigquery_dataset.retained
  id       = "projects/${var.project_id}/datasets/${local.existing_metadata.dataset.dataset_id}"
}

module "order_history" {
  count      = contains(var.existing_resources, "order_history") ? 1 : 0
  source     = "../../modules/retained-order-history"
  project_id = var.project_id
  dataset_id = try(local.existing_metadata.order_history.dataset_id, "missing_private_metadata")
  field_order = try(local.existing_metadata.order_history.field_order, [
    "seat_id", "ordered_at", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"
  ])
}
import {
  for_each = contains(var.existing_resources, "order_history") ? toset(["existing"]) : toset([])
  to       = module.order_history[0].google_bigquery_table.retained
  id       = "projects/${var.project_id}/datasets/${local.existing_metadata.order_history.dataset_id}/tables/order-history"
}

module "user_activity_history" {
  count      = contains(var.existing_resources, "user_activity_history") ? 1 : 0
  source     = "../../modules/retained-user-activity-history"
  project_id = var.project_id
  dataset_id = try(local.existing_metadata.user_activity_history.dataset_id, "missing_private_metadata")
  field_order = try(local.existing_metadata.user_activity_history.field_order, [
    "seat_id", "taken_at", "user_id", "activity_type", "is_member_seat", "__key__", "__error__", "__has_error__"
  ])
  field_descriptions = try(local.existing_metadata.user_activity_history.field_descriptions, {})
}
import {
  for_each = contains(var.existing_resources, "user_activity_history") ? toset(["existing"]) : toset([])
  to       = module.user_activity_history[0].google_bigquery_table.retained
  id       = "projects/${var.project_id}/datasets/${local.existing_metadata.user_activity_history.dataset_id}/tables/user-activity-history"
}

module "export_topic" {
  count      = contains(var.existing_resources, "export_topic") ? 1 : 0
  source     = "../../modules/firestore-export-topic"
  project_id = var.project_id
  topic_name = try(local.existing_metadata.export_topic.name, "missing-private-metadata")
}
import {
  for_each = contains(var.existing_resources, "export_topic") ? toset(["existing"]) : toset([])
  to       = module.export_topic[0].google_pubsub_topic.export
  id       = "projects/${var.project_id}/topics/${local.existing_metadata.export_topic.name}"
}

module "export_scheduler" {
  count      = contains(var.existing_resources, "export_scheduler") ? 1 : 0
  source     = "../../modules/firestore-export-scheduler"
  project_id = var.project_id
  name       = try(local.existing_metadata.export_scheduler.name, "missing-private-metadata")
  region     = try(local.existing_metadata.export_scheduler.region, var.region)
  topic_id   = try(local.existing_metadata.export_scheduler.topic_id, "")
}
import {
  for_each = contains(var.existing_resources, "export_scheduler") ? toset(["existing"]) : toset([])
  to       = module.export_scheduler[0].google_cloud_scheduler_job.export
  id       = "projects/${var.project_id}/locations/${local.existing_metadata.export_scheduler.region}/jobs/${local.existing_metadata.export_scheduler.name}"
}

module "export_function" {
  count                           = contains(var.existing_resources, "export_function") ? 1 : 0
  source                          = "../../modules/firestore-export-function"
  project_id                      = var.project_id
  name                            = try(local.existing_metadata.export_function.name, "missing-private-metadata")
  region                          = try(local.existing_metadata.export_function.region, var.region)
  topic_id                        = try(local.existing_metadata.export_function.topic_id, "")
  available_memory_mb             = try(local.existing_metadata.export_function.available_memory_mb, 256)
  timeout                         = try(local.existing_metadata.export_function.timeout, 60)
  environment                     = "production"
  execution_service_account_email = try(local.existing_metadata.export_function.execution_service_account_email, "")
}
import {
  for_each = contains(var.existing_resources, "export_function") ? toset(["existing"]) : toset([])
  to       = module.export_function[0].google_cloudfunctions_function.export
  id       = "projects/${var.project_id}/locations/${local.existing_metadata.export_function.region}/functions/${local.existing_metadata.export_function.name}"
}

module "existing_youtube_quota_alerts" {
  source                       = "../../modules/youtube-quota-alerts"
  project_id                   = var.project_id
  environment                  = "production"
  import_existing_policies     = length(local.existing_quota_keys) > 0
  policy_keys                  = local.existing_quota_keys
  legacy_display_names         = { for key, value in local.existing_metadata.quota_policies : key => value.display_name }
  legacy_notification_channels = { for key, value in local.existing_metadata.quota_policies : key => value.notification_channels }
  legacy_documentation         = { for key, value in local.existing_metadata.quota_policies : key => value.documentation }
  policy_settings = {
    for key in ["minute_80", "day_80", "day_60"] : key => try({
      threshold = local.existing_metadata.quota_policies[key].threshold,
      duration  = local.existing_metadata.quota_policies[key].duration,
      enabled   = local.existing_metadata.quota_policies[key].enabled,
      severity  = local.existing_metadata.quota_policies[key].severity
    }, { threshold = key == "day_60" ? 0.6 : 0.8, duration = "60s", enabled = true, severity = null })
  }
}
import {
  for_each = local.existing_quota_keys
  to       = module.existing_youtube_quota_alerts.google_monitoring_alert_policy.existing[each.key]
  id       = local.existing_metadata.quota_policies[each.key].id
}
