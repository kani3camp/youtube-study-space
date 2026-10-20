mock_provider "google" {}
variables {
  project_id  = "channel-test-dev"
  environment = "development"
}
run "migration_defaults_are_inert" {
  command = plan
  assert {
    condition     = length(google_monitoring_notification_channel.primary_email) == 0
    error_message = "Channel ownership must stay opt-in during migration."
  }
}
run "development_owns_only_private_enabled_email" {
  command = plan
  variables {
    manage_primary_email = true
    email_address        = "fixture@example.invalid"
  }
  assert {
    condition     = length(google_monitoring_notification_channel.primary_email) == 1 && google_monitoring_notification_channel.primary_email[0].type == "email" && google_monitoring_notification_channel.primary_email[0].enabled && !google_monitoring_notification_channel.primary_email[0].force_delete && google_monitoring_notification_channel.primary_email[0].project == "channel-test-dev" && google_monitoring_notification_channel.primary_email[0].display_name == "[development] Study Space primary email" && google_monitoring_notification_channel.primary_email[0].labels == tomap({ email_address = "fixture@example.invalid" })
    error_message = "Own one enabled target-project Email channel with force deletion disabled."
  }
}
run "production_uses_same_logical_structure" {
  command = plan
  variables {
    manage_primary_email = true
    project_id           = "channel-test-prod"
    environment          = "production"
    email_address        = "other@example.invalid"
  }
  assert {
    condition     = google_monitoring_notification_channel.primary_email[0].type == "email" && google_monitoring_notification_channel.primary_email[0].project == "channel-test-prod" && google_monitoring_notification_channel.primary_email[0].display_name == "[production] Study Space primary email" && google_monitoring_notification_channel.primary_email[0].labels == tomap({ email_address = "other@example.invalid" })
    error_message = "Each project owns its own channel; mailbox is an input."
  }
}
run "missing_private_address_stops_ownership" {
  command = plan
  variables { manage_primary_email = true }
  expect_failures = [google_monitoring_notification_channel.primary_email]
}
run "invalid_private_address_stops_ownership" {
  command = plan
  variables {
    manage_primary_email = true
    email_address        = "invalid"
  }
  expect_failures = [google_monitoring_notification_channel.primary_email]
}
