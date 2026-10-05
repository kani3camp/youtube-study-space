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
