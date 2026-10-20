variable "own_runtime_wif_pool" {
  type    = bool
  default = false
  validation {
    condition     = !var.own_runtime_wif_pool || var.project_id == "test-youtube-study-space"
    error_message = "Runtime WIF adoption is development-only in this phase."
  }
}
variable "own_runtime_wif_provider" {
  type    = bool
  default = false
}
variable "runtime_wif_grant_keys" {
  type    = set(string)
  default = []
}
variable "runtime_wif_inventory" {
  description = "Private inventory supplied only after fresh metadata review; never commit real identities."
  type        = any
  sensitive   = true
  default     = null
}
variable "owned_api_keys" {
  type    = set(string)
  default = []
  validation {
    condition     = length(var.owned_api_keys) == 0 || var.project_id == "test-youtube-study-space"
    error_message = "API adoption is development-only in this phase."
  }
}
variable "api_classification" {
  type      = any
  sensitive = true
  default   = {}
}

module "runtime_wif" {
  source       = "../../modules/runtime-aws-wif"
  project_id   = var.project_id
  own_pool     = var.own_runtime_wif_pool
  own_provider = var.own_runtime_wif_provider
  grant_keys   = var.runtime_wif_grant_keys
  inventory    = var.runtime_wif_inventory
}
module "owned_apis" {
  source         = "../../modules/owned-project-apis"
  project_id     = var.project_id
  service_keys   = var.owned_api_keys
  classification = var.api_classification
}

import {
  for_each = var.own_runtime_wif_pool ? toset(["existing"]) : toset([])
  to       = module.runtime_wif.google_iam_workload_identity_pool.runtime[0]
  id       = "projects/${var.project_id}/locations/global/workloadIdentityPools/aws-runtime"
}
import {
  for_each = var.own_runtime_wif_provider ? toset(["existing"]) : toset([])
  to       = module.runtime_wif.google_iam_workload_identity_pool_provider.runtime[0]
  id       = "projects/${var.project_id}/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider"
}
import {
  for_each = var.runtime_wif_grant_keys
  to       = module.runtime_wif.google_service_account_iam_member.runtime[each.key]
  id = join(" ", compact([
    var.runtime_wif_inventory.service_account_id,
    "roles/iam.workloadIdentityUser",
    var.runtime_wif_inventory.grants[each.key].member,
    try(var.runtime_wif_inventory.grants[each.key].condition.title, ""),
  ]))
}
import {
  for_each = var.owned_api_keys
  to       = module.owned_apis.google_project_service.owned[each.key]
  id       = "${var.project_id}/${each.key}"
}
