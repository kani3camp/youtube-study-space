terraform {
  required_version = "= 1.16.4"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }
  backend "s3" {}
}

provider "google" {
  project = var.project_id
}

module "oauth_transaction_ttl" {
  source      = "../../modules/oauth-transaction-ttl"
  project_id  = var.project_id
  database_id = var.database_id
}

output "scope" {
  value = {
    environment = "development"
    projectID   = var.project_id
    databaseID  = var.database_id
    collection  = "oauth-transactions"
    field       = "expiresAt"
  }
}
