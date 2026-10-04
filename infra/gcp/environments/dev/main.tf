terraform {
  required_version = "= 1.16.4"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }

  backend "gcs" {}
}

provider "google" {
  project = var.project_id
  region  = var.region
}

variable "project_id" {
  description = "GCP project managed by this Terraform root."
  type        = string
  default     = "test-youtube-study-space"
}

variable "region" {
  description = "Default GCP region for regional resources."
  type        = string
  default     = "asia-southeast2"
}

output "environment" {
  value = "development"
}

output "project_id" {
  value = var.project_id
}
