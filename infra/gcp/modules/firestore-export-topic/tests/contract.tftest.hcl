mock_provider "google" {}

variables {
  project_id = "test-youtube-study-space"
  topic_name = "initiateFirestoreCollectionsExport"
}

run "development_preserves_existing_topic_contract" {
  command = plan
  assert {
    condition     = google_pubsub_topic.export.project == "test-youtube-study-space" && google_pubsub_topic.export.name == "initiateFirestoreCollectionsExport" && length(google_pubsub_topic.export.labels) == 0 && length(google_pubsub_topic.export.message_storage_policy) == 0 && length(google_pubsub_topic.export.schema_settings) == 0 && length(google_pubsub_topic.export.ingestion_data_source_settings) == 0 && length(google_pubsub_topic.export.message_transforms) == 0
    error_message = "Preserve the existing topic without labels, new storage policy, schema, ingestion or transformations."
  }
}

run "production_uses_same_topic_structure_without_root_ownership" {
  command = plan
  variables {
    project_id = "youtube-study-space"
    topic_name = "initiateFirestoreExport"
  }
  assert {
    condition     = google_pubsub_topic.export.project == "youtube-study-space" && google_pubsub_topic.export.name == "initiateFirestoreExport" && length(google_pubsub_topic.export.labels) == 0
    error_message = "The shared module must preserve production parameters without enabling production root ownership."
  }
}
