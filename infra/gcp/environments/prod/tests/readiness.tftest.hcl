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

run "email_opt_in_is_a_create_not_an_import" {
  command = plan
  variables {
    manage_primary_email  = true
    primary_email_address = "fixture@example.invalid"
  }
}

run "quota_opt_in_is_a_create_not_an_import" {
  command = plan
  variables {
    manage_youtube_quota_alerts         = true
    youtube_quota_notification_channels = ["projects/youtube-study-space/notificationChannels/fixture"]
  }
}
