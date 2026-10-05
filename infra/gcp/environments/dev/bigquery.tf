variable "firestore_export_location" {
  description = "Existing retained BigQuery dataset location; never migrate its location during import."
  type        = string
  default     = "asia-southeast2"
}

module "firestore_export_dataset" {
  source     = "../../modules/retained-bigquery-dataset"
  project_id = var.project_id
  dataset_id = "firestore_export"
  location   = var.firestore_export_location
}

import {
  to = module.firestore_export_dataset.google_bigquery_dataset.retained
  id = "projects/test-youtube-study-space/datasets/firestore_export"
}
