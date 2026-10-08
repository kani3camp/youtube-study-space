variable "manage_user_activity_history" {
  description = "Default-off retained user-activity-history adoption; enable only after legacy schema repair and fresh protected import review."
  type        = bool
  default     = false

  validation {
    condition     = !var.manage_user_activity_history || var.project_id == "test-youtube-study-space"
    error_message = "user-activity-history adoption is development-only in this phase."
  }
}

variable "user_activity_history_field_order" {
  description = "Fresh canonical development field order after legacy timestamp removal. Required only when adoption is enabled."
  type        = list(string)
  default     = []
}

variable "user_activity_history_field_descriptions" {
  description = "Existing column descriptions keyed by canonical field path, supplied only through private metadata inputs."
  type        = map(string)
  sensitive   = true
  nullable    = false
  default     = {}
}

module "user_activity_history" {
  count              = var.manage_user_activity_history ? 1 : 0
  source             = "../../modules/retained-user-activity-history"
  project_id         = var.project_id
  dataset_id         = module.firestore_export_dataset.dataset_id
  field_order        = var.user_activity_history_field_order
  field_descriptions = var.user_activity_history_field_descriptions
}

import {
  for_each = var.manage_user_activity_history ? toset(["existing"]) : toset([])
  to       = module.user_activity_history[0].google_bigquery_table.retained
  id       = "projects/test-youtube-study-space/datasets/firestore_export/tables/user-activity-history"
}
