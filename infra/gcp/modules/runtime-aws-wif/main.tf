terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.5.0"
    }
  }
}

variable "project_id" { type = string }
variable "own_pool" {
  type    = bool
  default = false
}
variable "own_provider" {
  type    = bool
  default = false
  validation {
    condition     = !var.own_provider || var.own_pool
    error_message = "Provider adoption requires persistent pool ownership."
  }
}
variable "grant_keys" {
  description = "Reviewed public aliases only; never put an IAM member or AWS role in an address."
  type        = set(string)
  default     = []
  validation {
    condition = alltrue([for key in var.grant_keys : can(regex("^grant-[0-9]{2}$", key))]) && (
      length(var.grant_keys) == 0 || var.own_provider
    )
    error_message = "Exact grant aliases require persistent provider ownership."
  }
}
variable "inventory" {
  description = "Private fresh REST metadata; no live identity defaults or generated trust expressions."
  sensitive   = true
  default     = null
  type = object({
    project_number = string
    pool = object({
      display_name = string
      description  = string
      disabled     = bool
      mode         = optional(string)
    })
    provider = object({
      display_name        = string
      description         = string
      disabled            = bool
      aws_account_id      = string
      attribute_mapping   = map(string)
      attribute_condition = string
    })
    service_account_id = string
    grants = map(object({
      member = string
      condition = optional(object({
        title       = string
        description = string
        expression  = string
      }))
    }))
  })
  validation {
    condition = (!var.own_pool && !var.own_provider && length(var.grant_keys) == 0) || try(
      can(regex("^[0-9]+$", var.inventory.project_number)) &&
      !var.inventory.pool.disabled && (var.inventory.pool.mode == null || var.inventory.pool.mode == "" || var.inventory.pool.mode == "FEDERATION_ONLY") &&
      !var.inventory.provider.disabled &&
      can(regex("^[0-9]{12}$", var.inventory.provider.aws_account_id)) &&
      length(trimspace(var.inventory.provider.attribute_condition)) > 0 &&
      alltrue([for key in ["google.subject", "attribute.aws_role", "attribute.account"] : length(trimspace(var.inventory.provider.attribute_mapping[key])) > 0]) &&
      var.inventory.service_account_id == "projects/${var.project_id}/serviceAccounts/${var.project_id}@appspot.gserviceaccount.com" &&
      alltrue([for key in var.grant_keys : can(regex(
        "^principalSet://iam\\.googleapis\\.com/projects/${var.inventory.project_number}/locations/global/workloadIdentityPools/aws-runtime/attribute\\.aws_role/[A-Za-z0-9_+=,.@-]+$",
        var.inventory.grants[key].member
      ))]), false
    )
    error_message = "Adoption requires the existing active AWS federation and exact role-scoped grants on the runtime default SA."
  }
}

# NOTE: Preserve fresh mappings/condition byte-for-byte. Never reconstruct the
# working runtime trust from historical AWS roles, CDK audience, or test values.
resource "google_iam_workload_identity_pool" "runtime" {
  count                     = var.own_pool ? 1 : 0
  project                   = var.project_id
  workload_identity_pool_id = "aws-runtime"
  display_name              = var.inventory.pool.display_name
  description               = var.inventory.pool.description
  disabled                  = var.inventory.pool.disabled
  mode                      = var.inventory.pool.mode
  lifecycle { prevent_destroy = true }
}

resource "google_iam_workload_identity_pool_provider" "runtime" {
  count                              = var.own_provider ? 1 : 0
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.runtime[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "aws-provider"
  display_name                       = var.inventory.provider.display_name
  description                        = var.inventory.provider.description
  disabled                           = var.inventory.provider.disabled
  attribute_mapping                  = var.inventory.provider.attribute_mapping
  attribute_condition                = var.inventory.provider.attribute_condition
  aws { account_id = var.inventory.provider.aws_account_id }
  lifecycle { prevent_destroy = true }
}

# NOTE: Non-authoritative individual members only. SA identity, whole policies,
# role bindings, project IAM, service agents and keys remain externally owned.
resource "google_service_account_iam_member" "runtime" {
  for_each           = var.grant_keys
  service_account_id = var.inventory.service_account_id
  role               = "roles/iam.workloadIdentityUser"
  member             = var.inventory.grants[each.key].member
  dynamic "condition" {
    for_each = var.inventory.grants[each.key].condition == null ? [] : [var.inventory.grants[each.key].condition]
    content {
      title       = condition.value.title
      description = condition.value.description
      expression  = condition.value.expression
    }
  }
  depends_on = [google_iam_workload_identity_pool_provider.runtime]
  lifecycle { prevent_destroy = true }
}
