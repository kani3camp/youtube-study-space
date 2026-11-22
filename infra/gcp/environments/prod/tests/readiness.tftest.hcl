# All runs are provider-mocked plans with in-memory state, never cloud operations.
mock_provider "google" {}

run "default_production_owns_nothing" {
  command = plan
  assert {
    condition     = output.environment == "production" && output.project_id == "youtube-study-space"
    error_message = "The inert root still identifies its exact production target."
  }
  assert {
    condition     = module.notification_channels.primary_email_name == null && length(module.youtube_quota_alerts.policy_names) == 0
    error_message = "Notification ownership must remain default-off."
  }
}

run "wrong_project_stops_before_ownership" {
  command = plan
  variables {
    project_id = "fixture-wrong-project"
  }
  expect_failures = [var.project_id]
}

run "email_creation_is_rejected_in_production" {
  command = plan
  variables {
    manage_primary_email  = true
    primary_email_address = "fixture@example.invalid"
  }
  expect_failures = [var.manage_primary_email]
}

run "email_adoption_requires_a_numeric_production_channel" {
  command = plan
  variables {
    manage_primary_email       = true
    primary_email_address      = "fixture@example.invalid"
    primary_email_channel_name = "projects/youtube-study-space/notificationChannels/fixture"
  }
  expect_failures = [var.manage_primary_email]
}

run "email_adoption_imports_the_existing_channel" {
  command = plan
  variables {
    manage_primary_email       = true
    primary_email_address      = "fixture@example.invalid"
    primary_email_channel_name = "projects/youtube-study-space/notificationChannels/10000000000000000009"
  }
  override_resource {
    target = module.notification_channels.google_monitoring_notification_channel.primary_email[0]
    values = {
      id   = "projects/youtube-study-space/notificationChannels/10000000000000000009"
      name = "projects/youtube-study-space/notificationChannels/10000000000000000009"
    }
  }
  assert {
    condition     = nonsensitive(module.notification_channels.primary_email_name) == "projects/youtube-study-space/notificationChannels/10000000000000000009"
    error_message = "Adoption must own the selected existing channel."
  }
}

run "quota_opt_in_is_a_create_not_an_import" {
  command = plan
  variables {
    manage_youtube_quota_alerts         = true
    youtube_quota_notification_channels = ["projects/youtube-study-space/notificationChannels/fixture"]
  }
}
