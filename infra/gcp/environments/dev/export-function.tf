variable "manage_export_function" {
  description = "Default-off Gen1 definition; activate only after separate Function GET approval and protected full-root review."
  type        = bool
  default     = false

  validation {
    condition     = !var.manage_export_function || (var.project_id == "test-youtube-study-space" && var.manage_export_topic && var.manage_export_scheduler && try(length(trimspace(var.export_function_execution_service_account_email)) > 0, false))
    error_message = "Function adoption requires development topic/Scheduler ownership and the freshly verified execution identity."
  }
}

variable "export_function_execution_service_account_email" {
  description = "Freshly inventoried existing execution identity, supplied privately for read-only probes or separately approved adoption."
  type        = string
  default     = null
}

module "export_function" {
  count                           = var.manage_export_function ? 1 : 0
  source                          = "../../modules/firestore-export-function"
  project_id                      = var.project_id
  region                          = "asia-southeast2"
  topic_id                        = var.manage_export_topic ? module.export_topic[0].topic_id : ""
  execution_service_account_email = var.export_function_execution_service_account_email
}

import {
  for_each = var.manage_export_function ? toset(["existing"]) : toset([])
  to       = module.export_function[0].google_cloudfunctions_function.export
  id       = "projects/test-youtube-study-space/locations/asia-southeast2/functions/firestoreCollectionsExport"
}
