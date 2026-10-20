# Only synthetic metadata/readbacks are used. Mock import no-ops exercise the
# source contract; a live import still requires fresh exact metadata and state.
mock_provider "google" {}
override_resource {
  target = google_firestore_backup_schedule.daily[0]
  values = { id = "projects/youtube-study-space/databases/(default)/backupSchedules/00000000-0000-4000-8000-000000000001" }
}
override_resource {
  target = module.backup_bucket[0].google_storage_bucket.retained
  values = { id = "fixture-retained-backup" }
}
override_resource {
  target = module.retained_dataset[0].google_bigquery_dataset.retained
  values = { id = "projects/youtube-study-space/datasets/fixture_export" }
}
override_resource {
  target = module.order_history[0].google_bigquery_table.retained
  values = { id = "projects/youtube-study-space/datasets/fixture_export/tables/order-history" }
}
override_resource {
  target = module.user_activity_history[0].google_bigquery_table.retained
  values = { id = "projects/youtube-study-space/datasets/fixture_export/tables/user-activity-history" }
}
override_resource {
  target = module.export_topic[0].google_pubsub_topic.export
  values = { id = "projects/youtube-study-space/topics/fixture-existing-topic" }
}
override_resource {
  target = module.export_scheduler[0].google_cloud_scheduler_job.export
  values = { id = "projects/youtube-study-space/locations/asia-northeast2/jobs/fixture-existing-scheduler" }
}
override_resource {
  target = module.export_function[0].google_cloudfunctions_function.export
  values = { id = "projects/youtube-study-space/locations/asia-northeast2/functions/fixtureExistingFunction" }
}
override_resource {
  target = module.existing_youtube_quota_alerts.google_monitoring_alert_policy.existing["minute_80"]
  values = { id = "projects/youtube-study-space/alertPolicies/10000000000000000001" }
}
override_resource {
  target = module.existing_youtube_quota_alerts.google_monitoring_alert_policy.existing["day_80"]
  values = { id = "projects/youtube-study-space/alertPolicies/10000000000000000002" }
}
override_resource {
  target = module.existing_youtube_quota_alerts.google_monitoring_alert_policy.existing["day_60"]
  values = { id = "projects/youtube-study-space/alertPolicies/10000000000000000003" }
}
variables {
  existing_resource_inputs = {
    backup_schedule = {
      id        = "projects/youtube-study-space/databases/(default)/backupSchedules/00000000-0000-4000-8000-000000000001"
      retention = "1728000s"
    }
    backup_bucket = {
      name                = "fixture-retained-backup"
      location            = "ASIA-NORTHEAST2"
      storage_class       = "COLDLINE"
      retention_policy    = { seconds = 8035200, locked = false }
      soft_delete_seconds = 604800
    }
    dataset = { dataset_id = "fixture_export", location = "asia-northeast2" }
    order_history = {
      dataset_id  = "fixture_export"
      field_order = ["ordered_at", "seat_id", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"]
    }
    user_activity_history = {
      dataset_id         = "fixture_export"
      field_order        = ["seat_id", "taken_at", "activity_type", "user_id", "__key__", "__error__", "__has_error__", "is_member_seat"]
      field_descriptions = { taken_at = "Synthetic existing description" }
    }
    export_topic = { name = "fixture-existing-topic" }
    export_scheduler = {
      name     = "fixture-existing-scheduler"
      region   = "asia-northeast2"
      topic_id = "projects/youtube-study-space/topics/fixture-existing-topic"
    }
    export_function = {
      name                            = "fixtureExistingFunction"
      region                          = "asia-northeast2"
      topic_id                        = "projects/youtube-study-space/topics/fixture-existing-topic"
      execution_service_account_email = "fixture-execution@youtube-study-space.iam.gserviceaccount.com"
      available_memory_mb             = 512
      timeout                         = 120
    }
    quota_policies = {
      minute_80 = {
        id                    = "projects/youtube-study-space/alertPolicies/10000000000000000001"
        display_name          = "Existing minute fixture"
        notification_channels = ["projects/youtube-study-space/notificationChannels/fixture-missing-minute"]
        documentation         = "Existing minute documentation fixture"
        threshold             = 0.8
        duration              = "60s"
        enabled               = true
      }
      day_80 = {
        id                    = "projects/youtube-study-space/alertPolicies/10000000000000000002"
        display_name          = "Existing high day fixture"
        notification_channels = ["projects/youtube-study-space/notificationChannels/fixture-missing-day-high"]
        documentation         = "Existing high day documentation fixture"
        threshold             = 0.8
        duration              = "60s"
        enabled               = true
      }
      day_60 = {
        id                    = "projects/youtube-study-space/alertPolicies/10000000000000000003"
        display_name          = "Existing warning day fixture"
        notification_channels = ["projects/youtube-study-space/notificationChannels/fixture-missing-day-warning"]
        documentation         = "Existing warning day documentation fixture"
        threshold             = 0.6
        duration              = "60s"
        enabled               = true
      }
    }
  }
}
run "private_metadata_alone_does_not_activate_ownership" {
  command = plan
}
run "one_existing_backup_schedule" {
  command = plan
  variables { existing_resources = ["backup_schedule"] }
  assert {
    condition     = google_firestore_backup_schedule.daily[0].retention == "1728000s" && length(google_firestore_backup_schedule.daily[0].daily_recurrence) == 1
    error_message = "Preserve the existing native daily schedule and observed retention."
  }
}
run "one_existing_backup_bucket" {
  command = plan
  variables { existing_resources = ["backup_bucket"] }
}
run "one_existing_dataset" {
  command = plan
  variables { existing_resources = ["dataset"] }
}
run "one_existing_order_history" {
  command = plan
  variables { existing_resources = ["order_history"] }
}
run "one_existing_user_activity_history" {
  command = plan
  variables { existing_resources = ["user_activity_history"] }
}
run "one_existing_export_topic" {
  command = plan
  variables { existing_resources = ["export_topic"] }
}
run "one_existing_export_scheduler" {
  command = plan
  variables { existing_resources = ["export_scheduler"] }
}
run "one_existing_export_function" {
  command = plan
  variables { existing_resources = ["export_function"] }
}
run "one_existing_quota_minute_80" {
  command = plan
  variables { existing_resources = ["quota_minute_80"] }
}
run "one_existing_quota_day_80" {
  command = plan
  variables { existing_resources = ["quota_day_80"] }
}
run "one_existing_quota_day_60" {
  command = plan
  variables { existing_resources = ["quota_day_60"] }
}
run "cumulative_existing_resources" {
  command = plan
  variables {
    existing_resources = [
      "backup_schedule", "backup_bucket", "dataset", "order_history", "user_activity_history", "export_topic",
      "export_scheduler", "export_function", "quota_minute_80", "quota_day_80", "quota_day_60"
    ]
  }
}
run "unsupported_ownership_alias_is_rejected" {
  command = plan
  variables { existing_resources = ["create_notification_channel"] }
  expect_failures = [var.existing_resources]
}
run "missing_private_metadata_is_rejected" {
  command = plan
  variables {
    existing_resources       = ["export_function"]
    existing_resource_inputs = {}
  }
  expect_failures = [var.existing_resources]
}
run "foreign_existing_id_is_rejected" {
  command = plan
  variables {
    existing_resources = ["backup_schedule"]
    existing_resource_inputs = {
      backup_schedule = { id = "projects/foreign-project/databases/(default)/backupSchedules/fixture", retention = "1728000s" }
    }
  }
  expect_failures = [var.existing_resource_inputs]
}
run "foreign_existing_topic_is_rejected" {
  command = plan
  variables {
    existing_resources = ["export_scheduler"]
    existing_resource_inputs = {
      export_scheduler = { name = "fixture", region = "asia-northeast2", topic_id = "projects/foreign-project/topics/fixture" }
    }
  }
  expect_failures = [var.existing_resource_inputs]
}
run "foreign_function_identity_is_rejected" {
  command = plan
  variables {
    existing_resources = ["export_function"]
    existing_resource_inputs = {
      export_function = {
        name                            = "fixture", region = "asia-northeast2", topic_id = "projects/youtube-study-space/topics/fixture",
        execution_service_account_email = "fixture@foreign-project.iam.gserviceaccount.com", available_memory_mb = 512, timeout = 120
      }
    }
  }
  expect_failures = [var.existing_resource_inputs]
}
run "import_cannot_mix_notification_creation" {
  command = plan
  variables {
    existing_resources    = ["export_topic"]
    manage_primary_email  = true
    primary_email_address = "fixture@example.invalid"
  }
  expect_failures = [var.existing_resources]
}
run "duplicate_existing_quota_ownership_is_rejected" {
  command = plan
  variables {
    existing_resources = ["quota_minute_80", "quota_day_80"]
    existing_resource_inputs = {
      quota_policies = {
        minute_80 = {
          id                    = "projects/youtube-study-space/alertPolicies/10000000000000000001"
          display_name          = "Existing minute fixture"
          notification_channels = ["projects/youtube-study-space/notificationChannels/fixture-missing"]
          documentation         = "Synthetic existing documentation"
          threshold             = 0.8
          duration              = "60s"
          enabled               = true
        }
        day_80 = {
          id                    = "projects/youtube-study-space/alertPolicies/10000000000000000001"
          display_name          = "Existing day fixture"
          notification_channels = ["projects/youtube-study-space/notificationChannels/fixture-missing"]
          documentation         = "Synthetic existing documentation"
          threshold             = 0.8
          duration              = "60s"
          enabled               = true
        }
      }
    }
  }
  expect_failures = [var.existing_resources]
}
