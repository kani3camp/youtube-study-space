mock_provider "google" {}
variables { project_id = "demo-monitor-auth" }
run "email_password_login_is_enabled_without_registration_guard" {
  command = plan
  assert {
    condition     = google_identity_platform_config.auth.sign_in[0].email[0].enabled && google_identity_platform_config.auth.sign_in[0].email[0].password_required && length(google_identity_platform_config.auth.blocking_functions) == 0
    error_message = "メール・パスワード認証を維持し、登録制限を再導入しないでください。"
  }
}
