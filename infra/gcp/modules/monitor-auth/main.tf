variable "project_id" { type = string }

# メール・パスワード認証だけを管理する。登録制限は設けない。
resource "google_identity_platform_config" "auth" {
  project = var.project_id
  sign_in {
    email {
      enabled           = true
      password_required = true
    }
  }
  lifecycle {
    prevent_destroy = true
    # Auth全体の削除や、他の認証・トリガー設定の初期化を防ぐ。
    ignore_changes = [
      authorized_domains, autodelete_anonymous_users, client, mfa, monitoring,
      multi_tenant, quota, sms_region_config, blocking_functions,
      sign_in[0].allow_duplicate_emails, sign_in[0].anonymous, sign_in[0].phone_number,
    ]
  }
}
