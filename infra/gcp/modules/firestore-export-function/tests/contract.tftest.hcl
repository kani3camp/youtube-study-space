mock_provider "google" {}

variables {
  project_id                      = "test-youtube-study-space"
  region                          = "asia-southeast2"
  topic_id                        = "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"
  execution_service_account_email = "existing-execution@example.invalid"
}

run "preserve_existing_gen1_runtime_trigger_and_limits" {
  command = plan
  assert {
    condition     = google_cloudfunctions_function.export.name == "firestoreCollectionsExport" && google_cloudfunctions_function.export.project == var.project_id && google_cloudfunctions_function.export.region == var.region && google_cloudfunctions_function.export.runtime == "nodejs22" && google_cloudfunctions_function.export.entry_point == "scheduledFirestoreExport"
    error_message = "Preserve the existing Gen1 Node22 deployment identity and entry point."
  }
  assert {
    condition     = google_cloudfunctions_function.export.available_memory_mb == 256 && google_cloudfunctions_function.export.timeout == 60 && google_cloudfunctions_function.export.max_instances == 1 && google_cloudfunctions_function.export.min_instances == 0 && google_cloudfunctions_function.export.ingress_settings == "ALLOW_ALL" && google_cloudfunctions_function.export.service_account_email == var.execution_service_account_email
    error_message = "Preserve execution identity, resource limits and ingress."
  }
  assert {
    condition     = google_cloudfunctions_function.export.event_trigger[0].resource == var.topic_id && google_cloudfunctions_function.export.event_trigger[0].event_type == "google.pubsub.topic.publish" && google_cloudfunctions_function.export.event_trigger[0].failure_policy[0].retry == false && google_cloudfunctions_function.export.environment_variables == tomap({ YSS_EXPORT_ENVIRONMENT = "development", YSS_EXPORT_PROJECT_ID = var.project_id })
    error_message = "Preserve the adopted topic, no-retry semantics and the two explicit runtime environment values."
  }
  assert {
    condition     = google_cloudfunctions_function.export.source_archive_bucket == null && google_cloudfunctions_function.export.source_archive_object == null && length(google_cloudfunctions_function.export.source_repository) == 0 && (google_cloudfunctions_function.export.labels == null || try(length(google_cloudfunctions_function.export.labels) == 0, false)) && google_cloudfunctions_function.export.trigger_http == null
    error_message = "Do not introduce source ownership, reserved labels or an HTTP trigger."
  }
}

run "preserve_production_environment_without_changing_runtime_or_trigger" {
  command = plan
  variables {
    project_id          = "youtube-study-space"
    region              = "asia-northeast2"
    topic_id            = "projects/youtube-study-space/topics/fixture-prod-export"
    environment         = "production"
    name                = "fixtureProdExportFunction"
    available_memory_mb = 512
    timeout             = 120
  }
  assert {
    condition     = google_cloudfunctions_function.export.name == "fixtureProdExportFunction" && google_cloudfunctions_function.export.region == "asia-northeast2" && google_cloudfunctions_function.export.available_memory_mb == 512 && google_cloudfunctions_function.export.timeout == 120 && google_cloudfunctions_function.export.max_instances == 1 && google_cloudfunctions_function.export.min_instances == 0
    error_message = "Preserve an independently observed production name, region, memory and timeout without altering concurrency."
  }
  assert {
    condition     = google_cloudfunctions_function.export.environment_variables == tomap({ YSS_EXPORT_ENVIRONMENT = "production", YSS_EXPORT_PROJECT_ID = "youtube-study-space" })
    error_message = "Production must retain its own project and export runtime environment."
  }
  assert {
    condition     = google_cloudfunctions_function.export.runtime == "nodejs22" && google_cloudfunctions_function.export.entry_point == "scheduledFirestoreExport" && google_cloudfunctions_function.export.event_trigger[0].resource == var.topic_id && google_cloudfunctions_function.export.event_trigger[0].failure_policy[0].retry == false && google_cloudfunctions_function.export.source_archive_bucket == null && google_cloudfunctions_function.export.source_archive_object == null && length(google_cloudfunctions_function.export.source_repository) == 0
    error_message = "Selecting an environment must preserve the existing runtime, trigger and external source ownership."
  }
}

run "reject_unknown_runtime_environment" {
  command = plan
  variables {
    environment = "staging"
  }
  expect_failures = [var.environment]
}

run "allow_other_google_memory_and_timeout_limits" {
  command = plan
  variables {
    available_memory_mb = 8192
    timeout             = 540
  }
  assert {
    condition     = google_cloudfunctions_function.export.available_memory_mb == 8192 && google_cloudfunctions_function.export.timeout == 540
    error_message = "Do not restrict reusable limits to the two observed environment settings."
  }
}

run "reject_function_resource_path_instead_of_short_name" {
  command = plan
  variables {
    name = "projects/fixture/locations/fixture/functions/fixture"
  }
  expect_failures = [var.name]
}

run "reject_nonpositive_memory" {
  command = plan
  variables {
    available_memory_mb = 0
  }
  expect_failures = [var.available_memory_mb]
}

run "reject_fractional_memory" {
  command = plan
  variables {
    available_memory_mb = 256.5
  }
  expect_failures = [var.available_memory_mb]
}

run "reject_nonpositive_timeout" {
  command = plan
  variables {
    timeout = 0
  }
  expect_failures = [var.timeout]
}

run "reject_timeout_above_gen1_limit" {
  command = plan
  variables {
    timeout = 541
  }
  expect_failures = [var.timeout]
}

run "reject_fractional_timeout" {
  command = plan
  variables {
    timeout = 60.5
  }
  expect_failures = [var.timeout]
}
