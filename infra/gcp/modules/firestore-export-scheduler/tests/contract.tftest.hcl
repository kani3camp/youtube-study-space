mock_provider "google" {}

variables {
  project_id = "test-youtube-study-space"
  region     = "asia-southeast2"
  topic_id   = "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"
}

run "preserves_existing_development_schedule_and_delivery" {
  command = plan
  assert {
    condition     = google_cloud_scheduler_job.export.project == "test-youtube-study-space" && google_cloud_scheduler_job.export.region == "asia-southeast2" && google_cloud_scheduler_job.export.name == "scheduledFirestoreCollectionsExport" && google_cloud_scheduler_job.export.schedule == "0 0 * * *" && google_cloud_scheduler_job.export.time_zone == "Asia/Tokyo" && google_cloud_scheduler_job.export.paused == false
    error_message = "Preserve the observed enabled midnight JST job in its existing project/region."
  }
  assert {
    condition     = google_cloud_scheduler_job.export.pubsub_target[0].topic_name == var.topic_id && base64decode(google_cloud_scheduler_job.export.pubsub_target[0].data) == "start export" && length(google_cloud_scheduler_job.export.http_target) == 0 && length(google_cloud_scheduler_job.export.app_engine_http_target) == 0
    error_message = "Deliver the same Pub/Sub payload to the adopted topic only."
  }
  assert {
    condition     = google_cloud_scheduler_job.export.retry_config[0].retry_count == 3 && google_cloud_scheduler_job.export.retry_config[0].max_retry_duration == "0s" && google_cloud_scheduler_job.export.retry_config[0].min_backoff_duration == "5s" && google_cloud_scheduler_job.export.retry_config[0].max_backoff_duration == "3600s" && google_cloud_scheduler_job.export.retry_config[0].max_doublings == 5 && google_cloud_scheduler_job.export.description == null && google_cloud_scheduler_job.export.attempt_deadline == null
    error_message = "Preserve exact retry semantics and absent description/deadline."
  }
}
