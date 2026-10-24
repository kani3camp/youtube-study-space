terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
    }
  }
}

variable "project_id" {
  type = string
}

variable "database_id" {
  type = string
}

# expiresAt is already the absolute expiry (createdAt + 10 minutes).
# Omitting index_config preserves inherited indexes; an empty block would disable them.
resource "google_firestore_field" "oauth_transaction_ttl" {
  project         = var.project_id
  database        = var.database_id
  collection      = "oauth-transactions"
  field           = "expiresAt"
  deletion_policy = "PREVENT"
  skip_wait       = false

  ttl_config {}

  lifecycle {
    prevent_destroy = true
  }
}

output "policy" {
  value = {
    project         = google_firestore_field.oauth_transaction_ttl.project
    database        = google_firestore_field.oauth_transaction_ttl.database
    collection      = google_firestore_field.oauth_transaction_ttl.collection
    field           = google_firestore_field.oauth_transaction_ttl.field
    ttl_enabled     = length(google_firestore_field.oauth_transaction_ttl.ttl_config) == 1
    deletion_policy = google_firestore_field.oauth_transaction_ttl.deletion_policy
    skip_wait       = google_firestore_field.oauth_transaction_ttl.skip_wait
    index_config    = google_firestore_field.oauth_transaction_ttl.index_config
  }
}
