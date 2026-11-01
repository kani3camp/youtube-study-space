mock_provider "google" {}

variables {
  project_id  = "quota-test-dev"
  environment = "development"
  policy_settings = {
    minute_80 = { threshold = 0.8, duration = "60s" }
    day_80    = { threshold = 0.8, duration = "60s" }
    day_60    = { threshold = 0.6, duration = "60s" }
  }
}

run "migration_is_inert" {
  command = plan
  assert {
    condition     = length(google_monitoring_alert_policy.quota) == 0
    error_message = "Migration defaults must not own or create alerts."
  }
}

run "three_types_preserve_quota_semantics" {
  command = plan
  variables {
    manage_policies       = true
    notification_channels = ["projects/quota-test-dev/notificationChannels/verified-fixture"]
  }
  assert {
    condition = length(google_monitoring_alert_policy.quota) == 3 && alltrue([
      for policy in values(google_monitoring_alert_policy.quota) :
      policy.enabled && policy.combiner == "OR" && startswith(policy.display_name, "[development]") &&
      policy.conditions[0].condition_monitoring_query_language[0].duration == "60s" &&
      policy.conditions[0].condition_monitoring_query_language[0].trigger[0].count == 1 &&
      strcontains(policy.conditions[0].condition_monitoring_query_language[0].query, "resource.project_id=='quota-test-dev'") &&
      policy.notification_channels == tolist(["projects/quota-test-dev/notificationChannels/verified-fixture"])
    ])
    error_message = "All quota types must use the target project, notification path, and preserved condition semantics."
  }
  assert {
    condition = (strcontains(google_monitoring_alert_policy.quota["minute_80"].conditions[0].condition_monitoring_query_language[0].query, "align delta_gauge(1m)") &&
      strcontains(google_monitoring_alert_policy.quota["day_60"].conditions[0].condition_monitoring_query_language[0].query, "America/Los_Angeles") &&
      strcontains(google_monitoring_alert_policy.quota["day_60"].conditions[0].condition_monitoring_query_language[0].query, "0.6 '1'") &&
    google_monitoring_alert_policy.quota["day_60"].alert_strategy[0].notification_prompts == tolist(["OPENED", "CLOSED"]))
    error_message = "Preserve minute alignment, YouTube's Pacific quota day, and warning ratio/prompts."
  }
}

run "production_values_are_parameters" {
  command = plan
  variables {
    project_id            = "quota-test-prod"
    environment           = "production"
    manage_policies       = true
    notification_channels = ["projects/quota-test-prod/notificationChannels/verified-fixture"]
    policy_settings = {
      minute_80 = { threshold = 0.9, duration = "120s" }
      day_80    = { threshold = 0.85, duration = "120s" }
      day_60    = { threshold = 0.5, duration = "120s" }
    }
  }
  assert {
    condition = alltrue([
      for policy in values(google_monitoring_alert_policy.quota) : startswith(policy.display_name, "[production]") &&
      strcontains(policy.conditions[0].condition_monitoring_query_language[0].query, "resource.project_id=='quota-test-prod'") &&
      policy.conditions[0].condition_monitoring_query_language[0].duration == "120s"
    ]) && strcontains(google_monitoring_alert_policy.quota["day_60"].conditions[0].condition_monitoring_query_language[0].query, "0.5 '1'")
    error_message = "Environment values must be parameterized without separate query definitions."
  }
}

run "missing_notification_channel_is_rejected" {
  command = plan
  variables {
    manage_policies = true
  }
  expect_failures = [google_monitoring_alert_policy.quota]
}

run "foreign_notification_channel_is_rejected" {
  command = plan
  variables {
    manage_policies       = true
    notification_channels = ["projects/quota-test-prod/notificationChannels/foreign-fixture"]
  }
  expect_failures = [google_monitoring_alert_policy.quota]
}
