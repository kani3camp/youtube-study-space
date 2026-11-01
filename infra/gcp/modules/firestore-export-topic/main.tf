terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }
}

variable "project_id" { type = string }
variable "topic_name" { type = string }

# NOTE: Own only the user-managed topic. Gen1 delivery subscriptions and IAM
# remain externally owned. Empty labels preserve the unlabelled live topic.
resource "google_pubsub_topic" "export" {
  project = var.project_id
  name    = var.topic_name
  labels  = {}

  lifecycle {
    prevent_destroy = true
  }
}

output "topic_id" {
  value = google_pubsub_topic.export.id
}
