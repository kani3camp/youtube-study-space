# Firebase Authの管理元。firebase.json / Firebase CLIではこの設定・関数を配備しない。
# 有効化前に既存Auth設定、Identity Platform、専用SAと権限を確認する。
variable "monitor_auth_registration" {
  description = "登録制限の明示的な有効化。nullは既存Auth設定も関数も変更しない。"
  type = object({
    runtime_service_account_email = string
    build_service_account_email   = string
    preserved_before_sign_in_uri  = optional(string, "")
  })
  default = null
  validation {
    condition     = var.monitor_auth_registration == null || var.project_id == "youtube-study-space"
    error_message = "認証用関数はこの環境のプロジェクトにだけ配備できます。"
  }
}

module "monitor_auth_registration" {
  count                         = var.monitor_auth_registration == null ? 0 : 1
  source                        = "../../modules/monitor-auth-registration"
  project_id                    = var.project_id
  region                        = "asia-northeast2"
  runtime_service_account_email = var.monitor_auth_registration.runtime_service_account_email
  build_service_account_email   = var.monitor_auth_registration.build_service_account_email
  preserved_before_sign_in_uri  = var.monitor_auth_registration.preserved_before_sign_in_uri
}

# 既存Auth設定を採用する。新しいFirebaseプロジェクトやAuth構成は作成しない。
import {
  for_each = var.monitor_auth_registration == null ? toset([]) : toset(["existing"])
  to       = module.monitor_auth_registration[0].google_identity_platform_config.auth
  id       = "projects/youtube-study-space/config"
}
