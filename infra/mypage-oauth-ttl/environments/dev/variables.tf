variable "project_id" {
  description = "Owner-approved project ID for this environment's existing database. No live default."
  type        = string
  nullable    = false
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project_id))
    error_message = "Supply an explicit valid project ID from the approved inventory."
  }
}

variable "database_id" {
  description = "Owner-approved existing Firestore database ID; explicitly supply (default) when appropriate."
  type        = string
  nullable    = false
  validation {
    condition     = var.database_id == "(default)" || can(regex("^[a-z][a-z0-9-]{2,61}[a-z0-9]$", var.database_id))
    error_message = "Supply an explicit valid existing database ID from the approved inventory."
  }
}
