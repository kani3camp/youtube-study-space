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
  default     = "youtube-study-space"
}

variable "region" {
  description = "Default GCP region for regional resources."
  type        = string
  default     = "asia-northeast2"
}

output "environment" {
  value = "production"
}

output "project_id" {
  value = var.project_id
}
