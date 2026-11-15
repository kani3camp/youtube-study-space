# モニターのメール・パスワード認証だけを管理する。公開登録は許可する。
variable "monitor_email_password_enabled" {
  description = "既存Auth設定のメール・パスワード認証をTerraformで管理する。"
  type        = bool
  default     = false
}
module "monitor_auth" {
  count      = var.monitor_email_password_enabled ? 1 : 0
  source     = "../../modules/monitor-auth"
  project_id = var.project_id
}
# Authの実体・既存ユーザーを削除せず、旧モジュールから管理アドレスだけを移す。
moved {
  from = module.monitor_auth_registration[0].google_identity_platform_config.auth
  to   = module.monitor_auth[0].google_identity_platform_config.auth
}
import {
  for_each = var.monitor_email_password_enabled ? toset(["existing"]) : toset([])
  to       = module.monitor_auth[0].google_identity_platform_config.auth
  id       = "projects/test-youtube-study-space/config"
}
