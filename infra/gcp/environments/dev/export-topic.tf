variable "manage_export_topic" {
  description = "Enable only after strict full-root import-only and CI read-permission prerequisites pass."
  type        = bool
  default     = false

  validation {
    condition     = !var.manage_export_topic || var.project_id == "test-youtube-study-space"
    error_message = "This import wave is development-only."
  }
}

# #1173 natural E2E and the approved quota refresh/topic import completed.
# Credentialless roots default off; the protected development workflow owns it.
module "export_topic" {
  count      = var.manage_export_topic ? 1 : 0
  source     = "../../modules/firestore-export-topic"
  project_id = var.project_id
  topic_name = "initiateFirestoreCollectionsExport"
}

import {
  for_each = var.manage_export_topic ? toset(["existing"]) : toset([])
  to       = module.export_topic[0].google_pubsub_topic.export
  id       = "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"
}
