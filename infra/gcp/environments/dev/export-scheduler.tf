variable "manage_export_scheduler" {
  description = "Remain disabled until exact Scheduler GET permission is separately approved and the protected full-root wave is reviewed."
  type        = bool
  default     = false

  validation {
    condition     = !var.manage_export_scheduler || (var.project_id == "test-youtube-study-space" && var.manage_export_topic)
    error_message = "Scheduler adoption requires the existing development topic ownership."
  }
}

module "export_scheduler" {
  count      = var.manage_export_scheduler ? 1 : 0
  source     = "../../modules/firestore-export-scheduler"
  project_id = var.project_id
  region     = "asia-southeast2"
  topic_id   = var.manage_export_topic ? module.export_topic[0].topic_id : ""
}

import {
  for_each = var.manage_export_scheduler ? toset(["existing"]) : toset([])
  to       = module.export_scheduler[0].google_cloud_scheduler_job.export
  id       = "projects/test-youtube-study-space/locations/asia-southeast2/jobs/scheduledFirestoreCollectionsExport"
}
