variable "backup_bucket_location" {
  type    = string
  default = "ASIA-SOUTHEAST2"
}
variable "backup_bucket_storage_class" {
  type    = string
  default = "STANDARD"
}
variable "backup_bucket_retention_policy" {
  type    = object({ seconds = number, locked = bool })
  default = null
}
variable "backup_bucket_soft_delete_seconds" {
  type    = number
  default = 604800
}

module "backup_bucket" {
  source              = "../../modules/retained-backup-bucket"
  project_id          = var.project_id
  bucket_name         = "firestore-backup-${var.project_id}"
  location            = var.backup_bucket_location
  storage_class       = var.backup_bucket_storage_class
  retention_policy    = var.backup_bucket_retention_policy
  soft_delete_seconds = var.backup_bucket_soft_delete_seconds
}

import {
  to = module.backup_bucket.google_storage_bucket.retained
  id = "firestore-backup-test-youtube-study-space"
}
