variable "order_history_field_order" {
  description = "Current development schema order, observed from table metadata."
  type        = list(string)
  default     = ["seat_id", "ordered_at", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"]
}

module "order_history" {
  source      = "../../modules/retained-order-history"
  project_id  = var.project_id
  dataset_id  = module.firestore_export_dataset.dataset_id
  field_order = var.order_history_field_order
}

import {
  to = module.order_history.google_bigquery_table.retained
  id = "projects/test-youtube-study-space/datasets/firestore_export/tables/order-history"
}
