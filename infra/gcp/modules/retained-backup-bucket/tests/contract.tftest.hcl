mock_provider "google" {}

variables {
  project_id          = "backup-test-dev"
  bucket_name         = "backup-test-dev"
  location            = "ASIA-SOUTHEAST2"
  storage_class       = "STANDARD"
  soft_delete_seconds = 604800
}

run "development_preserves_data_boundary" {
  command = plan
  assert {
    condition     = google_storage_bucket.retained.location == "ASIA-SOUTHEAST2" && google_storage_bucket.retained.storage_class == "STANDARD" && length(google_storage_bucket.retained.retention_policy) == 0 && google_storage_bucket.retained.soft_delete_policy[0].retention_duration_seconds == 604800 && google_storage_bucket.retained.uniform_bucket_level_access && google_storage_bucket.retained.public_access_prevention == "enforced" && !google_storage_bucket.retained.force_destroy && !google_storage_bucket.retained.default_event_based_hold && !google_storage_bucket.retained.requester_pays && length(google_storage_bucket.retained.lifecycle_rule) == 0 && length(google_storage_bucket.retained.versioning) == 0
    error_message = "Preserve development metadata, private access, soft delete and object safety."
  }
}

run "production_preserves_retention_and_region" {
  command = plan
  variables {
    project_id       = "backup-test-prod"
    bucket_name      = "backup-test-prod"
    location         = "ASIA-NORTHEAST2"
    storage_class    = "COLDLINE"
    retention_policy = { seconds = 8035200, locked = false }
  }
  assert {
    condition     = google_storage_bucket.retained.location == "ASIA-NORTHEAST2" && google_storage_bucket.retained.storage_class == "COLDLINE" && tonumber(google_storage_bucket.retained.retention_policy[0].retention_period) == 8035200 && !google_storage_bucket.retained.retention_policy[0].is_locked && google_storage_bucket.retained.public_access_prevention == "enforced" && google_storage_bucket.retained.soft_delete_policy[0].retention_duration_seconds == 604800 && !google_storage_bucket.retained.force_destroy
    error_message = "Production has intentional region, class and retention differences; preserve them."
  }
}
