# Native Firestore backups are independent of the Scheduler/Pub/Sub export chain.
# Preserve the existing daily recurrence and development's 30-day retention.
resource "google_firestore_backup_schedule" "daily" {
  project   = var.project_id
  database  = "(default)"
  retention = "2592000s"

  daily_recurrence {}

  lifecycle {
    prevent_destroy = true
  }
}

import {
  to = google_firestore_backup_schedule.daily
  id = "projects/test-youtube-study-space/databases/(default)/backupSchedules/1813831c-2e48-4b69-b4ff-799e07909061"
}
